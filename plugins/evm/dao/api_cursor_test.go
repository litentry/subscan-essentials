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

func setupAccountsCursorTest(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Account{}, &Contract{}, &balanceModel.Account{}))

	originalSg := sg
	sg = &Storage{db: db}
	t.Cleanup(func() { sg = originalSg })

	return db
}

func newAccountsCursorTestServer() *ApiSrv {
	return &ApiSrv{
		nativeBalanceResolver: func(context.Context, string) (decimal.Decimal, bool) {
			return decimal.Zero, false
		},
		runtimeCodeResolver: func(context.Context, string) (string, bool) {
			return "", false
		},
	}
}

func TestAccountsCursorFiltersByEvmAccount(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	target := "0x74af439cd3af5b42a5ee71551af0ca61b61f5fbb"
	requestedTarget := "0x74aF439cD3aF5B42A5EE71551Af0ca61b61F5fBb"
	other := "0x1111111111111111111111111111111111111111"

	require.NoError(t, db.Create(&Account{Address: "target-account", EvmAccount: target}).Error)
	require.NoError(t, db.Create(&Account{Address: "other-account", EvmAccount: other}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "target-account", Balance: decimal.NewFromInt(5)}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "other-account", Balance: decimal.NewFromInt(10)}).Error)

	srv := newAccountsCursorTestServer()
	srv.nativeBalanceResolver = func(_ context.Context, address string) (decimal.Decimal, bool) {
		assert.Equal(t, target, address)
		return decimal.NewFromInt(5), true
	}
	list, page, err := srv.AccountsCursor(ctx, requestedTarget, false, 10, nil, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, target, list[0].EvmAccount)
	assert.Equal(t, decimal.NewFromInt(5), list[0].Balance)
	assert.Equal(t, false, page["has_next_page"])
}

func TestAccountsCursorBeforeUsesBeforeCursor(t *testing.T) {
	db := setupAccountsCursorTest(t)

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
	srv := &ApiSrv{
		nativeBalanceResolver: func(context.Context, string) (decimal.Decimal, bool) {
			t.Fatal("account list must not query the EVM balance RPC")
			return decimal.Zero, false
		},
		runtimeCodeResolver: func(context.Context, string) (string, bool) {
			t.Fatal("account list must not query the EVM code RPC")
			return "", false
		},
	}
	list, page, err := srv.AccountsCursor(ctx, "", false, 10, &cursor, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, accounts[0].account, list[0].EvmAccount)
	assert.Equal(t, false, page["has_previous_page"])
	assert.Equal(t, true, page["has_next_page"])
}

func TestAccountsCursorAfterAndLimitKeepListPagination(t *testing.T) {
	db := setupAccountsCursorTest(t)

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

	srv := &ApiSrv{
		nativeBalanceResolver: func(context.Context, string) (decimal.Decimal, bool) {
			t.Fatal("account list must not query the EVM balance RPC")
			return decimal.Zero, false
		},
		runtimeCodeResolver: func(context.Context, string) (string, bool) {
			t.Fatal("account list must not query the EVM code RPC")
			return "", false
		},
	}
	list, page, err := srv.AccountsCursor(ctx, "", false, 1, nil, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, accounts[0].account, list[0].EvmAccount)
	assert.Equal(t, false, page["has_previous_page"])
	assert.Equal(t, true, page["has_next_page"])
	endCursor, ok := page["end_cursor"].(*string)
	require.True(t, ok)
	require.NotNil(t, endCursor)

	list, page, err = srv.AccountsCursor(ctx, "", false, 1, nil, endCursor)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, accounts[1].account, list[0].EvmAccount)
	assert.Equal(t, true, page["has_previous_page"])
	assert.Equal(t, true, page["has_next_page"])
}

func TestContractsCursorVerifiedSourceOnly(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	contracts := []Contract{
		{
			Address:          "0x0000000000000000000000000000000000000001",
			ContractName:     "VerifiedWithSource",
			VerifyStatus:     "verified",
			SourceCode:       "pragma solidity ^0.8.0; contract VerifiedWithSource {}",
			TransactionCount: 30,
		},
		{
			Address:          "0x0000000000000000000000000000000000000002",
			ContractName:     "VerifiedWithoutSource",
			VerifyStatus:     "verified",
			TransactionCount: 20,
		},
		{
			Address:          "0x0000000000000000000000000000000000000003",
			ContractName:     "UnverifiedWithSource",
			SourceCode:       "pragma solidity ^0.8.0; contract UnverifiedWithSource {}",
			TransactionCount: 10,
		},
	}
	require.NoError(t, db.Create(&contracts).Error)

	list, page := (&ApiSrv{}).ContractsCursor(ctx, 10, nil, nil, true)

	require.Len(t, list, 1)
	assert.Equal(t, "VerifiedWithSource", list[0].ContractName)
	assert.Equal(t, "verified", list[0].VerifyStatus)
	assert.Equal(t, false, page["has_next_page"])
}

func TestAccountsCursorExcludesSmartContracts(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	eoa := "0x0000000000000000000000000000000000000001"
	contract := "0x0000000000000000000000000000000000000002"

	require.NoError(t, db.Create(&Account{Address: "substrate-eoa", EvmAccount: eoa}).Error)
	require.NoError(t, db.Create(&Account{Address: "substrate-contract", EvmAccount: contract}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "substrate-eoa", Balance: decimal.NewFromInt(10)}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "substrate-contract", Balance: decimal.NewFromInt(20)}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&Contract{Address: contract}).Error)

	srv := newAccountsCursorTestServer()
	list, page, err := srv.AccountsCursor(ctx, "", false, 10, nil, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, eoa, list[0].EvmAccount)
	assert.Equal(t, decimal.NewFromInt(10), list[0].Balance)
	assert.Equal(t, false, page["has_next_page"])

	list, page, err = srv.AccountsCursor(ctx, contract, false, 10, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)
	assert.Nil(t, page["start_cursor"])
	assert.Nil(t, page["end_cursor"])

	srv.nativeBalanceResolver = func(context.Context, string) (decimal.Decimal, bool) {
		return decimal.NewFromInt(20), true
	}
	list, page, err = srv.AccountsCursor(ctx, contract, true, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, contract, list[0].EvmAccount)
	assert.Equal(t, decimal.NewFromInt(20), list[0].Balance)
	assert.Equal(t, false, page["has_next_page"])
}

func TestAccountsCursorDoesNotReportContractBalanceWhenUnavailable(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	contract := "0x0000000000000000000000000000000000000003"

	require.NoError(t, db.Create(&Account{Address: "substrate-contract", EvmAccount: contract}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&Contract{Address: contract}).Error)

	list, page, err := newAccountsCursorTestServer().AccountsCursor(ctx, contract, true, 10, nil, nil)

	assert.ErrorIs(t, err, ErrEvmAccountUnavailable)
	assert.Empty(t, list)
	assert.Nil(t, page)
}

func TestAccountsCursorSynthesizesUnindexedEOAFromRPC(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	requestedAddress := "0x74aF439cD3aF5B42A5EE71551Af0ca61b61F5fBb"
	normalizedAddress := "0x74af439cd3af5b42a5ee71551af0ca61b61f5fbb"
	liveBalance := decimal.NewFromInt(900_000_000_000_000_000)
	srv := &ApiSrv{
		nativeBalanceResolver: func(_ context.Context, address string) (decimal.Decimal, bool) {
			assert.Equal(t, normalizedAddress, address)
			return liveBalance, true
		},
		runtimeCodeResolver: func(_ context.Context, address string) (string, bool) {
			assert.Equal(t, normalizedAddress, address)
			return "0x", true
		},
	}

	list, page, err := srv.AccountsCursor(ctx, requestedAddress, false, 10, nil, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, normalizedAddress, list[0].EvmAccount)
	assert.True(t, liveBalance.Equal(list[0].Balance))
	assert.NotNil(t, page["start_cursor"])
	assert.NotNil(t, page["end_cursor"])
	assert.Equal(t, false, page["has_previous_page"])
	assert.Equal(t, false, page["has_next_page"])

	var accountCount int64
	require.NoError(t, db.Model(&Account{}).Count(&accountCount).Error)
	assert.Zero(t, accountCount, "detail lookup must not create an evm_accounts row")
}

func TestAccountsCursorReturnsZeroBalanceFromSuccessfulRPC(t *testing.T) {
	setupAccountsCursorTest(t)

	srv := &ApiSrv{
		nativeBalanceResolver: func(context.Context, string) (decimal.Decimal, bool) {
			return decimal.Zero, true
		},
		runtimeCodeResolver: func(context.Context, string) (string, bool) {
			return "0x", true
		},
	}

	list, _, err := srv.AccountsCursor(context.Background(), "0x0000000000000000000000000000000000000001", false, 10, nil, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, list[0].Balance.IsZero())
}

func TestAccountsCursorReturnsUnavailableOnBalanceRPCFailure(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	address := "0x0000000000000000000000000000000000000004"
	mappedAddress := h160ToAccountIdByNetwork(ctx, address, "")
	require.NotEmpty(t, mappedAddress)
	require.NoError(t, db.Create(&balanceModel.Account{Address: mappedAddress, Balance: decimal.NewFromInt(1_000_000_000_000_000_000)}).Error)

	srv := &ApiSrv{
		nativeBalanceResolver: func(context.Context, string) (decimal.Decimal, bool) {
			return decimal.Zero, false
		},
		runtimeCodeResolver: func(context.Context, string) (string, bool) {
			return "0x", true
		},
	}

	list, page, err := srv.AccountsCursor(ctx, address, false, 10, nil, nil)

	assert.ErrorIs(t, err, ErrEvmAccountUnavailable)
	assert.Empty(t, list)
	assert.Nil(t, page)
}

func TestAccountsCursorReturnsUnavailableWhenCodeRPCFails(t *testing.T) {
	setupAccountsCursorTest(t)

	srv := &ApiSrv{
		nativeBalanceResolver: func(context.Context, string) (decimal.Decimal, bool) {
			t.Fatal("balance RPC must not run when the account type is unknown")
			return decimal.Zero, false
		},
		runtimeCodeResolver: func(context.Context, string) (string, bool) {
			return "", false
		},
	}

	list, page, err := srv.AccountsCursor(context.Background(), "0x0000000000000000000000000000000000000009", false, 10, nil, nil)

	assert.ErrorIs(t, err, ErrEvmAccountUnavailable)
	assert.Empty(t, list)
	assert.Nil(t, page)
}

func TestAccountsCursorReturnsUnavailableOnListDatabaseFailure(t *testing.T) {
	db := setupAccountsCursorTest(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	list, page, err := newAccountsCursorTestServer().AccountsCursor(context.Background(), "", false, 10, nil, nil)

	assert.ErrorIs(t, err, ErrEvmAccountUnavailable)
	assert.Empty(t, list)
	assert.Nil(t, page)
}

func TestAccountsCursorRespectsContractFilterForUnindexedAddress(t *testing.T) {
	setupAccountsCursorTest(t)

	ctx := context.Background()
	contract := "0x0000000000000000000000000000000000000005"
	liveBalance := decimal.NewFromInt(5)
	srv := &ApiSrv{
		nativeBalanceResolver: func(context.Context, string) (decimal.Decimal, bool) {
			return liveBalance, true
		},
		runtimeCodeResolver: func(context.Context, string) (string, bool) {
			return "0x6000", true
		},
	}

	list, _, err := srv.AccountsCursor(ctx, contract, false, 10, nil, nil)
	require.NoError(t, err)
	assert.Empty(t, list)

	list, _, err = srv.AccountsCursor(ctx, contract, true, 10, nil, nil)
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, contract, list[0].EvmAccount)
	assert.True(t, liveBalance.Equal(list[0].Balance))
}

func TestAccountsCursorUsesLiveBalanceForIndexedEOA(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	address := "0x0000000000000000000000000000000000000006"
	require.NoError(t, db.Create(&Account{Address: "substrate-eoa", EvmAccount: address}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "substrate-eoa", Balance: decimal.NewFromInt(1_000_000_000_000_000_000)}).Error)
	liveBalance := decimal.NewFromInt(900_000_000_000_000_000)
	srv := newAccountsCursorTestServer()
	srv.nativeBalanceResolver = func(context.Context, string) (decimal.Decimal, bool) {
		return liveBalance, true
	}

	list, _, err := srv.AccountsCursor(ctx, address, false, 10, nil, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.True(t, liveBalance.Equal(list[0].Balance))
}

func TestAccountsCursorIncludesIndexedEOAWithoutBalanceRow(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	address := "0x0000000000000000000000000000000000000007"
	require.NoError(t, db.Create(&Account{Address: "substrate-eoa", EvmAccount: address}).Error)
	liveBalance := decimal.NewFromInt(7)
	srv := newAccountsCursorTestServer()
	srv.nativeBalanceResolver = func(context.Context, string) (decimal.Decimal, bool) {
		return liveBalance, true
	}

	list, _, err := srv.AccountsCursor(ctx, address, false, 10, nil, nil)

	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, address, list[0].EvmAccount)
	assert.True(t, liveBalance.Equal(list[0].Balance))
}

func TestAccountsCursorDoesNotReportIndexedEOAWithoutBalanceOnRPCFailure(t *testing.T) {
	db := setupAccountsCursorTest(t)

	ctx := context.Background()
	address := "0x0000000000000000000000000000000000000008"
	require.NoError(t, db.Create(&Account{Address: "substrate-eoa", EvmAccount: address}).Error)

	list, page, err := newAccountsCursorTestServer().AccountsCursor(ctx, address, false, 10, nil, nil)

	assert.ErrorIs(t, err, ErrEvmAccountUnavailable)
	assert.Empty(t, list)
	assert.Nil(t, page["start_cursor"])
	assert.Nil(t, page["end_cursor"])
}
