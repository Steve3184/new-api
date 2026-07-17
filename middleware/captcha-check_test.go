package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCaptchaCheckRedemptionDisabledPassesThrough(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRequired := common.ForceRedemptionCaptcha
	previousType := common.CaptchaType
	common.ForceRedemptionCaptcha = false
	common.CaptchaType = "cap"
	t.Cleanup(func() {
		common.ForceRedemptionCaptcha = previousRequired
		common.CaptchaType = previousType
	})

	router := gin.New()
	router.POST("/redeem", CaptchaCheckRedemption(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/redeem", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"success":true}`, recorder.Body.String())
}

func TestCaptchaCheckRedemptionRequiresEnabledProvider(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousRequired := common.ForceRedemptionCaptcha
	previousType := common.CaptchaType
	previousCapEnabled := common.CapEnabled
	common.ForceRedemptionCaptcha = true
	common.CaptchaType = "cap"
	common.CapEnabled = false
	t.Cleanup(func() {
		common.ForceRedemptionCaptcha = previousRequired
		common.CaptchaType = previousType
		common.CapEnabled = previousCapEnabled
	})

	router := gin.New()
	router.POST("/redeem", CaptchaCheckRedemption(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, "/redeem", nil))

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"success":false,"message":"Cap is not enabled"}`, recorder.Body.String())
}

func TestCaptchaCheckRedemptionUsesLoginCapSite(t *testing.T) {
	gin.SetMode(gin.TestMode)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/redemption-site/siteverify", request.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"success":true}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	previousRequired := common.ForceRedemptionCaptcha
	previousType := common.CaptchaType
	previousCapEnabled := common.CapEnabled
	previousServerURL := common.CapServerURL
	previousSiteKey := common.CapSiteKey
	previousSecretKey := common.CapSecretKey
	previousClient := capHTTPClient
	common.ForceRedemptionCaptcha = true
	common.CaptchaType = "cap"
	common.CapEnabled = true
	common.CapServerURL = server.URL
	common.CapSiteKey = "redemption-site"
	common.CapSecretKey = "redemption-secret"
	capHTTPClient = server.Client()
	t.Cleanup(func() {
		common.ForceRedemptionCaptcha = previousRequired
		common.CaptchaType = previousType
		common.CapEnabled = previousCapEnabled
		common.CapServerURL = previousServerURL
		common.CapSiteKey = previousSiteKey
		common.CapSecretKey = previousSecretKey
		capHTTPClient = previousClient
	})

	router := gin.New()
	router.POST("/redeem", CaptchaCheckRedemption(), func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/redeem?cap_token=valid-token", nil)
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"success":true}`, recorder.Body.String())
}
