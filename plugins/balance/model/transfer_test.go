package model

import (
	"encoding/json"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransferJSONIncludesSourceMetadata(t *testing.T) {
	transfer := Transfer{
		Id:             971637600003,
		Sender:         "omnibridge",
		Receiver:       "00f160c0e8fff2d4f00ab03e18dced9f2ac52a6b865cda497a33aee5b3fe335b",
		Amount:         decimal.RequireFromString("10000000000000000000"),
		Category:       "bridge_in",
		SourceModule:   "omnibridge",
		SourceEvent:    "PaidOut",
		BalanceEvent:   "Minted",
		ExtrinsicIndex: "9716376-2",
	}

	raw, err := json.Marshal(transfer)
	require.NoError(t, err)

	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "bridge_in", got["category"])
	assert.Equal(t, "omnibridge", got["source_module"])
	assert.Equal(t, "PaidOut", got["source_event"])
	assert.Equal(t, "Minted", got["balance_event"])
}
