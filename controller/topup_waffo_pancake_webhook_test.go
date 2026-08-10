package controller

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestWaffoPancakeWebhookReturnsFailureForUnresolvedSubscriptionOrder(t *testing.T) {
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMainType, previousLogType := common.MainDatabaseType(), common.LogDatabaseType()
	previousSettings := *operation_setting.GetPaymentSetting()
	previousMerchantID := setting.WaffoPancakeMerchantID
	previousPrivateKey := setting.WaffoPancakePrivateKey
	previousProductID := setting.WaffoPancakeProductID
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.SubscriptionOrder{}, &model.Log{}))
	*operation_setting.GetPaymentSetting() = operation_setting.PaymentSetting{
		ComplianceConfirmed:    true,
		ComplianceTermsVersion: operation_setting.CurrentComplianceTermsVersion,
	}
	setting.WaffoPancakeMerchantID = "merchant"
	setting.WaffoPancakePrivateKey = "private"
	setting.WaffoPancakeProductID = "product"
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.SetDatabaseTypes(previousMainType, previousLogType)
		*operation_setting.GetPaymentSetting() = previousSettings
		setting.WaffoPancakeMerchantID = previousMerchantID
		setting.WaffoPancakePrivateKey = previousPrivateKey
		setting.WaffoPancakeProductID = previousProductID
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	require.NoError(t, err)
	t.Setenv("WAFFO_WEBHOOK_TEST_PUBLIC_KEY", string(pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: publicKeyBytes,
	})))

	payloadBytes, err := common.Marshal(map[string]any{
		"id":        "EVT_unresolved_subscription",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"eventType": "order.completed",
		"eventId":   "PAY_unresolved_subscription",
		"storeId":   "STO_test",
		"mode":      "test",
		"data": map[string]any{
			"orderId":                       "ORD_unresolved_subscription",
			"orderMerchantExternalId":       "WAFFO_PANCAKE_SUB-missing",
			"merchantProvidedBuyerIdentity": "buyer@example.com",
			"currency":                      "USD",
			"amount":                        "1.00",
		},
	})
	require.NoError(t, err)
	payload := string(payloadBytes)
	timestamp := fmt.Sprintf("%d", time.Now().UnixMilli())
	digest := sha256.Sum256([]byte(timestamp + "." + payload))
	signature, err := rsa.SignPKCS1v15(rand.Reader, privateKey, crypto.SHA256, digest[:])
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "env", Value: "test"}}
	c.Request = httptest.NewRequest(http.MethodPost, "/api/waffo-pancake/webhook/test", strings.NewReader(payload))
	c.Request.Header.Set("X-Waffo-Signature", "t="+timestamp+",v1="+base64.StdEncoding.EncodeToString(signature))

	WaffoPancakeWebhook(c)

	assert.Equal(t, http.StatusInternalServerError, recorder.Code)
	assert.Equal(t, "retry", recorder.Body.String())
}
