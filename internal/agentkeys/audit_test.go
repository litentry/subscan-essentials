package agentkeys

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDecodeEnvelopeSignEip712(t *testing.T) {
	actor := bytesOf(0x11)
	operator := bytesOf(0x22)
	commitment := bytesOf(0x33)
	intent := "Approve USDC 1000 to Uniswap v4 router"

	body := map[string]interface{}{
		"chain_id":           uint64(212013),
		"verifying_contract": "0x1111111111111111111111111111111111111111",
		"primary_type":       "Permit",
		"type_hash":          "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"domain_separator":   "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"digest":             "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	envelope := map[string]interface{}{
		"version":           uint8(1),
		"ts_unix":           uint64(1710000000),
		"actor_omni":        actor,
		"operator_omni":     operator,
		"op_kind":           uint8(21),
		"op_body":           body,
		"result":            uint8(0),
		"intent_text":       intent,
		"intent_commitment": commitment,
	}

	cborBytes, err := EncodeCanonicalEnvelope(envelope)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(hexOf(cborBytes), "a966726573756c74"), "top-level keys must start with canonical shortest key: result")

	hash := crypto.Keccak256Hash(cborBytes).Hex()
	decoded, err := DecodeEnvelope(cborBytes, hash)
	require.NoError(t, err)

	assert.Equal(t, uint8(21), decoded.OpKind)
	assert.Equal(t, "SignEip712", decoded.OpKindName)
	assert.Equal(t, "Success", decoded.ResultName)
	assert.True(t, decoded.HashVerified)
	assert.Equal(t, "0x"+strings.Repeat("11", 32), decoded.ActorOmni)
	require.NotNil(t, decoded.IntentText)
	assert.Equal(t, intent, *decoded.IntentText)

	rendered, ok := decoded.Body.(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, uint64(212013), rendered["chain_id"])
	assert.Equal(t, "Permit", rendered["primary_type"])
	assert.Equal(t, body["digest"], rendered["digest"])
}

func TestCriticalOpKindRenderers(t *testing.T) {
	tests := []struct {
		name      string
		opKind    uint8
		opName    string
		body      map[string]interface{}
		assertion func(t *testing.T, rendered map[string]interface{})
	}{
		{
			name:   "ScopeGrant",
			opKind: 40,
			opName: "ScopeGrant",
			body: map[string]interface{}{
				"agent_omni": "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"service":    "credential-vault",
				"max_calls":  uint64(5),
				"max_amount": "1000000000000000000",
			},
			assertion: func(t *testing.T, rendered map[string]interface{}) {
				assert.Equal(t, uint64(5), rendered["max_calls"])
				assert.Equal(t, "1000000000000000000", rendered["max_amount"])
			},
		},
		{
			name:   "DeviceAdd",
			opKind: 50,
			opName: "DeviceAdd",
			body: map[string]interface{}{
				"device_key_hash":  "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"role_bits":        uint64(7),
				"attestation_hash": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			},
			assertion: func(t *testing.T, rendered map[string]interface{}) {
				assert.Equal(t, uint64(7), rendered["role_bits"])
				assert.Equal(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", rendered["device_key_hash"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cborBytes, err := EncodeCanonicalEnvelope(map[string]interface{}{
				"version":           uint8(1),
				"ts_unix":           uint64(1710000003),
				"actor_omni":        bytesOf(0x12),
				"operator_omni":     bytesOf(0x34),
				"op_kind":           tt.opKind,
				"op_body":           tt.body,
				"result":            uint8(0),
				"intent_text":       nil,
				"intent_commitment": nil,
			})
			require.NoError(t, err)

			decoded, err := DecodeEnvelope(cborBytes, crypto.Keccak256Hash(cborBytes).Hex())
			require.NoError(t, err)
			assert.Equal(t, tt.opName, decoded.OpKindName)

			rendered, ok := decoded.Body.(map[string]interface{})
			require.True(t, ok)
			tt.assertion(t, rendered)
		})
	}
}

func TestUnknownOpKindNonBreakFallback(t *testing.T) {
	for _, opKind := range []uint8{250, 255} {
		opBody := map[string]interface{}{"future_field": "opaque"}
		opBodyBytes, err := encodeCanonical(opBody)
		require.NoError(t, err)
		envelope := map[string]interface{}{
			"version":           uint8(1),
			"ts_unix":           uint64(1710000001),
			"actor_omni":        bytesOf(0x44),
			"operator_omni":     bytesOf(0x55),
			"op_kind":           opKind,
			"op_body":           opBody,
			"result":            uint8(2),
			"intent_text":       nil,
			"intent_commitment": nil,
		}
		cborBytes, err := EncodeCanonicalEnvelope(envelope)
		require.NoError(t, err)

		decoded, err := DecodeEnvelope(cborBytes, "")
		require.NoError(t, err)

		assert.Equal(t, fmt.Sprintf("Unknown(%d)", opKind), decoded.OpKindName)
		assert.Equal(t, "NotPermitted", decoded.ResultName)
		fallback, ok := decoded.Body.(UnknownOpBody)
		require.True(t, ok)
		assert.Equal(t, opKind, fallback.OpKindByte)
		assert.Equal(t, base64.StdEncoding.EncodeToString(opBodyBytes), fallback.OpBodyB64)
	}
}

func TestDecodeEnvelopeRejectsVersionAndNonCanonicalMap(t *testing.T) {
	envelope := map[string]interface{}{
		"version":           uint8(2),
		"ts_unix":           uint64(1),
		"actor_omni":        bytesOf(0x01),
		"operator_omni":     bytesOf(0x02),
		"op_kind":           uint8(21),
		"op_body":           map[string]interface{}{"chain_id": uint64(1), "verifying_contract": "0x0", "primary_type": "Permit", "type_hash": "0x1", "domain_separator": "0x2", "digest": "0x3"},
		"result":            uint8(0),
		"intent_text":       nil,
		"intent_commitment": nil,
	}
	cborBytes, err := EncodeCanonicalEnvelope(envelope)
	require.NoError(t, err)
	_, err = DecodeEnvelope(cborBytes, "")
	require.ErrorContains(t, err, "unsupported AuditEnvelope version 2")

	_, err = decodeCBOR([]byte{0xa2, 0x61, 0x62, 0x01, 0x61, 0x61, 0x02})
	require.ErrorContains(t, err, "canonical order")
}

func TestKnownOpKindTableMatchesCanonicalIssueTable(t *testing.T) {
	expected := map[uint8]string{
		0: "CredStore", 1: "CredFetch", 2: "CredTeardown",
		10: "MemoryPut", 11: "MemoryGet", 12: "MemoryTeardown",
		20: "SignEip191", 21: "SignEip712",
		30: "PaymentEscrowRedeem", 31: "PaymentDirect",
		40: "ScopeGrant", 41: "ScopeRevoke",
		50: "DeviceAdd", 51: "DeviceRevoke", 52: "K10Rotate",
		60: "EmailSend", 61: "EmailReceive",
		70: "K3EpochAdvance",
	}
	assert.Equal(t, expected, KnownOpKinds())
}

func TestDecodeAuditEventLogs(t *testing.T) {
	operator := "0x" + strings.Repeat("aa", 32)
	actor := "0x" + strings.Repeat("bb", 32)
	envelopeHash := "0x" + strings.Repeat("cc", 32)
	opKindTopic := "0x" + strings.Repeat("0", 62) + "15"

	appended, err := DecodeAuditAppendedV2Log([]string{AuditAppendedV2Topic, operator, actor, opKindTopic}, envelopeHash)
	require.NoError(t, err)
	assert.Equal(t, operator, appended.OperatorOmni)
	assert.Equal(t, actor, appended.ActorOmni)
	assert.Equal(t, uint8(21), appended.OpKind)
	assert.Equal(t, envelopeHash, appended.EnvelopeHash)

	rootData := "0x" + strings.Repeat("dd", 32) + strings.Repeat("0", 63) + "7"
	root, err := DecodeAuditRootAppendedV2Log([]string{AuditRootAppendedV2Topic, operator, envelopeHash}, rootData)
	require.NoError(t, err)
	assert.Equal(t, "0x"+strings.Repeat("dd", 32), root.OpKindBitmapU256)
	assert.Equal(t, uint64(7), root.EntryCount)
}

func TestFetchEnvelopeAndDecodeAcceptsHashWithoutPrefix(t *testing.T) {
	cborBytes, err := EncodeCanonicalEnvelope(map[string]interface{}{
		"version":           uint8(1),
		"ts_unix":           uint64(1710000002),
		"actor_omni":        bytesOf(0x66),
		"operator_omni":     bytesOf(0x77),
		"op_kind":           uint8(50),
		"op_body":           map[string]interface{}{"device_key_hash": "0xabc", "role_bits": uint64(7), "attestation_hash": "0xdef"},
		"result":            uint8(1),
		"intent_text":       nil,
		"intent_commitment": nil,
	})
	require.NoError(t, err)
	hash := crypto.Keccak256Hash(cborBytes).Hex()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/audit/envelope/"+strings.TrimPrefix(hash, "0x"), r.URL.Path)
		w.Header().Set("Content-Type", "application/cbor")
		_, _ = w.Write(cborBytes)
	}))
	defer srv.Close()

	fetched, err := FetchEnvelope(context.Background(), srv.URL, strings.TrimPrefix(hash, "0x"))
	require.NoError(t, err)
	decoded, err := DecodeEnvelope(fetched, strings.TrimPrefix(hash, "0x"))
	require.NoError(t, err)
	assert.Equal(t, "DeviceAdd", decoded.OpKindName)
	assert.Equal(t, "Failure", decoded.ResultName)
}

func TestEnvelopeCacheFetchesByImmutableHashOnce(t *testing.T) {
	cborBytes, err := EncodeCanonicalEnvelope(map[string]interface{}{
		"version":           uint8(1),
		"ts_unix":           uint64(1710000004),
		"actor_omni":        bytesOf(0x88),
		"operator_omni":     bytesOf(0x99),
		"op_kind":           uint8(40),
		"op_body":           map[string]interface{}{"agent_omni": "0xabc", "service": "vault", "max_calls": uint64(1), "max_amount": "0"},
		"result":            uint8(0),
		"intent_text":       nil,
		"intent_commitment": nil,
	})
	require.NoError(t, err)
	hash := crypto.Keccak256Hash(cborBytes).Hex()

	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/cbor")
		_, _ = w.Write(cborBytes)
	}))
	defer srv.Close()

	cache := NewEnvelopeCache()
	for i := 0; i < 2; i++ {
		body, decoded, err := cache.FetchAndDecode(context.Background(), srv.URL, hash)
		require.NoError(t, err)
		assert.Equal(t, cborBytes, body)
		assert.Equal(t, "ScopeGrant", decoded.OpKindName)
	}
	assert.Equal(t, 1, requests)
}

func bytesOf(b byte) []byte {
	out := make([]byte, 32)
	for i := range out {
		out[i] = b
	}
	return out
}

func hexOf(b []byte) string {
	const digits = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = digits[v>>4]
		out[i*2+1] = digits[v&0x0f]
	}
	return string(out)
}
