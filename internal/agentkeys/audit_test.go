package agentkeys

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
		{
			name:   "PaymentDirect",
			opKind: 31,
			opName: "PaymentDirect",
			body: map[string]interface{}{
				"rail":         "stripe",
				"ref":          "invoice-123",
				"amount_minor": "12345",
				"currency":     "USD",
			},
			assertion: func(t *testing.T, rendered map[string]interface{}) {
				assert.Equal(t, "12345", rendered["amount_minor"])
				assert.Equal(t, "USD", rendered["currency"])
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

func TestUnknownOpKindOpaqueArrayBodyDoesNotBreak(t *testing.T) {
	opBody := []interface{}{"future", uint64(7), map[string]interface{}{"nested": []interface{}{"shape"}}}
	opBodyBytes, err := encodeCanonical(opBody)
	require.NoError(t, err)
	cborBytes, err := EncodeCanonicalEnvelope(map[string]interface{}{
		"version":           uint8(1),
		"ts_unix":           uint64(1710000005),
		"actor_omni":        bytesOf(0x44),
		"operator_omni":     bytesOf(0x55),
		"op_kind":           uint8(250),
		"op_body":           opBody,
		"result":            uint8(0),
		"intent_text":       nil,
		"intent_commitment": nil,
	})
	require.NoError(t, err)

	decoded, err := DecodeEnvelope(cborBytes, "")
	require.NoError(t, err)
	fallback, ok := decoded.Body.(UnknownOpBody)
	require.True(t, ok)
	assert.Equal(t, uint8(250), fallback.OpKindByte)
	assert.Equal(t, base64.StdEncoding.EncodeToString(opBodyBytes), fallback.OpBodyB64)
}

func TestAuditEnvelopeEvidenceMatrix(t *testing.T) {
	type evidenceRow struct {
		Case                  string      `json:"case"`
		OpKind                uint8       `json:"op_kind"`
		OpKindName            string      `json:"op_kind_name"`
		ResultName            string      `json:"result_name"`
		HashVerified          bool        `json:"hash_verified"`
		EnvelopeHash          string      `json:"envelope_hash"`
		Body                  interface{} `json:"body"`
		OpaqueOpBodyCBORBytes bool        `json:"opaque_op_body_cbor_bytes"`
	}

	tests := []struct {
		name       string
		opKind     uint8
		opName     string
		body       map[string]interface{}
		assertBody func(t *testing.T, body interface{})
	}{
		{
			name:   "SignEip712",
			opKind: 21,
			opName: "SignEip712",
			body: map[string]interface{}{
				"chain_id":           uint64(212013),
				"verifying_contract": "0x1111111111111111111111111111111111111111",
				"primary_type":       "Permit",
				"type_hash":          "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"domain_separator":   "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				"digest":             "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
			},
			assertBody: func(t *testing.T, body interface{}) {
				rendered, ok := body.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, uint64(212013), rendered["chain_id"])
				assert.Equal(t, "Permit", rendered["primary_type"])
				assert.Equal(t, "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", rendered["digest"])
			},
		},
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
			assertBody: func(t *testing.T, body interface{}) {
				rendered, ok := body.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "credential-vault", rendered["service"])
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
			assertBody: func(t *testing.T, body interface{}) {
				rendered, ok := body.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", rendered["device_key_hash"])
				assert.Equal(t, uint64(7), rendered["role_bits"])
				assert.Equal(t, "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", rendered["attestation_hash"])
			},
		},
		{
			name:   "PaymentDirect",
			opKind: 31,
			opName: "PaymentDirect",
			body: map[string]interface{}{
				"rail":         "stripe",
				"ref":          "invoice-123",
				"amount_minor": "12345",
				"currency":     "USD",
			},
			assertBody: func(t *testing.T, body interface{}) {
				rendered, ok := body.(map[string]interface{})
				require.True(t, ok)
				assert.Equal(t, "stripe", rendered["rail"])
				assert.Equal(t, "invoice-123", rendered["ref"])
				assert.Equal(t, "12345", rendered["amount_minor"])
				assert.Equal(t, "USD", rendered["currency"])
			},
		},
		{
			name:   "UnknownFuture",
			opKind: 250,
			opName: "Unknown(250)",
			body: map[string]interface{}{
				"future_field": "opaque",
				"future_nonce": uint64(9),
			},
			assertBody: func(t *testing.T, body interface{}) {
				fallback, ok := body.(UnknownOpBody)
				require.True(t, ok)
				assert.Equal(t, uint8(250), fallback.OpKindByte)
				assert.NotEmpty(t, fallback.OpBodyB64)
			},
		},
	}

	rows := make([]evidenceRow, 0, len(tests))
	for i, tt := range tests {
		envelopeBytes, err := EncodeCanonicalEnvelope(map[string]interface{}{
			"version":           uint8(1),
			"ts_unix":           uint64(1710000100 + i),
			"actor_omni":        bytesOf(0x42),
			"operator_omni":     bytesOf(0x24),
			"op_kind":           tt.opKind,
			"op_body":           tt.body,
			"result":            uint8(0),
			"intent_text":       "fixture-backed evidence matrix",
			"intent_commitment": bytesOf(0x77),
		})
		require.NoError(t, err)

		hash := crypto.Keccak256Hash(envelopeBytes).Hex()
		decoded, err := DecodeEnvelope(envelopeBytes, hash)
		require.NoError(t, err)
		require.True(t, decoded.HashVerified)
		require.NotEmpty(t, decoded.OpaqueOpBodyCBOR)
		require.Equal(t, tt.opKind, decoded.OpKind)
		require.Equal(t, tt.opName, decoded.OpKindName)
		require.Equal(t, "Success", decoded.ResultName)
		require.Equal(t, hash, decoded.EnvelopeHash)
		tt.assertBody(t, decoded.Body)

		rows = append(rows, evidenceRow{
			Case:                  tt.name,
			OpKind:                decoded.OpKind,
			OpKindName:            decoded.OpKindName,
			ResultName:            decoded.ResultName,
			HashVerified:          decoded.HashVerified,
			EnvelopeHash:          decoded.EnvelopeHash,
			Body:                  decoded.Body,
			OpaqueOpBodyCBORBytes: decoded.OpaqueOpBodyCBOR != "",
		})
	}

	payload, err := json.Marshal(rows)
	require.NoError(t, err)
	t.Logf("AGENTKEYS_EVIDENCE_JSON=%s", payload)
}

func TestDecodeTypedAuditRowsAndRootLeaves(t *testing.T) {
	operator := "0x" + strings.Repeat("24", 32)
	actor := "0x" + strings.Repeat("42", 32)
	signBody := map[string]interface{}{
		"chain_id":           uint64(212013),
		"verifying_contract": "0x1111111111111111111111111111111111111111",
		"primary_type":       "Permit",
		"type_hash":          "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"domain_separator":   "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		"digest":             "0xcccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
	}
	deviceBody := map[string]interface{}{
		"device_key_hash":  "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		"role_bits":        uint64(7),
		"attestation_hash": "0xbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
	}
	signBytes, signHash := canonicalFixtureEnvelope(t, 21, operator, actor, signBody)
	deviceBytes, deviceHash := canonicalFixtureEnvelope(t, 50, operator, actor, deviceBody)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch strings.TrimPrefix(r.URL.Path, "/v1/audit/envelope/") {
		case strings.TrimPrefix(signHash, "0x"):
			_, _ = w.Write(signBytes)
		case strings.TrimPrefix(deviceHash, "0x"):
			_, _ = w.Write(deviceBytes)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	logs := []EVMLogRecord{
		auditAppendedLog(operator, actor, 21, signHash, 12, 1),
		auditAppendedLog(operator, actor, 50, deviceHash, 12, 2),
	}
	rows, err := DecodeTypedAuditRows(context.Background(), logs, srv.URL, NewEnvelopeCache())
	require.NoError(t, err)
	require.Len(t, rows, 2)
	assert.Equal(t, "SignEip712", rows[0].OpKindName)
	assert.Equal(t, uint64(12), rows[0].Block)
	assert.Equal(t, uint64(1), rows[0].LogIndex)
	assert.Equal(t, "12:1", rows[0].StreamPosition)
	assert.Equal(t, "DeviceAdd", rows[1].OpKindName)

	rootHash := "0x" + strings.Repeat("ab", 32)
	rootRows, err := DecodeAuditRootRows(context.Background(), auditRootLog(operator, rootHash, []uint8{21, 50}, 2, 12, 3), logs, srv.URL, NewEnvelopeCache())
	require.NoError(t, err)
	assert.Equal(t, rootHash, rootRows.MerkleRoot)
	assert.Equal(t, uint64(2), rootRows.EntryCount)
	assert.Equal(t, []string{signHash, deviceHash}, rootRows.Leaves)
	require.Len(t, rootRows.Rows, 2)
	assert.Equal(t, "SignEip712", rootRows.Rows[0].OpKindName)
	assert.Equal(t, "DeviceAdd", rootRows.Rows[1].OpKindName)
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

func canonicalFixtureEnvelope(t *testing.T, opKind uint8, operator, actor string, body map[string]interface{}) ([]byte, string) {
	t.Helper()
	cborBytes, err := EncodeCanonicalEnvelope(map[string]interface{}{
		"version":           uint8(1),
		"ts_unix":           uint64(1710000200),
		"actor_omni":        mustHexBytes(t, actor),
		"operator_omni":     mustHexBytes(t, operator),
		"op_kind":           opKind,
		"op_body":           body,
		"result":            uint8(0),
		"intent_text":       nil,
		"intent_commitment": nil,
	})
	require.NoError(t, err)
	return cborBytes, crypto.Keccak256Hash(cborBytes).Hex()
}

func auditAppendedLog(operator, actor string, opKind uint8, envelopeHash string, block uint64, logIndex uint64) EVMLogRecord {
	return EVMLogRecord{
		Address:          "0x1111111111111111111111111111111111111111",
		Topics:           []string{AuditAppendedV2Topic, operator, actor, PaddedOpKindTopic(opKind)},
		Data:             envelopeHash,
		BlockNumber:      fmt.Sprintf("0x%x", block),
		BlockHash:        "0x" + strings.Repeat("11", 32),
		Timestamp:        "0x65f00000",
		LogIndex:         fmt.Sprintf("0x%x", logIndex),
		TransactionHash:  "0x" + strings.Repeat("22", 32),
		TransactionIndex: "0x0",
	}
}

func auditRootLog(operator, merkleRoot string, opKinds []uint8, entryCount uint64, block uint64, logIndex uint64) EVMLogRecord {
	bitmap := make([]byte, 32)
	for _, opKind := range opKinds {
		bitmap[31-int(opKind)/8] |= byte(1 << uint(opKind%8))
	}
	return EVMLogRecord{
		Address:          "0x1111111111111111111111111111111111111111",
		Topics:           []string{AuditRootAppendedV2Topic, operator, merkleRoot},
		Data:             "0x" + hexOf(bitmap) + fmt.Sprintf("%064x", entryCount),
		BlockNumber:      fmt.Sprintf("0x%x", block),
		BlockHash:        "0x" + strings.Repeat("33", 32),
		Timestamp:        "0x65f00000",
		LogIndex:         fmt.Sprintf("0x%x", logIndex),
		TransactionHash:  "0x" + strings.Repeat("44", 32),
		TransactionIndex: "0x0",
	}
}

func mustHexBytes(t *testing.T, value string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.TrimPrefix(value, "0x"))
	require.NoError(t, err)
	return b
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
