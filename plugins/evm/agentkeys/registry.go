package agentkeys

import (
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/itering/subscan/util"
	"github.com/itering/subscan/util/address"
)

const (
	ChainID    = 212013
	RPCURL     = "https://rpc.heima-parachain.heima.network"
	DeployDate = "2026-05-19"
	Compiler   = "solc 0.8.20"
	EVMVersion = "london"
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
