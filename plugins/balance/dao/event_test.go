package dao

import (
	"encoding/json"
	"testing"

	"github.com/itering/subscan-plugin/storage"
	"github.com/itering/subscan/share/token"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const bridgeReceiver = "00f160c0e8fff2d4f00ab03e18dced9f2ac52a6b865cda497a33aee5b3fe335b"

func TestBalanceTransferFromEventMarksNormalTransferMetadata(t *testing.T) {
	token.SetDefault(&token.Token{Symbol: "HEI", TokenId: "HEI"})
	event := eventWithParams(100000000001, 1000000, 0, TransferSourceBalances, TransferEventTransfer, []storage.EventParam{
		{Type: "AccountId", Value: "242f0781faa44f34ddcbc9e731d0ddb51c97f5b58bb2202090a3a1c679fc4c63"},
		{Type: "AccountId", Value: bridgeReceiver},
		{Type: "Balance", Value: "12345"},
	})

	transfer := BalanceTransferFromEvent(&event, &storage.Block{BlockTimestamp: 1770000000})

	require.NotNil(t, transfer)
	assert.Equal(t, TransferCategoryTransfer, transfer.Category)
	assert.Equal(t, TransferSourceBalances, transfer.SourceModule)
	assert.Equal(t, TransferEventTransfer, transfer.SourceEvent)
	assert.Equal(t, TransferEventTransfer, transfer.BalanceEvent)
	assert.True(t, decimal.RequireFromString("12345").Equal(transfer.Amount))
}

func TestOmniBridgePayoutTransfersCreatesBridgeInFromPaidOutAndMinted(t *testing.T) {
	token.SetDefault(&token.Token{Symbol: "HEI", TokenId: "HEI"})
	events := []storage.Event{
		eventWithParams(971637600004, 9716376, 2, TransferSourceOmniBridge, TransferEventPaidOut, nil),
		eventWithParams(971637600003, 9716376, 2, TransferSourceBalances, TransferEventMinted, []storage.EventParam{
			{Type: "AccountId", Value: bridgeReceiver},
			{Type: "Balance", Value: "0000E8890423C78A0000000000000000"},
		}),
	}

	transfers := OmniBridgePayoutTransfers(events, &storage.Block{BlockTimestamp: 1780000000})

	require.Len(t, transfers, 1)
	assert.Equal(t, uint(971637600003), transfers[0].Id)
	assert.Equal(t, OmniBridgeSyntheticSender, transfers[0].Sender)
	assert.Equal(t, bridgeReceiver, transfers[0].Receiver)
	assert.True(t, decimal.RequireFromString("10000000000000000000").Equal(transfers[0].Amount))
	assert.Equal(t, uint(9716376), transfers[0].BlockNum)
	assert.Equal(t, int64(1780000000), transfers[0].BlockTimestamp)
	assert.Equal(t, "9716376-2", transfers[0].ExtrinsicIndex)
	assert.Equal(t, TransferCategoryBridgeIn, transfers[0].Category)
	assert.Equal(t, TransferSourceOmniBridge, transfers[0].SourceModule)
	assert.Equal(t, TransferEventPaidOut, transfers[0].SourceEvent)
	assert.Equal(t, TransferEventMinted, transfers[0].BalanceEvent)
}

func TestOmniBridgePayoutTransfersIgnoresUnrelatedMinted(t *testing.T) {
	token.SetDefault(&token.Token{Symbol: "HEI", TokenId: "HEI"})
	events := []storage.Event{
		eventWithParams(971637600003, 9716376, 2, TransferSourceBalances, TransferEventMinted, []storage.EventParam{
			{Type: "AccountId", Value: bridgeReceiver},
			{Type: "Balance", Value: "10000000000000000000"},
		}),
	}

	assert.Empty(t, OmniBridgePayoutTransfers(events, nil))
}

func TestOmniBridgePayoutTransfersIsStableForDuplicateReprocessing(t *testing.T) {
	token.SetDefault(&token.Token{Symbol: "HEI", TokenId: "HEI"})
	events := []storage.Event{
		eventWithParams(971637600004, 9716376, 2, "OmniBridge", TransferEventPaidOut, nil),
		eventWithParams(971637600003, 9716376, 2, "Balances", TransferEventMinted, []storage.EventParam{
			{Type: "AccountId", Value: bridgeReceiver},
			{Type: "Balance", Value: "10000000000000000000"},
		}),
	}

	first := OmniBridgePayoutTransfers(events, nil)
	second := OmniBridgePayoutTransfers(events, nil)

	require.Len(t, first, 1)
	require.Len(t, second, 1)
	assert.Equal(t, first[0].Id, second[0].Id)
	assert.Equal(t, uint(971637600003), second[0].Id)
	assert.True(t, first[0].Amount.Equal(second[0].Amount))
}

func eventWithParams(id uint, blockNum int, extrinsicIdx int, moduleID string, eventID string, params []storage.EventParam) storage.Event {
	raw, _ := json.Marshal(params)
	return storage.Event{
		Id:           id,
		BlockNum:     blockNum,
		ExtrinsicIdx: extrinsicIdx,
		ModuleId:     moduleID,
		EventId:      eventID,
		Params:       raw,
	}
}
