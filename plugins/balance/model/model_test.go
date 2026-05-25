package model

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func TestAccountDataLockedBalanceSupportsLegacyAndCurrentFields(t *testing.T) {
	accountData := AccountData{}
	accountData.Data.MiscFrozen = decimal.NewFromInt(3)
	accountData.Data.FeeFrozen = decimal.NewFromInt(5)
	accountData.Data.Frozen = decimal.NewFromInt(7)

	assert.Equal(t, "7", accountData.LockedBalance().String())
}

func TestAccountLockSummaryIncludesVestingLock(t *testing.T) {
	accountData := AccountData{}
	accountData.Data.Frozen = decimal.NewFromInt(10)

	summary := AccountLockSummary(&accountData, []BalanceLock{
		{ID: "0x7374616b696e6720", Amount: decimal.NewFromInt(12)},
		{ID: VestingLockId, Amount: decimal.NewFromInt(8)},
	})

	assert.Equal(t, "12", summary.Locked.String())
	assert.Equal(t, "8", summary.Vested.String())
}
