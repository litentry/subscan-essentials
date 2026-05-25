package dao

import (
	"context"
	"github.com/itering/subscan-plugin/storage"
	"github.com/itering/subscan/model"
	bModel "github.com/itering/subscan/plugins/balance/model"
	"github.com/itering/subscan/util"
	"github.com/itering/subscan/util/address"
	"github.com/itering/substrate-api-rpc/rpc"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
)

func GetAccountListCursor(db storage.DB, limit int, before, after *uint) ([]bModel.Account, bool, bool) {
	var accounts []bModel.Account
	d := db.GetDbInstance().(*gorm.DB)
	fetch := limit + 1
	var hasPrev, hasNext bool
	q := d.Model(bModel.Account{})
	if after != nil && *after > 0 {
		q = q.Where("id < ?", *after).Order("id desc")
	} else if before != nil && *before > 0 {
		q = q.Where("id > ?", *before).Order("id asc")
	} else {
		q = q.Order("id desc")
	}
	q = q.Limit(fetch).Find(&accounts)
	if q.Error != nil {
		return nil, false, false
	}
	if before != nil && *before > 0 {
		hasPrev = len(accounts) > limit
		if hasPrev {
			accounts = accounts[:limit]
		}
		for i, j := 0, len(accounts)-1; i < j; i, j = i+1, j-1 {
			accounts[i], accounts[j] = accounts[j], accounts[i]
		}
		hasNext = true
	} else {
		hasNext = len(accounts) > limit
		if hasNext {
			accounts = accounts[:limit]
		}
		hasPrev = after != nil && *after > 0
	}
	return accounts, hasPrev, hasNext
}

func GetAccountByAddress(ctx context.Context, db storage.DB, address string) *bModel.Account {
	var account bModel.Account
	d := db.GetDbInstance().(*gorm.DB)
	q := d.WithContext(ctx).Where("address = ?", address).First(&account)
	if q.Error != nil {
		return nil
	}
	return &account

}

func GetMultisigAccountInfo(ctx context.Context, db storage.DB, accountID string) (string, string) {
	if accountID == "" {
		return "", ""
	}
	d := db.GetDbInstance().(*gorm.DB)
	currentBlock, err := db.GetCurrentBlockNum(ctx)
	if err != nil {
		return "", ""
	}
	accountID = strings.TrimPrefix(strings.ToLower(accountID), "0x")
	maxTableIndex := int(currentBlock / uint64(model.SplitTableBlockNum))
	for index := maxTableIndex; index >= 0; index-- {
		var events []model.ChainEvent
		table := model.TableNameFromInterface(&model.ChainEvent{BlockNum: uint(index) * model.SplitTableBlockNum}, d)
		q := d.WithContext(ctx).
			Table(table).
			Where("module_id = ?", "multisig").
			Where("event_id = ?", "NewMultisig").
			Order("id desc").
			Limit(200).
			Find(&events)
		if q.Error != nil {
			continue
		}
		for _, event := range events {
			if composer, ok := multisigComposerFromEvent(event, accountID); ok {
				return "Multisig", composer
			}
		}
	}
	return "", ""
}

func multisigComposerFromEvent(event model.ChainEvent, accountID string) (string, bool) {
	var composer string
	var matched bool
	for _, param := range event.Params {
		value := strings.TrimPrefix(strings.ToLower(util.ToString(param.Value)), "0x")
		switch strings.ToLower(param.Name) {
		case "approving":
			composer = value
		case "multisig":
			matched = value == accountID
		}
	}
	if !matched {
		return "", false
	}
	if composer == "" {
		return "", true
	}
	return address.Encode(composer), true
}

func RefreshAccount(ctx context.Context, s *Storage, accountId string) error {
	accountId = address.Format(accountId)
	if accountId == "" {
		return nil
	}
	db := s.Dao.GetDbInstance().(*gorm.DB)
	var account = bModel.Account{Address: accountId}
	q := db.WithContext(ctx).Scopes(model.IgnoreDuplicate).Where("address = ?", accountId).FirstOrCreate(&account)
	if q.RowsAffected == 1 {
		_, _ = s.Pool.HINCRBY(ctx, model.MetadataCacheKey(), "total_account", 1)
	}
	currentBlock, _ := s.Dao.GetCurrentBlockNum(ctx)
	return AfterAccountCreate(ctx, db, &account, currentBlock)
}

func AfterAccountCreate(ctx context.Context, db *gorm.DB, account *bModel.Account, currentBlock uint64) error {
	accountDataRaw, err := rpc.ReadStorage(nil, "system", "account", "", account.Address)
	if err != nil {
		return err
	}
	accountData := new(bModel.AccountData)
	accountDataRaw.ToAny(accountData)
	locks := ReadBalanceLocks(account.Address)
	vesting := ReadVesting(account.Address)
	lockSummary := bModel.AccountLockSummary(accountData, locks)
	return db.WithContext(ctx).Model(account).Where("address = ?", account.Address).UpdateColumns(map[string]interface{}{
		"nonce":    accountData.Nonce,
		"balance":  accountData.Data.Free.Add(accountData.Data.Reserved),
		"locked":   lockSummary.Locked,
		"reserved": accountData.Data.Reserved,
		"vested":   bModel.SummarizeVesting(vesting, currentBlock),
	}).Error
}

func ReadBalanceLocks(accountId string) []bModel.BalanceLock {
	locksRaw, err := rpc.ReadStorage(nil, "balances", "locks", "", accountId)
	if err != nil {
		return nil
	}
	var locks []bModel.BalanceLock
	locksRaw.ToAny(&locks)
	return locks
}

func ReadVesting(accountId string) []bModel.VestingInfo {
	vestingRaw, err := rpc.ReadStorage(nil, "vesting", "vesting", "", accountId)
	if err != nil {
		return nil
	}
	var vesting []bModel.VestingInfo
	vestingRaw.ToAny(&vesting)
	return vesting
}

func (s *Storage) AddOrUpdateItem(c context.Context, item interface{}, keys []string, updates ...string) *gorm.DB {
	var keyFields []clause.Column
	for _, key := range keys {
		keyFields = append(keyFields, clause.Column{Name: key})
	}
	db := s.Dao.GetDbInstance().(*gorm.DB)
	if len(updates) > 0 {
		return db.WithContext(c).Clauses(clause.OnConflict{
			Columns:   keyFields,
			DoUpdates: clause.AssignmentColumns(updates),
		}).Create(item)
	}
	return db.WithContext(c).Clauses(clause.OnConflict{
		Columns:   keyFields,
		UpdateAll: true,
	}).Create(item)
}
