package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetUserRankingsReturnsMetricSpecificTopGroups(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.QuotaData{}))
	now := time.Now().Unix()
	require.NoError(t, db.Create(&[]model.User{
		{Id: 1, Username: "alice-current", DisplayName: "Alice", Status: common.UserStatusEnabled, AffCode: "rank-aff-alice"},
		{Id: 2, Username: "bob", Status: common.UserStatusEnabled, AffCode: "rank-aff-bob"},
	}).Error)
	require.NoError(t, db.Create(&[]model.QuotaData{
		{UserID: 1, Username: "alice", CreatedAt: now, UseGroup: "vip", Quota: 90, TokenUsed: 5},
		{UserID: 1, Username: "alice", CreatedAt: now, UseGroup: "default", Quota: 10, TokenUsed: 100},
		{UserID: 2, Username: "bob", CreatedAt: now, UseGroup: "default", Quota: 80, TokenUsed: 80},
	}).Error)

	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/api/rankings/users?period=today", nil)
	GetUserRankings(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool                         `json:"success"`
		Data    service.UserRankingsResponse `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.Len(t, payload.Data.ByQuota, 2)
	require.Len(t, payload.Data.ByTokens, 2)
	assert.Equal(t, "alice-current", payload.Data.ByQuota[0].Username)
	assert.Equal(t, "Alice", payload.Data.ByQuota[0].DisplayName)
	assert.Equal(t, "vip", payload.Data.ByQuota[0].TopGroup)
	assert.Equal(t, "alice-current", payload.Data.ByTokens[0].Username)
	assert.Equal(t, "default", payload.Data.ByTokens[0].TopGroup)

}

func TestGetPlatformTrafficCountsTheLastMinuteAcrossUsersAndCachesIt(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	now := time.Now().Unix()
	require.NoError(t, db.Create(&[]model.Log{
		{UserId: 1, CreatedAt: now - 5, Type: model.LogTypeConsume, PromptTokens: 100, CompletionTokens: 20},
		{UserId: 2, CreatedAt: now - 30, Type: model.LogTypeConsume, PromptTokens: 7, CompletionTokens: 3},
		{UserId: 1, CreatedAt: now - 300, Type: model.LogTypeConsume, PromptTokens: 900, CompletionTokens: 900},
		{UserId: 1, CreatedAt: now - 5, Type: model.LogTypeTopup, PromptTokens: 50},
	}).Error)

	fetch := func() service.PlatformTraffic {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest(http.MethodGet, "/api/rankings/traffic", nil)
		GetPlatformTraffic(ctx)

		require.Equal(t, http.StatusOK, recorder.Code)
		var payload struct {
			Success bool                    `json:"success"`
			Data    service.PlatformTraffic `json:"data"`
		}
		require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
		require.True(t, payload.Success)
		return payload.Data
	}

	assert.Equal(t, service.PlatformTraffic{Rpm: 2, Tpm: 130}, fetch())

	// The endpoint is public, so a second visitor moments later is served the
	// same answer instead of another read of the log table.
	require.NoError(t, db.Create(&model.Log{UserId: 3, CreatedAt: now, Type: model.LogTypeConsume, PromptTokens: 1000}).Error)
	assert.Equal(t, service.PlatformTraffic{Rpm: 2, Tpm: 130}, fetch())
}
