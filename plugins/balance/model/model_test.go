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

func TestAccountLockSummaryUsesMaxLockedValue(t *testing.T) {
	accountData := AccountData{}
	accountData.Data.Frozen = decimal.NewFromInt(10)

	summary := AccountLockSummary(&accountData, []BalanceLock{
		{ID: "0x7374616b696e6720", Amount: decimal.NewFromInt(12)},
	})

	assert.Equal(t, "12", summary.Locked.String())
}

func TestSummarizeVestingCalculatesVestedAmount(t *testing.T) {
	summary := SummarizeVesting([]VestingInfo{
		{
			Locked:        decimal.NewFromInt(100),
			PerBlock:      decimal.NewFromInt(3),
			StartingBlock: 10,
		},
		{
			Locked:        decimal.NewFromInt(50),
			PerBlock:      decimal.NewFromInt(10),
			StartingBlock: 12,
		},
	}, 16)

	assert.Equal(t, "58", summary.String())
}

func TestVestingInfoVestedAtCapsAtLocked(t *testing.T) {
	schedule := VestingInfo{
		Locked:        decimal.NewFromInt(100),
		PerBlock:      decimal.NewFromInt(3),
		StartingBlock: 10,
	}

	assert.Equal(t, "0", schedule.VestedAt(10).String())
	assert.Equal(t, "15", schedule.VestedAt(15).String())
	assert.Equal(t, "100", schedule.VestedAt(100).String())
}
