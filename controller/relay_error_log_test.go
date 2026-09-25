package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestProcessChannelErrorUsesSnapshotWithoutLeakingChannelMetadata(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousErrorLogEnabled := constant.ErrorLogEnabled

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Log{}))
	model.DB, model.LOG_DB = database, database
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	constant.ErrorLogEnabled = true
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		constant.ErrorLogEnabled = previousErrorLogEnabled
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, database.Create(&model.User{Id: 7, Username: "log-owner", Group: "default"}).Error)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 7)
	ctx.Set("username", "log-owner")
	ctx.Set("token_name", "test-token")
	ctx.Set("token_id", 11)
	ctx.Set("original_model", "gpt-test")
	ctx.Set("group", "default")
	ctx.Set("channel_id", 202)
	ctx.Set("channel_name", "mutable-context-channel")
	ctx.Set("channel_type", 9)
	ctx.Set("use_channel", []string{"101"})
	common.SetContextKey(ctx, constant.ContextKeyRequestStartTime, time.Now().Add(-time.Second))

	channelSnapshot := types.ChannelError{
		ChannelId:   101,
		ChannelType: 1,
		ChannelName: "snapshot-channel",
		AutoBan:     false,
	}
	apiErr := types.NewOpenAIError(errors.New("upstream failed"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)

	processChannelError(ctx, channelSnapshot, apiErr, nil)

	var stored model.Log
	require.NoError(t, database.First(&stored).Error)
	assert.Equal(t, channelSnapshot.ChannelId, stored.ChannelId)
	storedOther, err := common.StrToMap(stored.Other)
	require.NoError(t, err)
	assert.Equal(t, float64(http.StatusBadGateway), storedOther["status_code"])
	for _, key := range []string{"channel_id", "channel_name", "channel_type"} {
		assert.NotContains(t, storedOther, key)
	}
	adminInfo, ok := storedOther["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, []any{"101"}, adminInfo["use_channel"])

	logs, total, err := model.GetUserLogs(7, model.LogTypeError, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, logs, 1)
	assert.Equal(t, channelSnapshot.ChannelId, logs[0].ChannelId)
	assert.Empty(t, logs[0].ChannelName)
	userOther, err := common.StrToMap(logs[0].Other)
	require.NoError(t, err)
	assert.NotContains(t, userOther, "admin_info")
	for _, key := range []string{"channel_id", "channel_name", "channel_type"} {
		assert.NotContains(t, userOther, key)
	}
}

func TestProcessChannelErrorRewritesUserLogButPreservesAdminOriginal(t *testing.T) {
	gin.SetMode(gin.TestMode)
	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousErrorLogEnabled := constant.ErrorLogEnabled
	previousRewriteSetting := operation_setting.GetErrorRewriteSetting()

	database, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := database.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Log{}))
	model.DB, model.LOG_DB = database, database
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	constant.ErrorLogEnabled = true
	replacementStatus := http.StatusServiceUnavailable
	rewriteSetting := operation_setting.ErrorRewriteSetting{
		Enabled:         true,
		AffectUsageLogs: true,
		Rules: []operation_setting.ErrorRewriteRule{{
			StatusCode:        http.StatusBadGateway,
			RewriteStatusCode: &replacementStatus,
			Message:           "rewritten upstream error",
		}},
	}
	rulesJSON, err := common.Marshal(rewriteSetting.Rules)
	require.NoError(t, err)
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"error_rewrite.enabled":           strconv.FormatBool(rewriteSetting.Enabled),
		"error_rewrite.affect_usage_logs": strconv.FormatBool(rewriteSetting.AffectUsageLogs),
		"error_rewrite.rules":             string(rulesJSON),
	}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		constant.ErrorLogEnabled = previousErrorLogEnabled
		originalRules, marshalErr := common.Marshal(previousRewriteSetting.Rules)
		originalKeywords, keywordsErr := common.Marshal(previousRewriteSetting.BodyKeywordTriggers)
		if marshalErr == nil && keywordsErr == nil {
			_ = config.GlobalConfig.LoadFromDB(map[string]string{
				"error_rewrite.enabled":                      strconv.FormatBool(previousRewriteSetting.Enabled),
				"error_rewrite.affect_usage_logs":            strconv.FormatBool(previousRewriteSetting.AffectUsageLogs),
				"error_rewrite.body_keyword_trigger_enabled": strconv.FormatBool(previousRewriteSetting.BodyKeywordTriggerEnabled),
				"error_rewrite.body_keyword_triggers":        string(originalKeywords),
				"error_rewrite.rules":                        string(originalRules),
			})
		}
		require.NoError(t, sqlDB.Close())
	})

	require.NoError(t, database.Create(&model.User{Id: 8, Username: "rewrite-owner", Group: "default"}).Error)
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	ctx.Set("id", 8)
	ctx.Set("username", "rewrite-owner")
	ctx.Set("original_model", "gpt-test")
	ctx.Set("group", "default")
	common.SetContextKey(ctx, constant.ContextKeyRequestStartTime, time.Now())

	apiErr := types.NewOpenAIError(errors.New("original upstream error"), types.ErrorCodeBadResponseStatusCode, http.StatusBadGateway)
	apiErr.SetUpstreamStatusCode(http.StatusBadGateway)
	processChannelError(ctx, *types.NewChannelError(101, 1, "test", false, "", false), apiErr, nil)

	var stored model.Log
	require.NoError(t, database.First(&stored).Error)
	assert.Equal(t, "status_code=503, rewritten upstream error", stored.Content)

	userLogs, total, err := model.GetUserLogs(8, model.LogTypeError, 0, 0, "", "", 0, 10, "", "", "")
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, userLogs, 1)
	assert.Equal(t, stored.Content, userLogs[0].Content)

	adminLogs := []*model.Log{&stored}
	model.FormatAdminLogs(adminLogs)
	adminOther, err := common.StrToMap(adminLogs[0].Other)
	require.NoError(t, err)
	adminInfo, ok := adminOther["admin_info"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "status_code=502, original upstream error", adminInfo["original_error"])
	assert.Equal(t, float64(http.StatusBadGateway), adminInfo["original_status_code"])
}
