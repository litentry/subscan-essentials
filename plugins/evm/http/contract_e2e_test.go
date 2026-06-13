package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/itering/subscan/plugins/evm/dao"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contractRouteMockServer struct {
	MockServer
}

func (m contractRouteMockServer) ContractsByAddr(_ context.Context, address string) *dao.Contract {
	return &dao.Contract{
		Address:        address,
		VerifyStatus:   "perfect",
		DepositBalance: decimal.NewFromInt(2),
	}
}

func TestContractRouteReturnsDepositBalance(t *testing.T) {
	originalSrv := srv
	srv = contractRouteMockServer{}
	t.Cleanup(func() { srv = originalSrv })

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/plugin/evm/contract",
		strings.NewReader(`{"address":"0x0000000000000000000000000000000000000002"}`),
	)
	recorder := httptest.NewRecorder()
	require.NoError(t, contractHandle(recorder, request))
	require.Equal(t, http.StatusOK, recorder.Code)

	var response struct {
		Code int `json:"code"`
		Data struct {
			DepositBalance decimal.Decimal `json:"deposit_balance"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &response))
	require.Zero(t, response.Code)
	assert.True(t, response.Data.DepositBalance.Equal(decimal.NewFromInt(2)))
}
