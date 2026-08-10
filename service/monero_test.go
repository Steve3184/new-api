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
	previousMaxSubaddresses := setting.MoneroMaxSubaddresses
	previousUSDToCurrencyRate := setting.MoneroUSDToCurrencyRate
	previousMinTopUp := operation_setting.MinTopUp
	previousDisplayType := operation_setting.GetGeneralSetting().QuotaDisplayType
	previousCustomCurrencyExchangeRate := operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate

	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	common.RedisEnabled = false
	common.QuotaPerUnit = 500000
	setting.MoneroMaxSubaddresses = 10000
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
		setting.MoneroMaxSubaddresses = previousMaxSubaddresses
		setting.MoneroUSDToCurrencyRate = previousUSDToCurrencyRate
		operation_setting.MinTopUp = previousMinTopUp
		operation_setting.GetGeneralSetting().QuotaDisplayType = previousDisplayType
		operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = previousCustomCurrencyExchangeRate
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil {
			_ = sqlDB.Close()
		}
	})
}

func TestMoneroQuoteUSDUsesConfiguredSystemCurrencyRate(t *testing.T) {
	setupMoneroServiceTest(t)

	operation_setting.GetGeneralSetting().QuotaDisplayType = operation_setting.QuotaDisplayTypeCustom
	operation_setting.GetGeneralSetting().CustomCurrencyExchangeRate = 1
	setting.MoneroUSDToCurrencyRate = 7.25

	quote, err := moneroQuoteUSD(725, "default")
	require.NoError(t, err)
	assert.Equal(t, "100.000000000000", quote.StringFixed(12))

	setting.MoneroUSDToCurrencyRate = 0
	quote, err = moneroQuoteUSD(725, "default")
	require.NoError(t, err)
	assert.Equal(t, "725.000000000000", quote.StringFixed(12))
}

func TestMoneroInvoiceTestnetConfirmationCreditsRequestedAmount(t *testing.T) {
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
		case "get_address":
			response = map[string]any{"result": map[string]any{
				"addresses": []map[string]any{{"address": "9testnetPrimaryAddress"}},
			}}
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
						"amount":            10000000000,
						"double_spend_seen": false,
						"txid":              "testnet-transaction-id",
					}},
				}}
			} else {
				response = map[string]any{"result": map[string]any{
					"in": []map[string]any{{
						"amount":            10000000000,
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
	setting.MoneroUSDToCurrencyRate = 10

	beforeInvoice := common.GetTimestamp()
	invoice, err := CreateMoneroInvoice(context.Background(), 991, 10)
	afterInvoice := common.GetTimestamp()
	require.NoError(t, err)
	assert.Equal(t, "testnet", invoice.Network)
	assert.Equal(t, int64(10), invoice.QuotaAmount)
	assert.Equal(t, "0.010000000000", invoice.AmountXMR)
	assert.Equal(t, "10000000000", invoice.AmountAtomic)
	assert.GreaterOrEqual(t, invoice.ExpiresAt, beforeInvoice+int64(3*time.Hour/time.Second))
	assert.LessOrEqual(t, invoice.ExpiresAt, afterInvoice+int64(3*time.Hour/time.Second))
	var pendingPayment model.MoneroPayment
	require.NoError(t, model.DB.Where("address = ?", invoice.Address).First(&pendingPayment).Error)
	assert.Equal(t, "5000000.000000000000", pendingPayment.QuotaPerUSD)
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
	assert.InDelta(t, 1.0, settledTopUp.Money, 0.000001)
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
		case "get_address":
			response = map[string]any{"result": map[string]any{
				"addresses": []map[string]any{{"address": "9selfTransferPrimaryAddress"}},
			}}
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

func TestCreateMoneroInvoiceStopsAtWalletSubaddressLimit(t *testing.T) {
	setupMoneroServiceTest(t)

	createAddressCalls := 0
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request moneroRPCRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		var response any
		switch request.Method {
		case "get_address":
			response = map[string]any{"result": map[string]any{
				"addresses": []map[string]any{{"address": "9primaryAddress"}, {"address": "9existingSubaddress"}},
			}}
		case "create_address":
			createAddressCalls++
			response = map[string]any{"result": map[string]any{
				"address":       "9unexpectedNewSubaddress",
				"address_index": 2,
			}}
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
	setting.MoneroNetwork = setting.MoneroNetworkTestnet
	setting.MoneroMaxSubaddresses = 2

	_, err := CreateMoneroInvoice(context.Background(), 991, 10)
	require.EqualError(t, err, "monero wallet subaddress limit (2) has been reached")
	assert.Zero(t, createAddressCalls)
}

func TestCreateMoneroInvoiceRetriesUsedWalletSubaddress(t *testing.T) {
	setupMoneroServiceTest(t)

	existingTopUp := &model.TopUp{
		UserId:          991,
		TradeNo:         "monero-existing-address",
		PaymentMethod:   model.PaymentMethodMonero,
		PaymentProvider: model.PaymentProviderMonero,
		CreateTime:      common.GetTimestamp(),
		Status:          common.TopUpStatusExpired,
	}
	require.NoError(t, model.DB.Create(existingTopUp).Error)
	require.NoError(t, model.DB.Create(&model.MoneroPayment{
		TopUpID:      existingTopUp.Id,
		Address:      "9usedMoneroInvoiceAddress",
		AccountIndex: 0,
		AddressIndex: 17,
		Network:      setting.MoneroNetworkTestnet,
		Status:       model.MoneroPaymentStatusExpired,
		CreateTime:   common.GetTimestamp(),
	}).Error)

	createAddressCalls := 0
	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request moneroRPCRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		var response any
		switch request.Method {
		case "get_address":
			response = map[string]any{"result": map[string]any{
				"addresses": []map[string]any{{"address": "9primaryAddress"}},
			}}
		case "create_address":
			createAddressCalls++
			address := "9usedMoneroInvoiceAddress"
			addressIndex := 17
			if createAddressCalls == 2 {
				address = "9freshMoneroInvoiceAddress"
				addressIndex = 18
			}
			response = map[string]any{"result": map[string]any{
				"address":       address,
				"address_index": addressIndex,
			}}
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
	setting.MoneroNetwork = setting.MoneroNetworkTestnet
	setting.MoneroConfirmations = 1
	setting.MoneroUSDToCurrencyRate = 1

	invoice, err := CreateMoneroInvoice(context.Background(), 991, 10)
	require.NoError(t, err)
	assert.Equal(t, "9freshMoneroInvoiceAddress", invoice.Address)
	assert.Equal(t, 2, createAddressCalls)

	var topUpCount int64
	require.NoError(t, model.DB.Model(&model.TopUp{}).Count(&topUpCount).Error)
	assert.Equal(t, int64(2), topUpCount)
	var paymentCount int64
	require.NoError(t, model.DB.Model(&model.MoneroPayment{}).Count(&paymentCount).Error)
	assert.Equal(t, int64(2), paymentCount)
}

func TestAuditMoneroTerminalAddressesReportsOnlyFullyUnlockedAddresses(t *testing.T) {
	setupMoneroServiceTest(t)

	for _, fixture := range []struct {
		tradeNo      string
		addressIndex int
		status       string
	}{
		{tradeNo: "monero-audit-unlocked", addressIndex: 1, status: common.TopUpStatusSuccess},
		{tradeNo: "monero-audit-locked", addressIndex: 2, status: common.TopUpStatusSuccess},
		{tradeNo: "monero-audit-unreported", addressIndex: 3, status: common.TopUpStatusExpired},
	} {
		topUp := &model.TopUp{
			UserId:          991,
			TradeNo:         fixture.tradeNo,
			PaymentMethod:   model.PaymentMethodMonero,
			PaymentProvider: model.PaymentProviderMonero,
			Status:          fixture.status,
			CreateTime:      common.GetTimestamp(),
		}
		require.NoError(t, model.DB.Create(topUp).Error)
		paymentStatus := model.MoneroPaymentStatusSuccess
		if fixture.status == common.TopUpStatusExpired {
			paymentStatus = model.MoneroPaymentStatusExpired
		}
		require.NoError(t, model.DB.Create(&model.MoneroPayment{
			TopUpID:      topUp.Id,
			Address:      fmt.Sprintf("9auditAddress%d", fixture.addressIndex),
			AccountIndex: 0,
			AddressIndex: fixture.addressIndex,
			Network:      setting.MoneroNetworkTestnet,
			Status:       paymentStatus,
			CreateTime:   common.GetTimestamp(),
		}).Error)
	}

	rpcServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request moneroRPCRequest
		require.NoError(t, common.DecodeJson(r.Body, &request))
		require.Equal(t, "get_balance", request.Method)
		body, err := common.Marshal(map[string]any{"result": map[string]any{
			"per_subaddress": []map[string]any{
				{"address_index": 1, "balance": 100, "unlocked_balance": 100},
				{"address_index": 2, "balance": 100, "unlocked_balance": 50},
			},
		}})
		require.NoError(t, err)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(body)
	}))
	defer rpcServer.Close()

	setting.MoneroWalletRPCURL = rpcServer.URL + "/json_rpc"
	setting.MoneroNetwork = setting.MoneroNetworkTestnet

	result, err := AuditMoneroTerminalAddresses(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, result.Candidates)
	assert.Equal(t, 1, result.FullyUnlocked)
	assert.Equal(t, 1, result.Locked)
	assert.Equal(t, 1, result.NotReported)
}
