package dao

import (
	"context"
	"fmt"
	"strings"

	subscan_plugin "github.com/itering/subscan-plugin"
	"github.com/itering/subscan-plugin/storage"
	"github.com/itering/subscan/model"
	bModel "github.com/itering/subscan/plugins/balance/model"
	"github.com/itering/subscan/share/token"
	"github.com/itering/subscan/util"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type Storage struct {
	Dao  storage.Dao
	Pool subscan_plugin.RedisPool
}

const (
	TransferCategoryTransfer = "transfer"
	TransferCategoryBridgeIn = "bridge_in"

	TransferSourceBalances   = "balances"
	TransferSourceOmniBridge = "omnibridge"

	TransferEventTransfer = "Transfer"
	TransferEventMinted   = "Minted"
	TransferEventPaidOut  = "PaidOut"

	// OmniBridge payout rows use a stable synthetic sender because the chain
	// emits incoming funds as balances.Minted without a source account.
	OmniBridgeSyntheticSender = "omnibridge"
)

func EmitEvent(ctx context.Context, d *Storage, event *storage.Event, block *storage.Block) error {
	var paramEvent []storage.EventParam
	_ = util.UnmarshalAny(&paramEvent, event.Params)
	switch event.EventId {
	// [accountId, balance]
	case "Endowed", "Reserved", "Unreserved", "Deposit", "Minted", "Issued", "Locked", "Unlocked", "Withdraw":
		return RefreshAccount(ctx, d, model.CheckoutParamValueAddress(paramEvent[0].Value))
		// ["AccountId","AccountId","Balance"]
	case "Transfer":
		transfer := BalanceTransferFromEvent(event, block)
		if transfer == nil {
			return nil
		}
		return CreateTransfer(ctx, d, transfer)
	}
	return nil
}

func BalanceTransferFromEvent(event *storage.Event, block *storage.Block) *bModel.Transfer {
	if event == nil || !strings.EqualFold(event.ModuleId, TransferSourceBalances) || !strings.EqualFold(event.EventId, TransferEventTransfer) {
		return nil
	}
	var paramEvent []storage.EventParam
	_ = util.UnmarshalAny(&paramEvent, event.Params)
	if len(paramEvent) < 3 {
		return nil
	}
	t := token.GetDefaultToken()
	blockTimestamp := int64(0)
	if block != nil {
		blockTimestamp = int64(block.BlockTimestamp)
	}
	return &bModel.Transfer{
		Id:             event.Id,
		Sender:         model.CheckoutParamValueAddress(paramEvent[0].Value),
		Receiver:       model.CheckoutParamValueAddress(paramEvent[1].Value),
		Amount:         util.DecimalFromInterface(paramEvent[2].Value),
		BlockNum:       uint(event.BlockNum),
		BlockTimestamp: blockTimestamp,
		Symbol:         t.Symbol,
		TokenId:        t.TokenId,
		ExtrinsicIndex: fmt.Sprintf("%d-%d", event.BlockNum, event.ExtrinsicIdx),
		Category:       TransferCategoryTransfer,
		SourceModule:   TransferSourceBalances,
		SourceEvent:    TransferEventTransfer,
		BalanceEvent:   TransferEventTransfer,
	}
}

func CreateOmniBridgePayoutTransfers(ctx context.Context, d *Storage, events []storage.Event, block *storage.Block) error {
	_, err := CreateOmniBridgePayoutTransfersWithResult(ctx, d, events, block)
	return err
}

func CreateOmniBridgePayoutTransfersWithResult(ctx context.Context, d *Storage, events []storage.Event, block *storage.Block) (int, error) {
	var inserted int
	for _, transfer := range OmniBridgePayoutTransfers(events, block) {
		created, err := CreateTransferIfMissing(ctx, d, transfer)
		if err != nil {
			return inserted, err
		}
		if created {
			inserted++
		}
	}
	return inserted, nil
}

func OmniBridgePayoutTransfers(events []storage.Event, block *storage.Block) []*bModel.Transfer {
	if !hasOmniBridgePaidOut(events) {
		return nil
	}
	t := token.GetDefaultToken()
	blockTimestamp := int64(0)
	if block != nil {
		blockTimestamp = int64(block.BlockTimestamp)
	}
	var transfers []*bModel.Transfer
	for index := range events {
		event := events[index]
		if !strings.EqualFold(event.ModuleId, TransferSourceBalances) || !strings.EqualFold(event.EventId, TransferEventMinted) {
			continue
		}
		var paramEvent []storage.EventParam
		_ = util.UnmarshalAny(&paramEvent, event.Params)
		if len(paramEvent) < 2 {
			continue
		}
		receiver := model.CheckoutParamValueAddress(paramEvent[0].Value)
		if receiver == "" {
			continue
		}
		transfers = append(transfers, &bModel.Transfer{
			Id:             event.Id,
			Sender:         OmniBridgeSyntheticSender,
			Receiver:       receiver,
			Amount:         balanceAmountFromEventParam(paramEvent[1].Value),
			BlockNum:       uint(event.BlockNum),
			BlockTimestamp: blockTimestamp,
			Symbol:         t.Symbol,
			TokenId:        t.TokenId,
			ExtrinsicIndex: fmt.Sprintf("%d-%d", event.BlockNum, event.ExtrinsicIdx),
			Category:       TransferCategoryBridgeIn,
			SourceModule:   TransferSourceOmniBridge,
			SourceEvent:    TransferEventPaidOut,
			BalanceEvent:   TransferEventMinted,
		})
	}
	return transfers
}

func hasOmniBridgePaidOut(events []storage.Event) bool {
	for index := range events {
		event := events[index]
		if strings.EqualFold(event.ModuleId, TransferSourceOmniBridge) && strings.EqualFold(event.EventId, TransferEventPaidOut) {
			return true
		}
	}
	return false
}

func balanceAmountFromEventParam(value interface{}) decimal.Decimal {
	amount := util.DecimalFromInterface(value)
	if !amount.IsZero() {
		return amount
	}
	valueString := strings.TrimSpace(util.ToString(value))
	trimmed := strings.TrimPrefix(valueString, "0x")
	if valueString != "" && len(trimmed)%2 == 0 && len(trimmed) >= 32 {
		return util.EvmReverseU256Decoder(valueString)
	}
	return amount
}

func RefreshMetadata(ctx context.Context, d *Storage) {
	// account
	var count int64
	db := d.Dao.GetDbInstance().(*gorm.DB)
	_ = db.Model(&bModel.Account{}).Count(&count)
	var transferCount int64
	_ = db.Model(&bModel.Transfer{}).Count(&transferCount)
	_ = d.Pool.HmSetEx(ctx, model.MetadataCacheKey(), map[string]int{"total_transfer": int(transferCount), "total_account": int(count)}, -1)
}
