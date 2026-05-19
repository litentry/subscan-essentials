package http

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/itering/subscan/model"
	"github.com/itering/subscan/plugins/evm/agentkeys"
	"github.com/itering/subscan/plugins/evm/dao"
	"github.com/itering/subscan/util"
	"github.com/itering/subscan/util/address"
	"github.com/itering/subscan/util/validator"
)

type agentKeysContractParams struct {
	Address string `json:"address" validate:"required,eth_addr"`
}

type agentKeysEventsParams struct {
	Address   string `json:"address" validate:"omitempty,eth_addr"`
	Keyword   string `json:"keyword" validate:"omitempty"`
	Topic0    string `json:"topic0" validate:"omitempty,len=66"`
	ActorOmni string `json:"actor_omni" validate:"omitempty,len=66"`
	Limit     int    `json:"row" validate:"omitempty,min=1,max=100"`
}

type agentKeysActorParams struct {
	ActorOmni string `json:"actor_omni" validate:"required,len=66"`
}

type agentKeysSearchParams struct {
	Query string `json:"query" validate:"required,min=1,max=80"`
}

type agentKeysEventLog struct {
	dao.EtherscanLogsRes
	ContractName   string                 `json:"contract_name"`
	EventName      string                 `json:"event_name"`
	EventSignature string                 `json:"event_signature"`
	Topic0         string                 `json:"topic0"`
	Decoded        map[string]interface{} `json:"decoded,omitempty"`
	Source         string                 `json:"source"`
}

type agentKeysActorSummary struct {
	ActorOmni         string               `json:"actor_omni"`
	DevicesRegistered int                  `json:"devices_registered"`
	ScopeGrants       int                  `json:"scope_grants"`
	AuditEntries      int                  `json:"audit_entries"`
	CurrentK3Epoch    uint64               `json:"current_k3_epoch"`
	Devices           []interface{}        `json:"devices"`
	Scopes            []agentKeysEventLog  `json:"scopes"`
	Audits            []agentKeysEventLog  `json:"audits"`
	K3Rotations       []agentKeysEventLog  `json:"k3_rotations"`
	Contracts         []agentkeys.Contract `json:"contracts"`
	EventKeywords     []agentkeys.Event    `json:"event_keywords"`
}

func agentKeysContractsHandle(w http.ResponseWriter, _ *http.Request) error {
	toJson(w, 0, map[string]interface{}{
		"chain_id":  agentkeys.ChainID,
		"rpc":       agentkeys.RPCURL,
		"contracts": agentkeys.Contracts(),
		"events":    agentkeys.Events(),
	}, nil)
	return nil
}

func agentKeysContractHandle(w http.ResponseWriter, r *http.Request) error {
	p := new(agentKeysContractParams)
	if err := validator.Validate(r.Body, p); err != nil {
		toJson(w, 10001, nil, err)
		return nil
	}
	contract, ok := agentkeys.ContractByAddress(p.Address)
	if !ok {
		toJson(w, 10002, nil, fmt.Errorf("agentkeys contract not found"))
		return nil
	}
	toJson(w, 0, map[string]interface{}{
		"contract":       contract,
		"events":         agentkeys.EventsByAddress(contract.Address),
		"indexed_record": srv.ContractsByAddr(r.Context(), contract.Address),
	}, nil)
	return nil
}

func agentKeysEventsHandle(w http.ResponseWriter, r *http.Request) error {
	p := new(agentKeysEventsParams)
	if err := validator.Validate(r.Body, p); err != nil {
		toJson(w, 10001, nil, err)
		return nil
	}
	if p.Limit == 0 {
		p.Limit = 50
	}
	logs := agentKeysQueryLogs(r, p.Address, p.Keyword, p.Topic0, p.ActorOmni, p.Limit)
	toJson(w, 0, map[string]interface{}{
		"list":   logs,
		"events": agentkeys.Events(),
	}, nil)
	return nil
}

func agentKeysActorHandle(w http.ResponseWriter, r *http.Request) error {
	p := new(agentKeysActorParams)
	if err := validator.Validate(r.Body, p); err != nil {
		toJson(w, 10001, nil, err)
		return nil
	}
	actorOmni, ok := agentkeys.NormalizeBytes32(p.ActorOmni)
	if !ok {
		toJson(w, 10001, nil, fmt.Errorf("actor_omni must be 0x-prefixed bytes32"))
		return nil
	}
	devices := agentKeysQueryLogs(r, "", "DeviceRegistered", "", actorOmni, 100)
	scopes := agentKeysQueryLogs(r, "", "ScopeUpdated", "", actorOmni, 100)
	audits := agentKeysQueryLogs(r, "", "AuditAppended", "", actorOmni, 100)
	k3 := agentKeysQueryLogs(r, "", "K3Rotated", "", "", 1)
	currentEpoch := agentkeys.CurrentK3Epoch
	if len(k3) > 0 && len(k3[0].Topics) > 1 {
		if epoch := topicUint64(k3[0].Topics[1]); epoch > 0 {
			currentEpoch = epoch
		}
	}

	deviceItems := make([]interface{}, 0, len(devices))
	for _, item := range devices {
		deviceItems = append(deviceItems, item)
	}

	toJson(w, 0, agentKeysActorSummary{
		ActorOmni:         actorOmni,
		DevicesRegistered: len(devices),
		ScopeGrants:       len(scopes),
		AuditEntries:      len(audits),
		CurrentK3Epoch:    currentEpoch,
		Devices:           deviceItems,
		Scopes:            scopes,
		Audits:            audits,
		K3Rotations:       k3,
		Contracts:         agentkeys.Contracts(),
		EventKeywords:     agentkeys.Events(),
	}, nil)
	return nil
}

func agentKeysSearchHandle(w http.ResponseWriter, r *http.Request) error {
	p := new(agentKeysSearchParams)
	if err := validator.Validate(r.Body, p); err != nil {
		toJson(w, 10001, nil, err)
		return nil
	}
	query := strings.TrimSpace(p.Query)
	if contract, ok := agentkeys.ContractByAddress(query); ok {
		toJson(w, 0, map[string]string{
			"type":     "agentkeys_contract",
			"route":    "/contract/" + contract.Address,
			"address":  contract.Address,
			"name":     contract.Name,
			"chain_id": strconv.Itoa(agentkeys.ChainID),
		}, nil)
		return nil
	}
	if ev, ok := agentkeys.EventByName(query); ok {
		toJson(w, 0, map[string]string{
			"type":      "agentkeys_event",
			"route":     "/contract/" + ev.Address + "/events?topic0=" + ev.Topic0,
			"address":   ev.Address,
			"event":     ev.Name,
			"signature": ev.Signature,
			"topic0":    ev.Topic0,
		}, nil)
		return nil
	}
	if strings.EqualFold(query, agentkeys.BootstrapTx) {
		toJson(w, 0, map[string]string{
			"type":  "evm_transaction",
			"route": "/evm/transaction/" + strings.ToLower(query),
			"hash":  strings.ToLower(query),
		}, nil)
		return nil
	}
	if strings.HasPrefix(strings.ToLower(query), "actor_omni:") {
		actorOmni, ok := agentkeys.NormalizeBytes32(strings.TrimPrefix(strings.ToLower(query), "actor_omni:"))
		if ok {
			toJson(w, 0, map[string]string{
				"type":       "agentkeys_actor",
				"route":      "/agentkeys/actor/" + actorOmni,
				"actor_omni": actorOmni,
			}, nil)
			return nil
		}
	}
	toJson(w, 10002, nil, fmt.Errorf("agentkeys search result not found"))
	return nil
}

func agentKeysQueryLogs(r *http.Request, contractAddress, keyword, topic0, actorOmni string, limit int) []agentKeysEventLog {
	var opts []model.Option
	if contractAddress != "" {
		if c, ok := agentkeys.ContractByAddress(contractAddress); ok {
			opts = append(opts, model.Where("address = ?", c.Address))
		}
	}
	if keyword != "" {
		if ev, ok := agentkeys.EventByName(keyword); ok {
			topic0 = ev.Topic0
			opts = append(opts, model.Where("address = ?", ev.Address))
		}
	}
	if topic0 != "" {
		opts = append(opts, model.Where("method_hash = ?", strings.ToLower(util.AddHex(topic0))))
	}
	if actorOmni != "" {
		actorOmni, _ = agentkeys.NormalizeBytes32(actorOmni)
		opts = append(opts, model.Where("topic1 = ? or topic2 = ? or topic3 = ?", actorOmni, actorOmni, actorOmni))
	}
	if limit > 0 {
		opts = append(opts, model.WithLimit(0, limit))
	}

	indexed := decorateAgentKeysLogs(srv.API_GetLogs(r.Context(), opts...))
	if len(indexed) > 0 {
		return indexed
	}
	if bootstrapMatches(contractAddress, keyword, topic0, actorOmni) {
		return []agentKeysEventLog{bootstrapDeviceLog()}
	}
	return nil
}

func decorateAgentKeysLogs(logs []dao.EtherscanLogsRes) []agentKeysEventLog {
	out := make([]agentKeysEventLog, 0, len(logs))
	for _, log := range logs {
		if len(log.Topics) == 0 {
			continue
		}
		ev, ok := agentkeys.EventByTopic(log.Topics[0])
		if !ok {
			continue
		}
		out = append(out, agentKeysEventLog{
			EtherscanLogsRes: log,
			ContractName:     ev.ContractName,
			EventName:        ev.Name,
			EventSignature:   ev.Signature,
			Topic0:           ev.Topic0,
			Decoded:          decodeIndexedAgentKeysLog(ev.Name, log),
			Source:           "indexed_db",
		})
	}
	return out
}

func bootstrapMatches(contractAddress, keyword, topic0, actorOmni string) bool {
	if contractAddress != "" {
		c, ok := agentkeys.ContractByAddress(contractAddress)
		if !ok || c.Name != "SidecarRegistry" {
			return false
		}
	}
	if keyword != "" && !strings.EqualFold(keyword, "DeviceRegistered") && !strings.EqualFold(keyword, "SidecarRegistry.DeviceRegistered") {
		return false
	}
	if topic0 != "" {
		ev, _ := agentkeys.EventByName("DeviceRegistered")
		if !strings.EqualFold(util.AddHex(topic0), ev.Topic0) {
			return false
		}
	}
	if actorOmni != "" {
		actorOmni, _ = agentkeys.NormalizeBytes32(actorOmni)
		if actorOmni != agentkeys.LiveActorOmni {
			return false
		}
	}
	return true
}

func bootstrapDeviceLog() agentKeysEventLog {
	bootstrap := agentkeys.BootstrapDeviceRegistered()
	return agentKeysEventLog{
		EtherscanLogsRes: dao.EtherscanLogsRes{
			Address:          bootstrap.Address,
			Topics:           bootstrap.Topics,
			Data:             bootstrap.Data,
			BlockNumber:      util.IntToHexNumber(bootstrap.BlockNumber),
			LogIndex:         strconv.FormatUint(bootstrap.LogIndex, 10),
			TransactionHash:  bootstrap.TransactionHash,
			TransactionIndex: strconv.FormatUint(bootstrap.TransactionIndex, 10),
		},
		ContractName:   bootstrap.ContractName,
		EventName:      bootstrap.EventName,
		EventSignature: bootstrap.EventSignature,
		Topic0:         bootstrap.Topic0,
		Decoded:        bootstrap.Decoded,
		Source:         bootstrap.Source,
	}
}

func decodeIndexedAgentKeysLog(eventName string, log dao.EtherscanLogsRes) map[string]interface{} {
	if len(log.Topics) == 0 {
		return nil
	}
	switch eventName {
	case "DeviceRegistered":
		if len(log.Topics) < 4 {
			return nil
		}
		return map[string]interface{}{
			"deviceKeyHash": log.Topics[1],
			"operatorOmni":  log.Topics[2],
			"actorOmni":     log.Topics[3],
		}
	case "ScopeUpdated":
		if len(log.Topics) < 3 {
			return nil
		}
		return map[string]interface{}{"operatorOmni": log.Topics[1], "agentOmni": log.Topics[2]}
	case "AuditAppended":
		if len(log.Topics) < 4 {
			return nil
		}
		return map[string]interface{}{"operatorOmni": log.Topics[1], "actorOmni": log.Topics[2], "serviceHash": log.Topics[3]}
	case "K3Rotated":
		if len(log.Topics) < 2 {
			return nil
		}
		return map[string]interface{}{"newEpoch": topicUint64(log.Topics[1])}
	}
	return nil
}

func topicUint64(topic string) uint64 {
	topic = strings.TrimPrefix(topic, "0x")
	if len(topic) > 16 {
		topic = topic[len(topic)-16:]
	}
	v, _ := strconv.ParseUint(topic, 16, 64)
	return v
}

func agentKeysIsKnownAddress(q string) bool {
	return address.VerifyEthereumAddress(q) && func() bool {
		_, ok := agentkeys.ContractByAddress(q)
		return ok
	}()
}
