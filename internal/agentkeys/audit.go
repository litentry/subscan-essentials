package agentkeys

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	EnvelopeVersion = 1

	AuditAppendedV2Signature     = "AuditAppendedV2(bytes32,bytes32,uint8,bytes32)"
	AuditRootAppendedV2Signature = "AuditRootAppendedV2(bytes32,bytes32,bytes32,uint64)"
)

var (
	AuditAppendedV2Topic     = crypto.Keccak256Hash([]byte(AuditAppendedV2Signature)).Hex()
	AuditRootAppendedV2Topic = crypto.Keccak256Hash([]byte(AuditRootAppendedV2Signature)).Hex()
	ErrEnvelopeNotFound      = errors.New("agentkeys audit envelope not found")
)

type Envelope struct {
	Version           uint8       `json:"version"`
	TsUnix            uint64      `json:"ts_unix"`
	ActorOmni         string      `json:"actor_omni"`
	OperatorOmni      string      `json:"operator_omni"`
	OpKind            uint8       `json:"op_kind"`
	OpKindName        string      `json:"op_kind_name"`
	Result            uint8       `json:"result"`
	ResultName        string      `json:"result_name"`
	IntentText        *string     `json:"intent_text"`
	IntentCommitment  *string     `json:"intent_commitment"`
	EnvelopeHash      string      `json:"envelope_hash"`
	HashVerified      bool        `json:"hash_verified"`
	Body              interface{} `json:"body"`
	OpaqueOpBodyCBOR  string      `json:"opaque_op_body_cbor,omitempty"`
	canonicalCBORBody []byte
}

type UnknownOpBody struct {
	OpKindByte uint8  `json:"op_kind_byte"`
	OpBodyB64  string `json:"op_body_b64"`
}

type AuditAppendedV2Event struct {
	OperatorOmni string `json:"operator_omni"`
	ActorOmni    string `json:"actor_omni"`
	OpKind       uint8  `json:"op_kind"`
	EnvelopeHash string `json:"envelope_hash"`
}

type AuditRootAppendedV2Event struct {
	OperatorOmni     string `json:"operator_omni"`
	MerkleRoot       string `json:"merkle_root"`
	OpKindBitmapU256 string `json:"op_kind_bitmap_u256"`
	EntryCount       uint64 `json:"entry_count"`
}

type EnvelopeCache struct {
	mu     sync.RWMutex
	bodies map[string][]byte
}

type opFieldType string

const (
	opText opFieldType = "text"
	opUint opFieldType = "uint"
)

type opKindSpec struct {
	Name   string
	Family string
	Fields []opFieldSpec
}

type opFieldSpec struct {
	Name string
	Type opFieldType
}

var opKindSpecs = map[uint8]opKindSpec{
	0:  {"CredStore", "creds", fields(text("service"), text("payload_hash"))},
	1:  {"CredFetch", "creds", fields(text("service"), text("cap_hash"))},
	2:  {"CredTeardown", "creds", fields(text("actor_target"))},
	10: {"MemoryPut", "memory", fields(text("key"), text("payload_hash"))},
	11: {"MemoryGet", "memory", fields(text("key"), text("cap_hash"))},
	12: {"MemoryTeardown", "memory", fields(text("actor_target"))},
	20: {"SignEip191", "signs", fields(text("message_digest"), text("wallet"))},
	21: {"SignEip712", "signs", fields(uintf("chain_id"), text("verifying_contract"), text("primary_type"), text("type_hash"), text("domain_separator"), text("digest"))},
	30: {"PaymentEscrowRedeem", "payments", fields(text("escrow_addr"), text("amount"), text("recipient"), uintf("chain_id"))},
	31: {"PaymentDirect", "payments", fields(text("rail"), text("ref"), text("amount_minor"), text("currency"))},
	40: {"ScopeGrant", "scope", fields(text("agent_omni"), text("service"), uintf("max_calls"), text("max_amount"))},
	41: {"ScopeRevoke", "scope", fields(text("agent_omni"), text("service"))},
	50: {"DeviceAdd", "device", fields(text("device_key_hash"), uintf("role_bits"), text("attestation_hash"))},
	51: {"DeviceRevoke", "device", fields(text("device_key_hash"))},
	52: {"K10Rotate", "device", fields(text("old_device_key_hash"), text("new_device_key_hash"))},
	60: {"EmailSend", "email", fields(text("to_hash"), text("subject_hash"), text("message_id"))},
	61: {"EmailReceive", "email", fields(text("from_hash"), text("message_id"), text("payload_hash"))},
	70: {"K3EpochAdvance", "K3", fields(uintf("old_epoch"), uintf("new_epoch"), text("gov_tx"))},
}

func fields(f ...opFieldSpec) []opFieldSpec { return f }
func text(name string) opFieldSpec          { return opFieldSpec{Name: name, Type: opText} }
func uintf(name string) opFieldSpec         { return opFieldSpec{Name: name, Type: opUint} }

func OpKindName(opKind uint8) string {
	if spec, ok := opKindSpecs[opKind]; ok {
		return spec.Name
	}
	return fmt.Sprintf("Unknown(%d)", opKind)
}

func DecodeEnvelope(data []byte, expectedHash string) (*Envelope, error) {
	v, err := decodeCBOR(data)
	if err != nil {
		return nil, err
	}
	top, err := v.textMap()
	if err != nil {
		return nil, err
	}
	for _, key := range []string{"version", "ts_unix", "actor_omni", "operator_omni", "op_kind", "op_body", "result", "intent_text", "intent_commitment"} {
		if _, ok := top[key]; !ok {
			return nil, fmt.Errorf("missing envelope field %q", key)
		}
	}

	version, err := requireUint8(top["version"], "version")
	if err != nil {
		return nil, err
	}
	if version != EnvelopeVersion {
		return nil, fmt.Errorf("unsupported AuditEnvelope version %d", version)
	}
	opKind, err := requireUint8(top["op_kind"], "op_kind")
	if err != nil {
		return nil, err
	}
	result, err := requireUint8(top["result"], "result")
	if err != nil {
		return nil, err
	}
	tsUnix, err := requireUint64(top["ts_unix"], "ts_unix")
	if err != nil {
		return nil, err
	}
	actor, err := requireBytesHex(top["actor_omni"], "actor_omni", 32)
	if err != nil {
		return nil, err
	}
	operator, err := requireBytesHex(top["operator_omni"], "operator_omni", 32)
	if err != nil {
		return nil, err
	}
	intentCommitment, err := optionalBytesHex(top["intent_commitment"], "intent_commitment", 32)
	if err != nil {
		return nil, err
	}
	intentText, err := optionalText(top["intent_text"], "intent_text")
	if err != nil {
		return nil, err
	}
	body, err := RenderOpBody(opKind, top["op_body"])
	if err != nil {
		return nil, err
	}

	actualHash := crypto.Keccak256Hash(data).Hex()
	expectedHash = normalizeHexHash(expectedHash)
	hashVerified := expectedHash == "" || strings.EqualFold(actualHash, expectedHash)
	if expectedHash != "" && !hashVerified {
		return nil, fmt.Errorf("envelope hash mismatch: expected %s got %s", expectedHash, actualHash)
	}

	return &Envelope{
		Version:           version,
		TsUnix:            tsUnix,
		ActorOmni:         actor,
		OperatorOmni:      operator,
		OpKind:            opKind,
		OpKindName:        OpKindName(opKind),
		Result:            result,
		ResultName:        resultName(result),
		IntentText:        intentText,
		IntentCommitment:  intentCommitment,
		EnvelopeHash:      actualHash,
		HashVerified:      hashVerified,
		Body:              body,
		OpaqueOpBodyCBOR:  base64.StdEncoding.EncodeToString(top["op_body"].raw),
		canonicalCBORBody: append([]byte(nil), data...),
	}, nil
}

func RenderOpBody(opKind uint8, opBody cborValue) (interface{}, error) {
	spec, ok := opKindSpecs[opKind]
	if !ok {
		return UnknownOpBody{
			OpKindByte: opKind,
			OpBodyB64:  base64.StdEncoding.EncodeToString(opBody.raw),
		}, nil
	}
	m, err := opBody.textMap()
	if err != nil {
		return nil, fmt.Errorf("%s op_body: %w", spec.Name, err)
	}
	if len(m) != len(spec.Fields) {
		return nil, fmt.Errorf("%s op_body field count mismatch", spec.Name)
	}
	out := make(map[string]interface{}, len(spec.Fields))
	for _, field := range spec.Fields {
		value, ok := m[field.Name]
		if !ok {
			return nil, fmt.Errorf("%s op_body missing field %q", spec.Name, field.Name)
		}
		switch field.Type {
		case opText:
			if value.kind != cborText {
				return nil, fmt.Errorf("%s op_body field %q must be text", spec.Name, field.Name)
			}
			out[field.Name] = value.s
		case opUint:
			if value.kind != cborUint {
				return nil, fmt.Errorf("%s op_body field %q must be uint", spec.Name, field.Name)
			}
			out[field.Name] = value.u
		}
	}
	return out, nil
}

func KnownOpKinds() map[uint8]string {
	out := make(map[uint8]string, len(opKindSpecs))
	for opKind, spec := range opKindSpecs {
		out[opKind] = spec.Name
	}
	return out
}

func NewEnvelopeCache() *EnvelopeCache {
	return &EnvelopeCache{bodies: make(map[string][]byte)}
}

func (c *EnvelopeCache) FetchAndDecode(ctx context.Context, workerBaseURL string, hash string) ([]byte, *Envelope, error) {
	if c == nil {
		body, err := FetchEnvelope(ctx, workerBaseURL, hash)
		if err != nil {
			return nil, nil, err
		}
		envelope, err := DecodeEnvelope(body, hash)
		return body, envelope, err
	}

	key := normalizeHexHash(hash)
	c.mu.RLock()
	if cached, ok := c.bodies[key]; ok {
		body := append([]byte(nil), cached...)
		c.mu.RUnlock()
		envelope, err := DecodeEnvelope(body, key)
		return body, envelope, err
	}
	c.mu.RUnlock()

	body, err := FetchEnvelope(ctx, workerBaseURL, key)
	if err != nil {
		return nil, nil, err
	}
	envelope, err := DecodeEnvelope(body, key)
	if err != nil {
		return nil, nil, err
	}

	c.mu.Lock()
	if cached, ok := c.bodies[key]; ok {
		body = append([]byte(nil), cached...)
		c.mu.Unlock()
		envelope, err = DecodeEnvelope(body, key)
		return body, envelope, err
	}
	c.bodies[key] = append([]byte(nil), body...)
	c.mu.Unlock()

	return append([]byte(nil), body...), envelope, nil
}

func EncodeCanonicalEnvelope(envelope map[string]interface{}) ([]byte, error) {
	return encodeCanonical(envelope)
}

func DecodeAuditAppendedV2Log(topics []string, data string) (*AuditAppendedV2Event, error) {
	if len(topics) != 4 {
		return nil, fmt.Errorf("AuditAppendedV2 requires 4 topics")
	}
	if !strings.EqualFold(topics[0], AuditAppendedV2Topic) {
		return nil, fmt.Errorf("unexpected AuditAppendedV2 topic0 %s", topics[0])
	}
	opKind, err := topicUint8(topics[3])
	if err != nil {
		return nil, err
	}
	hash, err := abiBytes32(data, 0)
	if err != nil {
		return nil, fmt.Errorf("envelope_hash: %w", err)
	}
	return &AuditAppendedV2Event{
		OperatorOmni: normalizeBytes32Topic(topics[1]),
		ActorOmni:    normalizeBytes32Topic(topics[2]),
		OpKind:       opKind,
		EnvelopeHash: hash,
	}, nil
}

func DecodeAuditRootAppendedV2Log(topics []string, data string) (*AuditRootAppendedV2Event, error) {
	if len(topics) != 3 {
		return nil, fmt.Errorf("AuditRootAppendedV2 requires 3 topics")
	}
	if !strings.EqualFold(topics[0], AuditRootAppendedV2Topic) {
		return nil, fmt.Errorf("unexpected AuditRootAppendedV2 topic0 %s", topics[0])
	}
	bitmap, err := abiBytes32(data, 0)
	if err != nil {
		return nil, fmt.Errorf("op_kind_bitmap: %w", err)
	}
	countHex, err := abiWord(data, 1)
	if err != nil {
		return nil, fmt.Errorf("entry_count: %w", err)
	}
	count, err := strconv.ParseUint(countHex[48:], 16, 64)
	if err != nil {
		return nil, fmt.Errorf("entry_count: %w", err)
	}
	return &AuditRootAppendedV2Event{
		OperatorOmni:     normalizeBytes32Topic(topics[1]),
		MerkleRoot:       normalizeBytes32Topic(topics[2]),
		OpKindBitmapU256: bitmap,
		EntryCount:       count,
	}, nil
}

func FetchEnvelope(ctx context.Context, workerBaseURL string, hash string) ([]byte, error) {
	base, err := url.Parse(workerBaseURL)
	if err != nil {
		return nil, err
	}
	base.Path = strings.TrimRight(base.Path, "/") + "/v1/audit/envelope/" + strings.TrimPrefix(strings.ToLower(hash), "0x")

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base.String(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint: errcheck
	if resp.StatusCode == http.StatusNotFound {
		return nil, ErrEnvelopeNotFound
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("audit worker returned HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func requireUint8(v cborValue, name string) (uint8, error) {
	if v.kind != cborUint {
		return 0, fmt.Errorf("%s must be uint", name)
	}
	if v.u > 255 {
		return 0, fmt.Errorf("%s must fit uint8", name)
	}
	return uint8(v.u), nil
}

func requireUint64(v cborValue, name string) (uint64, error) {
	if v.kind != cborUint {
		return 0, fmt.Errorf("%s must be uint", name)
	}
	return v.u, nil
}

func requireBytesHex(v cborValue, name string, size int) (string, error) {
	if v.kind != cborBytes {
		return "", fmt.Errorf("%s must be bytes", name)
	}
	if len(v.b) != size {
		return "", fmt.Errorf("%s must be %d bytes", name, size)
	}
	return "0x" + hex.EncodeToString(v.b), nil
}

func optionalBytesHex(v cborValue, name string, size int) (*string, error) {
	if v.kind == cborNull {
		return nil, nil
	}
	s, err := requireBytesHex(v, name, size)
	if err != nil {
		return nil, err
	}
	return &s, nil
}

func optionalText(v cborValue, name string) (*string, error) {
	if v.kind == cborNull {
		return nil, nil
	}
	if v.kind != cborText {
		return nil, fmt.Errorf("%s must be text or null", name)
	}
	return &v.s, nil
}

func resultName(result uint8) string {
	switch result {
	case 0:
		return "Success"
	case 1:
		return "Failure"
	case 2:
		return "NotPermitted"
	default:
		return fmt.Sprintf("Reserved(%d)", result)
	}
}

func topicUint8(topic string) (uint8, error) {
	word := strings.TrimPrefix(strings.ToLower(topic), "0x")
	if len(word) != 64 {
		return 0, fmt.Errorf("topic is not bytes32")
	}
	prefix := strings.TrimLeft(word[:62], "0")
	if prefix != "" {
		return 0, fmt.Errorf("topic does not fit uint8")
	}
	n, err := strconv.ParseUint(word[62:], 16, 8)
	return uint8(n), err
}

func normalizeBytes32Topic(topic string) string {
	return "0x" + strings.TrimPrefix(strings.ToLower(topic), "0x")
}

func normalizeHexHash(hash string) string {
	hash = strings.TrimSpace(strings.ToLower(hash))
	if hash == "" {
		return ""
	}
	return "0x" + strings.TrimPrefix(hash, "0x")
}

func abiBytes32(data string, wordOffset int) (string, error) {
	word, err := abiWord(data, wordOffset)
	if err != nil {
		return "", err
	}
	return "0x" + word, nil
}

func abiWord(data string, wordOffset int) (string, error) {
	clean := strings.TrimPrefix(strings.ToLower(data), "0x")
	start := wordOffset * 64
	end := start + 64
	if len(clean) < end {
		return "", fmt.Errorf("ABI data shorter than word %d", wordOffset)
	}
	return clean[start:end], nil
}
