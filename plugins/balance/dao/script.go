package dao

import (
	"context"
	"fmt"
	"github.com/itering/subscan-plugin/storage"
	"github.com/itering/subscan/model"
	bModel "github.com/itering/subscan/plugins/balance/model"
	"github.com/itering/subscan/share/substrate"
	"github.com/itering/subscan/util"
	"github.com/itering/subscan/util/address"
	"github.com/panjf2000/ants/v2"
	"gorm.io/gorm"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

func InitAccount(sg *Storage) {
	ctx := context.Background()
	locksByAddress := readAllBalanceLocks(ctx)
	vestingByAddress := readAllVesting(ctx)
	currentBlock, _ := sg.Dao.GetCurrentBlockNum(ctx)
	wg := new(sync.WaitGroup)
	bp, _ := ants.NewPoolWithFunc(10, func(i interface{}) {
		wg.Add(1)
		defer wg.Done()
		params := i.([]interface{})
		addr := params[0].(string)
		info := params[1].(*bModel.AccountData)
		lockSummary := bModel.AccountLockSummary(info, locksByAddress[addr])
		vested := bModel.SummarizeVesting(vestingByAddress[addr], currentBlock)
		sg.AddOrUpdateItem(ctx, &bModel.Account{
			Address:  addr,
			Nonce:    info.Nonce,
			Balance:  info.Data.Free.Add(info.Data.Reserved),
			Locked:   lockSummary.Locked,
			Reserved: info.Data.Reserved,
			Vested:   vested,
		}, []string{"address"}, "nonce", "balance", "locked", "reserved", "vested")
	})
	defer bp.Release()

	// refresh account balance
	if err := substrate.BatchReadKeysPaged(ctx, "System", "Account", "", func(keys []string, scaleType string) error {
		r, _ := substrate.BatchStorageByKey(ctx, keys, scaleType, "")
		for key, v := range r {
			val, _ := substrate.ParseStorageKey(key)
			addr := address.Format(val[0].ToString())
			accountData := new(bModel.AccountData)
			v.ToAny(accountData)
			util.Logger().Error(bp.Invoke([]interface{}{addr, accountData}))
		}
		return nil
	}); err != nil {
		log.Panic(err)
	}
	wg.Wait()
}

func readAllBalanceLocks(ctx context.Context) map[string][]bModel.BalanceLock {
	result := map[string][]bModel.BalanceLock{}
	_ = substrate.BatchReadKeysPaged(ctx, "Balances", "Locks", "", func(keys []string, scaleType string) error {
		r, _ := substrate.BatchStorageByKey(ctx, keys, scaleType, "")
		for key, v := range r {
			val, err := substrate.ParseStorageKey(key)
			if err != nil || len(val) == 0 {
				continue
			}
			addr := address.Format(val[0].ToString())
			if addr == "" {
				continue
			}
			var locks []bModel.BalanceLock
			v.ToAny(&locks)
			result[addr] = locks
		}
		return nil
	})
	return result
}

func readAllVesting(ctx context.Context) map[string][]bModel.VestingInfo {
	result := map[string][]bModel.VestingInfo{}
	_ = substrate.BatchReadKeysPaged(ctx, "Vesting", "Vesting", "", func(keys []string, scaleType string) error {
		r, _ := substrate.BatchStorageByKey(ctx, keys, scaleType, "")
		for key, v := range r {
			val, err := substrate.ParseStorageKey(key)
			if err != nil || len(val) == 0 {
				continue
			}
			addr := address.Format(val[0].ToString())
			if addr == "" {
				continue
			}
			var vesting []bModel.VestingInfo
			v.ToAny(&vesting)
			result[addr] = vesting
		}
		return nil
	})
	return result
}

type RefreshAllAccountOptions struct {
	Limit        int
	StartID      uint
	SleepSeconds int
	Mode         string
	StartIDSet   bool
}

type refreshProgressCache interface {
	GetCacheString(context.Context, string) string
	SetCache(context.Context, string, interface{}, int) error
}

func RefreshAllAccount(sg *Storage, options ...RefreshAllAccountOptions) error {
	opt := RefreshAllAccountOptions{
		Limit:        10,
		SleepSeconds: 3,
	}
	if len(options) > 0 {
		opt = options[0]
	}
	if opt.Limit <= 0 {
		opt.Limit = 10
	}
	if opt.SleepSeconds < 0 {
		opt.SleepSeconds = 3
	}
	opt.Mode = strings.ToLower(strings.TrimSpace(opt.Mode))
	if opt.Mode == "" {
		opt.Mode = "resume"
	}

	ctx := context.Background()
	progressKey := model.RedisKeyPrefix() + "balance_refresh_all_account_progress"
	cache, cacheOK := sg.Pool.(refreshProgressCache)
	if opt.Mode == "reset" {
		opt.StartID = 0
		if cacheOK {
			_ = cache.SetCache(ctx, progressKey, 0, -1)
		}
		fmt.Printf("RefreshAllAccount: reset progress key=%s start_id=0\n", progressKey)
	} else if opt.Mode == "resume" && !opt.StartIDSet && cacheOK {
		if saved := cache.GetCacheString(ctx, progressKey); saved != "" {
			if savedID, err := strconv.ParseUint(saved, 10, 64); err == nil {
				opt.StartID = uint(savedID)
				fmt.Printf("RefreshAllAccount: resume from progress key=%s start_id=%d\n", progressKey, opt.StartID)
			}
		}
	} else if opt.Mode != "resume" && opt.Mode != "reset" {
		return fmt.Errorf("unsupported mode %q, use resume or reset", opt.Mode)
	}
	if !cacheOK {
		fmt.Printf("RefreshAllAccount: progress cache unavailable, using start_id=%d\n", opt.StartID)
	}

	db := sg.Dao.GetDbInstance().(*gorm.DB)
	var remaining int64
	if err := db.WithContext(ctx).Model(&bModel.Account{}).Where("id > ?", opt.StartID).Count(&remaining).Error; err != nil {
		return err
	}
	var accounts []bModel.Account
	if err := db.WithContext(ctx).
		Select("id", "address").
		Where("id > ?", opt.StartID).
		Order("id asc").
		Limit(opt.Limit).
		Find(&accounts).Error; err != nil {
		return err
	}
	if len(accounts) == 0 {
		fmt.Printf("RefreshAllAccount: no accounts found after id %d\n", opt.StartID)
		return nil
	}

	fmt.Printf("RefreshAllAccount: mode=%s start_id=%d limit=%d sleep_seconds=%d remaining_after_start=%d batch_size=%d progress_key=%s\n",
		opt.Mode, opt.StartID, opt.Limit, opt.SleepSeconds, remaining, len(accounts), progressKey)

	sleepDuration := time.Duration(opt.SleepSeconds) * time.Second
	var success, failed int
	for index, account := range accounts {
		if err := RefreshAccount(ctx, sg, account.Address); err != nil {
			failed++
			fmt.Printf("RefreshAllAccount: progress=%d/%d id=%d address=%s status=failed success=%d failed=%d err=%v\n",
				index+1, len(accounts), account.ID, account.Address, success, failed, err)
		} else {
			success++
			if cacheOK {
				_ = cache.SetCache(ctx, progressKey, int(account.ID), -1)
			}
			fmt.Printf("RefreshAllAccount: progress=%d/%d id=%d address=%s status=refreshed success=%d failed=%d next_start_id=%d\n",
				index+1, len(accounts), account.ID, account.Address, success, failed, account.ID)
		}
		if sleepDuration > 0 {
			fmt.Printf("RefreshAllAccount: sleeping %s before next account\n", sleepDuration)
			time.Sleep(sleepDuration)
		}
	}

	lastID := accounts[len(accounts)-1].ID
	if cacheOK {
		_ = cache.SetCache(ctx, progressKey, int(lastID), -1)
	}
	fmt.Printf("RefreshAllAccount: done success=%d failed=%d last_id=%d next_start_id=%d progress_key=%s\n", success, failed, lastID, lastID, progressKey)
	return nil
}

func InitTransfer(sg *Storage) {
	c := context.TODO()
	db := sg.Dao.GetDbInstance().(*gorm.DB)
	MarkMissingTransferMetadata(c, db)

	blockNum, _ := sg.Dao.GetCurrentBlockNum(c)
	for i := int(blockNum); i >= 0; i -= int(model.SplitTableBlockNum) {

		tableName := model.TableNameFromInterface(&model.ChainEvent{BlockNum: uint(i)}, db)
		var events []*model.ChainEvent

		query := db.Table(tableName).
			Where("module_id = ?", "balances").
			Where("event_id = ?", "Transfer")
		query.FindInBatches(&events, 50000, func(tx *gorm.DB, batch int) error {
			var blocks = make(map[int]*storage.Block)
			var blockNums []uint

			for _, e := range events {
				blockNums = append(blockNums, e.BlockNum)
			}

			for _, b := range sg.Dao.GetBlocksByNums(c, blockNums, "id,block_num,block_timestamp") {
				blocks[b.BlockNum] = b
			}

			for index := range events {
				event := events[index]
				_ = EmitEvent(c, sg, event.AsPlugin(), blocks[int(event.BlockNum)])
			}
			return nil
		})
		backfillOmniBridgePayoutTransfers(c, sg, db, tableName)
	}

}

func MarkMissingTransferMetadata(ctx context.Context, db *gorm.DB) error {
	return db.WithContext(ctx).
		Model(&bModel.Transfer{}).
		Where(
			"category = '' OR category IS NULL OR source_module = '' OR source_module IS NULL OR source_event = '' OR source_event IS NULL OR balance_event = '' OR balance_event IS NULL",
		).
		Updates(map[string]interface{}{
			"category":      TransferCategoryTransfer,
			"source_module": TransferSourceBalances,
			"source_event":  TransferEventTransfer,
			"balance_event": TransferEventTransfer,
		}).Error
}

func backfillOmniBridgePayoutTransfers(ctx context.Context, sg *Storage, db *gorm.DB, tableName string) {
	var paidOutEvents []*model.ChainEvent
	query := db.Table(tableName).
		Where("LOWER(module_id) = ?", TransferSourceOmniBridge).
		Where("LOWER(event_id) = ?", strings.ToLower(TransferEventPaidOut))
	query.FindInBatches(&paidOutEvents, 50000, func(tx *gorm.DB, batch int) error {
		extrinsicSeen := make(map[string]bool)
		var extrinsicIds []string
		for _, e := range paidOutEvents {
			if e.ExtrinsicIndex == "" || extrinsicSeen[e.ExtrinsicIndex] {
				continue
			}
			extrinsicSeen[e.ExtrinsicIndex] = true
			extrinsicIds = append(extrinsicIds, e.ExtrinsicIndex)
		}
		if len(extrinsicIds) == 0 {
			return nil
		}

		var groupedEvents []*model.ChainEvent
		if err := tx.Table(tableName).
			Where("extrinsic_index IN ?", extrinsicIds).
			Where("(LOWER(module_id) = ? AND LOWER(event_id) = ?) OR (LOWER(module_id) = ? AND LOWER(event_id) = ?)",
				TransferSourceOmniBridge, strings.ToLower(TransferEventPaidOut),
				TransferSourceBalances, strings.ToLower(TransferEventMinted),
			).
			Order("id asc").
			Find(&groupedEvents).Error; err != nil {
			return err
		}

		var blockNums []uint
		eventsByExtrinsic := make(map[string][]storage.Event)
		for _, e := range groupedEvents {
			eventsByExtrinsic[e.ExtrinsicIndex] = append(eventsByExtrinsic[e.ExtrinsicIndex], *e.AsPlugin())
			if strings.EqualFold(e.ModuleId, TransferSourceBalances) && strings.EqualFold(e.EventId, TransferEventMinted) {
				blockNums = append(blockNums, e.BlockNum)
			}
		}
		blocks := make(map[int]*storage.Block)
		for _, b := range sg.Dao.GetBlocksByNums(ctx, blockNums, "id,block_num,block_timestamp") {
			blocks[b.BlockNum] = b
		}
		for _, events := range eventsByExtrinsic {
			if len(events) == 0 {
				continue
			}
			_ = CreateOmniBridgePayoutTransfers(ctx, sg, events, blocks[events[0].BlockNum])
		}
		return nil
	})
}
