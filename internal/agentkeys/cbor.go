package agentkeys

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
)

type cborKind int

const (
	cborUint cborKind = iota
	cborBytes
	cborText
	cborBool
	cborNull
	cborArray
	cborMap
)

type cborValue struct {
	kind  cborKind
	u     uint64
	b     []byte
	s     string
	bool  bool
	array []cborValue
	pairs []cborPair
	raw   []byte
}

type cborPair struct {
	key   cborValue
	value cborValue
}

func decodeCBOR(data []byte) (cborValue, error) {
	p := cborParser{data: data}
	v, err := p.parse()
	if err != nil {
		return cborValue{}, err
	}
	if p.off != len(data) {
		return cborValue{}, fmt.Errorf("trailing CBOR bytes at offset %d", p.off)
	}
	return v, nil
}

type cborParser struct {
	data []byte
	off  int
}

func (p *cborParser) parse() (cborValue, error) {
	if p.off >= len(p.data) {
		return cborValue{}, fmt.Errorf("unexpected end of CBOR")
	}
	start := p.off
	head := p.data[p.off]
	p.off++
	major := head >> 5
	ai := head & 0x1f

	switch major {
	case 0:
		n, err := p.readArgument(ai)
		if err != nil {
			return cborValue{}, err
		}
		return cborValue{kind: cborUint, u: n, raw: p.data[start:p.off]}, nil
	case 2:
		n, err := p.readArgument(ai)
		if err != nil {
			return cborValue{}, err
		}
		if n > uint64(len(p.data)-p.off) {
			return cborValue{}, fmt.Errorf("byte string length %d exceeds remaining CBOR bytes", n)
		}
		b := append([]byte(nil), p.data[p.off:p.off+int(n)]...)
		p.off += int(n)
		return cborValue{kind: cborBytes, b: b, raw: p.data[start:p.off]}, nil
	case 3:
		n, err := p.readArgument(ai)
		if err != nil {
			return cborValue{}, err
		}
		if n > uint64(len(p.data)-p.off) {
			return cborValue{}, fmt.Errorf("text string length %d exceeds remaining CBOR bytes", n)
		}
		s := string(p.data[p.off : p.off+int(n)])
		p.off += int(n)
		return cborValue{kind: cborText, s: s, raw: p.data[start:p.off]}, nil
	case 4:
		n, err := p.readArgument(ai)
		if err != nil {
			return cborValue{}, err
		}
		items := make([]cborValue, 0, n)
		for i := uint64(0); i < n; i++ {
			item, err := p.parse()
			if err != nil {
				return cborValue{}, fmt.Errorf("array item %d: %w", i, err)
			}
			items = append(items, item)
		}
		return cborValue{kind: cborArray, array: items, raw: p.data[start:p.off]}, nil
	case 5:
		n, err := p.readArgument(ai)
		if err != nil {
			return cborValue{}, err
		}
		pairs := make([]cborPair, 0, n)
		var prev []byte
		seen := make(map[string]struct{}, n)
		for i := uint64(0); i < n; i++ {
			key, err := p.parse()
			if err != nil {
				return cborValue{}, fmt.Errorf("map key %d: %w", i, err)
			}
			if prev != nil && bytes.Compare(prev, key.raw) >= 0 {
				return cborValue{}, fmt.Errorf("CBOR map keys are not in canonical order")
			}
			prev = key.raw
			if _, ok := seen[string(key.raw)]; ok {
				return cborValue{}, fmt.Errorf("duplicate CBOR map key")
			}
			seen[string(key.raw)] = struct{}{}
			value, err := p.parse()
			if err != nil {
				return cborValue{}, fmt.Errorf("map value %d: %w", i, err)
			}
			pairs = append(pairs, cborPair{key: key, value: value})
		}
		return cborValue{kind: cborMap, pairs: pairs, raw: p.data[start:p.off]}, nil
	case 7:
		switch ai {
		case 20:
			return cborValue{kind: cborBool, bool: false, raw: p.data[start:p.off]}, nil
		case 21:
			return cborValue{kind: cborBool, bool: true, raw: p.data[start:p.off]}, nil
		case 22:
			return cborValue{kind: cborNull, raw: p.data[start:p.off]}, nil
		default:
			return cborValue{}, fmt.Errorf("unsupported CBOR simple or float item 0x%x", head)
		}
	default:
		return cborValue{}, fmt.Errorf("unsupported CBOR major type %d", major)
	}
}

func (p *cborParser) readArgument(ai byte) (uint64, error) {
	switch ai {
	case 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23:
		return uint64(ai), nil
	case 24:
		n, err := p.readN(1)
		if err != nil {
			return 0, err
		}
		if n < 24 {
			return 0, fmt.Errorf("non-shortest CBOR integer or length")
		}
		return n, nil
	case 25:
		n, err := p.readN(2)
		if err != nil {
			return 0, err
		}
		if n <= 0xff {
			return 0, fmt.Errorf("non-shortest CBOR integer or length")
		}
		return n, nil
	case 26:
		n, err := p.readN(4)
		if err != nil {
			return 0, err
		}
		if n <= 0xffff {
			return 0, fmt.Errorf("non-shortest CBOR integer or length")
		}
		return n, nil
	case 27:
		n, err := p.readN(8)
		if err != nil {
			return 0, err
		}
		if n <= 0xffffffff {
			return 0, fmt.Errorf("non-shortest CBOR integer or length")
		}
		return n, nil
	default:
		return 0, fmt.Errorf("indefinite-length CBOR item is not canonical")
	}
}

func (p *cborParser) readN(n int) (uint64, error) {
	if len(p.data)-p.off < n {
		return 0, fmt.Errorf("unexpected end of CBOR")
	}
	var out uint64
	for i := 0; i < n; i++ {
		out = (out << 8) | uint64(p.data[p.off+i])
	}
	p.off += n
	return out, nil
}

func (v cborValue) textMap() (map[string]cborValue, error) {
	if v.kind != cborMap {
		return nil, fmt.Errorf("expected CBOR map")
	}
	out := make(map[string]cborValue, len(v.pairs))
	for _, pair := range v.pairs {
		if pair.key.kind != cborText {
			return nil, fmt.Errorf("expected text map key")
		}
		out[pair.key.s] = pair.value
	}
	return out, nil
}

func encodeCanonical(v interface{}) ([]byte, error) {
	var out []byte
	if err := appendCanonical(&out, v); err != nil {
		return nil, err
	}
	return out, nil
}

func appendCanonical(out *[]byte, v interface{}) error {
	switch x := v.(type) {
	case nil:
		*out = append(*out, 0xf6)
	case bool:
		if x {
			*out = append(*out, 0xf5)
		} else {
			*out = append(*out, 0xf4)
		}
	case uint8:
		appendMajor(out, 0, uint64(x))
	case uint64:
		appendMajor(out, 0, x)
	case uint:
		appendMajor(out, 0, uint64(x))
	case int:
		if x < 0 {
			return fmt.Errorf("negative CBOR integers are not supported")
		}
		appendMajor(out, 0, uint64(x))
	case int64:
		if x < 0 {
			return fmt.Errorf("negative CBOR integers are not supported")
		}
		appendMajor(out, 0, uint64(x))
	case json.Number:
		n, err := x.Int64()
		if err != nil || n < 0 {
			return fmt.Errorf("invalid unsigned JSON number %q", x)
		}
		appendMajor(out, 0, uint64(n))
	case string:
		appendMajor(out, 3, uint64(len(x)))
		*out = append(*out, x...)
	case []byte:
		appendMajor(out, 2, uint64(len(x)))
		*out = append(*out, x...)
	case map[string]interface{}:
		return appendCanonicalMap(out, x)
	case map[string]string:
		m := make(map[string]interface{}, len(x))
		for k, v := range x {
			m[k] = v
		}
		return appendCanonicalMap(out, m)
	case []interface{}:
		appendMajor(out, 4, uint64(len(x)))
		for i := range x {
			if err := appendCanonical(out, x[i]); err != nil {
				return fmt.Errorf("encode array item %d: %w", i, err)
			}
		}
	case []string:
		appendMajor(out, 4, uint64(len(x)))
		for i := range x {
			if err := appendCanonical(out, x[i]); err != nil {
				return fmt.Errorf("encode array item %d: %w", i, err)
			}
		}
	case cborValue:
		*out = append(*out, x.raw...)
	default:
		return fmt.Errorf("unsupported CBOR encode type %T", v)
	}
	return nil
}

func appendCanonicalMap(out *[]byte, m map[string]interface{}) error {
	type encodedPair struct {
		key   string
		kcbor []byte
	}
	pairs := make([]encodedPair, 0, len(m))
	for k := range m {
		kcbor, err := encodeCanonical(k)
		if err != nil {
			return err
		}
		pairs = append(pairs, encodedPair{key: k, kcbor: kcbor})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return bytes.Compare(pairs[i].kcbor, pairs[j].kcbor) < 0
	})
	appendMajor(out, 5, uint64(len(pairs)))
	for _, pair := range pairs {
		*out = append(*out, pair.kcbor...)
		if err := appendCanonical(out, m[pair.key]); err != nil {
			return fmt.Errorf("encode map value %q: %w", pair.key, err)
		}
	}
	return nil
}

func appendMajor(out *[]byte, major byte, n uint64) {
	head := major << 5
	switch {
	case n < 24:
		*out = append(*out, head|byte(n))
	case n <= 0xff:
		*out = append(*out, head|24, byte(n))
	case n <= 0xffff:
		*out = append(*out, head|25, byte(n>>8), byte(n))
	case n <= 0xffffffff:
		*out = append(*out, head|26, byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	default:
		*out = append(*out, head|27, byte(n>>56), byte(n>>48), byte(n>>40), byte(n>>32), byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
	}
}
