package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHCaptchaCheckVerifiesToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var receivedToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		require.NoError(t, request.ParseForm())
		assert.Equal(t, "test-secret", request.Form.Get("secret"))
		assert.Equal(t, "test-site", request.Form.Get("sitekey"))
		receivedToken = request.Form.Get("response")
		assert.NotEmpty(t, request.Form.Get("remoteip"))
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"success":true}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	previousEnabled := common.HCaptchaEnabled
	previousSecret := common.HCaptchaSecretKey
	previousSiteKey := common.HCaptchaSiteKey
	previousURL := hCaptchaVerifyURL
	previousClient := hCaptchaHTTPClient
	common.HCaptchaEnabled = true
	common.HCaptchaSecretKey = "test-secret"
	common.HCaptchaSiteKey = "test-site"
	hCaptchaVerifyURL = server.URL
	hCaptchaHTTPClient = server.Client()
	t.Cleanup(func() {
		common.HCaptchaEnabled = previousEnabled
		common.HCaptchaSecretKey = previousSecret
		common.HCaptchaSiteKey = previousSiteKey
		hCaptchaVerifyURL = previousURL
		hCaptchaHTTPClient = previousClient
	})

	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("secret"))))
	router.GET("/protected", HCaptchaCheckFresh(), func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"success": true})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected?hcaptcha=test-token", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "test-token", receivedToken)
	assert.JSONEq(t, `{"success":true}`, recorder.Body.String())
}

func TestHCaptchaCheckRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousEnabled := common.HCaptchaEnabled
	common.HCaptchaEnabled = true
	t.Cleanup(func() { common.HCaptchaEnabled = previousEnabled })

	router := gin.New()
	router.Use(sessions.Sessions("test", cookie.NewStore([]byte("secret"))))
	router.GET("/protected", HCaptchaCheckFresh(), func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"success": true})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"success":false,"message":"hCaptcha token 为空"}`, recorder.Body.String())
}
