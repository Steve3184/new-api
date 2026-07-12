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

func TestCapCheckVerifiesTokenWithConfiguredSite(t *testing.T) {
	gin.SetMode(gin.TestMode)

	var receivedResponse string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		assert.Equal(t, "/login-site/siteverify", request.URL.Path)
		assert.Equal(t, http.MethodPost, request.Method)

		var payload map[string]string
		require.NoError(t, common.DecodeJson(request.Body, &payload))
		assert.Equal(t, "login-secret", payload["secret"])
		receivedResponse = payload["response"]

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"success":true}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	previousEnabled := common.CapEnabled
	previousServerURL := common.CapServerURL
	previousClient := capHTTPClient
	common.CapEnabled = true
	common.CapServerURL = server.URL
	capHTTPClient = server.Client()
	t.Cleanup(func() {
		common.CapEnabled = previousEnabled
		common.CapServerURL = previousServerURL
		capHTTPClient = previousClient
	})

	router := gin.New()
	router.GET("/protected", capCheck("login-site", "login-secret"), func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"success": true})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected?cap_token=login-site:challenge:token", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, "login-site:challenge:token", receivedResponse)
	assert.JSONEq(t, `{"success":true}`, recorder.Body.String())
}

func TestCapCheckRejectsMissingToken(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousEnabled := common.CapEnabled
	common.CapEnabled = true
	t.Cleanup(func() { common.CapEnabled = previousEnabled })

	router := gin.New()
	router.GET("/protected", capCheck("login-site", "login-secret"), func(context *gin.Context) {
		context.JSON(http.StatusOK, gin.H{"success": true})
	})

	request := httptest.NewRequest(http.MethodGet, "/protected", nil)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.JSONEq(t, `{"success":false,"message":"Cap token 为空"}`, recorder.Body.String())
}
