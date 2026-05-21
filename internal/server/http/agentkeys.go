package http

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/itering/subscan/internal/agentkeys"
	"github.com/itering/subscan/model"
	evmdao "github.com/itering/subscan/plugins/evm/dao"
)

const defaultAgentKeysAuditWorkerURL = "https://audit.litentry.org"

var agentkeysEnvelopeCache = agentkeys.NewEnvelopeCache()
var agentkeysEvmAPI = &evmdao.ApiSrv{}

type agentkeysAuditCursor struct {
	Block    uint64 `json:"block"`
	LogIndex uint64 `json:"log_index"`
}

func agentkeysAuditEnvelopeHandle(c *gin.Context) {
	hash := c.Param("hash")
	body, _, err := agentkeysEnvelopeCache.FetchAndDecode(c.Request.Context(), agentkeysAuditWorkerURL(), hash)
	if errors.Is(err, agentkeys.ErrEnvelopeNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.Data(http.StatusOK, "application/octet-stream", body)
}

func agentkeysAuditRowsHandle(c *gin.Context) {
	limit, err := agentkeysAuditLimit(c.Query("limit"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	sortDir := strings.ToLower(c.DefaultQuery("sort", "desc"))
	if sortDir != "asc" && sortDir != "desc" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "sort must be asc or desc"})
		return
	}

	opts, err := agentkeysAuditLogFilters(c, c.Param("operator_omni"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if cursorRaw := c.Query("cursor"); cursorRaw != "" {
		cursor, err := decodeAgentKeysCursor(cursorRaw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if sortDir == "asc" {
			opts = append(opts, model.Where("(block_num > ? OR (block_num = ? AND `index` > ?))", cursor.Block, cursor.Block, cursor.LogIndex))
		} else {
			opts = append(opts, model.Where("(block_num < ? OR (block_num = ? AND `index` < ?))", cursor.Block, cursor.Block, cursor.LogIndex))
		}
	}

	order := "block_num desc, `index` desc"
	if sortDir == "asc" {
		order = "block_num asc, `index` asc"
	}
	logs := agentkeysEvmAPI.API_GetLogsForAgentKeys(c.Request.Context(), order, limit+1, opts...)
	hasNext := len(logs) > limit
	if hasNext {
		logs = logs[:limit]
	}
	rows, err := agentkeys.DecodeTypedAuditRows(c.Request.Context(), toAgentKeysLogs(logs), agentkeysAuditWorkerURL(), agentkeysEnvelopeCache)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	var nextCursor *string
	if hasNext && len(rows) > 0 {
		cursor := encodeAgentKeysCursor(agentkeysAuditCursor{Block: rows[len(rows)-1].Block, LogIndex: rows[len(rows)-1].LogIndex})
		nextCursor = &cursor
	}
	c.JSON(http.StatusOK, agentkeys.AuditRowsPage{Events: rows, NextCursor: nextCursor})
}

func agentkeysAuditRootHandle(c *gin.Context) {
	root := normalizeAgentKeysBytes32(c.Param("merkle_root"))
	rootLogs := agentkeysEvmAPI.API_GetLogsForAgentKeys(c.Request.Context(), "block_num desc, `index` desc", 1,
		model.Where("method_hash = ?", agentkeys.AuditRootAppendedV2Topic),
		model.Where("topic2 = ?", root),
	)
	if len(rootLogs) == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "not_found"})
		return
	}

	rootRecord := toAgentKeysLogs(rootLogs)[0]
	rootEvent, err := agentkeys.DecodeAuditRootAppendedV2Log(rootRecord.Topics, rootRecord.Data)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	rootBlock, err := parseAgentKeysUint(rootRecord.BlockNumber)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "root blockNumber: " + err.Error()})
		return
	}
	rootLogIndex, err := parseAgentKeysUint(rootRecord.LogIndex)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": "root logIndex: " + err.Error()})
		return
	}
	opKindTopics, err := agentkeys.OpKindTopicsFromBitmap(rootEvent.OpKindBitmapU256)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	leafLogs := []evmdao.EtherscanLogsRes{}
	if rootEvent.EntryCount > 0 && len(opKindTopics) > 0 {
		leafLogs = agentkeysEvmAPI.API_GetLogsForAgentKeys(c.Request.Context(), "block_num desc, `index` desc", int(rootEvent.EntryCount),
			model.Where("method_hash = ?", agentkeys.AuditAppendedV2Topic),
			model.Where("topic1 = ?", rootEvent.OperatorOmni),
			model.Where("topic3 in ?", opKindTopics),
			model.Where("(block_num < ? OR (block_num = ? AND `index` < ?))", rootBlock, rootBlock, rootLogIndex),
		)
	}

	rows, err := agentkeys.DecodeAuditRootRows(c.Request.Context(), rootRecord, toAgentKeysLogs(leafLogs), agentkeysAuditWorkerURL(), agentkeysEnvelopeCache)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rows)
}

func agentkeysAuditWorkerURL() string {
	workerURL := os.Getenv("AGENTKEYS_AUDIT_WORKER_URL")
	if workerURL == "" {
		return defaultAgentKeysAuditWorkerURL
	}
	return workerURL
}

func agentkeysAuditLimit(raw string) (int, error) {
	if raw == "" {
		return 50, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit < 1 {
		return 0, fmt.Errorf("limit must be a positive integer")
	}
	if limit > 500 {
		return 0, fmt.Errorf("limit must be <= 500")
	}
	return limit, nil
}

func agentkeysAuditLogFilters(c *gin.Context, operator string) ([]model.Option, error) {
	opts := []model.Option{
		model.Where("method_hash = ?", agentkeys.AuditAppendedV2Topic),
		model.Where("topic1 = ?", normalizeAgentKeysBytes32(operator)),
	}
	if opKindRaw := c.Query("op_kind"); opKindRaw != "" {
		opKind, err := strconv.ParseUint(opKindRaw, 10, 8)
		if err != nil {
			return nil, fmt.Errorf("op_kind must fit uint8")
		}
		opts = append(opts, model.Where("topic3 = ?", agentkeys.PaddedOpKindTopic(uint8(opKind))))
	}
	if actor := c.Query("actor_omni"); actor != "" {
		opts = append(opts, model.Where("topic2 = ?", normalizeAgentKeysBytes32(actor)))
	}
	if from := c.Query("from_block"); from != "" {
		n, err := strconv.ParseUint(from, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("from_block must be uint")
		}
		opts = append(opts, model.Where("block_num >= ?", n))
	}
	if to := c.Query("to_block"); to != "" {
		n, err := strconv.ParseUint(to, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("to_block must be uint")
		}
		opts = append(opts, model.Where("block_num <= ?", n))
	}
	return opts, nil
}

func toAgentKeysLogs(logs []evmdao.EtherscanLogsRes) []agentkeys.EVMLogRecord {
	out := make([]agentkeys.EVMLogRecord, 0, len(logs))
	for _, log := range logs {
		out = append(out, agentkeys.EVMLogRecord{
			Address:          log.Address,
			Topics:           log.Topics,
			Data:             log.Data,
			BlockNumber:      log.BlockNumber,
			BlockHash:        log.BlockHash,
			Timestamp:        log.Timestamp,
			LogIndex:         log.LogIndex,
			TransactionHash:  log.TransactionHash,
			TransactionIndex: log.TransactionIndex,
		})
	}
	return out
}

func normalizeAgentKeysBytes32(value string) string {
	return "0x" + strings.TrimPrefix(strings.ToLower(strings.TrimSpace(value)), "0x")
}

func encodeAgentKeysCursor(cursor agentkeysAuditCursor) string {
	payload, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(payload)
}

func decodeAgentKeysCursor(raw string) (agentkeysAuditCursor, error) {
	payload, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return agentkeysAuditCursor{}, fmt.Errorf("invalid cursor")
	}
	var cursor agentkeysAuditCursor
	if err := json.Unmarshal(payload, &cursor); err != nil {
		return agentkeysAuditCursor{}, fmt.Errorf("invalid cursor")
	}
	return cursor, nil
}

func parseAgentKeysUint(value string) (uint64, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if strings.HasPrefix(value, "0x") {
		return strconv.ParseUint(strings.TrimPrefix(value, "0x"), 16, 64)
	}
	return strconv.ParseUint(value, 10, 64)
}
