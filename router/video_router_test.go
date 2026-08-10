package router

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestThreeDContentRouteAllowsAnonymousPublicTaskAndUsesSavedKey(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	type upstreamRequest struct {
		method        string
		path          string
		authorization string
	}
	received := make(chan upstreamRequest, 1)
	glb := []byte("glTF-test-content")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received <- upstreamRequest{
			method:        r.Method,
			path:          r.URL.Path,
			authorization: r.Header.Get("Authorization"),
		}
		w.Header().Set("Content-Type", "model/gltf-binary")
		w.Header().Set("Content-Disposition", `attachment; filename="model.glb"`)
		_, _ = w.Write(glb)
	}))
	defer upstream.Close()

	channel := &model.Channel{
		Type:    constant.ChannelTypeMeshy2API,
		Key:     "channel-fallback-key",
		Name:    "meshy-test",
		Status:  common.ChannelStatusEnabled,
		BaseURL: &upstream.URL,
	}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_public_meshy_content",
		Platform:  constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeMeshy2API)),
		UserId:    42,
		ChannelId: channel.Id,
		Status:    model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{
			Key:            "saved-task-key",
			UpstreamTaskID: "upstream-meshy-task",
		},
	}).Error)

	engine := gin.New()
	SetVideoRouter(engine)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/3d/task_public_meshy_content/content", nil)
	require.Empty(t, request.Header.Get("Authorization"))
	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code)
	assert.Equal(t, glb, recorder.Body.Bytes())
	assert.Equal(t, "model/gltf-binary", recorder.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="model.glb"`, recorder.Header().Get("Content-Disposition"))

	upstreamCall := <-received
	assert.Equal(t, http.MethodGet, upstreamCall.method)
	assert.Equal(t, "/v1/3d/upstream-meshy-task/content", upstreamCall.path)
	assert.Equal(t, "Bearer saved-task-key", upstreamCall.authorization)
}

func TestThreeDContentRouteDoesNotExposeNonMeshyTaskStatus(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousDB, previousLogDB := model.DB, model.LOG_DB
	previousMemoryCacheEnabled := common.MemoryCacheEnabled
	previousRedisEnabled := common.RedisEnabled
	previousMainDatabaseType, previousLogDatabaseType := common.MainDatabaseType(), common.LogDatabaseType()
	common.MemoryCacheEnabled = false
	common.RedisEnabled = false
	common.SetDatabaseTypes(common.DatabaseTypeSQLite, common.DatabaseTypeSQLite)

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Task{}))
	t.Cleanup(func() {
		model.DB, model.LOG_DB = previousDB, previousLogDB
		common.MemoryCacheEnabled = previousMemoryCacheEnabled
		common.RedisEnabled = previousRedisEnabled
		common.SetDatabaseTypes(previousMainDatabaseType, previousLogDatabaseType)
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
	})

	channel := &model.Channel{
		Type:   constant.ChannelTypeOpenAI,
		Key:    "channel-key",
		Name:   "openai-test",
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.Task{
		TaskID:    "task_public_non_meshy",
		Platform:  constant.TaskPlatform(fmt.Sprintf("%d", constant.ChannelTypeOpenAI)),
		UserId:    42,
		ChannelId: channel.Id,
		Status:    model.TaskStatusInProgress,
	}).Error)

	engine := gin.New()
	SetVideoRouter(engine)
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/v1/3d/task_public_non_meshy/content", nil)
	engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
