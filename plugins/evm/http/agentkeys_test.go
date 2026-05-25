package http

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/itering/subscan/plugins/evm/agentkeys"
	"github.com/itering/subscan/plugins/evm/dao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	liveDeviceRegisteredTx = "0x8f1d7cca5710c2859b4f8b942c36df41d3c6b8b02a862d1f506285a6176c988b"
	liveActorOmni          = "0x941cb1c3260518bbf40eac7d02663517fc7cff304d9b03e80d2cc54126c6bef2"
)

func TestAgentKeysContractsHandle(t *testing.T) {
	req, err := http.NewRequest(http.MethodPost, "/agentkeys/contracts", nil)
	require.NoError(t, err)

	rr := httptest.NewRecorder()
	require.NoError(t, agentKeysContractsHandle(rr, req))

	body := rr.Body.String()
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, body, `"chain_id":212013`)
	assert.Contains(t, body, `"name":"AgentKeysScope"`)
	assert.Contains(t, body, `"bytecode_size":3146`)
	assert.Contains(t, body, `"name":"DeviceRegistered"`)
}

func TestAgentKeysSearchHandle(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "contract address routes to contract overview",
			body: `{"query":"0x14C23B5D1cE20c094af643a20e6b0972dAD12aa8"}`,
			want: `"route":"/contract/0x14c23b5d1ce20c094af643a20e6b0972dad12aa8"`,
		},
		{
			name: "event keyword routes to filtered events",
			body: `{"query":"DeviceRegistered"}`,
			want: `"event":"DeviceRegistered"`,
		},
		{
			name: "indexed agentkeys tx routes to evm transaction",
			body: `{"query":"` + liveDeviceRegisteredTx + `"}`,
			want: `"route":"/tx/` + liveDeviceRegisteredTx + `"`,
		},
		{
			name: "actor filter routes to actor view",
			body: `{"query":"actor_omni:941cb1c3260518bbf40eac7d02663517fc7cff304d9b03e80d2cc54126c6bef2"}`,
			want: `"route":"/agentkeys/actor/0x941cb1c3260518bbf40eac7d02663517fc7cff304d9b03e80d2cc54126c6bef2"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodPost, "/agentkeys/search", strings.NewReader(tt.body))
			require.NoError(t, err)
			rr := httptest.NewRecorder()

			require.NoError(t, agentKeysSearchHandle(rr, req))

			assert.Equal(t, http.StatusOK, rr.Code)
			assert.Contains(t, rr.Body.String(), tt.want)
		})
	}
}

func TestAgentKeysActorHandleUsesIndexedLogsOnly(t *testing.T) {
	req, err := http.NewRequest(
		http.MethodPost,
		"/agentkeys/actor",
		strings.NewReader(`{"actor_omni":"0x941cb1c3260518bbf40eac7d02663517fc7cff304d9b03e80d2cc54126c6bef2"}`),
	)
	require.NoError(t, err)
	rr := httptest.NewRecorder()

	require.NoError(t, agentKeysActorHandle(rr, req))

	body := rr.Body.String()
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, body, `"devices_registered":0`)
	assert.Contains(t, body, `"scope_grants":0`)
	assert.Contains(t, body, `"audit_entries":0`)
	assert.Contains(t, body, `"current_k3_epoch":0`)
	assert.NotContains(t, body, `"source":"heima_mainnet_bootstrap_receipt"`)
}

func TestDecorateAgentKeysLogsDecodesIndexedDeviceRegistered(t *testing.T) {
	device, ok := agentkeys.EventByName("DeviceRegistered")
	require.True(t, ok)

	logs := decorateAgentKeysLogs([]dao.EtherscanLogsRes{
		{
			Address:          device.Address,
			Topics:           []string{device.Topic0, "0x9b78c2e7380f23fd602a759f1de316f07e7705e5e279e211ef5036d7215a3260", liveActorOmni, liveActorOmni},
			Data:             "0x",
			BlockNumber:      "0x92cc43",
			LogIndex:         "0x1",
			TransactionHash:  liveDeviceRegisteredTx,
			TransactionIndex: "0x0",
		},
	})

	require.Len(t, logs, 1)
	assert.Equal(t, "indexed_db", logs[0].Source)
	assert.Equal(t, "SidecarRegistry", logs[0].ContractName)
	assert.Equal(t, "DeviceRegistered", logs[0].EventName)
	assert.Equal(t, liveActorOmni, logs[0].Decoded["actorOmni"])
}
