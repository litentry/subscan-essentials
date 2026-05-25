package agentkeys

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContractRegistry(t *testing.T) {
	contracts := Contracts()
	require.Len(t, contracts, 4)

	scope, ok := ContractByAddress("0x14C23B5D1cE20c094af643a20e6b0972dAD12aa8")
	require.True(t, ok)
	assert.Equal(t, "AgentKeysScope", scope.Name)
	assert.Equal(t, 3146, scope.BytecodeSize)

	sidecar, ok := ContractByAddress("0x76D574a107727bE87fc1422661A030FEFda70786")
	require.True(t, ok)
	assert.Equal(t, "SidecarRegistry", sidecar.Name)
	assert.Contains(t, sidecar.ReadFunctions, "isActive(bytes32)")
	assert.Contains(t, sidecar.WriteFunctions, "registerMasterDevice(bytes32,bytes32,bytes32,bytes32,bytes,uint8,bytes)")
}

func TestEventKeywords(t *testing.T) {
	device, ok := EventByName("DeviceRegistered")
	require.True(t, ok)
	assert.Equal(t, "SidecarRegistry", device.ContractName)
	assert.Equal(t, "0xc90d996f0da43d3260ad31f4ee28d42367086761020f318df849816382f5b9ce", device.Topic0)

	qualified, ok := EventByName("SidecarRegistry.DeviceRegistered")
	require.True(t, ok)
	assert.Equal(t, device.Topic0, qualified.Topic0)

	audit, ok := EventByName("AuditAppended")
	require.True(t, ok)
	assert.Equal(t, EventTopic(audit.Signature), audit.Topic0)
}
