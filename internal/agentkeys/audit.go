package agentkeys

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
)

const (
	EnvelopeVersion = 1

	HeimaChainID                   = 212013
	CredentialAuditContractAddress = "0x63c4545ac01c77cc74044f25b8edea3880224577"

	AuditAppendedV2Signature          = "AuditAppendedV2(bytes32,bytes32,uint8,bytes32)"
	AuditAppendedCurrentSignature     = "AuditAppended(bytes32,bytes32,bytes32,uint8,uint256,bytes32)"
	AuditRootAppendedV2Signature      = "AuditRootAppendedV2(bytes32,bytes32,bytes32,uint64)"
	AuditRootAppendedCurrentSignature = "AuditRootAppended(bytes32,bytes32,uint256,uint64)"
)

var (
	AuditAppendedV2Topic          = crypto.Keccak256Hash([]byte(AuditAppendedV2Signature)).Hex()
	AuditAppendedCurrentTopic     = crypto.Keccak256Hash([]byte(AuditAppendedCurrentSignature)).Hex()
	AuditRootAppendedV2Topic      = crypto.Keccak256Hash([]byte(AuditRootAppendedV2Signature)).Hex()
	AuditRootAppendedCurrentTopic = crypto.Keccak256Hash([]byte(AuditRootAppendedCurrentSignature)).Hex()
	ErrEnvelopeNotFound           = errors.New("agentkeys audit envelope not found")
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
	EventName         string `json:"event_name"`
	EventTopic        string `json:"event_topic"`
	OperatorOmni      string `json:"operator_omni"`
	ActorOmni         string `json:"actor_omni"`
	OpKind            uint8  `json:"op_kind"`
	EnvelopeHash      string `json:"envelope_hash"`
	CurrentIndexedKey string `json:"current_indexed_key,omitempty"`
	CurrentSequence   string `json:"current_sequence,omitempty"`
}

type AuditRootAppendedV2Event struct {
	EventName        string `json:"event_name"`
	EventTopic       string `json:"event_topic"`
	OperatorOmni     string `json:"operator_omni"`
	MerkleRoot       string `json:"merkle_root"`
	OpKindBitmapU256 string `json:"op_kind_bitmap_u256"`
	EntryCount       uint64 `json:"entry_count"`
}

type EVMLogRecord struct {
	Address          string   `json:"address"`
	Topics           []string `json:"topics"`
	Data             string   `json:"data"`
	BlockNumber      string   `json:"blockNumber"`
	BlockHash        string   `json:"blockHash"`
	Timestamp        string   `json:"timestamp"`
	LogIndex         string   `json:"logIndex"`
	TransactionHash  string   `json:"transactionHash"`
	TransactionIndex string   `json:"transactionIndex"`
}

type TypedAuditRow struct {
	Envelope
	ChainID            uint64  `json:"chain_id"`
	ContractAddress    string  `json:"contract_address"`
	EventName          string  `json:"event_name"`
	EventTopic         string  `json:"event_topic"`
	Block              uint64  `json:"block"`
	BlockHash          string  `json:"block_hash"`
	Timestamp          uint64  `json:"timestamp"`
	Tx                 string  `json:"tx"`
	TransactionIndex   uint64  `json:"transaction_index"`
	LogIndex           uint64  `json:"log_index"`
	StreamPosition     string  `json:"stream_position"`
	CurrentIndexedKey  string  `json:"current_indexed_key,omitempty"`
	CurrentSequence    string  `json:"current_sequence,omitempty"`
	EnvelopeAvailable  bool    `json:"envelope_available"`
	EnvelopeFetchError *string `json:"envelope_fetch_error,omitempty"`
}

type AuditRowsPage struct {
	ChainID         uint64          `json:"chain_id"`
	ContractAddress string          `json:"contract_address"`
	Events          []TypedAuditRow `json:"events"`
	NextCursor      *string         `json:"next_cursor"`
}

type AuditRootRows struct {
	ChainID          uint64          `json:"chain_id"`
	ContractAddress  string          `json:"contract_address"`
	MerkleRoot       string          `json:"merkle_root"`
	OperatorOmni     string          `json:"operator_omni"`
	OpKindBitmapU256 string          `json:"op_kind_bitmap_u256"`
	EntryCount       uint64          `json:"entry_count"`
	Block            uint64          `json:"block"`
	BlockHash        string          `json:"block_hash"`
	Tx               string          `json:"tx"`
	LogIndex         uint64          `json:"log_index"`
	Leaves           []string        `json:"leaves"`
	Rows             []TypedAuditRow `json:"rows"`
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

func DecodeTypedAuditRow(ctx context.Context, log EVMLogRecord, workerBaseURL string, cache *EnvelopeCache) (*TypedAuditRow, error) {
	return decodeTypedAuditRow(ctx, log, workerBaseURL, cache, false)
}

func DecodeTypedAuditRowBestEffort(ctx context.Context, log EVMLogRecord, workerBaseURL string, cache *EnvelopeCache) (*TypedAuditRow, error) {
	return decodeTypedAuditRow(ctx, log, workerBaseURL, cache, true)
}

func decodeTypedAuditRow(ctx context.Context, log EVMLogRecord, workerBaseURL string, cache *EnvelopeCache, allowMissingEnvelope bool) (*TypedAuditRow, error) {
	event, err := DecodeAuditAppendedLog(log.Topics, log.Data)
	if err != nil {
		return nil, err
	}
	row, err := auditRowSkeleton(log, event)
	if err != nil {
		return nil, err
	}
	if cache == nil {
		cache = NewEnvelopeCache()
	}
	_, envelope, err := cache.FetchAndDecode(ctx, workerBaseURL, event.EnvelopeHash)
	if err != nil {
		if allowMissingEnvelope && errors.Is(err, ErrEnvelopeNotFound) {
			errText := err.Error()
			row.Envelope = Envelope{
				Version:      EnvelopeVersion,
				ActorOmni:    event.ActorOmni,
				OperatorOmni: event.OperatorOmni,
				OpKind:       event.OpKind,
				OpKindName:   OpKindName(event.OpKind),
				EnvelopeHash: event.EnvelopeHash,
			}
			row.EnvelopeFetchError = &errText
			return row, nil
		}
		return nil, err
	}
	if !strings.EqualFold(event.OperatorOmni, envelope.OperatorOmni) {
		return nil, fmt.Errorf("operator_omni mismatch for envelope %s", event.EnvelopeHash)
	}
	if !strings.EqualFold(event.ActorOmni, envelope.ActorOmni) {
		return nil, fmt.Errorf("actor_omni mismatch for envelope %s", event.EnvelopeHash)
	}
	if event.OpKind != envelope.OpKind {
		return nil, fmt.Errorf("op_kind mismatch for envelope %s", event.EnvelopeHash)
	}

	row.Envelope = *envelope
	row.EnvelopeAvailable = true
	return row, nil
}

func auditRowSkeleton(log EVMLogRecord, event *AuditAppendedV2Event) (*TypedAuditRow, error) {
	block, err := parseUintAuto(log.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("blockNumber: %w", err)
	}
	timestamp, err := parseUintAuto(log.Timestamp)
	if err != nil {
		return nil, fmt.Errorf("timestamp: %w", err)
	}
	logIndex, err := parseUintAuto(log.LogIndex)
	if err != nil {
		return nil, fmt.Errorf("logIndex: %w", err)
	}
	txIndex, err := parseUintAuto(log.TransactionIndex)
	if err != nil {
		return nil, fmt.Errorf("transactionIndex: %w", err)
	}

	return &TypedAuditRow{
		ChainID:           HeimaChainID,
		ContractAddress:   normalizeHexHash(log.Address),
		EventName:         event.EventName,
		EventTopic:        event.EventTopic,
		Block:             block,
		BlockHash:         normalizeHexHash(log.BlockHash),
		Timestamp:         timestamp,
		Tx:                normalizeHexHash(log.TransactionHash),
		TransactionIndex:  txIndex,
		LogIndex:          logIndex,
		StreamPosition:    fmt.Sprintf("%d:%d", block, logIndex),
		CurrentIndexedKey: event.CurrentIndexedKey,
		CurrentSequence:   event.CurrentSequence,
	}, nil
}

func DecodeTypedAuditRows(ctx context.Context, logs []EVMLogRecord, workerBaseURL string, cache *EnvelopeCache) ([]TypedAuditRow, error) {
	rows := make([]TypedAuditRow, 0, len(logs))
	for _, log := range logs {
		row, err := DecodeTypedAuditRow(ctx, log, workerBaseURL, cache)
		if err != nil {
			return nil, err
		}
		rows = append(rows, *row)
	}
	return rows, nil
}

func DecodeTypedAuditRowsBestEffort(ctx context.Context, logs []EVMLogRecord, workerBaseURL string, cache *EnvelopeCache) ([]TypedAuditRow, error) {
	rows := make([]TypedAuditRow, 0, len(logs))
	for _, log := range logs {
		row, err := DecodeTypedAuditRowBestEffort(ctx, log, workerBaseURL, cache)
		if err != nil {
			return nil, err
		}
		rows = append(rows, *row)
	}
	return rows, nil
}

func DecodeAuditRootRows(ctx context.Context, rootLog EVMLogRecord, leafLogs []EVMLogRecord, workerBaseURL string, cache *EnvelopeCache) (*AuditRootRows, error) {
	event, err := DecodeAuditRootAppendedLog(rootLog.Topics, rootLog.Data)
	if err != nil {
		return nil, err
	}
	block, err := parseUintAuto(rootLog.BlockNumber)
	if err != nil {
		return nil, fmt.Errorf("root blockNumber: %w", err)
	}
	logIndex, err := parseUintAuto(rootLog.LogIndex)
	if err != nil {
		return nil, fmt.Errorf("root logIndex: %w", err)
	}

	rows, err := DecodeTypedAuditRowsBestEffort(ctx, leafLogs, workerBaseURL, cache)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Block == rows[j].Block {
			return rows[i].LogIndex < rows[j].LogIndex
		}
		return rows[i].Block < rows[j].Block
	})
	leaves := make([]string, 0, len(rows))
	for _, row := range rows {
		leaves = append(leaves, row.EnvelopeHash)
	}

	return &AuditRootRows{
		ChainID:          HeimaChainID,
		ContractAddress:  normalizeHexHash(rootLog.Address),
		MerkleRoot:       event.MerkleRoot,
		OperatorOmni:     event.OperatorOmni,
		OpKindBitmapU256: event.OpKindBitmapU256,
		EntryCount:       event.EntryCount,
		Block:            block,
		BlockHash:        normalizeHexHash(rootLog.BlockHash),
		Tx:               normalizeHexHash(rootLog.TransactionHash),
		LogIndex:         logIndex,
		Leaves:           leaves,
		Rows:             rows,
	}, nil
}

func PaddedOpKindTopic(opKind uint8) string {
	return "0x" + strings.Repeat("0", 62) + fmt.Sprintf("%02x", opKind)
}

func CurrentAuditOpKindDataPrefix(opKind uint8) string {
	return fmt.Sprintf("%064x", opKind)
}

func OpKindTopicsFromBitmap(bitmap string) ([]string, error) {
	bytes, err := hex.DecodeString(strings.TrimPrefix(strings.ToLower(bitmap), "0x"))
	if err != nil {
		return nil, err
	}
	if len(bytes) != 32 {
		return nil, fmt.Errorf("op_kind_bitmap must be 32 bytes")
	}
	topics := make([]string, 0)
	for opKind := 0; opKind < 256; opKind++ {
		byteIndex := 31 - opKind/8
		mask := byte(1 << uint(opKind%8))
		if bytes[byteIndex]&mask != 0 {
			topics = append(topics, PaddedOpKindTopic(uint8(opKind)))
		}
	}
	return topics, nil
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
		EventName:    "AuditAppendedV2",
		EventTopic:   AuditAppendedV2Topic,
		OperatorOmni: normalizeBytes32Topic(topics[1]),
		ActorOmni:    normalizeBytes32Topic(topics[2]),
		OpKind:       opKind,
		EnvelopeHash: hash,
	}, nil
}

func DecodeAuditAppendedLog(topics []string, data string) (*AuditAppendedV2Event, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("audit event requires topic0")
	}
	switch {
	case strings.EqualFold(topics[0], AuditAppendedV2Topic):
		return DecodeAuditAppendedV2Log(topics, data)
	case strings.EqualFold(topics[0], AuditAppendedCurrentTopic):
		return DecodeAuditAppendedCurrentLog(topics, data)
	default:
		return nil, fmt.Errorf("unexpected audit event topic0 %s", topics[0])
	}
}

func DecodeAuditAppendedCurrentLog(topics []string, data string) (*AuditAppendedV2Event, error) {
	if len(topics) != 4 {
		return nil, fmt.Errorf("AuditAppended requires 4 topics")
	}
	if !strings.EqualFold(topics[0], AuditAppendedCurrentTopic) {
		return nil, fmt.Errorf("unexpected AuditAppended topic0 %s", topics[0])
	}
	opKind, err := abiUint8(data, 0)
	if err != nil {
		return nil, fmt.Errorf("op_kind: %w", err)
	}
	sequence, err := abiUint256Decimal(data, 1)
	if err != nil {
		return nil, fmt.Errorf("current_sequence: %w", err)
	}
	hash, err := abiBytes32(data, 2)
	if err != nil {
		return nil, fmt.Errorf("envelope_hash: %w", err)
	}
	return &AuditAppendedV2Event{
		EventName:         "AuditAppended",
		EventTopic:        AuditAppendedCurrentTopic,
		OperatorOmni:      normalizeBytes32Topic(topics[1]),
		ActorOmni:         normalizeBytes32Topic(topics[2]),
		OpKind:            opKind,
		EnvelopeHash:      hash,
		CurrentIndexedKey: normalizeBytes32Topic(topics[3]),
		CurrentSequence:   sequence,
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
		EventName:        "AuditRootAppendedV2",
		EventTopic:       AuditRootAppendedV2Topic,
		OperatorOmni:     normalizeBytes32Topic(topics[1]),
		MerkleRoot:       normalizeBytes32Topic(topics[2]),
		OpKindBitmapU256: bitmap,
		EntryCount:       count,
	}, nil
}

func DecodeAuditRootAppendedLog(topics []string, data string) (*AuditRootAppendedV2Event, error) {
	if len(topics) == 0 {
		return nil, fmt.Errorf("audit root event requires topic0")
	}
	switch {
	case strings.EqualFold(topics[0], AuditRootAppendedV2Topic):
		return DecodeAuditRootAppendedV2Log(topics, data)
	case strings.EqualFold(topics[0], AuditRootAppendedCurrentTopic):
		return DecodeAuditRootAppendedCurrentLog(topics, data)
	default:
		return nil, fmt.Errorf("unexpected audit root event topic0 %s", topics[0])
	}
}

func DecodeAuditRootAppendedCurrentLog(topics []string, data string) (*AuditRootAppendedV2Event, error) {
	if len(topics) != 3 {
		return nil, fmt.Errorf("AuditRootAppended requires 3 topics")
	}
	if !strings.EqualFold(topics[0], AuditRootAppendedCurrentTopic) {
		return nil, fmt.Errorf("unexpected AuditRootAppended topic0 %s", topics[0])
	}
	bitmap, err := abiWord(data, 0)
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
		EventName:        "AuditRootAppended",
		EventTopic:       AuditRootAppendedCurrentTopic,
		OperatorOmni:     normalizeBytes32Topic(topics[1]),
		MerkleRoot:       normalizeBytes32Topic(topics[2]),
		OpKindBitmapU256: "0x" + bitmap,
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

func parseUintAuto(value string) (uint64, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return 0, nil
	}
	if strings.HasPrefix(value, "0x") {
		return strconv.ParseUint(strings.TrimPrefix(value, "0x"), 16, 64)
	}
	return strconv.ParseUint(value, 10, 64)
}

func abiBytes32(data string, wordOffset int) (string, error) {
	word, err := abiWord(data, wordOffset)
	if err != nil {
		return "", err
	}
	return "0x" + word, nil
}

func abiUint8(data string, wordOffset int) (uint8, error) {
	word, err := abiWord(data, wordOffset)
	if err != nil {
		return 0, err
	}
	prefix := strings.TrimLeft(word[:62], "0")
	if prefix != "" {
		return 0, fmt.Errorf("ABI word does not fit uint8")
	}
	n, err := strconv.ParseUint(word[62:], 16, 8)
	return uint8(n), err
}

func abiUint256Decimal(data string, wordOffset int) (string, error) {
	word, err := abiWord(data, wordOffset)
	if err != nil {
		return "", err
	}
	n := new(big.Int)
	if _, ok := n.SetString(word, 16); !ok {
		return "", fmt.Errorf("invalid ABI uint256")
	}
	return n.String(), nil
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
