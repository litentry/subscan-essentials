package agentkeys

import (
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/itering/subscan/util"
	"github.com/itering/subscan/util/address"
)

const (
	ChainID        = 212013
	RPCURL         = "https://rpc.heima-parachain.heima.network"
	DeployDate     = "2026-05-19"
	Compiler       = "solc 0.8.20"
	EVMVersion     = "london"
	BootstrapBlock = uint64(9620483)
	BootstrapTx    = "0x8f1d7cca5710c2859b4f8b942c36df41d3c6b8b02a862d1f506285a6176c988b"
	LiveActorOmni  = "0x941cb1c3260518bbf40eac7d02663517fc7cff304d9b03e80d2cc54126c6bef2"
	CurrentK3Epoch = uint64(1)
)

type Contract struct {
	Name           string   `json:"name"`
	Address        string   `json:"address"`
	ChainID        int      `json:"chain_id"`
	BytecodeSize   int      `json:"bytecode_size"`
	DeployDate     string   `json:"deploy_date"`
	Compiler       string   `json:"compiler"`
	EVMVersion     string   `json:"evm_version"`
	ReadFunctions  []string `json:"read_functions"`
	WriteFunctions []string `json:"write_functions"`
}

type Event struct {
	ContractName string `json:"contract_name"`
	Address      string `json:"address"`
	Name         string `json:"name"`
	Signature    string `json:"signature"`
	Topic0       string `json:"topic0"`
}

type BootstrapEvent struct {
	Address          string                 `json:"address"`
	ContractName     string                 `json:"contract_name"`
	EventName        string                 `json:"event_name"`
	EventSignature   string                 `json:"event_signature"`
	Topic0           string                 `json:"topic0"`
	Topics           []string               `json:"topics"`
	Data             string                 `json:"data"`
	BlockNumber      uint64                 `json:"block_number"`
	TransactionHash  string                 `json:"transaction_hash"`
	TransactionIndex uint64                 `json:"transaction_index"`
	LogIndex         uint64                 `json:"log_index"`
	Decoded          map[string]interface{} `json:"decoded"`
	Source           string                 `json:"source"`
}

var contracts = []Contract{
	{
		Name:         "AgentKeysScope",
		Address:      "0x14c23b5d1ce20c094af643a20e6b0972dad12aa8",
		ChainID:      ChainID,
		BytecodeSize: 3146,
		DeployDate:   DeployDate,
		Compiler:     Compiler,
		EVMVersion:   EVMVersion,
		ReadFunctions: []string{
			"registry()",
			"getScope(bytes32,bytes32)",
			"isServiceInScope(bytes32,bytes32,bytes32)",
		},
		WriteFunctions: []string{
			"setScopeWithWebauthn(bytes32,bytes32,bytes32[],bool,uint128,uint128,uint128,uint32,bytes)",
			"revokeScope(bytes32,bytes32,bytes)",
		},
	},
	{
		Name:         "SidecarRegistry",
		Address:      "0x76d574a107727be87fc1422661a030fefda70786",
		ChainID:      ChainID,
		BytecodeSize: 3301,
		DeployDate:   DeployDate,
		Compiler:     Compiler,
		EVMVersion:   EVMVersion,
		ReadFunctions: []string{
			"ROLE_CAP_MINT()",
			"ROLE_RECOVERY()",
			"ROLE_SCOPE_MGMT()",
			"TIER_MASTER()",
			"TIER_AGENT()",
			"devices(bytes32)",
			"getDevice(bytes32)",
			"getOperatorDevices(bytes32)",
			"isActive(bytes32)",
			"operatorMasterWallet(bytes32)",
		},
		WriteFunctions: []string{
			"registerMasterDevice(bytes32,bytes32,bytes32,bytes32,bytes,uint8,bytes)",
			"registerAgentDevice(bytes32,bytes32,bytes32,bytes,bytes)",
			"revokeDevice(bytes32,bytes)",
		},
	},
	{
		Name:         "K3EpochCounter",
		Address:      "0x8396dec50ff755d6de7728dabb00be2efbcdf4df",
		ChainID:      ChainID,
		BytecodeSize: 687,
		DeployDate:   DeployDate,
		Compiler:     Compiler,
		EVMVersion:   EVMVersion,
		ReadFunctions: []string{
			"currentEpoch()",
			"signerGovernance()",
			"epochStartedAt(uint256)",
		},
		WriteFunctions: []string{
			"advanceEpoch()",
			"setSignerGovernance(address)",
		},
	},
	{
		Name:         "CredentialAudit",
		Address:      "0x1801ded1a4fbd8c9224ab18b9ecbb293b8674c06",
		ChainID:      ChainID,
		BytecodeSize: 1421,
		DeployDate:   DeployDate,
		Compiler:     Compiler,
		EVMVersion:   EVMVersion,
		ReadFunctions: []string{
			"OP_STORE()",
			"OP_READ()",
			"OP_TEARDOWN()",
			"getEntries(bytes32,uint256,uint256)",
			"entryCount(bytes32)",
		},
		WriteFunctions: []string{
			"append(bytes32,bytes32,bytes32,uint8,bytes32)",
		},
	},
}

var events = []Event{
	{ContractName: "AgentKeysScope", Address: contracts[0].Address, Name: "ScopeUpdated", Signature: "ScopeUpdated(bytes32,bytes32,bytes32[],bool,uint128,uint128,uint128,uint32)"},
	{ContractName: "AgentKeysScope", Address: contracts[0].Address, Name: "ScopeRevoked", Signature: "ScopeRevoked(bytes32,bytes32)"},
	{ContractName: "SidecarRegistry", Address: contracts[1].Address, Name: "DeviceRegistered", Signature: "DeviceRegistered(bytes32,bytes32,bytes32,uint8,uint8,bytes32)"},
	{ContractName: "SidecarRegistry", Address: contracts[1].Address, Name: "DeviceRevoked", Signature: "DeviceRevoked(bytes32,bytes32)"},
	{ContractName: "SidecarRegistry", Address: contracts[1].Address, Name: "OperatorBootstrapped", Signature: "OperatorBootstrapped(bytes32,address)"},
	{ContractName: "K3EpochCounter", Address: contracts[2].Address, Name: "K3Rotated", Signature: "K3Rotated(uint256,uint256)"},
	{ContractName: "K3EpochCounter", Address: contracts[2].Address, Name: "SignerGovernanceTransferred", Signature: "SignerGovernanceTransferred(address,address)"},
	{ContractName: "CredentialAudit", Address: contracts[3].Address, Name: "AuditAppended", Signature: "AuditAppended(bytes32,bytes32,bytes32,uint8,uint256,bytes32)"},
}

func init() {
	for i := range events {
		events[i].Topic0 = EventTopic(events[i].Signature)
	}
}

func EventTopic(signature string) string {
	return crypto.Keccak256Hash([]byte(signature)).Hex()
}

func Contracts() []Contract {
	out := make([]Contract, len(contracts))
	copy(out, contracts)
	return out
}

func Events() []Event {
	out := make([]Event, len(events))
	copy(out, events)
	return out
}

func ContractByAddress(addr string) (Contract, bool) {
	if !address.VerifyEthereumAddress(addr) {
		return Contract{}, false
	}
	addr = strings.ToLower(util.AddHex(addr))
	for _, c := range contracts {
		if c.Address == addr {
			return c, true
		}
	}
	return Contract{}, false
}

func EventByName(name string) (Event, bool) {
	for _, e := range events {
		if strings.EqualFold(e.Name, name) || strings.EqualFold(e.ContractName+"."+e.Name, name) {
			return e, true
		}
	}
	return Event{}, false
}

func EventByTopic(topic0 string) (Event, bool) {
	topic0 = strings.ToLower(util.AddHex(topic0))
	for _, e := range events {
		if strings.ToLower(e.Topic0) == topic0 {
			return e, true
		}
	}
	return Event{}, false
}

func EventsByAddress(addr string) []Event {
	c, ok := ContractByAddress(addr)
	if !ok {
		return nil
	}
	var out []Event
	for _, e := range events {
		if e.Address == c.Address {
			out = append(out, e)
		}
	}
	return out
}

func NormalizeBytes32(v string) (string, bool) {
	v = strings.ToLower(util.AddHex(v))
	if len(v) != 66 {
		return "", false
	}
	for _, r := range strings.TrimPrefix(v, "0x") {
		if (r < '0' || r > '9') && (r < 'a' || r > 'f') {
			return "", false
		}
	}
	return v, true
}

func BootstrapDeviceRegistered() BootstrapEvent {
	ev, _ := EventByName("DeviceRegistered")
	deviceKeyHash := "0x9b78c2e7380f23fd602a759f1de316f07e7705e5e279e211ef5036d7215a3260"
	return BootstrapEvent{
		Address:        contracts[1].Address,
		ContractName:   "SidecarRegistry",
		EventName:      "DeviceRegistered",
		EventSignature: ev.Signature,
		Topic0:         ev.Topic0,
		Topics: []string{
			ev.Topic0,
			deviceKeyHash,
			LiveActorOmni,
			LiveActorOmni,
		},
		Data:             "0x000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000070000000000000000000000000000000000000000000000000000000000000000",
		BlockNumber:      BootstrapBlock,
		TransactionHash:  BootstrapTx,
		TransactionIndex: 0,
		LogIndex:         1,
		Decoded: map[string]interface{}{
			"deviceKeyHash": deviceKeyHash,
			"operatorOmni":  LiveActorOmni,
			"actorOmni":     LiveActorOmni,
			"tier":          1,
			"roles":         7,
			"k11CredId":     "0x0000000000000000000000000000000000000000000000000000000000000000",
		},
		Source: "heima_mainnet_bootstrap_receipt",
	}
}
