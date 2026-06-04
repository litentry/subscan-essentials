package model

import "github.com/shopspring/decimal"

type Account struct {
	ID       uint            `gorm:"primary_key" json:"-"`
	Address  string          `gorm:"default: null;size:100;index:address,unique;index:balance_address,priority:2" json:"address"`
	Nonce    int             `json:"nonce"`
	Balance  decimal.Decimal `json:"balance" gorm:"type:decimal(65,0);index:balance;index:balance_address,priority:1"`
	Locked   decimal.Decimal `json:"locked" gorm:"type:decimal(65,0);"`
	Reserved decimal.Decimal `json:"reserved" gorm:"type:decimal(65,0);"`
	Vested   decimal.Decimal `json:"vested" gorm:"type:decimal(65,0);"`
}

func (a *Account) TableName() string {
	return "balance_accounts"
}

type AccountData struct {
	Nonce    int `json:"nonce"`
	RefCount int `json:"ref_count"`
	Data     struct {
		Free       decimal.Decimal `json:"free"`
		Reserved   decimal.Decimal `json:"reserved"`
		MiscFrozen decimal.Decimal `json:"miscFrozen"`
		FeeFrozen  decimal.Decimal `json:"feeFrozen"`
		Frozen     decimal.Decimal `json:"frozen"`
	} `json:"data"`
}

type BalanceLock struct {
	ID     string          `json:"id"`
	Amount decimal.Decimal `json:"amount"`
}

type LockSummary struct {
	Locked decimal.Decimal
}

func (a AccountData) LockedBalance() decimal.Decimal {
	return decimal.Max(a.Data.Frozen, decimal.Max(a.Data.MiscFrozen, a.Data.FeeFrozen))
}

func SummarizeLocks(locks []BalanceLock) LockSummary {
	var summary LockSummary
	for _, lock := range locks {
		if lock.Amount.GreaterThan(summary.Locked) {
			summary.Locked = lock.Amount
		}
	}
	return summary
}

func AccountLockSummary(accountData *AccountData, locks []BalanceLock) LockSummary {
	summary := SummarizeLocks(locks)
	if accountData == nil {
		return summary
	}
	if dataLocked := accountData.LockedBalance(); dataLocked.GreaterThan(summary.Locked) {
		summary.Locked = dataLocked
	}
	return summary
}

type VestingInfo struct {
	Locked        decimal.Decimal `json:"locked"`
	PerBlock      decimal.Decimal `json:"per_block"`
	StartingBlock uint64          `json:"starting_block"`
}

func (v VestingInfo) VestedAt(blockNum uint64) decimal.Decimal {
	if blockNum <= v.StartingBlock {
		return decimal.Zero
	}
	vested := v.PerBlock.Mul(decimal.NewFromUint64(blockNum - v.StartingBlock))
	if vested.GreaterThan(v.Locked) {
		return v.Locked
	}
	return vested
}

func SummarizeVesting(vesting []VestingInfo, blockNum uint64) decimal.Decimal {
	var vested decimal.Decimal
	for _, schedule := range vesting {
		vested = vested.Add(schedule.VestedAt(blockNum))
	}
	return vested
}

type Transfer struct {
	Id             uint            `json:"id" gorm:"primary_key;autoIncrement:false"`
	BlockNum       uint            `json:"blockNum" gorm:"size:32"`
	Sender         string          `json:"sender" gorm:"size:255;index:query_function"`
	Receiver       string          `json:"receiver" gorm:"size:255;index:query_function"`
	Amount         decimal.Decimal `json:"amount" gorm:"decimal(65)"`
	BlockTimestamp int64           `json:"block_timestamp" `
	Symbol         string          `json:"symbol" gorm:"size:255"`
	TokenId        string          `json:"token_id" gorm:"size:255"`
	ExtrinsicIndex string          `json:"extrinsic_index" gorm:"size:255;index:extrinsic_index"`
	Category       string          `json:"category" gorm:"size:64;index"`
	SourceModule   string          `json:"source_module" gorm:"size:64;index"`
	SourceEvent    string          `json:"source_event" gorm:"size:64;index"`
	BalanceEvent   string          `json:"balance_event" gorm:"size:64"`
}

func (a *Transfer) TableName() string {
	return "balance_transfers"
}
