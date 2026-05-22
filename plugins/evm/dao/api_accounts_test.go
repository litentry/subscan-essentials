package dao

import (
	"context"
	"testing"

	balanceModel "github.com/itering/subscan/plugins/balance/model"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAccountsCursorIncludesAccountWithoutBalance(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Account{}, &balanceModel.Account{}))

	previousStorage := sg
	sg = &Storage{db: db}
	t.Cleanup(func() { sg = previousStorage })

	const evmAccount = "0x63c4545ac01c77cc74044f25b8edea3880224577"
	require.NoError(t, db.Create(&Account{
		Address:    "4c4a0baf647e07cc63c83684b35c7974421fda41e99d31741d1c8826c5abfc39",
		EvmAccount: evmAccount,
	}).Error)
	require.NoError(t, db.Create(&Account{
		Address:    "4c4a0baf647e07cc63c83684b35c7974421fda41e99d31741d1c8826c5abfc40",
		EvmAccount: "0xffffffffffffffffffffffffffffffffffffffff",
	}).Error)

	list, page := (&ApiSrv{}).AccountsCursor(context.Background(), "0x63C4545AC01C77CC74044F25B8EDEA3880224577", 10, nil, nil)

	require.Len(t, list, 1)
	require.Equal(t, evmAccount, list[0].EvmAccount)
	require.True(t, decimal.Zero.Equal(list[0].Balance))
	require.Equal(t, false, page["has_previous_page"])
	require.Equal(t, false, page["has_next_page"])
}
