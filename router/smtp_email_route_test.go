package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSMTPTestEmailRouteRequiresRootAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/option/smtp/test",
		strings.NewReader(`{"email":"root@example.com"}`),
	)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusUnauthorized, response.Code)
}

func TestSMTPTestEmailRouteRejectsNonRootAdministrator(t *testing.T) {
	previousDB := model.DB
	previousRedisEnabled := common.RedisEnabled
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	model.DB = db
	common.RedisEnabled = false
	t.Cleanup(func() {
		model.DB = previousDB
		common.RedisEnabled = previousRedisEnabled
	})

	accessToken := "smtp-non-root-admin"
	user := model.User{
		Username:    "smtp-admin",
		Password:    "unused-password-hash",
		Role:        common.RoleAdminUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		AffCode:     "smtp-admin",
		AuthVersion: 1,
	}
	user.SetAccessToken(accessToken)
	require.NoError(t, db.Create(&user).Error)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	SetApiRouter(engine)

	request := httptest.NewRequest(
		http.MethodPost,
		"/api/option/smtp/test",
		strings.NewReader(`{"email":"root@example.com"}`),
	)
	request.Header.Set("Authorization", "Bearer "+accessToken)
	response := httptest.NewRecorder()
	engine.ServeHTTP(response, request)

	require.Equal(t, http.StatusForbidden, response.Code)
}
