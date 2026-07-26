package service

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupMoneroServiceTest(t *testing.T) {
	t.Helper()
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousRedisEnabled := common.RedisEnabled
	previousQuotaPerUnit := common.QuotaPerUnit
	previousPriceURL := moneroUSDPriceURL
	previousSettings := *operation_setting.GetPaymentSetting()
	previousEnabled := setting.MoneroEnabled
	previousURL := setting.MoneroWalletRPCURL
	previousUsername := setting.MoneroWalletRPCUsername
	previousPassword := setting.MoneroWalletRPCPassword
	previousNetwork := setting.MoneroNetwork
	previousConfirmations := setting.MoneroConfirmations
	previousMinTopUp := operation_setting.MinTopUp
	previousDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.QuotaPerUnit = 500000
	operation_setting.MinTopUp = 1
	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeUSD
	*operation_setting.GetPaymentSetting() = operation_setting.PaymentSetting{
		ComplianceConfirmed:    true,
		ComplianceTermsVersion: operation_setting.CurrentComplianceTermsVersion,
		AmountDiscount:         map[int]float64{},
	}

	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.TopUp{}, &model.MoneroPayment{}, &model.Log{}))
	require.NoError(t, db.Create(&model.User{Id: 991, Username: "monero-test-user", Group: "default", Status: common.UserStatusEnabled}).Error)

	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		common.RedisEnabled = previousRedisEnabled
		common.QuotaPerUnit = previousQuotaPerUnit
		moneroUSDPriceURL = previousPriceURL
		*operation_setting.GetPaymentSetting() = previousSettings
		setting.MoneroEnabled = previousEnabled
		setting.MoneroWalletRPCURL = previousURL
		setting.MoneroWalletRPCUsername = previousUsername
		setting.MoneroWalletRPCPassword = previousPassword
		setting.MoneroNetwork = previousNetwork
		setting.MoneroConfirmations = previousConfirmations
		operation_setting.MinTopUp = previousMinTopUp
		operation_setting.GetGeneralSetting().QuotaDisplayType = previousDisplayType
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestMoneroInvoiceTestnetConfirmationCreditsQuotedUSD(t *testing.T) {
	setupMoneroServiceTest(t)

	var mutex sync.Mutex
	confirmations := int64(0)
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization := r.Header.Get("Authorization")
		if authorization == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="monero-rpc", nonce="test-nonce", algorithm=MD5, qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		parameters, ok := moneroDigestParameters(authorization)
		require.True(t, ok)
		assert.Equal(t, "rpc-user", parameters["username"])
		assert.Equal(t, "monero-rpc", parameters["realm"])
		assert.Equal(t, "test-nonce", parameters["nonce"])
		assert.Equal(t, "/json_rpc", parameters["uri"])
		assert.Equal(t, "auth", parameters["qop"])
		assert.Equal(t, "00000001", parameters["nc"])
		ha1 := fmt.Sprintf("%x", md5.Sum([]byte("rpc-user:monero-rpc:rpc-password")))
		ha2 := fmt.Sprintf("%x", md5.Sum([]byte("POST:/json_rpc")))
		expectedResponse := fmt.Sprintf("%x", md5.Sum([]byte(strings.Join([]string{ha1, "test-nonce", "00000001", parameters["cnonce"], "auth", ha2}, ":"))))
		assert.Equal(t, expectedResponse, parameters["response"])

		var request moneroRPCRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		var response any
		switch request.Method {
		case "create_address":
			response = map[string]any{"result": map[string]any{
				"address":       "9testnetMoneroInvoiceAddress",
				"address_index": 41,
			}}
		case "get_transfers":
			mutex.Lock()
			currentConfirmations := confirmations
			mutex.Unlock()
			if currentConfirmations == 0 {
				response = map[string]any{"result": map[string]any{
					"pool": []map[string]any{{
						"amount":            100000000000,
						"double_spend_seen": false,
						"txid":              "testnet-transaction-id",
					}},
				}}
			} else {
				response = map[string]any{"result": map[string]any{
					"in": []map[string]any{{
						"amount":            100000000000,
						"confirmations":     currentConfirmations,
						"double_spend_seen": false,
						"txid":              "testnet-transaction-id",
					}},
				}}
			}
		default:
			response = map[string]any{"error": map[string]any{"code": -1, "message": "unexpected method"}}
		}
		body, err := common.Marshal(response)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer rpcServer.Close()

	priceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, err := common.Marshal(map[string]any{"monero": map[string]any{"usd": 100}})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer priceServer.Close()

	moneroUSDPriceURL = priceServer.URL
	setting.MoneroEnabled = true
	setting.MoneroWalletRPCURL = rpcServer.URL + "/json_rpc"
	setting.MoneroWalletRPCUsername = "rpc-user"
	setting.MoneroWalletRPCPassword = "rpc-password"
	setting.MoneroNetwork = setting.MoneroNetworkTestnet
	setting.MoneroConfirmations = 1

	beforeInvoice := common.GetTimestamp()
	invoice, err := CreateMoneroInvoice(context.Background(), 991, 10)
	afterInvoice := common.GetTimestamp()
	require.NoError(t, err)
	assert.Equal(t, "testnet", invoice.Network)
	assert.Equal(t, int64(10), invoice.QuotaAmount)
	assert.Equal(t, "0.100000000000", invoice.AmountXMR)
	assert.Equal(t, "100000000000", invoice.AmountAtomic)
	assert.GreaterOrEqual(t, invoice.ExpiresAt, beforeInvoice+int64(time.Hour/time.Second))
	assert.LessOrEqual(t, invoice.ExpiresAt, afterInvoice+int64(time.Hour/time.Second))
	paymentStatus, err := GetMoneroInvoicePaymentStatus(context.Background(), 991, invoice.Address)
	require.NoError(t, err)
	assert.Equal(t, model.MoneroPaymentStatusPending, paymentStatus.Status)
	assert.True(t, paymentStatus.TransactionDetected)
	assert.Equal(t, 0, paymentStatus.Confirmations)
	assert.Equal(t, 1, paymentStatus.RequiredConfirmations)

	require.NoError(t, MonitorMoneroPayments(context.Background()))
	var pendingTopUp model.TopUp
	require.NoError(t, model.DB.Where("payment_provider = ?", model.PaymentProviderMonero).First(&pendingTopUp).Error)
	assert.Equal(t, common.TopUpStatusPending, pendingTopUp.Status)

	mutex.Lock()
	confirmations = 1
	mutex.Unlock()
	require.NoError(t, MonitorMoneroPayments(context.Background()))

	var creditedUser model.User
	require.NoError(t, model.DB.First(&creditedUser, 991).Error)
	assert.Equal(t, 10*500000, creditedUser.Quota)
	var settledTopUp model.TopUp
	require.NoError(t, model.DB.First(&settledTopUp, pendingTopUp.Id).Error)
	assert.Equal(t, common.TopUpStatusSuccess, settledTopUp.Status)
	assert.InDelta(t, 10.0, settledTopUp.Money, 0.000001)
}

func TestMoneroInvoiceSelfTransferCreditsDestination(t *testing.T) {
	setupMoneroServiceTest(t)

	transferCalls := 0
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "" {
			w.Header().Set("WWW-Authenticate", `Digest realm="monero-rpc", nonce="self-transfer-nonce", algorithm=MD5, qop="auth"`)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var request moneroRPCRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		var response any
		switch request.Method {
		case "create_address":
			response = map[string]any{"result": map[string]any{
				"address":       "9selfTransferMoneroInvoiceAddress",
				"address_index": 42,
			}}
		case "get_transfers":
			transferCalls++
			if transferCalls == 1 {
				response = map[string]any{"result": map[string]any{
					"out": []map[string]any{{
						"confirmations":     1,
						"double_spend_seen": false,
						"txid":              "self-transfer-transaction-id",
						"destinations": []map[string]any{{
							"address": "9selfTransferMoneroInvoiceAddress",
							"amount":  100000000000,
						}},
					}},
				}}
			} else {
				response = map[string]any{"result": map[string]any{"in": []map[string]any{}}}
			}
		default:
			response = map[string]any{"error": map[string]any{"code": -1, "message": "unexpected method"}}
		}
		body, err := common.Marshal(response)
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer rpcServer.Close()

	priceServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		body, err := common.Marshal(map[string]any{"monero": map[string]any{"usd": 100}})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer priceServer.Close()

	moneroUSDPriceURL = priceServer.URL
	setting.MoneroEnabled = true
	setting.MoneroWalletRPCURL = rpcServer.URL + "/json_rpc"
	setting.MoneroWalletRPCUsername = "rpc-user"
	setting.MoneroWalletRPCPassword = "rpc-password"
	setting.MoneroNetwork = setting.MoneroNetworkTestnet
	setting.MoneroConfirmations = 1

	_, err := CreateMoneroInvoice(context.Background(), 991, 10)
	require.NoError(t, err)
	require.NoError(t, MonitorMoneroPayments(context.Background()))

	var creditedUser model.User
	require.NoError(t, model.DB.First(&creditedUser, 991).Error)
	assert.Equal(t, 10*500000, creditedUser.Quota)
	var settledPayment model.MoneroPayment
	require.NoError(t, model.DB.Where("address = ?", "9selfTransferMoneroInvoiceAddress").First(&settledPayment).Error)
	assert.Equal(t, model.MoneroPaymentStatusSuccess, settledPayment.Status)
	assert.Equal(t, "100000000000", settledPayment.ReceivedAtomic)
	assert.Equal(t, "self-transfer-transaction-id", settledPayment.TransactionIDs)
}
