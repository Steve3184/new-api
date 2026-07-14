package relay

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestResolveOriginTaskLocksTextureToSourceChannel(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalMainDatabaseType := common.MainDatabaseType()
	originalLogDatabaseType := common.LogDatabaseType()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	t.Cleanup(func() {
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.SetDatabaseTypes(originalMainDatabaseType, originalLogDatabaseType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	baseURL := "https://meshy.example.com"
	sourceChannel := &model.Channel{
		Type:    constant.ChannelTypeMeshy2API,
		Key:     "mk_source",
		Status:  common.ChannelStatusEnabled,
		Name:    "meshy-source",
		BaseURL: &baseURL,
	}
	require.NoError(t, db.Create(sourceChannel).Error)

	sourceTask := &model.Task{
		TaskID:    "task_public_draft",
		Platform:  constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeMeshy2API)),
		UserId:    42,
		ChannelId: sourceChannel.Id,
		Status:    model.TaskStatusSuccess,
		Properties: model.Properties{
			OriginModelName: "meshy-6-draft",
		},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: "upstream-draft-id",
		},
	}
	require.NoError(t, db.Create(sourceTask).Error)

	request := httptest.NewRequest(http.MethodPost, "/v1/3d", strings.NewReader(`{
		"model":"meshy-6-texture",
		"source_task_id":"task_public_draft"
	}`))
	request.Header.Set("Content-Type", "application/json")
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request
	common.SetContextKey(context, constant.ContextKeyUserId, 42)
	common.SetContextKey(context, constant.ContextKeyOriginalModel, "meshy-6-texture")
	common.SetContextKey(context, constant.ContextKeyChannelId, sourceChannel.Id+1)
	common.SetContextKey(context, constant.ContextKeyChannelType, constant.ChannelTypeMeshy2API)
	common.SetContextKey(context, constant.ContextKeyChannelKey, "mk_initial")
	common.SetContextKey(context, constant.ContextKeyChannelBaseUrl, "https://initial.example.com")

	info := &relaycommon.RelayInfo{
		UserId:          42,
		OriginModelName: "meshy-6-texture",
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{},
	}

	require.Nil(t, ResolveOriginTask(context, info))
	require.NotNil(t, info.ChannelMeta)
	assert.Equal(t, sourceChannel.Id+1, info.ChannelId)
	assert.Equal(t, "upstream-draft-id", info.OriginUpstreamTaskID)
	assert.Equal(t, sourceChannel.Id+1, common.GetContextKeyInt(context, constant.ContextKeyChannelId))
	lockedChannel, ok := info.LockedChannel.(*model.Channel)
	require.True(t, ok)
	assert.Equal(t, sourceChannel.Id, lockedChannel.Id)
}
