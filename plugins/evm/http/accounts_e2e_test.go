package http

import (
	"bytes"
	"encoding/json"
	nethttp "net/http"
	"net/http/httptest"
	"strings"
	"testing"

	balanceModel "github.com/itering/subscan/plugins/balance/model"
	"github.com/itering/subscan/plugins/evm/dao"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAccountsRouteExcludesSmartContracts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&dao.Account{}, &dao.Contract{}, &balanceModel.Account{}))

	dao.Init(db, nil)
	originalSrv := srv
	srv = &dao.ApiSrv{}
	t.Cleanup(func() { srv = originalSrv })

	eoa := "0x0000000000000000000000000000000000000001"
	contract := "0x0000000000000000000000000000000000000002"

	require.NoError(t, db.Create(&dao.Account{Address: "substrate-eoa", EvmAccount: eoa}).Error)
	require.NoError(t, db.Create(&dao.Account{Address: "substrate-contract", EvmAccount: contract}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "substrate-eoa", Balance: decimal.NewFromInt(10)}).Error)
	require.NoError(t, db.Create(&balanceModel.Account{Address: "substrate-contract", Balance: decimal.NewFromInt(20)}).Error)
	require.NoError(t, db.Session(&gorm.Session{SkipHooks: true}).Create(&dao.Contract{Address: contract}).Error)

	request := httptest.NewRequest(
		nethttp.MethodPost,
		"/api/plugin/evm/accounts",
		strings.NewReader(`{"row":10}`),
	)
	recorder := httptest.NewRecorder()
	handler := nethttp.HandlerFunc(func(w nethttp.ResponseWriter, r *nethttp.Request) {
		_ = accountsHandle(w, r)
	})

	handler.ServeHTTP(recorder, request)
	require.Equal(t, nethttp.StatusOK, recorder.Code)

	var response struct {
		Code int `json:"code"`
		Data struct {
			List []dao.AccountsJson `json:"list"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Zero(t, response.Code)
	require.Len(t, response.Data.List, 1)
	assert.Equal(t, eoa, response.Data.List[0].EvmAccount)
	assert.NotEqual(t, contract, response.Data.List[0].EvmAccount)

	var pretty bytes.Buffer
	require.NoError(t, json.Indent(&pretty, recorder.Body.Bytes(), "", "  "))
	t.Logf("POST /api/plugin/evm/accounts response with seeded evm_accounts and evm_contracts:\n%s", pretty.String())
}
