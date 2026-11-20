package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type taskPollingFetchAdaptor struct {
	mu           sync.Mutex
	initInfo     *relaycommon.RelayInfo
	taskIDs      []string
	models       []string
	keys         []string
	fetched      chan string
	beforeFetch  chan string
	releaseFetch chan struct{}
	blockTaskID  string
	blockStarted chan struct{}
	releaseBlock chan struct{}
	blockOnce    sync.Once
}

type batchPollingAdaptor struct {
	taskPollingFetchAdaptor
	batchCalls int
	batchIDs   []string
	results    map[string]*BatchTaskResult
}

func (a *batchPollingAdaptor) FetchMode() string { return "batch" }
func (a *batchPollingAdaptor) FetchBatchTasks(_ string, _ string, taskIDs []string, _ string) (*http.Response, error) {
	a.batchCalls++
	a.batchIDs = append([]string(nil), taskIDs...)
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte(`{}`)))}, nil
}
func (a *batchPollingAdaptor) ParseBatchResult([]byte) (map[string]*BatchTaskResult, error) {
	if a.results != nil {
		return a.results, nil
	}
	results := make(map[string]*BatchTaskResult, len(a.batchIDs))
	for _, taskID := range a.batchIDs {
		results[taskID] = &BatchTaskResult{TaskInfo: relaycommon.TaskInfo{TaskID: taskID, Status: model.TaskStatusInProgress, Url: "https://example.com/result"}}
	}
	return results, nil
}

func (a *taskPollingFetchAdaptor) Init(info *relaycommon.RelayInfo) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.initInfo = info
}
func (a *taskPollingFetchAdaptor) initChannelMeta() *relaycommon.ChannelMeta {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.initInfo == nil {
		return nil
	}
	return a.initInfo.ChannelMeta
}

func (a *taskPollingFetchAdaptor) FetchTask(_ string, key string, body map[string]any, _ string) (*http.Response, error) {
	taskID, _ := body["task_id"].(string)
	modelName, _ := body["model"].(string)
	if a.beforeFetch != nil {
		a.beforeFetch <- key
		<-a.releaseFetch
	}
	if taskID == a.blockTaskID && a.releaseBlock != nil {
		a.blockOnce.Do(func() {
			if a.blockStarted != nil {
				close(a.blockStarted)
			}
		})
		<-a.releaseBlock
	}

	a.mu.Lock()
	a.taskIDs = append(a.taskIDs, taskID)
	a.models = append(a.models, modelName)
	a.keys = append(a.keys, key)
	a.mu.Unlock()
	if a.fetched != nil {
		select {
		case a.fetched <- taskID:
		default:
		}
	}

	response := taskdto.TaskResponse[model.Task]{
		Code: taskdto.TaskSuccessCode,
		Data: model.Task{
			TaskID:   taskID,
			Status:   model.TaskStatusInProgress,
			Progress: "30%",
		},
	}
	responseBody, err := common.Marshal(response)
	if err != nil {
		return nil, err
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(responseBody)),
	}, nil
}

func (a *taskPollingFetchAdaptor) ParseTaskResult([]byte) (*relaycommon.TaskInfo, error) {
	return &relaycommon.TaskInfo{Status: model.TaskStatusInProgress}, nil
}

func (a *taskPollingFetchAdaptor) AdjustBillingOnComplete(_ *model.Task, _ *relaycommon.TaskInfo) int {
	return 0
}

func (a *taskPollingFetchAdaptor) fetchCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.taskIDs)
}

func (a *taskPollingFetchAdaptor) fetchedTaskIDs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.taskIDs...)
}

func (a *taskPollingFetchAdaptor) fetchedModels() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.models...)
}

func (a *taskPollingFetchAdaptor) fetchedKeys() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.keys...)
}

func TestRedactVideoResponseBodyPreservesPollingPayloadShape(t *testing.T) {
	rawVideo := strings.Repeat("a", 300)
	body, err := common.Marshal(map[string]any{
		"done": true,
		"name": "operations/provider-task",
		"response": map[string]any{
			"bytesBase64Encoded": "secret-bytes",
			"video":              rawVideo,
			"videos": []any{
				map[string]any{
					"bytesBase64Encoded": "other-secret-bytes",
					"mimeType":           "video/mp4",
					"uri":                "https://media.example/video.mp4",
				},
			},
		},
	})
	require.NoError(t, err)

	var stored map[string]any
	require.NoError(t, common.Unmarshal(redactVideoResponseBody(body), &stored))
	assert.Equal(t, true, stored["done"])
	assert.Equal(t, "operations/provider-task", stored["name"])
	response, ok := stored["response"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, response, "bytesBase64Encoded")
	assert.Equal(t, strings.Repeat("a", 256)+"...", response["video"])
	videos, ok := response["videos"].([]any)
	require.True(t, ok)
	require.Len(t, videos, 1)
	video, ok := videos[0].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, video, "bytesBase64Encoded")
	assert.Equal(t, "video/mp4", video["mimeType"])
	assert.Equal(t, "https://media.example/video.mp4", video["uri"])
}

func TestRedactVideoResponseBodyDropsInteractionInlineVideo(t *testing.T) {
	body := []byte(`{"id":"int_1","status":"completed","steps":[{"type":"model_output","content":[{"type":"video","mime_type":"video/mp4","data":"` + strings.Repeat("a", 1024) + `"},{"type":"text","text":"done"}]}]}`)

	var stored map[string]any
	require.NoError(t, common.Unmarshal(redactVideoResponseBody(body), &stored))
	content := stored["steps"].([]any)[0].(map[string]any)["content"].([]any)
	assert.Equal(t, map[string]any{"type": "video", "mime_type": "video/mp4"}, content[0])
	assert.Equal(t, "done", content[1].(map[string]any)["text"])
}

func TestRedactVideoResponseBodyKeepsUntouchedBytes(t *testing.T) {
	body := []byte(`{"name":"op","response":{"videos":[{"uri":"u","bytesBase64Encoded":"x"}],"durationSeconds":8.0,"bytesBase64Encoded":"y"},"done":true}`)

	assert.Equal(t, `{"name":"op","response":{"videos":[{"uri":"u"}],"durationSeconds":8.0},"done":true}`, string(redactVideoResponseBody(body)))
}

func seedTaskPollingChannel(t *testing.T, id int, disableSleep bool) {
	t.Helper()
	ch := &model.Channel{
		Id:     id,
		Type:   constant.ChannelTypeKling,
		Name:   "polling_channel",
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
	}
	if disableSleep {
		ch.SetOtherSettings(dto.ChannelOtherSettings{DisableTaskPollingSleep: true})
	}
	require.NoError(t, model.DB.Create(ch).Error)
}

func seedPollingTask(t *testing.T, channelID int, publicID string, upstreamID string) *model.Task {
	t.Helper()
	task := &model.Task{
		TaskID:    publicID,
		Platform:  constant.TaskPlatform("kling"),
		UserId:    1,
		ChannelId: channelID,
		Action:    constant.TaskActionImageToVideo,
		Status:    model.TaskStatusInProgress,
		Progress:  "30%",
		CreatedAt: time.Now().Unix(),
		UpdatedAt: time.Now().Unix(),
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID: upstreamID,
		},
	}
	require.NoError(t, model.DB.Create(task).Error)
	return task
}

func TestUpdateVideoTasksDefaultSleepWaitsBetweenTasks(t *testing.T) {
	truncate(t)

	const channelID = 101
	seedTaskPollingChannel(t, channelID, false)
	first := seedPollingTask(t, channelID, "task_public_1", "upstream_1")
	second := seedPollingTask(t, channelID, "task_public_2", "upstream_2")

	adaptor := &taskPollingFetchAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := UpdateVideoTasks(ctx, constant.TaskPlatform("kling"), map[int][]string{
		channelID: {
			first.GetUpstreamTaskID(),
			second.GetUpstreamTaskID(),
		},
	}, map[string]*model.Task{
		first.GetUpstreamTaskID():  first,
		second.GetUpstreamTaskID(): second,
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Equal(t, 1, adaptor.fetchCount())
}

func TestPollingPassesExecutingChannelTypeToAdaptor(t *testing.T) {
	truncate(t)
	const channelID = 112
	baseURL := "https://gateway.example"
	gateway := &model.Channel{Id: channelID, Type: constant.ChannelTypeNewAPI, Name: "gateway", Key: "sk-gateway", Status: common.ChannelStatusEnabled, BaseURL: &baseURL}
	gateway.HeaderOverride = common.GetPointer(`{"X-API-Key":"{api_key}"}`)
	gateway.SetOtherSettings(dto.ChannelOtherSettings{DisableTaskPollingSleep: true})
	require.NoError(t, model.DB.Create(gateway).Error)
	task := seedPollingTask(t, channelID, "task_gateway", "upstream_gateway")
	taskChannels := map[int][]string{channelID: {task.GetUpstreamTaskID()}}
	tasks := map[string]*model.Task{task.GetUpstreamTaskID(): task}

	perTask := &taskPollingFetchAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return perTask }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })
	require.NoError(t, UpdateVideoTasks(context.Background(), constant.TaskPlatform("kling"), taskChannels, tasks))
	meta := perTask.initChannelMeta()
	require.NotNil(t, meta, "per-task polling initializes the adaptor with channel metadata")
	assert.Equal(t, constant.ChannelTypeNewAPI, meta.ChannelType, "the adaptor derives the upstream kind from the channel type")
	assert.Equal(t, channelID, meta.ChannelId)
	assert.Equal(t, baseURL, meta.ChannelBaseUrl)
	assert.Equal(t, map[string]any{"X-API-Key": "{api_key}"}, meta.HeadersOverride)

	batch := &batchPollingAdaptor{}
	require.NoError(t, UpdateBatchTasks(context.Background(), batch, taskChannels, tasks))
	meta = batch.initChannelMeta()
	require.NotNil(t, meta, "batch polling initializes the adaptor with channel metadata")
	assert.Equal(t, constant.ChannelTypeNewAPI, meta.ChannelType)
	assert.Equal(t, channelID, meta.ChannelId)
	assert.Equal(t, baseURL, meta.ChannelBaseUrl)
	assert.Equal(t, map[string]any{"X-API-Key": "{api_key}"}, meta.HeadersOverride)
}

func TestDispatchPlatformUpdateUsesFetchMode(t *testing.T) {
	truncate(t)
	const channelID = 109
	seedTaskPollingChannel(t, channelID, true)
	task := seedPollingTask(t, channelID, "task_batch", "upstream_batch")
	taskChannels := map[int][]string{channelID: {task.GetUpstreamTaskID()}}
	tasks := map[string]*model.Task{task.GetUpstreamTaskID(): task}

	batch := &batchPollingAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return batch }
	DispatchPlatformUpdate(context.Background(), "batch-plugin", taskChannels, tasks)
	assert.Equal(t, 1, batch.batchCalls)
	assert.Equal(t, 0, batch.fetchCount())
	var persisted model.Task
	require.NoError(t, model.DB.First(&persisted, task.ID).Error)
	assert.Equal(t, "https://example.com/result", persisted.GetResultURL())

	perTask := &taskPollingFetchAdaptor{}
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return perTask }
	DispatchPlatformUpdate(context.Background(), "per-task-plugin", taskChannels, tasks)
	assert.Equal(t, 1, perTask.fetchCount())

	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return nil }
	assert.NotPanics(t, func() { DispatchPlatformUpdate(context.Background(), "missing-plugin", taskChannels, tasks) })
	GetTaskAdaptorFunc = previousFactory
}

func TestDispatchPlatformUpdatePassesUpstreamModelToFetcher(t *testing.T) {
	truncate(t)

	const channelID = 110
	seedTaskPollingChannel(t, channelID, true)
	task := seedPollingTask(t, channelID, "task_public_model", "upstream_model")
	task.Properties = model.Properties{
		UpstreamModelName: "agnes-video-2.5-flash",
		OriginModelName:   "agnes-video-2.5-flash",
	}
	require.NoError(t, task.Update())

	adaptor := &taskPollingFetchAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	DispatchPlatformUpdate(context.Background(), constant.TaskPlatform("agnes"), map[int][]string{
		channelID: {task.GetUpstreamTaskID()},
	}, map[string]*model.Task{task.GetUpstreamTaskID(): task})

	require.Equal(t, []string{"agnes-video-2.5-flash"}, adaptor.fetchedModels())
}

func TestAgnesMultiKeyPollingUsesPersistedOrSingleFallbackKey(t *testing.T) {
	testCases := []struct {
		name         string
		persistedKey string
		loggedIndex  int
		hasLogIndex  bool
		wantKey      string
	}{
		{name: "persisted submit key", persistedKey: "agnes-selected", wantKey: "agnes-selected"},
		{name: "legacy task log index", loggedIndex: 1, hasLogIndex: true, wantKey: "agnes-second"},
		{name: "legacy task fallback", wantKey: "agnes-first"},
	}

	for index, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			truncate(t)
			channelID := 120 + index
			ch := &model.Channel{
				Id:     channelID,
				Type:   constant.ChannelTypeAgnes,
				Name:   "agnes_multi_key",
				Key:    "agnes-first\nagnes-second",
				Status: common.ChannelStatusEnabled,
				ChannelInfo: model.ChannelInfo{
					IsMultiKey:   true,
					MultiKeySize: 2,
				},
			}
			require.NoError(t, model.DB.Create(ch).Error)
			task := seedPollingTask(t, channelID, "task_public_agnes", "task_upstream_agnes")
			task.Platform = constant.TaskPlatform("63")
			task.PrivateData.Key = testCase.persistedKey
			task.PrivateData.Execution = &model.TaskExecutionSnapshot{RequestID: "agnes-request"}
			require.NoError(t, task.Update())
			if testCase.hasLogIndex {
				other, err := common.Marshal(map[string]any{
					"admin_info": map[string]any{"multi_key_index": testCase.loggedIndex},
				})
				require.NoError(t, err)
				require.NoError(t, model.LOG_DB.Create(&model.Log{
					RequestId: "agnes-request",
					ChannelId: channelID,
					Type:      model.LogTypeConsume,
					Other:     string(other),
				}).Error)
			}

			adaptor := &taskPollingFetchAdaptor{}
			previousFactory := GetTaskAdaptorFunc
			GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
			t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

			DispatchPlatformUpdate(context.Background(), task.Platform, map[int][]string{
				channelID: {task.GetUpstreamTaskID()},
			}, map[string]*model.Task{task.GetUpstreamTaskID(): task})

			require.Equal(t, []string{testCase.wantKey}, adaptor.fetchedKeys())
		})
	}
}

func TestUpdateBatchTasksSettlesTieredUsageForTerminalStates(t *testing.T) {
	testCases := []struct {
		name        string
		status      model.TaskStatus
		units       float64
		actualQuota int
	}{
		{name: "success with usage", status: model.TaskStatusSuccess, units: 3, actualQuota: 3_000},
		{name: "failure with usage", status: model.TaskStatusFailure, units: 3, actualQuota: 0},
		{name: "success with zero usage", status: model.TaskStatusSuccess, units: 0, actualQuota: 0},
		{name: "failure with zero usage", status: model.TaskStatusFailure, units: 0, actualQuota: 0},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			truncate(t)

			const userID, tokenID, channelID = 41, 41, 141
			const initialQuota, preConsumedQuota = 10_000, 5_000
			const tokenRemain = 8_000
			seedUser(t, userID, initialQuota)
			seedToken(t, tokenID, userID, "sk-batch-tiered", tokenRemain)
			seedTaskPollingChannel(t, channelID, true)

			expression := `tier("actual", u("units"))`
			task := makeTask(userID, channelID, preConsumedQuota, tokenID, BillingSourceWallet, 0)
			task.TaskID = "task_batch_tiered_" + string(testCase.status)
			task.Platform = "batch-plugin"
			task.PrivateData.UpstreamTaskID = "upstream_batch_tiered_" + string(testCase.status)
			task.SetData(map[string]any{"provider_payload": "must-be-preserved"})
			task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{
				ExprString:       expression,
				ExprHash:         billingexpr.ExprHashString(expression),
				GroupRatio:       1,
				QuotaPerUnit:     1_000,
				ExprVersion:      1,
				TaskUsageBilling: true,
			}
			require.NoError(t, model.DB.Create(task).Error)

			upstreamID := task.GetUpstreamTaskID()
			reason := ""
			if testCase.status == model.TaskStatusFailure {
				reason = "upstream failed"
			}
			result := &BatchTaskResult{TaskInfo: relaycommon.TaskInfo{
				TaskID:     upstreamID,
				Status:     string(testCase.status),
				Reason:     reason,
				UsageFacts: map[string]any{"units": testCase.units},
			}}
			adaptor := &batchPollingAdaptor{results: map[string]*BatchTaskResult{upstreamID: result}}
			taskIDs := []string{upstreamID}
			taskMap := map[string]*model.Task{upstreamID: task}

			require.NoError(t, UpdateBatchTasks(context.Background(), adaptor, map[int][]string{channelID: taskIDs}, taskMap))

			var persisted model.Task
			require.NoError(t, model.DB.First(&persisted, task.ID).Error)
			assert.Equal(t, testCase.status, persisted.Status)
			assert.Equal(t, testCase.actualQuota, persisted.Quota)
			var persistedData map[string]any
			require.NoError(t, common.Unmarshal(persisted.Data, &persistedData))
			assert.Equal(t, "must-be-preserved", persistedData["provider_payload"])
			assert.Equal(t, initialQuota+(preConsumedQuota-testCase.actualQuota), getUserQuota(t, userID))
			assert.Equal(t, tokenRemain+(preConsumedQuota-testCase.actualQuota), getTokenRemainQuota(t, tokenID))
			assert.Equal(t, int64(1), countLogs(t))
			if testCase.status == model.TaskStatusFailure {
				log := getLastLog(t)
				require.NotNil(t, log)
				assert.Equal(t, model.LogTypeRefund, log.Type)
			}

			// A duplicate terminal response must not settle the same task twice.
			require.NoError(t, UpdateBatchTasks(context.Background(), adaptor, map[int][]string{channelID: taskIDs}, taskMap))
			assert.Equal(t, initialQuota+(preConsumedQuota-testCase.actualQuota), getUserQuota(t, userID))
			assert.Equal(t, int64(1), countLogs(t))
		})
	}
}

func TestUpdateBatchTasksRefundsFailedTieredTask(t *testing.T) {
	truncate(t)

	const userID, tokenID, channelID = 43, 43, 143
	const initialQuota, preConsumedQuota, tokenRemain = 10_000, 5_000, 8_000
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-batch-tiered-refund", tokenRemain)
	seedTaskPollingChannel(t, channelID, true)

	expression := `tier("actual", u("units"))`
	task := makeTask(userID, channelID, preConsumedQuota, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_batch_tiered_refund"
	task.Platform = "batch-plugin"
	task.PrivateData.UpstreamTaskID = "upstream_batch_tiered_refund"
	task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{
		ExprString:       expression,
		ExprHash:         billingexpr.ExprHashString(expression),
		GroupRatio:       1,
		QuotaPerUnit:     1_000,
		ExprVersion:      1,
		TaskUsageBilling: true,
		UsageFacts:       map[string]any{"units": float64(5)},
		EstimatedTier:    "actual",
	}
	require.NoError(t, model.DB.Create(task).Error)

	upstreamID := task.GetUpstreamTaskID()
	adaptor := &batchPollingAdaptor{results: map[string]*BatchTaskResult{
		upstreamID: {TaskInfo: relaycommon.TaskInfo{
			TaskID:     upstreamID,
			Status:     model.TaskStatusFailure,
			Reason:     "upstream failed",
			UsageFacts: map[string]any{"units": float64(5)},
		}},
	}}
	require.NoError(t, UpdateBatchTasks(context.Background(), adaptor, map[int][]string{channelID: {upstreamID}}, map[string]*model.Task{upstreamID: task}))

	var persisted model.Task
	require.NoError(t, model.DB.First(&persisted, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, persisted.Status)
	assert.Zero(t, persisted.Quota)
	assert.Equal(t, initialQuota+preConsumedQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumedQuota, getTokenRemainQuota(t, tokenID))

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
	assert.Equal(t, preConsumedQuota, log.Quota)
	var other map[string]any
	require.NoError(t, common.UnmarshalJsonStr(log.Other, &other))
	assert.Equal(t, "tiered_expr", other["billing_mode"])
	assert.Equal(t, "actual", other["matched_tier"])
	facts, ok := other["usage_facts"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, map[string]any{"units": float64(5)}, facts)
}

func TestUpdateBatchTasksRefundsFailedTaskWithoutUsageSettlement(t *testing.T) {
	truncate(t)

	const userID, tokenID, channelID = 42, 42, 142
	const initialQuota, preConsumedQuota, tokenRemain = 10_000, 4_000, 7_000
	seedUser(t, userID, initialQuota)
	seedToken(t, tokenID, userID, "sk-batch-refund", tokenRemain)
	seedTaskPollingChannel(t, channelID, true)

	task := makeTask(userID, channelID, preConsumedQuota, tokenID, BillingSourceWallet, 0)
	task.TaskID = "task_batch_refund"
	task.Platform = "batch-plugin"
	task.Properties.OriginModelName = "missing-batch-token-price"
	task.PrivateData.UpstreamTaskID = "upstream_batch_refund"
	task.PrivateData.BillingContext.OriginModelName = "missing-batch-token-price"
	require.NoError(t, model.DB.Create(task).Error)

	upstreamID := task.GetUpstreamTaskID()
	adaptor := &batchPollingAdaptor{results: map[string]*BatchTaskResult{
		upstreamID: {TaskInfo: relaycommon.TaskInfo{TaskID: upstreamID, Status: model.TaskStatusFailure, Reason: "upstream failed", TotalTokens: 123}},
	}}
	require.NoError(t, UpdateBatchTasks(context.Background(), adaptor, map[int][]string{channelID: {upstreamID}}, map[string]*model.Task{upstreamID: task}))

	assert.Equal(t, initialQuota+preConsumedQuota, getUserQuota(t, userID))
	assert.Equal(t, tokenRemain+preConsumedQuota, getTokenRemainQuota(t, tokenID))
	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeRefund, log.Type)
}

func TestUpdateVideoTasksCanSkipPollingSleepPerChannel(t *testing.T) {
	truncate(t)

	const channelID = 102
	seedTaskPollingChannel(t, channelID, true)
	first := seedPollingTask(t, channelID, "task_public_3", "upstream_3")
	second := seedPollingTask(t, channelID, "task_public_4", "upstream_4")

	adaptor := &taskPollingFetchAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := UpdateVideoTasks(ctx, constant.TaskPlatform("kling"), map[int][]string{
		channelID: {
			first.GetUpstreamTaskID(),
			second.GetUpstreamTaskID(),
		},
	}, map[string]*model.Task{
		first.GetUpstreamTaskID():  first,
		second.GetUpstreamTaskID(): second,
	})

	require.NoError(t, err)
	assert.Equal(t, 2, adaptor.fetchCount())
}

func TestUpdateVideoTasksDefaultSleepDoesNotBlockOtherChannels(t *testing.T) {
	truncate(t)

	const firstChannelID = 201
	const secondChannelID = 202
	seedTaskPollingChannel(t, firstChannelID, false)
	seedTaskPollingChannel(t, secondChannelID, false)
	firstChannelFirst := seedPollingTask(t, firstChannelID, "task_public_5", "upstream_a_1")
	firstChannelSecond := seedPollingTask(t, firstChannelID, "task_public_6", "upstream_a_2")
	secondChannelFirst := seedPollingTask(t, secondChannelID, "task_public_7", "upstream_b_1")
	secondChannelSecond := seedPollingTask(t, secondChannelID, "task_public_8", "upstream_b_2")

	adaptor := &taskPollingFetchAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := UpdateVideoTasks(ctx, constant.TaskPlatform("kling"), map[int][]string{
		firstChannelID: {
			firstChannelFirst.GetUpstreamTaskID(),
			firstChannelSecond.GetUpstreamTaskID(),
		},
		secondChannelID: {
			secondChannelFirst.GetUpstreamTaskID(),
			secondChannelSecond.GetUpstreamTaskID(),
		},
	}, map[string]*model.Task{
		firstChannelFirst.GetUpstreamTaskID():   firstChannelFirst,
		firstChannelSecond.GetUpstreamTaskID():  firstChannelSecond,
		secondChannelFirst.GetUpstreamTaskID():  secondChannelFirst,
		secondChannelSecond.GetUpstreamTaskID(): secondChannelSecond,
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ElementsMatch(t, []string{"upstream_a_1", "upstream_b_1"}, adaptor.fetchedTaskIDs())
}

type terminalQuotaAdaptor struct {
	taskPollingFetchAdaptor
	actual int
}

func (a *terminalQuotaAdaptor) AdjustBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int {
	return a.actual
}

func TestSubscriptionExtraCreditsTerminalRefundSkipsSubmissionTopup(t *testing.T) {
	truncate(t)
	user := model.User{Username: "terminal-direct-refund", AffCode: "terminal-direct-refund", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Direct refund", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	id := "terminal-direct-refund"
	receipt, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
	target := 15
	task := makeTask(user.Id, 0, 10, 0, BillingSourceSubscription, sub.Id)
	task.TaskID = id
	task.Status = model.TaskStatusFailure
	task.SubscriptionBillingPending = true
	task.PrivateData.SubscriptionRequestID = id
	task.PrivateData.SubscriptionReceiptId = receipt.ReceiptId
	task.PrivateData.SubscriptionSubmitQuota = &target
	task.PrivateData.SubscriptionFinalBilling = &model.SubscriptionTaskFinalBilling{Quota: 0}
	require.NoError(t, model.DB.Create(task).Error)
	finalizeTerminalTask(t.Context(), nil, task, relaycommon.FailTaskInfo("failed"))
	require.NoError(t, model.DB.First(task, task.ID).Error)
	require.False(t, task.SubscriptionBillingPending, "a fixed refund never requires a higher submission debit")
	require.Zero(t, task.Quota)
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	require.Zero(t, sub.ExtraCreditsUsed)
	require.Zero(t, sub.FundingReserved)
}

func TestSubscriptionExtraCreditsTerminalRecoveryWaitsForAdaptor(t *testing.T) {
	truncate(t)
	user := model.User{Username: "terminal-no-adaptor", AffCode: "terminal-no-adaptor", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Recovery waits", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	id := "terminal-no-adaptor"
	receipt, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 6, "vip")
	require.NoError(t, err)
	task := makeTask(user.Id, 0, 6, 0, BillingSourceSubscription, sub.Id)
	task.TaskID = id
	task.Status = model.TaskStatusSuccess
	task.SubscriptionBillingPending = true
	task.PrivateData.SubscriptionRequestID = id
	task.PrivateData.SubscriptionReceiptId = receipt.ReceiptId
	require.NoError(t, model.DB.Create(task).Error)
	finalizeTerminalTask(t.Context(), nil, task, &relaycommon.TaskInfo{Status: model.TaskStatusSuccess})
	require.NoError(t, model.DB.First(task, task.ID).Error)
	require.True(t, task.SubscriptionBillingPending, "missing adaptor cannot decide an unknown final price")
	require.Nil(t, task.PrivateData.SubscriptionFinalBilling)
	finalizeTerminalTask(t.Context(), &terminalQuotaAdaptor{actual: 3}, task, &relaycommon.TaskInfo{Status: model.TaskStatusSuccess})
	require.NoError(t, model.DB.First(task, task.ID).Error)
	require.False(t, task.SubscriptionBillingPending)
	require.Equal(t, 3, task.Quota)
}

func TestSubscriptionExtraCreditsLocalFailureKeepsRefundDecision(t *testing.T) {
	truncate(t)
	user := model.User{Username: "terminal-local-failure", AffCode: "terminal-local-failure", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Local refund", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	id := "terminal-local-failure"
	receipt, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 6, "vip")
	require.NoError(t, err)
	task := makeTask(user.Id, 0, 6, 0, BillingSourceSubscription, sub.Id)
	task.TaskID = id
	task.PrivateData.SubscriptionRequestID = id
	task.PrivateData.SubscriptionReceiptId = receipt.ReceiptId
	task.SubscriptionBillingPending = true
	expression := `tier("short", u("seconds"))`
	task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{ExprString: expression, EstimatedTier: "short"}
	oldTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 1
	t.Cleanup(func() { constant.TaskTimeoutMinutes = oldTimeout })
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_local_refund BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT,'temporary settlement failure'); END`).Error)
	sweepTimedOutTasks(t.Context())
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_local_refund").Error)
	require.NoError(t, model.DB.First(task, task.ID).Error)
	require.True(t, task.SubscriptionBillingPending)
	require.NotNil(t, task.PrivateData.SubscriptionFinalBilling, "local refund must be pinned with the terminal status")
	finalizeTerminalTask(t.Context(), &terminalQuotaAdaptor{actual: 3}, task, relaycommon.FailTaskInfo(task.FailReason))
	require.NoError(t, model.DB.First(task, task.ID).Error)
	require.False(t, task.SubscriptionBillingPending)
	require.Zero(t, task.Quota, "recovering a local failure cannot charge an adaptor's different decision")
	require.NoError(t, model.FlushSubscriptionTaskBillingLogs())
	var refund model.Log
	require.NoError(t, model.LOG_DB.Where("type = ? AND user_id = ?", model.LogTypeRefund, user.Id).First(&refund).Error)
	assert.Equal(t, 6, refund.Quota)
	var other map[string]any
	require.NoError(t, common.Unmarshal([]byte(refund.Other), &other))
	assert.Equal(t, "Task timed out", other["reason"])
	assert.Equal(t, 0.02, other["model_price"])
	assert.Equal(t, 1.0, other["group_ratio"])
	assert.Equal(t, base64.StdEncoding.EncodeToString([]byte(expression)), other["expr_b64"])
	assert.Equal(t, "short", other["matched_tier"])
}

func TestSubscriptionExtraCreditsTerminalRetriesUnderContinuousArrivals(t *testing.T) {
	truncate(t)
	subscriptionBillingRetryCursor.Store(0)
	t.Cleanup(func() { subscriptionBillingRetryCursor.Store(0) })
	old := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return &terminalQuotaAdaptor{} }
	t.Cleanup(func() { GetTaskAdaptorFunc = old })
	user := model.User{Username: "fair-retry", AffCode: "fair-retry", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Fair recovery", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	blocked := make([]model.Task, 50)
	for i := range blocked {
		blocked[i] = model.Task{Status: model.TaskStatusSuccess, SubscriptionBillingPending: true}
	}
	require.NoError(t, model.DB.Create(&blocked).Error)
	id := "fair-retry"
	receipt, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	task := makeTask(user.Id, 0, 5, 0, BillingSourceSubscription, sub.Id)
	task.Status = model.TaskStatusSuccess
	task.SubscriptionBillingPending = true
	task.PrivateData.SubscriptionRequestID = id
	task.PrivateData.SubscriptionReceiptId = receipt.ReceiptId
	task.PrivateData.BillingContext.PerCallBilling = true
	require.NoError(t, model.DB.Create(task).Error)
	addRows := func() {
		rows := make([]model.Task, 50)
		for i := range rows {
			rows[i] = model.Task{Status: model.TaskStatusSuccess, SubscriptionBillingPending: true}
		}
		require.NoError(t, model.DB.Create(&rows).Error)
	}
	addRows()
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_fair_retry BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT,'temporary settlement failure'); END`).Error)
	RunTaskPollingOnce(t.Context(), nil)
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_fair_retry").Error)
	for range 3 {
		addRows()
		RunTaskPollingOnce(t.Context(), nil)
	}
	require.NoError(t, model.DB.First(task, task.ID).Error)
	require.False(t, task.SubscriptionBillingPending, "new full pages cannot indefinitely postpone a previous failed receipt")
}

func TestSubscriptionExtraCreditsTerminalPreparation(t *testing.T) {
	for _, scenario := range []string{"direct", "retry", "per-call", "invalid-expression", "no-usage"} {
		t.Run(scenario, func(t *testing.T) {
			retry := scenario == "retry" || scenario == "no-usage"
			want := 3
			truncate(t)
			require.NoError(t, model.DB.AutoMigrate(&model.QuotaData{}))
			user := model.User{Username: "terminal-preparation", AffCode: "terminal-preparation", Group: "default", Status: common.UserStatusEnabled}
			require.NoError(t, model.DB.Create(&user).Error)
			plan := model.SubscriptionPlan{Title: "Final retry", SubscriptionGroup: "vip"}
			require.NoError(t, model.DB.Create(&plan).Error)
			sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 20, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
			require.NoError(t, model.DB.Create(&sub).Error)
			id := fmt.Sprintf("terminal-prepare-%t", retry)
			_, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 10, "vip")
			require.NoError(t, err)
			token := model.Token{UserId: user.Id, Key: id, RemainQuota: 90, UsedQuota: 10}
			require.NoError(t, model.DB.Create(&token).Error)
			target := 6
			facts := relaycommon.TaskInfo{Status: model.TaskStatusFailure, TotalTokens: 3}
			task := model.Task{TaskID: id, UserId: user.Id, Quota: 10, Status: model.TaskStatusFailure, SubscriptionBillingPending: true, PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: sub.Id, SubscriptionRequestID: id, TokenId: token.Id, SubscriptionSubmitQuota: &target, SubscriptionSubmitLog: &model.RecordTaskBillingLogParams{ModelName: "model", Other: model.NewLogOther()}}}
			if scenario == "per-call" {
				task.Status = model.TaskStatusSuccess
				facts.Status = model.TaskStatusSuccess
				task.PrivateData.BillingContext = &model.TaskBillingContext{PerCallBilling: true}
				want = 6
			}
			if scenario == "invalid-expression" {
				task.Status = model.TaskStatusSuccess
				facts.Status = model.TaskStatusSuccess
				expression := `tier("bad", -1)`
				task.PrivateData.BillingContext = &model.TaskBillingContext{TieredSnapshot: &billingexpr.BillingSnapshot{ExprString: expression, ExprHash: billingexpr.ExprHashString(expression), GroupRatio: 1, QuotaPerUnit: 1, ExprVersion: 1, TaskUsageBilling: true}}
				want = 6
			}
			adaptor := &terminalQuotaAdaptor{actual: 3}
			if scenario == "no-usage" {
				task.Status = model.TaskStatusSuccess
				facts = relaycommon.TaskInfo{Status: model.TaskStatusSuccess}
				adaptor.actual = 0
				want = target
			}
			task.PrivateData.SubscriptionFinalResult, err = common.Marshal(facts)
			require.NoError(t, err)
			require.NoError(t, model.DB.Create(&task).Error)
			if retry {
				old := GetTaskAdaptorFunc
				GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
				t.Cleanup(func() { GetTaskAdaptorFunc = old })
				RunTaskPollingOnce(t.Context(), nil)
			} else {
				finalizeTerminalTask(t.Context(), adaptor, &task, &facts)
			}
			require.NoError(t, model.DB.First(&task, task.ID).Error)
			require.False(t, task.SubscriptionBillingPending)
			require.Equal(t, want, task.Quota)
			require.NoError(t, model.DB.First(&token, token.Id).Error)
			require.Equal(t, 100-want, token.RemainQuota)
			require.NoError(t, model.DB.First(&user, user.Id).Error)
			require.Equal(t, want, user.UsedQuota)
			require.Equal(t, 1, user.RequestCount)
		})
	}
}

func TestSubscriptionExtraCreditsTerminalRetryPassesBlockedBatch(t *testing.T) {
	truncate(t)
	old := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return &taskPollingFetchAdaptor{} }
	t.Cleanup(func() { GetTaskAdaptorFunc = old })
	user := model.User{Username: "retry-batch-user", AffCode: "retry-batch-user", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Retry after blocked", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	blocked := make([]model.Task, 100)
	for i := range blocked {
		blocked[i] = model.Task{UserId: user.Id, Status: model.TaskStatusSuccess, SubscriptionBillingPending: true}
	}
	require.NoError(t, model.DB.Create(&blocked).Error)
	id := "retry-after-blocked-batch"
	_, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	task := model.Task{UserId: user.Id, TaskID: id, Status: model.TaskStatusSuccess, Quota: 5, SubscriptionBillingPending: true, PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: sub.Id, SubscriptionRequestID: id, BillingContext: &model.TaskBillingContext{PerCallBilling: true}}}
	require.NoError(t, model.DB.Create(&task).Error)
	RunTaskPollingOnce(t.Context(), nil)
	RunTaskPollingOnce(t.Context(), nil)
	require.NoError(t, model.DB.First(&task, task.ID).Error)
	require.False(t, task.SubscriptionBillingPending)
}

func TestSubscriptionExtraCreditsTerminalRetry(t *testing.T) {
	truncate(t)
	oldFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return &taskPollingFetchAdaptor{} }
	t.Cleanup(func() { GetTaskAdaptorFunc = oldFactory })
	user := model.User{Username: "retry-extra-user", AffCode: "retry-extra-user", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Retry terminal", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	id := "terminal-retry-extra"
	_, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	task := model.Task{UserId: user.Id, TaskID: id, Status: model.TaskStatusSuccess, Quota: 5, SubscriptionBillingPending: true, PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: sub.Id, SubscriptionRequestID: id, BillingContext: &model.TaskBillingContext{PerCallBilling: true}}}
	require.NoError(t, model.DB.Create(&task).Error)
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_retry_extra BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT,'temporary settlement failure'); END`).Error)
	t.Cleanup(func() { model.DB.Exec("DROP TRIGGER IF EXISTS reject_retry_extra") })
	finishTaskSubscriptionFunding(context.Background(), &task)
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	require.True(t, saved.SubscriptionBillingPending)
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_retry_extra").Error)
	RunTaskPollingOnce(context.Background(), nil)
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	require.False(t, saved.SubscriptionBillingPending, "normal polling retries terminal funding after a database outage")
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	require.Zero(t, sub.FundingReserved)
	require.EqualValues(t, 5, sub.ExtraCreditsUsed)
}

func TestSubscriptionExtraCreditsStaleFailureCannotRefundWinner(t *testing.T) {
	truncate(t)
	user := model.User{Username: "cas-extra-user", AffCode: "cas-extra-user", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "CAS refunds", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	id := "stale-refund-after-success"
	_, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	stale := model.Task{UserId: user.Id, TaskID: id, Status: model.TaskStatusInProgress, Quota: 5, PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: sub.Id, SubscriptionRequestID: id}}
	require.NoError(t, model.DB.Create(&stale).Error)
	require.NoError(t, model.DB.Model(&stale).UpdateColumn("status", model.TaskStatusSuccess).Error)
	reportFailedTasks([]*model.Task{&stale}, "channel temporarily missing")
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, stale.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusSuccess), saved.Status)
	require.Equal(t, 5, saved.Quota)
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 5, sub.ExtraCreditsUsed, "a stale failure observer cannot refund a success winner")
}

func TestRunTaskPollingOnceChannelsRemainConcurrentAndIsolated(t *testing.T) {
	truncate(t)
	oldLimit, oldTimeout := constant.TaskQueryLimit, constant.TaskTimeoutMinutes
	constant.TaskQueryLimit = 10
	constant.TaskTimeoutMinutes = 0
	t.Cleanup(func() { constant.TaskQueryLimit = oldLimit; constant.TaskTimeoutMinutes = oldTimeout })
	const firstChannel, secondChannel = 270, 271
	seedTaskPollingChannel(t, firstChannel, true)
	seedTaskPollingChannel(t, secondChannel, true)
	first := seedPollingTask(t, firstChannel, "concurrent-public-first", "same-upstream-id")
	second := seedPollingTask(t, secondChannel, "concurrent-public-second", "same-upstream-id")
	for i, task := range []*model.Task{first, second} {
		task.PrivateData.Key = fmt.Sprintf("fixture-key-%d", i)
		require.NoError(t, model.DB.Model(task).Updates(map[string]any{"private_data": task.PrivateData, "submit_time": time.Now().Unix()}).Error)
	}
	started := make(chan string, 2)
	release := make(chan struct{})
	oldFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor {
		return &taskPollingFetchAdaptor{beforeFetch: started, releaseFetch: release}
	}
	t.Cleanup(func() { GetTaskAdaptorFunc = oldFactory })
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	done := make(chan struct{})
	t.Cleanup(func() { unblock(); <-done })
	go func() { RunTaskPollingOnce(context.Background(), nil); close(done) }()
	var keys []string
	for range 2 {
		select {
		case key := <-started:
			keys = append(keys, key)
		case <-time.After(2 * time.Second):
			t.Fatal("one channel blocked the other channel from starting")
		}
	}
	require.ElementsMatch(t, []string{first.PrivateData.Key, second.PrivateData.Key}, keys, "same upstream id keeps each channel's credentials")
	unblock()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("polling did not finish")
	}
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, first.ID).Error)
	require.Equal(t, model.TaskStatus(model.TaskStatusInProgress), saved.Status)
}

func TestUpdateVideoTasksSlowChannelDoesNotBlockOtherChannels(t *testing.T) {
	truncate(t)

	const slowChannelID = 251
	const fastChannelID = 252
	seedTaskPollingChannel(t, slowChannelID, false)
	seedTaskPollingChannel(t, fastChannelID, true)
	slowTask := seedPollingTask(t, slowChannelID, "task_public_slow", "upstream_slow_1")
	fastFirst := seedPollingTask(t, fastChannelID, "task_public_fast_1", "upstream_fast_parallel_1")
	fastSecond := seedPollingTask(t, fastChannelID, "task_public_fast_2", "upstream_fast_parallel_2")

	adaptor := &taskPollingFetchAdaptor{
		fetched:      make(chan string, 4),
		blockTaskID:  slowTask.GetUpstreamTaskID(),
		blockStarted: make(chan struct{}),
		releaseBlock: make(chan struct{}),
	}
	var releaseOnce sync.Once
	releaseBlockedTask := func() {
		releaseOnce.Do(func() {
			close(adaptor.releaseBlock)
		})
	}
	t.Cleanup(releaseBlockedTask)
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	errCh := make(chan error, 1)
	gopool.Go(func() {
		errCh <- UpdateVideoTasks(context.Background(), constant.TaskPlatform("kling"), map[int][]string{
			slowChannelID: {
				slowTask.GetUpstreamTaskID(),
			},
			fastChannelID: {
				fastFirst.GetUpstreamTaskID(),
				fastSecond.GetUpstreamTaskID(),
			},
		}, map[string]*model.Task{
			slowTask.GetUpstreamTaskID():   slowTask,
			fastFirst.GetUpstreamTaskID():  fastFirst,
			fastSecond.GetUpstreamTaskID(): fastSecond,
		})
	})

	select {
	case <-adaptor.blockStarted:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("slow channel did not start blocking")
	}

	require.Eventually(t, func() bool {
		fetchedTaskIDs := adaptor.fetchedTaskIDs()
		return len(fetchedTaskIDs) == 2 &&
			fetchedTaskIDs[0] == fastFirst.GetUpstreamTaskID() &&
			fetchedTaskIDs[1] == fastSecond.GetUpstreamTaskID()
	}, 500*time.Millisecond, 10*time.Millisecond)

	releaseBlockedTask()
	require.NoError(t, <-errCh)
	assert.ElementsMatch(t, []string{
		slowTask.GetUpstreamTaskID(),
		fastFirst.GetUpstreamTaskID(),
		fastSecond.GetUpstreamTaskID(),
	}, adaptor.fetchedTaskIDs())
}

func TestUpdateVideoTasksMixedChannelSleepSettings(t *testing.T) {
	truncate(t)

	const sleepyChannelID = 301
	const fastChannelID = 302
	seedTaskPollingChannel(t, sleepyChannelID, false)
	seedTaskPollingChannel(t, fastChannelID, true)
	sleepyFirst := seedPollingTask(t, sleepyChannelID, "task_public_9", "upstream_sleepy_1")
	sleepySecond := seedPollingTask(t, sleepyChannelID, "task_public_10", "upstream_sleepy_2")
	fastFirst := seedPollingTask(t, fastChannelID, "task_public_11", "upstream_fast_1")
	fastSecond := seedPollingTask(t, fastChannelID, "task_public_12", "upstream_fast_2")

	adaptor := &taskPollingFetchAdaptor{}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := UpdateVideoTasks(ctx, constant.TaskPlatform("kling"), map[int][]string{
		sleepyChannelID: {
			sleepyFirst.GetUpstreamTaskID(),
			sleepySecond.GetUpstreamTaskID(),
		},
		fastChannelID: {
			fastFirst.GetUpstreamTaskID(),
			fastSecond.GetUpstreamTaskID(),
		},
	}, map[string]*model.Task{
		sleepyFirst.GetUpstreamTaskID():  sleepyFirst,
		sleepySecond.GetUpstreamTaskID(): sleepySecond,
		fastFirst.GetUpstreamTaskID():    fastFirst,
		fastSecond.GetUpstreamTaskID():   fastSecond,
	})

	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.ElementsMatch(t, []string{"upstream_sleepy_1", "upstream_fast_1", "upstream_fast_2"}, adaptor.fetchedTaskIDs())
}

func TestUpdateSunoTasksStalePollsRefundExactlyOnce(t *testing.T) {
	truncate(t)

	const userID, tokenID, channelID = 401, 401, 401
	const initialUserQuota, initialTokenQuota, taskQuota = 10_000, 6_000, 2_500
	const publicTaskID, upstreamTaskID = "suno_public_refund_once", "suno_upstream_refund_once"

	seedUser(t, userID, initialUserQuota)
	seedToken(t, tokenID, userID, "sk-suno-refund-once", initialTokenQuota)
	baseURL := "https://suno.invalid"
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:      channelID,
		Type:    constant.ChannelTypeSunoAPI,
		Name:    "suno_refund_once",
		Key:     "sk-suno-channel",
		Status:  common.ChannelStatusEnabled,
		BaseURL: &baseURL,
	}).Error)

	task := makeTask(userID, channelID, taskQuota, tokenID, BillingSourceWallet, 0)
	task.TaskID = publicTaskID
	task.Platform = constant.TaskPlatformSuno
	task.Status = model.TaskStatusInProgress
	task.Progress = "50%"
	task.SubmitTime = time.Now().Unix()
	task.PrivateData.UpstreamTaskID = upstreamTaskID
	require.NoError(t, model.DB.Create(task).Error)

	var firstPollTask model.Task
	var staleSecondPollTask model.Task
	require.NoError(t, model.DB.First(&firstPollTask, task.ID).Error)
	require.NoError(t, model.DB.First(&staleSecondPollTask, task.ID).Error)

	adaptor := &batchPollingAdaptor{results: map[string]*BatchTaskResult{
		upstreamTaskID: {TaskInfo: relaycommon.TaskInfo{
			TaskID: upstreamTaskID,
			Status: model.TaskStatusFailure,
			Reason: "upstream failed",
		}},
	}}
	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	require.NoError(t, updateBatchTasks(context.Background(), adaptor, channelID, []string{upstreamTaskID}, map[string]*model.Task{
		upstreamTaskID: &firstPollTask,
	}))
	require.NoError(t, updateBatchTasks(context.Background(), adaptor, channelID, []string{upstreamTaskID}, map[string]*model.Task{
		upstreamTaskID: &staleSecondPollTask,
	}))

	var reloaded model.Task
	require.NoError(t, model.DB.First(&reloaded, task.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, reloaded.Status)
	assert.Zero(t, reloaded.Quota)
	assert.Equal(t, initialUserQuota+taskQuota, getUserQuota(t, userID))
	assert.Equal(t, initialTokenQuota+taskQuota, getTokenRemainQuota(t, tokenID))
	assert.Equal(t, int64(1), countLogs(t))
}

func TestRunTaskPollingOnceDoesNotRefundHistoricalFailedTask(t *testing.T) {
	truncate(t)

	const userID, initialQuota, taskQuota = 402, 10_000, 1_200
	seedUser(t, userID, initialQuota)

	task := makeTask(userID, 0, taskQuota, 0, BillingSourceWallet, 0)
	task.TaskID = "historical_failed_already_refunded"
	task.Status = model.TaskStatusFailure
	task.Progress = "100%"
	task.SubmitTime = time.Now().Add(-90 * 24 * time.Hour).Unix()
	task.UpdatedAt = time.Now().Add(-time.Minute).Unix()
	require.NoError(t, model.DB.Create(task).Error)

	previousFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor {
		return &taskPollingFetchAdaptor{}
	}
	t.Cleanup(func() { GetTaskAdaptorFunc = previousFactory })

	summary := RunTaskPollingOnce(context.Background(), nil)

	assert.Zero(t, summary.UnfinishedTasks)
	assert.Equal(t, initialQuota, getUserQuota(t, userID))
	assert.Equal(t, taskQuota, getTaskQuota(t, task.ID))
	assert.Equal(t, int64(0), countLogs(t))
}

func TestSweepTimedOutTasksHonorsRefundRolloutBoundary(t *testing.T) {
	truncate(t)

	const (
		userID          = 403
		initialQuota    = 10_000
		legacyTaskQuota = 1_800
		modernTaskQuota = 1_200
	)
	seedUser(t, userID, initialQuota)

	legacyTask := makeTask(userID, 0, legacyTaskQuota, 0, BillingSourceWallet, 0)
	legacyTask.TaskID = "legacy_timeout_without_refund"
	legacyTask.Progress = "50%"
	legacyTask.SubmitTime = 1771718399 // 2026-02-21 23:59:59 UTC
	require.NoError(t, model.DB.Create(legacyTask).Error)

	modernTask := makeTask(userID, 0, modernTaskQuota, 0, BillingSourceWallet, 0)
	modernTask.TaskID = "modern_timeout_with_refund"
	modernTask.Progress = "50%"
	modernTask.SubmitTime = 1771718400 // 2026-02-22 00:00:00 UTC
	require.NoError(t, model.DB.Create(modernTask).Error)

	previousTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 1
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previousTimeout })

	sweepTimedOutTasks(context.Background())

	var reloadedLegacy model.Task
	var reloadedModern model.Task
	require.NoError(t, model.DB.First(&reloadedLegacy, legacyTask.ID).Error)
	require.NoError(t, model.DB.First(&reloadedModern, modernTask.ID).Error)
	assert.EqualValues(t, model.TaskStatusFailure, reloadedLegacy.Status)
	assert.EqualValues(t, model.TaskStatusFailure, reloadedModern.Status)
	assert.Zero(t, reloadedLegacy.Quota)
	assert.Zero(t, reloadedModern.Quota)
	assert.Contains(t, reloadedLegacy.FailReason, "old system")
	assert.Equal(t, "Task timed out", reloadedModern.FailReason)
	assert.Equal(t, initialQuota+modernTaskQuota, getUserQuota(t, userID))
	assert.Equal(t, int64(1), countLogs(t))
}

// TASK_TIMEOUT_MINUTES defaults to 72 hours, the longest a vendor keeps a
// task running (Ark can be configured to 72h, Vidu off-peak runs 48h).
func TestSweepTimedOutTasksAtTheDefaultDeadline(t *testing.T) {
	truncate(t)

	const userID, initialQuota, taskQuota = 404, 10_000, 1_500
	seedUser(t, userID, initialQuota)
	running := makeTask(userID, 0, taskQuota, 0, BillingSourceWallet, 0)
	running.TaskID = "running_for_60_hours"
	running.Progress = "50%"
	running.SubmitTime = time.Now().Add(-60 * time.Hour).Unix()
	require.NoError(t, model.DB.Create(running).Error)
	stuck := makeTask(userID, 0, taskQuota, 0, BillingSourceWallet, 0)
	stuck.TaskID = "running_for_73_hours"
	stuck.Progress = "50%"
	stuck.SubmitTime = time.Now().Add(-73 * time.Hour).Unix()
	require.NoError(t, model.DB.Create(stuck).Error)

	previousTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 4320
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previousTimeout })

	sweepTimedOutTasks(context.Background())

	var reloadedRunning, reloadedStuck model.Task
	require.NoError(t, model.DB.First(&reloadedRunning, running.ID).Error)
	require.NoError(t, model.DB.First(&reloadedStuck, stuck.ID).Error)
	assert.NotEqualValues(t, model.TaskStatusFailure, reloadedRunning.Status)
	assert.Equal(t, taskQuota, reloadedRunning.Quota)
	assert.EqualValues(t, model.TaskStatusFailure, reloadedStuck.Status)
	assert.Equal(t, "Task timed out", reloadedStuck.FailReason)
	assert.Equal(t, initialQuota+taskQuota, getUserQuota(t, userID))
}

type scriptedPollingAdaptor struct {
	statusCode int
	body       []byte
	fetchErr   error
	parse      *relaycommon.TaskInfo
	parseErr   error
}

func (a *scriptedPollingAdaptor) Init(*relaycommon.RelayInfo) {}
func (a *scriptedPollingAdaptor) FetchTask(string, string, *model.Task, string) (*http.Response, error) {
	if a.fetchErr != nil {
		return nil, a.fetchErr
	}
	code := a.statusCode
	if code == 0 {
		code = http.StatusOK
	}
	body := a.body
	if body == nil {
		body = []byte(`{}`)
	}
	return &http.Response{StatusCode: code, Body: io.NopCloser(bytes.NewReader(body))}, nil
}
func (a *scriptedPollingAdaptor) ParseTaskResult(*model.Task, *http.Response, []byte) (*relaycommon.TaskInfo, error) {
	if a.parseErr != nil {
		return nil, a.parseErr
	}
	if a.parse != nil {
		return a.parse, nil
	}
	return &relaycommon.TaskInfo{Status: model.TaskStatusInProgress}, nil
}
func (a *scriptedPollingAdaptor) AdjustBillingOnComplete(*model.Task, *relaycommon.TaskInfo) int {
	return 0
}

type scriptedBatchPollingAdaptor struct {
	scriptedPollingAdaptor
	results map[string]*BatchTaskResult
}

func (a *scriptedBatchPollingAdaptor) FetchMode() string { return "batch" }
func (a *scriptedBatchPollingAdaptor) FetchBatchTasks(string, string, []*model.Task, string) (*http.Response, error) {
	return a.FetchTask("", "", nil, "")
}
func (a *scriptedBatchPollingAdaptor) ParseBatchResult([]*model.Task, *http.Response, []byte) (map[string]*BatchTaskResult, error) {
	if a.parseErr != nil {
		return nil, a.parseErr
	}
	return a.results, nil
}

func TestUpdateVideoSingleTaskPollClassification(t *testing.T) {
	testCases := []struct {
		name          string
		statusCode    int
		fetchErr      error
		parse         *relaycommon.TaskInfo
		parseErr      error
		priorFailures int
		priorState    string
		maxFailures   int
		wantStatus    model.TaskStatus
		wantFailures  int
		wantRefund    bool
		wantReason    string
		wantState     string
		wantUnchanged bool
	}{
		{
			name:          "404 fails immediately and refunds",
			statusCode:    http.StatusNotFound,
			wantStatus:    model.TaskStatusFailure,
			wantRefund:    true,
			wantReason:    "Upstream task not found",
			wantUnchanged: false,
		},
		{
			name:          "401 increments without changing status",
			statusCode:    http.StatusUnauthorized,
			wantStatus:    model.TaskStatusInProgress,
			wantFailures:  1,
			wantUnchanged: true,
		},
		{
			name:          "429 reaches threshold and refunds",
			statusCode:    http.StatusTooManyRequests,
			priorFailures: 2,
			maxFailures:   3,
			wantStatus:    model.TaskStatusFailure,
			wantFailures:  3,
			wantRefund:    true,
			wantReason:    "Failed to query the upstream task status",
		},
		{
			name:          "UNKNOWN increments",
			statusCode:    http.StatusOK,
			parse:         &relaycommon.TaskInfo{Status: model.TaskStatusUnknown, Reason: "weird"},
			wantStatus:    model.TaskStatusInProgress,
			wantFailures:  1,
			wantUnchanged: true,
		},
		{
			name:          "valid 2xx resets the failure counter",
			statusCode:    http.StatusOK,
			parse:         &relaycommon.TaskInfo{Status: model.TaskStatusInProgress},
			priorFailures: 5,
			wantStatus:    model.TaskStatusInProgress,
			wantFailures:  0,
		},
		{
			name:       "omit state preserves previous plugin state",
			statusCode: http.StatusOK,
			parse:      &relaycommon.TaskInfo{Status: model.TaskStatusInProgress},
			priorState: `{"req_key":"keep"}`,
			wantStatus: model.TaskStatusInProgress,
			wantState:  `{"req_key":"keep"}`,
		},
		{
			name:       "returned state replaces plugin state",
			statusCode: http.StatusOK,
			parse: &relaycommon.TaskInfo{
				Status:      model.TaskStatusInProgress,
				PluginState: []byte(`{"req_key":"new"}`),
			},
			priorState: `{"req_key":"old"}`,
			wantStatus: model.TaskStatusInProgress,
			wantState:  `{"req_key":"new"}`,
		},
		{
			name:          "other 4xx non-terminal is unrecognized",
			statusCode:    http.StatusBadRequest,
			parse:         &relaycommon.TaskInfo{Status: model.TaskStatusInProgress},
			wantStatus:    model.TaskStatusInProgress,
			wantFailures:  1,
			wantUnchanged: true,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			truncate(t)
			const userID, tokenID, channelID = 510, 510, 510
			const initialQuota, preConsumed, tokenRemain = 10_000, 4_000, 7_000
			seedUser(t, userID, initialQuota)
			seedToken(t, tokenID, userID, "sk-poll-class", tokenRemain)
			ch := &model.Channel{Id: channelID, Type: constant.ChannelTypeKling, Name: "poll", Key: "sk-test", Status: common.ChannelStatusEnabled}

			if testCase.maxFailures > 0 {
				previous := constant.TaskPollMaxFailures
				constant.TaskPollMaxFailures = testCase.maxFailures
				t.Cleanup(func() { constant.TaskPollMaxFailures = previous })
			}

			task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
			task.TaskID = "task_poll_class"
			task.PrivateData.UpstreamTaskID = "upstream_poll_class"
			task.PrivateData.PollFailures = testCase.priorFailures
			if testCase.priorState != "" {
				task.PrivateData.PluginState = []byte(testCase.priorState)
			}
			require.NoError(t, model.DB.Create(task).Error)

			adaptor := &scriptedPollingAdaptor{statusCode: testCase.statusCode, fetchErr: testCase.fetchErr, parse: testCase.parse, parseErr: testCase.parseErr}
			require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, ch, task.GetUpstreamTaskID(), map[string]*model.Task{
				task.GetUpstreamTaskID(): task,
			}))

			var persisted model.Task
			require.NoError(t, model.DB.First(&persisted, task.ID).Error)
			assert.EqualValues(t, testCase.wantStatus, persisted.Status)
			assert.Equal(t, testCase.wantFailures, persisted.PrivateData.PollFailures)
			if testCase.wantUnchanged {
				assert.Empty(t, persisted.FailReason)
			}
			if testCase.wantReason != "" {
				assert.Equal(t, testCase.wantReason, persisted.FailReason)
			}
			if testCase.wantState != "" {
				assert.JSONEq(t, testCase.wantState, string(persisted.PrivateData.PluginState))
			}
			if testCase.wantRefund {
				assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
				assert.Equal(t, tokenRemain+preConsumed, getTokenRemainQuota(t, tokenID))
				assert.Zero(t, persisted.Quota)
				log := getLastLog(t)
				require.NotNil(t, log)
				assert.Equal(t, model.LogTypeRefund, log.Type)
			} else {
				assert.Equal(t, initialQuota, getUserQuota(t, userID))
				assert.Equal(t, tokenRemain, getTokenRemainQuota(t, tokenID))
			}
		})
	}
}

func TestUpdateBatchTasksPollClassification(t *testing.T) {
	testCases := []struct {
		name         string
		statusCode   int
		resultStatus model.TaskStatus
		wantStatus   model.TaskStatus
		wantFailures int
		wantRefund   bool
		wantReason   string
	}{
		{
			name:       "404 fails the batch and refunds",
			statusCode: http.StatusNotFound,
			wantStatus: model.TaskStatusFailure,
			wantRefund: true,
			wantReason: "Upstream task not found",
		},
		{
			name:         "401 increments every task",
			statusCode:   http.StatusUnauthorized,
			wantStatus:   model.TaskStatusInProgress,
			wantFailures: 1,
		},
		{
			name:         "UNKNOWN increments",
			statusCode:   http.StatusOK,
			resultStatus: model.TaskStatusUnknown,
			wantStatus:   model.TaskStatusInProgress,
			wantFailures: 1,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			truncate(t)
			const userID, tokenID, channelID = 610, 610, 610
			const initialQuota, preConsumed, tokenRemain = 10_000, 4_000, 7_000
			seedUser(t, userID, initialQuota)
			seedToken(t, tokenID, userID, "sk-batch-class", tokenRemain)
			seedTaskPollingChannel(t, channelID, true)

			task := makeTask(userID, channelID, preConsumed, tokenID, BillingSourceWallet, 0)
			task.TaskID = "task_batch_class"
			task.PrivateData.UpstreamTaskID = "upstream_batch_class"
			require.NoError(t, model.DB.Create(task).Error)
			upstreamID := task.GetUpstreamTaskID()

			adaptor := &scriptedBatchPollingAdaptor{
				scriptedPollingAdaptor: scriptedPollingAdaptor{statusCode: testCase.statusCode},
			}
			if testCase.resultStatus != "" {
				adaptor.results = map[string]*BatchTaskResult{
					upstreamID: {TaskInfo: relaycommon.TaskInfo{TaskID: upstreamID, Status: string(testCase.resultStatus), Reason: "weird"}},
				}
			}

			require.NoError(t, UpdateBatchTasks(context.Background(), adaptor, map[int][]string{channelID: {upstreamID}}, map[string]*model.Task{upstreamID: task}))

			var persisted model.Task
			require.NoError(t, model.DB.First(&persisted, task.ID).Error)
			assert.EqualValues(t, testCase.wantStatus, persisted.Status)
			assert.Equal(t, testCase.wantFailures, persisted.PrivateData.PollFailures)
			if testCase.wantReason != "" {
				assert.Equal(t, testCase.wantReason, persisted.FailReason)
			}
			if testCase.wantRefund {
				assert.Equal(t, initialQuota+preConsumed, getUserQuota(t, userID))
				assert.Zero(t, persisted.Quota)
			} else {
				assert.Equal(t, initialQuota, getUserQuota(t, userID))
			}
		})
	}
}

// The billing snapshot keeps the settlement facts, not the media URL: the
// signed upstream address is already the task's ResultURL, and the recorded
// receipt must not carry a second copy of it.
func TestUpdateBatchTasksBillingSnapshotOmitsResultUrl(t *testing.T) {
	for _, signedURL := range []string{"https://cdn.example/signed-result.mp4?token=private", "data:video/mp4;base64,aW5saW5lLXZpZGVv"} {
		t.Run(signedURL, func(t *testing.T) {
			truncate(t)

			const channelID = 145
			const preConsumedQuota = 5
			user := model.User{Username: "batch-snapshot-url", AffCode: "batch-snapshot-url", Group: "default", Status: common.UserStatusEnabled}
			require.NoError(t, model.DB.Create(&user).Error)
			plan := model.SubscriptionPlan{Title: "Batch snapshot", SubscriptionGroup: "vip"}
			require.NoError(t, model.DB.Create(&plan).Error)
			sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
			require.NoError(t, model.DB.Create(&sub).Error)
			seedTaskPollingChannel(t, channelID, true)
			const requestID = "terminal-batch-snapshot-url"
			_, err := model.PreConsumeUserSubscriptionForGroup(requestID, user.Id, "model", 0, preConsumedQuota, "vip")
			require.NoError(t, err)
			task := makeTask(user.Id, channelID, preConsumedQuota, 0, BillingSourceSubscription, sub.Id)
			task.TaskID = requestID
			task.Platform = "batch-plugin"
			task.PrivateData.UpstreamTaskID = "upstream_batch_snapshot_url"
			task.PrivateData.SubscriptionRequestID = requestID
			task.PrivateData.BillingContext.PerCallBilling = true
			require.NoError(t, model.DB.Create(task).Error)

			upstreamID := task.GetUpstreamTaskID()
			adaptor := &batchPollingAdaptor{results: map[string]*BatchTaskResult{
				upstreamID: {TaskInfo: relaycommon.TaskInfo{
					TaskID:     upstreamID,
					Status:     model.TaskStatusSuccess,
					Url:        signedURL,
					UsageFacts: map[string]any{"seconds": float64(4)},
				}},
			}}
			require.NoError(t, UpdateBatchTasks(context.Background(), adaptor, map[int][]string{channelID: {upstreamID}}, map[string]*model.Task{upstreamID: task}))

			var persisted model.Task
			require.NoError(t, model.DB.First(&persisted, task.ID).Error)
			assert.Equal(t, signedURL, persisted.GetResultURL(), "the URL is the only copy of this batch result")
			assert.NotContains(t, string(persisted.PrivateData.SubscriptionFinalResult), "signed-result.mp4",
				"the recorded billing facts must not retain the media URL")
			var facts relaycommon.TaskInfo
			require.NoError(t, common.Unmarshal(persisted.PrivateData.SubscriptionFinalResult, &facts))
			assert.Empty(t, facts.Url)
			assert.Equal(t, model.TaskStatusSuccess, facts.Status)
			assert.Equal(t, map[string]any{"seconds": float64(4)}, facts.UsageFacts)
			assert.False(t, persisted.SubscriptionBillingPending)
		})
	}
}

type nonfiniteDurationAdaptor struct{ scriptedPollingAdaptor }

func (*nonfiniteDurationAdaptor) ProbeAudioDuration(context.Context, *model.Task) (float64, error) {
	return math.Inf(1), nil
}

func TestSubscriptionPollNonFiniteUsage(t *testing.T) {
	truncate(t)
	const channelID = 145
	seedTaskPollingChannel(t, channelID, true)
	user := model.User{Username: "nonfinite", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Usage", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 40, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	batch := &batchPollingAdaptor{results: map[string]*BatchTaskResult{}}
	tasks := map[string]*model.Task{}
	var ids []string
	for _, id := range []string{"per-call", "valid-expression", "invalid-expression", "normal", "single", "audio"} {
		r, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
		require.NoError(t, err)
		task := makeTask(user.Id, channelID, 5, 0, BillingSourceSubscription, sub.Id)
		task.TaskID, task.PrivateData.SubscriptionRequestID = id, id
		task.PrivateData.SubscriptionReceiptId = r.ReceiptId
		task.SubscriptionBillingPending = true
		facts := map[string]any{"units": 3.0, "noise": math.NaN()}
		if id == "per-call" || id == "normal" {
			task.PrivateData.BillingContext.PerCallBilling = true
			if id == "normal" {
				delete(facts, "noise")
			}
		} else {
			expr := `tier("actual", u("units") ?? 0)`
			if id == "audio" {
				expr = `tier("actual", u("seconds"))`
				facts["seconds"] = 3.0
			}
			task.PrivateData.BillingContext.TieredSnapshot = &billingexpr.BillingSnapshot{ExprString: expr, ExprHash: billingexpr.ExprHashString(expr), ExprVersion: 1, GroupRatio: 1, QuotaPerUnit: 1, TaskUsageBilling: true, UsageFacts: map[string]any{"units": 1.0}}
			if id == "invalid-expression" || id == "single" {
				facts["units"] = math.Inf(1)
			}
		}
		require.NoError(t, model.DB.Create(task).Error)
		tasks[id] = task
		result := relaycommon.TaskInfo{Status: model.TaskStatusSuccess, UsageFacts: facts, Url: "https://vendor.test/" + id}
		if id == "single" || id == "audio" {
			ch, err := model.CacheGetChannel(channelID)
			require.NoError(t, err)
			assert.NoError(t, updateVideoSingleTask(t.Context(), &nonfiniteDurationAdaptor{scriptedPollingAdaptor{parse: &result}}, ch, id, tasks))
		} else {
			ids = append(ids, id)
			batch.results[id] = &BatchTaskResult{TaskInfo: result}
		}
	}
	assert.NoError(t, updateBatchTasks(t.Context(), batch, channelID, ids, tasks))
	for id, task := range tasks {
		var saved model.Task
		require.NoError(t, model.DB.First(&saved, task.ID).Error)
		assert.EqualValues(t, model.TaskStatusSuccess, saved.Status, id)
		assert.Equal(t, "https://vendor.test/"+id, saved.GetResultURL(), id)
		assert.False(t, saved.SubscriptionBillingPending, id)
		want := 5
		if id == "valid-expression" || id == "audio" {
			want = 3
		}
		assert.Equal(t, want, saved.Quota, id)
	}
}

// The retry window serves the oldest pending receipts and the rows after the
// cursor in the same pass, without ever returning a row twice.
func TestTerminalReceiptRetryWindowCoversHeadAndCursor(t *testing.T) {
	truncate(t)
	newRow := func(status model.TaskStatus, pending bool) *model.Task {
		task := &model.Task{TaskID: fmt.Sprintf("receipt-window-%d", time.Now().UnixNano()), UserId: 1, Status: status, SubscriptionBillingPending: pending}
		require.NoError(t, model.DB.Create(task).Error)
		return task
	}
	ids := func(tasks []model.Task) []int64 {
		out := make([]int64, 0, len(tasks))
		for _, task := range tasks {
			out = append(out, task.ID)
		}
		return out
	}

	first, second := newRow(model.TaskStatusSuccess, true), newRow(model.TaskStatusSuccess, true)
	third := newRow(model.TaskStatusFailure, true)
	running := newRow(model.TaskStatusInProgress, true)
	settled := newRow(model.TaskStatusSuccess, false)

	// The cursor reached the end of the queue: the oldest rows are still served,
	// and the cursor restarts from the head because the tail is empty.
	pending, next, err := outstandingTerminalReceipts(third.ID, 4)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID}, ids(pending))
	assert.Zero(t, next)

	// A newer receipt does not replace the older ones in the same window, and a
	// short page after the cursor sends the next pass back to the head.
	newer := newRow(model.TaskStatusSuccess, true)
	pending, next, err = outstandingTerminalReceipts(third.ID, 4)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID, newer.ID}, ids(pending))
	assert.Zero(t, next)

	// Head and cursor overlap: the head is served first and the rest of the
	// window continues strictly after it, so no row is returned twice.
	pending, next, err = outstandingTerminalReceipts(0, 6)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID, third.ID, newer.ID}, ids(pending))
	assert.Zero(t, next, "an exhausted queue restarts the cursor at the head")

	// A half-sized window keeps the oldest row first and pages the next one.
	pending, next, err = outstandingTerminalReceipts(0, 2)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID}, ids(pending))
	assert.Equal(t, second.ID, next)

	// A full second page advances the cursor through the backlog.
	pending, next, err = outstandingTerminalReceipts(0, 4)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID, third.ID, newer.ID}, ids(pending))
	assert.Equal(t, newer.ID, next)

	// The next pass resumes after the cursor and still starts with the head,
	// so a receipt that keeps failing is attempted every pass.
	pending, next, err = outstandingTerminalReceipts(newer.ID, 4)
	require.NoError(t, err)
	assert.Equal(t, []int64{first.ID, second.ID}, ids(pending))
	assert.Zero(t, next)

	assert.NotContains(t, ids(pending), running.ID, "a running task is polled, not retried")
	assert.NotContains(t, ids(pending), settled.ID, "a settled receipt is not retried")
}

// A retry window that only advances forward buries a receipt whose settlement
// keeps failing under newer rows. The head of the queue is rescanned every pass.
func TestRunTaskPollingOnceRetriesFailedReceiptBehindNewerRows(t *testing.T) {
	truncate(t)
	subscriptionBillingRetryCursor.Store(0)
	t.Cleanup(func() { subscriptionBillingRetryCursor.Store(0) })
	oldFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return &terminalQuotaAdaptor{} }
	t.Cleanup(func() { GetTaskAdaptorFunc = oldFactory })

	user := model.User{Username: "retry-behind-newer", AffCode: "retry-behind-newer", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Retry fairness", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	pendingTerminalTask := func(requestID string) *model.Task {
		_, err := model.PreConsumeUserSubscriptionForGroup(requestID, user.Id, "model", 0, 5, "vip")
		require.NoError(t, err)
		task := makeTask(user.Id, 0, 5, 0, BillingSourceSubscription, sub.Id)
		task.TaskID = requestID
		task.Status = model.TaskStatusSuccess
		task.SubscriptionBillingPending = true
		task.PrivateData.SubscriptionRequestID = requestID
		require.NoError(t, model.DB.Create(task).Error)
		return task
	}

	blocked := pendingTerminalTask("retry-behind-newer-1")
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_receipt_retry BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT,'temporary settlement failure'); END`).Error)
	RunTaskPollingOnce(t.Context(), nil)
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_receipt_retry").Error)
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, blocked.ID).Error)
	require.True(t, saved.SubscriptionBillingPending, "the first settlement attempt failed")

	newer := pendingTerminalTask("retry-behind-newer-2")
	RunTaskPollingOnce(t.Context(), nil)

	var retried, newest model.Task
	require.NoError(t, model.DB.First(&retried, blocked.ID).Error)
	assert.False(t, retried.SubscriptionBillingPending, "a newer pending receipt must not starve the older retry")
	require.NoError(t, model.DB.First(&newest, newer.ID).Error)
	assert.False(t, newest.SubscriptionBillingPending)
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	assert.Zero(t, sub.FundingReserved)
	assert.EqualValues(t, 10, sub.ExtraCreditsUsed)
}

// Per-call, free and already-aligned costs are settled from the receipt and the
// stored facts alone, so a restart that loses the platform adaptor must not
// block them. Returning to the old failure mode leaves the funding reserved.
func TestRunTaskPollingOnceSettlesTerminalReceiptWithoutAdaptor(t *testing.T) {
	const preConsumedQuota = 5
	testCases := []struct {
		name        string
		status      model.TaskStatus
		configure   func(*model.Task)
		wantQuota   int
		wantCredits int64
	}{
		{
			name:        "per-call success",
			status:      model.TaskStatusSuccess,
			configure:   func(task *model.Task) { task.PrivateData.BillingContext.PerCallBilling = true },
			wantQuota:   preConsumedQuota,
			wantCredits: preConsumedQuota,
		},
		{
			name:      "per-call failure",
			status:    model.TaskStatusFailure,
			configure: func(task *model.Task) { task.PrivateData.BillingContext.PerCallBilling = true },
		},
		{
			name:      "free model",
			status:    model.TaskStatusSuccess,
			configure: func(task *model.Task) { task.PrivateData.BillingContext.BillingFree = true },
		},
		{
			name:   "pinned final cost",
			status: model.TaskStatusSuccess,
			configure: func(task *model.Task) {
				aligned := preConsumedQuota
				task.PrivateData.SubscriptionSubmitQuota = &aligned
				task.PrivateData.SubscriptionFinalBilling = &model.SubscriptionTaskFinalBilling{Quota: aligned}
			},
			wantQuota:   preConsumedQuota,
			wantCredits: preConsumedQuota,
		},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			truncate(t)
			subscriptionBillingRetryCursor.Store(0)
			t.Cleanup(func() { subscriptionBillingRetryCursor.Store(0) })
			oldFactory := GetTaskAdaptorFunc
			GetTaskAdaptorFunc = nil
			t.Cleanup(func() { GetTaskAdaptorFunc = oldFactory })

			user := model.User{Username: "retry-without-adaptor", AffCode: "retry-without-adaptor", Group: "default", Status: common.UserStatusEnabled}
			require.NoError(t, model.DB.Create(&user).Error)
			plan := model.SubscriptionPlan{Title: "Restart settlement", SubscriptionGroup: "vip"}
			require.NoError(t, model.DB.Create(&plan).Error)
			sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
			require.NoError(t, model.DB.Create(&sub).Error)
			requestID := "retry-without-adaptor-" + testCase.name
			_, err := model.PreConsumeUserSubscriptionForGroup(requestID, user.Id, "model", 0, preConsumedQuota, "vip")
			require.NoError(t, err)
			task := makeTask(user.Id, 0, preConsumedQuota, 0, BillingSourceSubscription, sub.Id)
			task.TaskID = requestID
			task.Status = testCase.status
			task.SubscriptionBillingPending = true
			task.PrivateData.SubscriptionRequestID = requestID
			testCase.configure(task)
			require.NoError(t, model.DB.Create(task).Error)

			RunTaskPollingOnce(t.Context(), nil)

			var saved model.Task
			require.NoError(t, model.DB.First(&saved, task.ID).Error)
			assert.False(t, saved.SubscriptionBillingPending)
			assert.Equal(t, testCase.wantQuota, saved.Quota)
			require.NoError(t, model.DB.First(&sub, sub.Id).Error)
			assert.Zero(t, sub.FundingReserved)
			assert.EqualValues(t, testCase.wantCredits, sub.ExtraCreditsUsed)
		})
	}
}

// A task that fails after its final amount was decided settles that amount
// instead of refunding everything: the pinned decision is final for a failure
// too, and returning false here would refund the whole reservation.
func TestPinnedFinalQuotaSurvivesAFailedTask(t *testing.T) {
	truncate(t)

	user := model.User{Username: "pinned-failure", AffCode: "pinned-failure", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Pinned failure", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	const requestID = "pinned-final-quota-failure"
	_, err := model.PreConsumeUserSubscriptionForGroup(requestID, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	task := makeTask(user.Id, 0, 5, 0, BillingSourceSubscription, sub.Id)
	task.TaskID = requestID
	task.Status = model.TaskStatusFailure
	task.PrivateData.SubscriptionRequestID = requestID
	task.PrivateData.SubscriptionFinalBilling = &model.SubscriptionTaskFinalBilling{Quota: 5}
	require.NoError(t, model.DB.Create(task).Error)

	assert.True(t, settleTaskBillingOnComplete(t.Context(), nil, task, relaycommon.FailTaskInfo("upstream failed")),
		"a pinned decision takes over terminal billing")

	var saved model.Task
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	assert.Equal(t, 5, saved.Quota, "the pinned amount is charged, not refunded")
	assert.False(t, saved.SubscriptionBillingPending)
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	assert.Zero(t, sub.FundingReserved)
	assert.EqualValues(t, 5, sub.ExtraCreditsUsed)
}

// A new subscription task reports only after its final receipt commits: a
// failed settlement reports nothing, the retry settles the pinned amount and
// reports once, and a second finalizer cannot report the same task again.
func TestSubscriptionFailureReportWaitsForFunds(t *testing.T) {
	truncate(t)
	oldReport := ReportTaskFunc
	reports := 0
	ReportTaskFunc = func(*model.Task) { reports++ }
	t.Cleanup(func() { ReportTaskFunc = oldReport })
	user := model.User{Username: "failure-report-funds", AffCode: "failure-report-funds", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Failure report", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	id := "failed-before-refund-committed"
	_, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	task := makeTask(user.Id, 0, 5, 0, BillingSourceSubscription, sub.Id)
	task.TaskID = id
	task.SubscriptionBillingPending = true
	task.PrivateData.SubscriptionRequestID = id
	task.PrivateData.RequestPlugins = &model.TaskRequestPlugins{Instances: []string{"audit"}}
	task.PrivateData.BillingContext.PerCallBilling = true
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_failure_refund BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT,'temporary refund failure'); END`).Error)
	reportFailedTasks([]*model.Task{task}, "channel missing")
	require.Zero(t, reports, "failed refund cannot report yet")
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_failure_refund").Error)
	finalizeTerminalTask(t.Context(), nil, task, &relaycommon.TaskInfo{Status: model.TaskStatusFailure})
	require.Equal(t, 1, reports)
	finalizeTerminalTask(t.Context(), nil, task, &relaycommon.TaskInfo{Status: model.TaskStatusFailure})
	require.Equal(t, 1, reports)
}

func TestSubscriptionTerminalReportWaitsForPinnedSettlement(t *testing.T) {
	truncate(t)
	subscriptionBillingRetryCursor.Store(0)
	t.Cleanup(func() { subscriptionBillingRetryCursor.Store(0) })
	adaptor := &terminalQuotaAdaptor{actual: 4}
	oldFactory := GetTaskAdaptorFunc
	GetTaskAdaptorFunc = func(constant.TaskPlatform) TaskPollingAdaptor { return adaptor }
	t.Cleanup(func() { GetTaskAdaptorFunc = oldFactory })
	var reported []string
	oldReport := ReportTaskFunc
	ReportTaskFunc = func(task *model.Task) { reported = append(reported, task.TaskID+":"+string(task.Status)) }
	t.Cleanup(func() { ReportTaskFunc = oldReport })

	user := model.User{Username: "pinned-report", AffCode: "pinned-report", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Pinned report", SubscriptionGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	const requestID = "pinned-report-once"
	_, err := model.PreConsumeUserSubscriptionForGroup(requestID, user.Id, "model", 0, 5, "vip")
	require.NoError(t, err)
	task := makeTask(user.Id, 0, 5, 0, BillingSourceSubscription, sub.Id)
	task.TaskID = requestID
	task.Status = model.TaskStatusSuccess
	task.SubscriptionBillingPending = true
	task.PrivateData.SubscriptionRequestID = requestID
	task.PrivateData.SubscriptionFinalBilling = &model.SubscriptionTaskFinalBilling{Quota: 3}
	task.PrivateData.RequestPlugins = &model.TaskRequestPlugins{Instances: []string{"audit"}}
	require.NoError(t, model.DB.Create(task).Error)

	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_pinned_settlement BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT,'temporary settlement failure'); END`).Error)
	RunTaskPollingOnce(t.Context(), nil)
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_pinned_settlement").Error)
	assert.Empty(t, reported, "a task whose final charge did not commit is not reported")
	var saved model.Task
	require.NoError(t, model.DB.First(&saved, task.ID).Error)
	require.True(t, saved.SubscriptionBillingPending)
	assert.Equal(t, 5, saved.Quota)

	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_report_claim BEFORE UPDATE OF private_data ON tasks WHEN json_extract(NEW.private_data, '$.subscription_result_reported') = 1 BEGIN SELECT RAISE(ABORT,'temporary claim failure'); END`).Error)
	RunTaskPollingOnce(t.Context(), nil)
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_report_claim").Error)
	assert.Empty(t, reported, "the report claim rolled back")
	var settled model.Task
	require.NoError(t, model.DB.First(&settled, task.ID).Error)
	assert.False(t, settled.SubscriptionBillingPending)
	assert.Equal(t, 3, settled.Quota, "the retry settles the pinned amount, not the adaptor's new answer")
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	assert.Zero(t, sub.FundingReserved)
	assert.EqualValues(t, 3, sub.ExtraCreditsUsed)

	RunTaskPollingOnce(t.Context(), nil)
	require.Equal(t, []string{requestID + ":SUCCESS"}, reported, "polling retries the report even though funds already settled")

	// Another finalizer of the same terminal task, such as the submitting
	// request, settles idempotently and does not report again.
	finalizeTerminalTask(t.Context(), adaptor, &settled, &relaycommon.TaskInfo{Status: model.TaskStatusSuccess})
	RunTaskPollingOnce(t.Context(), nil)
	assert.Len(t, reported, 1)
}

// A task reports once, when it reaches its final state: not per poll, not
// when another poller lost the race, never when no instance may report it.
func TestTaskReportsOnceAtTheFinalState(t *testing.T) {
	truncate(t)
	const userID, tokenID, channelID = 520, 520, 520
	seedUser(t, userID, 10_000)
	seedToken(t, tokenID, userID, "sk-poll-report", 7_000)
	ch := &model.Channel{Id: channelID, Type: constant.ChannelTypeKling, Name: "poll", Key: "sk-test", Status: common.ChannelStatusEnabled}

	var reported []string
	previous := ReportTaskFunc
	ReportTaskFunc = func(task *model.Task) { reported = append(reported, task.TaskID+":"+string(task.Status)) }
	t.Cleanup(func() { ReportTaskFunc = previous })

	newTask := func(id string, plugins bool) *model.Task {
		task := makeTask(userID, channelID, 1_000, tokenID, BillingSourceWallet, 0)
		task.TaskID = id
		task.PrivateData.UpstreamTaskID = "upstream_" + id
		if plugins {
			task.PrivateData.RequestPlugins = &model.TaskRequestPlugins{Instances: []string{"audit"}}
		}
		require.NoError(t, model.DB.Create(task).Error)
		return task
	}
	reload := func(task *model.Task) *model.Task {
		var stored model.Task
		require.NoError(t, model.DB.First(&stored, task.ID).Error)
		return &stored
	}
	poll := func(task *model.Task, status model.TaskStatus) {
		adaptor := &scriptedPollingAdaptor{parse: &relaycommon.TaskInfo{Status: string(status), Url: "https://cdn.example.com/v.mp4"}}
		require.NoError(t, updateVideoSingleTask(context.Background(), adaptor, ch, task.GetUpstreamTaskID(), map[string]*model.Task{
			task.GetUpstreamTaskID(): task,
		}))
	}

	task := newTask("report_once", true)
	stale := reload(task)
	poll(task, model.TaskStatusInProgress)
	poll(reload(task), model.TaskStatusInProgress)
	assert.Empty(t, reported)

	poll(reload(task), model.TaskStatusSuccess)
	assert.Equal(t, []string{"report_once:SUCCESS"}, reported)
	// Another poller that read the row before it finished loses the update.
	poll(stale, model.TaskStatusSuccess)
	assert.Len(t, reported, 1)

	plain := newTask("no_plugins", false)
	poll(plain, model.TaskStatusSuccess)
	assert.Len(t, reported, 1)

	failed := newTask("fails", true)
	poll(failed, model.TaskStatusFailure)
	assert.Equal(t, "fails:FAILURE", reported[1])

	previousTimeout := constant.TaskTimeoutMinutes
	constant.TaskTimeoutMinutes = 1
	t.Cleanup(func() { constant.TaskTimeoutMinutes = previousTimeout })
	timedOut := newTask("times_out", true)
	require.NoError(t, model.DB.Model(&model.Task{}).Where("id = ?", timedOut.ID).Update("submit_time", time.Now().Add(-time.Hour).Unix()).Error)
	sweepTimedOutTasks(context.Background())
	sweepTimedOutTasks(context.Background())
	assert.Equal(t, []string{"report_once:SUCCESS", "fails:FAILURE", "times_out:FAILURE"}, reported)
}
