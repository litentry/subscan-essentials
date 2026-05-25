package model

import (
	"encoding/json"
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

func TestVestingInfoDecodesChainStorageFields(t *testing.T) {
	var schedules []VestingInfo
	err := json.Unmarshal([]byte(`[{"locked":"24102589000000000000000000","per_block":"5555555555555555000","starting_block":6282695}]`), &schedules)

	assert.NoError(t, err)
	assert.Len(t, schedules, 1)
	assert.Equal(t, "24102589000000000000000000", schedules[0].Locked.String())
	assert.Equal(t, "5555555555555555000", schedules[0].PerBlock.String())
	assert.Equal(t, uint64(6282695), schedules[0].StartingBlock)
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
