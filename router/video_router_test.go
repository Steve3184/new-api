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

func TestGetOpenAIVideoRouteRendersJimengTask(t *testing.T) {
	gin.SetMode(gin.TestMode)

	previousDB := model.DB
	previousDatabaseType := common.MainDatabaseType()
	previousLogDatabaseType := common.LogDatabaseType()
	previousSQLitePath := common.SQLitePath
	previousMasterNode := common.IsMasterNode
	previousRedisEnabled := common.RedisEnabled
	common.SQLitePath = t.TempDir() + "/router-video.db"
	common.IsMasterNode = false
	common.RedisEnabled = false
	t.Setenv("SQL_DSN", "")
	require.NoError(t, model.InitDB())
	database := model.DB
	require.NoError(t, database.AutoMigrate(&model.User{}, &model.Token{}, &model.Channel{}, &model.Task{}))
	t.Cleanup(func() {
		sqlDB, closeErr := database.DB()
		require.NoError(t, closeErr)
		require.NoError(t, sqlDB.Close())
		model.DB = previousDB
		common.SetDatabaseTypes(previousDatabaseType, previousLogDatabaseType)
		common.SQLitePath = previousSQLitePath
		common.IsMasterNode = previousMasterNode
		common.RedisEnabled = previousRedisEnabled
	})

	require.NoError(t, database.Create(&model.User{
		Id:          91,
		Username:    "jimeng-fetch-user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Quota:       100,
		Group:       "default",
		AuthVersion: 1,
	}).Error)
	require.NoError(t, database.Create(&model.Token{
		Id:             1,
		UserId:         91,
		Key:            "jimengfetch",
		Status:         common.TokenStatusEnabled,
		Name:           "jimeng fetch",
		ExpiredTime:    -1,
		UnlimitedQuota: true,
	}).Error)
	require.NoError(t, database.Create(&model.Channel{
		Id:     17,
		Type:   constant.ChannelTypeJimeng,
		Key:    "unused",
		Status: common.ChannelStatusEnabled,
		Name:   "jimeng fetch",
		Models: "jimeng_vgfm_t2v_l20",
		Group:  "default",
	}).Error)

	task := &model.Task{
		CreatedAt: 1710000000,
		UpdatedAt: 1710000060,
		TaskID:    "task_jimeng_public",
		Platform:  constant.TaskPlatform("jimeng"),
		UserId:    91,
		Group:     "default",
		ChannelId: 17,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			ResultURL: "data:video/mp4;base64,ZGF0YQ==",
		},
	}
	task.SetData(map[string]any{
		"code": 10000,
		"data": map[string]any{
			"status":    "done",
			"task_id":   "jimeng-private-1",
			"video_url": "https://cdn.example/video.mp4",
		},
		"message": "success",
	})
	require.NoError(t, database.Create(task).Error)

	engine := gin.New()
	SetVideoRouter(engine)
	SetTaskPluginProtocolRouter(engine)
	request := httptest.NewRequest(http.MethodGet, "/v1/videos/task_jimeng_public", nil)
	request.Header.Set("Authorization", "Bearer sk-jimengfetch")
	recorder := httptest.NewRecorder()

	engine.ServeHTTP(recorder, request)

	require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	var response struct {
		ID          string `json:"id"`
		Object      string `json:"object"`
		Status      string `json:"status"`
		Progress    int    `json:"progress"`
		CreatedAt   int64  `json:"created_at"`
		CompletedAt int64  `json:"completed_at"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	assert.Equal(t, "task_jimeng_public", response.ID)
	assert.Equal(t, "video", response.Object)
	assert.Equal(t, "completed", response.Status)
	assert.Equal(t, 100, response.Progress)
	assert.Equal(t, int64(1710000000), response.CreatedAt)
	assert.Equal(t, int64(1710000060), response.CompletedAt)
	assert.NotContains(t, recorder.Body.String(), "cdn.example")
	assert.NotContains(t, recorder.Body.String(), "jimeng-private-1")

	for _, testCase := range []struct {
		name          string
		authorization string
		query         string
		wantStatus    int
	}{
		{name: "missing credential rejected", wantStatus: http.StatusUnauthorized},
		{name: "access rejected", query: "?access=not-a-video-credential", wantStatus: http.StatusUnauthorized},
		{name: "bearer accepted", authorization: "Bearer sk-jimengfetch", wantStatus: http.StatusOK},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(
				http.MethodGet,
				"/v1/videos/task_jimeng_public/content"+testCase.query,
				nil,
			)
			if testCase.authorization != "" {
				request.Header.Set("Authorization", testCase.authorization)
			}
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, request)
			assert.Equal(t, testCase.wantStatus, recorder.Code, recorder.Body.String())
			if testCase.wantStatus == http.StatusOK {
				assert.Equal(t, "data", recorder.Body.String())
			}
		})
	}
}
