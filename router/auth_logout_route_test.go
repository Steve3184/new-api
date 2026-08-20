package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLogoutRouteDoesNotConsumeCriticalRateLimit(t *testing.T) {
	previousCriticalEnabled := common.CriticalRateLimitEnable
	previousCriticalNum := common.CriticalRateLimitNum
	previousCriticalDuration := common.CriticalRateLimitDuration
	previousGlobalEnabled := common.GlobalApiRateLimitEnable
	previousRedisEnabled := common.RedisEnabled
	previousCookieSecure := common.SessionCookieSecure
	t.Cleanup(func() {
		common.CriticalRateLimitEnable = previousCriticalEnabled
		common.CriticalRateLimitNum = previousCriticalNum
		common.CriticalRateLimitDuration = previousCriticalDuration
		common.GlobalApiRateLimitEnable = previousGlobalEnabled
		common.RedisEnabled = previousRedisEnabled
		common.SessionCookieSecure = previousCookieSecure
	})

	common.CriticalRateLimitEnable = true
	common.CriticalRateLimitNum = 1
	common.CriticalRateLimitDuration = 60
	common.GlobalApiRateLimitEnable = false
	common.RedisEnabled = false
	common.SessionCookieSecure = false

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	for range 2 {
		request := httptest.NewRequest(http.MethodPost, "/api/user/auth/logout", nil)
		request.RemoteAddr = "198.51.100.211:1234"
		response := httptest.NewRecorder()
		engine.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
	}
}
