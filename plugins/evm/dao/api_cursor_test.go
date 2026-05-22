package dao

import (
	"context"
	"testing"

	balanceModel "github.com/itering/subscan/plugins/balance/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAccountsCursorFiltersByEvmAccount(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Account{}, &balanceModel.Account{}))

	sg = &Storage{db: db}

	ctx := context.Background()
	target := "0x63c4545ac01c77cc74044f25b8edea3880224577"
	other := "0x1111111111111111111111111111111111111111"

	require.NoError(t, db.Create(&Account{Address: "target-account", EvmAccount: target}).Error)
	require.NoError(t, db.Create(&Account{Address: "other-account", EvmAccount: other}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "target-account", Balance: decimal.NewFromInt(5)}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "other-account", Balance: decimal.NewFromInt(10)}).Error)

	list, page := (&ApiSrv{}).AccountsCursor(ctx, target, 10, nil, nil)

	require.Len(t, list, 1)
	assert.Equal(t, target, list[0].EvmAccount)
	assert.Equal(t, decimal.NewFromInt(5), list[0].Balance)
	assert.Equal(t, false, page["has_next_page"])
}

func TestAccountsCursorBeforeUsesBeforeCursor(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Account{}, &balanceModel.Account{}))

	sg = &Storage{db: db}

	ctx := context.Background()
	accounts := []struct {
		account string
		balance int64
	}{
		{account: "0x0000000000000000000000000000000000000003", balance: 30},
		{account: "0x0000000000000000000000000000000000000002", balance: 20},
		{account: "0x0000000000000000000000000000000000000001", balance: 10},
	}
	for _, account := range accounts {
		address := "substrate-" + account.account
		require.NoError(t, db.Create(&Account{Address: address, EvmAccount: account.account}).Error)
		require.NoError(t, db.Create(&balanceModel.Account{Address: address, Balance: decimal.NewFromInt(account.balance)}).Error)
	}

	cursor := AccountsJson{
		EvmAccount: "0x0000000000000000000000000000000000000002",
		Balance:    decimal.NewFromInt(20),
	}.Cursor()
	list, page := (&ApiSrv{}).AccountsCursor(ctx, "", 10, &cursor, nil)

	require.Len(t, list, 1)
	assert.Equal(t, accounts[0].account, list[0].EvmAccount)
	assert.Equal(t, false, page["has_previous_page"])
	assert.Equal(t, true, page["has_next_page"])
}
