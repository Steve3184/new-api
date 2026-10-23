package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunStatusCheckProbeTaskRecordsOneProbeAndHonorsBackoff(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.StatusCheckProbeState{}))

	previousGroups := common.StatusCheckGroups
	previousFlexibleMode := common.StatusCheckFlexibleMode
	previousProbeTest := runStatusCheckProbeChannelTest
	common.StatusCheckGroups = `["default"]`
	common.StatusCheckFlexibleMode = `{"groups":{"default":{"enabled":true,"idle_minutes":1,"max_consecutive_probes":1}}}`
	runStatusCheckProbeChannelTest = func(context.Context, *model.Channel, int, string, string, bool) testResult {
		return testResult{}
	}
	t.Cleanup(func() {
		common.StatusCheckGroups = previousGroups
		common.StatusCheckFlexibleMode = previousFlexibleMode
		runStatusCheckProbeChannelTest = previousProbeTest
	})

	rootUser := &model.User{
		Username: "status-probe-root",
		Password: "unused",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, db.Create(rootUser).Error)
	channel := &model.Channel{
		Name:   "status-probe-channel",
		Key:    "status-probe-key",
		Group:  "default",
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(channel).Error)
	require.NoError(t, db.Create(&model.StatusCheckProbeState{
		Group:       "default",
		LastProbeAt: time.Now().Add(-2 * time.Minute).Unix(),
	}).Error)

	summary, err := runStatusCheckProbeTask(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Tested)
	assert.Equal(t, 1, summary.Succeeded)

	var state model.StatusCheckProbeState
	require.NoError(t, db.First(&state, "`group` = ?", "default").Error)
	assert.Equal(t, 1, state.ConsecutiveProbeCount)

	state.LastProbeAt = time.Now().Add(-2 * time.Minute).Unix()
	require.NoError(t, db.Save(&state).Error)
	summary, err = runStatusCheckProbeTask(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.SkippedBackoff)
	assert.Zero(t, summary.Tested)
}

func TestStatusCheckFlexibleProbeGroupsUsesPerGroupSettings(t *testing.T) {
	previousGroups := common.StatusCheckGroups
	previousFlexibleMode := common.StatusCheckFlexibleMode
	t.Cleanup(func() {
		common.StatusCheckGroups = previousGroups
		common.StatusCheckFlexibleMode = previousFlexibleMode
	})

	common.StatusCheckGroups = `["default"]`
	common.StatusCheckFlexibleMode = `{"groups":{"default":{"enabled":false,"idle_minutes":5,"max_consecutive_probes":3}}}`
	assert.Empty(t, statusCheckFlexibleProbeGroups())

	common.StatusCheckFlexibleMode = `{"groups":{"default":{"enabled":true,"idle_minutes":5,"max_consecutive_probes":3}}}`
	groups := statusCheckFlexibleProbeGroups()
	require.Len(t, groups, 1)
	assert.Equal(t, "default", groups[0].Group)
	assert.Equal(t, 5, groups[0].Config.IdleMinutes)
	assert.Equal(t, 3, groups[0].Config.MaxConsecutiveProbes)
}

func TestRunStatusCheckDegradationTaskJudgesAnswersAndHonorsInterval(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.StatusCheckDegradationResult{}))

	previousGroups := common.StatusCheckGroups
	previousDegradationMode := common.StatusCheckDegradationMode
	previousMemoryCache := common.MemoryCacheEnabled
	previousChannelTest := runStatusCheckDegradationChannelTest
	common.StatusCheckGroups = `["default"]`
	common.StatusCheckDegradationMode = `{"groups":{"default":{"enabled":true,"interval_minutes":30,"models":[{"model":"good-model","effort":"high"},{"model":"bad-model","effort":""},{"model":"missing-model","effort":"low"}]}}}`
	common.MemoryCacheEnabled = false
	var sentEfforts []string
	runStatusCheckDegradationChannelTest = func(_ context.Context, _ *model.Channel, _ int, testModel string, _ string, isStream bool, _ int, request dto.Request) testResult {
		chatRequest, ok := request.(*dto.GeneralOpenAIRequest)
		require.True(t, ok)
		assert.True(t, isStream)
		require.NotNil(t, chatRequest.Stream)
		assert.True(t, *chatRequest.Stream)
		sentEfforts = append(sentEfforts, chatRequest.ReasoningEffort)
		if testModel == "good-model" {
			return testResult{responseBody: []byte(`{"object":"chat.completion","choices":[{"message":{"role":"assistant","content":"21"}}]}`)}
		}
		return testResult{responseBody: []byte("data: {\"choices\":[{\"delta\":{\"content\":\"2\"}}]}\n\ndata: {\"choices\":[{\"delta\":{\"content\":\"9\"}}]}\n\ndata: [DONE]\n")}
	}
	t.Cleanup(func() {
		common.StatusCheckGroups = previousGroups
		common.StatusCheckDegradationMode = previousDegradationMode
		common.MemoryCacheEnabled = previousMemoryCache
		runStatusCheckDegradationChannelTest = previousChannelTest
	})

	require.NoError(t, db.Create(&model.User{
		Username: "status-degradation-root",
		Password: "unused",
		Role:     common.RoleRootUser,
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}).Error)
	channel := &model.Channel{
		Name:   "status-degradation-channel",
		Key:    "status-degradation-key",
		Group:  "default",
		Models: "good-model,bad-model",
		Status: common.ChannelStatusEnabled,
	}
	require.NoError(t, db.Create(channel).Error)
	for _, modelName := range []string{"good-model", "bad-model"} {
		require.NoError(t, db.Create(&model.Ability{Group: "default", Model: modelName, ChannelId: channel.Id, Enabled: true}).Error)
	}

	summary, err := runStatusCheckDegradationTask(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 1, summary.Passed)
	assert.Equal(t, 1, summary.Degraded)
	assert.Equal(t, 1, summary.SkippedNoChannel)
	assert.Equal(t, []string{"high", ""}, sentEfforts)

	var results []model.StatusCheckDegradationResult
	require.NoError(t, db.Order("id asc").Find(&results).Error)
	require.Len(t, results, 3)
	assert.Equal(t, model.StatusCheckDegradationPassed, results[0].Status)
	assert.Equal(t, "21", results[0].Answer)
	assert.Equal(t, model.StatusCheckDegradationDegraded, results[1].Status)
	assert.Equal(t, "29", results[1].Answer)
	assert.Equal(t, model.StatusCheckDegradationError, results[2].Status)

	summary, err = runStatusCheckDegradationTask(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 3, summary.SkippedRecent)
	assert.Len(t, sentEfforts, 2)
}

func TestJudgeStatusCheckCandyAnswer(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus string
		wantAnswer string
	}{
		{name: "correct chat answer", body: `{"object":"chat.completion","choices":[{"message":{"content":"**21**"}}]}`, wantStatus: model.StatusCheckDegradationPassed, wantAnswer: "21"},
		{name: "inline thinking ignored", body: `{"object":"chat.completion","choices":[{"message":{"content":"<think>maybe 29</think>21"}}]}`, wantStatus: model.StatusCheckDegradationPassed, wantAnswer: "21"},
		{name: "responses output", body: `{"object":"response","output":[{"type":"reasoning"},{"type":"message","content":[{"type":"output_text","text":"29"}]}]}`, wantStatus: model.StatusCheckDegradationDegraded, wantAnswer: "29"},
		{name: "responses stream deltas", body: "data: {\"type\":\"response.output_text.delta\",\"delta\":\"2\"}\n\ndata: {\"type\":\"response.output_text.delta\",\"delta\":\"1\"}\n", wantStatus: model.StatusCheckDegradationPassed, wantAnswer: "21"},
		{name: "non numeric answer", body: `{"object":"chat.completion","choices":[{"message":{"content":"无法确定"}}]}`, wantStatus: model.StatusCheckDegradationDegraded, wantAnswer: "无法确定"},
		{name: "empty answer", body: `{"object":"chat.completion","choices":[{"message":{"content":""}}]}`, wantStatus: model.StatusCheckDegradationError, wantAnswer: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, answer := judgeStatusCheckCandyAnswer(extractTestResponseText([]byte(tt.body)))
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantAnswer, answer)
		})
	}
}

func TestQueryStatusFillsIdleHoursWithDegradationChecks(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))

	currentHour := time.Now().Unix() - time.Now().Unix()%3600
	busyHour := currentHour - 2*3600
	idleHour := currentHour - 3600
	require.NoError(t, db.Create(&[]model.PerfMetric{
		{ModelName: "gpt-5", Group: "status-fill", BucketTs: busyHour, RequestCount: 4, SuccessCount: 4, TotalLatencyMs: 4000, TtftSumMs: 800, TtftCount: 4},
		{ModelName: perfmetrics.StatusCheckDegradationProbeModel, Group: "status-fill", BucketTs: busyHour, RequestCount: 1, SuccessCount: 0, TotalLatencyMs: 30000, TtftSumMs: 9000, TtftCount: 1},
		{ModelName: perfmetrics.StatusCheckDegradationProbeModel, Group: "status-fill", BucketTs: idleHour, RequestCount: 1, SuccessCount: 1, TotalLatencyMs: 20000, TtftSumMs: 1500, TtftCount: 1},
		{ModelName: "gpt-5", Group: "status-fill", BucketTs: currentHour - 100*3600, RequestCount: 10, SuccessCount: 5, TotalLatencyMs: 10000},
	}).Error)

	result, err := perfmetrics.QueryStatus(24, []string{"status-fill"}, nil)
	require.NoError(t, err)
	require.Len(t, result.Groups, 1)
	group := result.Groups[0]
	require.Len(t, group.History24, 24)
	assert.Equal(t, currentHour, group.History24[23].Ts)
	assert.Zero(t, group.History24[23].RequestCount)

	busy := group.History24[21]
	assert.Equal(t, busyHour, busy.Ts)
	assert.Equal(t, int64(4), busy.RequestCount)
	assert.Equal(t, 100.0, busy.Availability)
	assert.Equal(t, int64(200), busy.AvgTtftMs)
	assert.False(t, busy.ProbeFilled)

	idle := group.History24[22]
	assert.Equal(t, idleHour, idle.Ts)
	assert.Equal(t, int64(1), idle.RequestCount)
	assert.Equal(t, int64(1500), idle.AvgTtftMs)
	assert.True(t, idle.ProbeFilled)

	assert.Equal(t, int64(5), group.RequestCount)
	assert.Equal(t, 100.0, group.Availability)

	require.Len(t, group.Beats, perfmetrics.StatusBeatCount)
	assert.Equal(t, currentHour, group.Beats[perfmetrics.StatusBeatCount-1].Ts)
	olderBeat := group.Beats[perfmetrics.StatusBeatCount-101]
	assert.Equal(t, currentHour-100*3600, olderBeat.Ts)
	assert.Equal(t, int64(10), olderBeat.RequestCount)
	assert.Equal(t, int64(5), olderBeat.SuccessCount)
	assert.Equal(t, 50.0, olderBeat.Availability)
	assert.True(t, group.Beats[perfmetrics.StatusBeatCount-2].ProbeFilled)
}

func TestQueryStatusUsesConfiguredBucketLength(t *testing.T) {
	db := setupModelListControllerTestDB(t)
	require.NoError(t, db.AutoMigrate(&model.PerfMetric{}))
	previousBucketTime := perf_metrics_setting.GetSetting().BucketTime
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.bucket_time": "5min"}))
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{"perf_metrics_setting.bucket_time": previousBucketTime}))
	})

	const bucket = int64(300)
	currentBucket := time.Now().Unix() - time.Now().Unix()%bucket
	realBucket := currentBucket - bucket
	idleBucket := currentBucket - 2*bucket
	require.NoError(t, db.Create(&[]model.PerfMetric{
		{ModelName: "gpt-5", Group: "status-5min", BucketTs: realBucket, RequestCount: 4, SuccessCount: 1, TotalLatencyMs: 4000, CacheSampleCount: 4, CachedTokens: 50, InputTokens: 100},
		{ModelName: perfmetrics.StatusCheckProbeModel, Group: "status-5min", BucketTs: realBucket, RequestCount: 1, SuccessCount: 1, TotalLatencyMs: 100, InputTokens: 1000},
		{ModelName: perfmetrics.StatusCheckDegradationProbeModel, Group: "status-5min", BucketTs: idleBucket, RequestCount: 1, SuccessCount: 0, TotalLatencyMs: 9000},
	}).Error)

	result, err := perfmetrics.QueryStatus(24, []string{"status-5min"}, nil)
	require.NoError(t, err)
	assert.Equal(t, bucket, result.BucketSeconds)
	require.Len(t, result.Groups, 1)
	group := result.Groups[0]

	require.Len(t, group.History24, 24*12)
	assert.Equal(t, currentBucket, group.History24[len(group.History24)-1].Ts)
	assert.Equal(t, bucket, group.History24[1].Ts-group.History24[0].Ts)

	require.Len(t, group.Beats, perfmetrics.StatusBeatCount)
	last := len(group.Beats) - 1
	assert.Equal(t, currentBucket, group.Beats[last].Ts)
	realBeat := group.Beats[last-1]
	assert.Equal(t, realBucket, realBeat.Ts)
	assert.Equal(t, int64(5), realBeat.RequestCount)
	assert.Equal(t, int64(2), realBeat.SuccessCount)
	assert.False(t, realBeat.ProbeFilled)
	idleBeat := group.Beats[last-2]
	assert.Equal(t, idleBucket, idleBeat.Ts)
	assert.Equal(t, int64(1), idleBeat.RequestCount)
	assert.Zero(t, idleBeat.SuccessCount)
	assert.True(t, idleBeat.ProbeFilled)

	// Probe samples never contribute input tokens to the cache hit rate.
	assert.Equal(t, int64(100), group.CacheInputTokens)
	assert.Equal(t, 50.0, group.CacheHitRate)
}

func TestRunStatusCheckProbeTaskCountsOnlyTheFinalPriorityAttempt(t *testing.T) {
	tests := []struct {
		name       string
		group      string
		probeModel string
	}{
		{name: "channel test models", group: "vip"},
		{name: "configured model", group: "svip", probeModel: "probe-model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupModelListControllerTestDB(t)
			require.NoError(t, db.AutoMigrate(&model.StatusCheckProbeState{}, &model.PerfMetric{}))

			previousGroups := common.StatusCheckGroups
			previousFlexibleMode := common.StatusCheckFlexibleMode
			previousMemoryCache := common.MemoryCacheEnabled
			previousRetryTimes := common.RetryTimes
			previousProbeTest := runStatusCheckProbeChannelTest
			common.StatusCheckGroups = `["` + tt.group + `"]`
			common.StatusCheckFlexibleMode = `{"groups":{"` + tt.group + `":{"enabled":true,"idle_minutes":1,"max_consecutive_probes":5,"model":"` + tt.probeModel + `"}}}`
			common.MemoryCacheEnabled = false
			common.RetryTimes = 3
			var testedChannels []string
			var testedModels []string
			runStatusCheckProbeChannelTest = func(_ context.Context, channel *model.Channel, _ int, testModel string, _ string, _ bool) testResult {
				testedChannels = append(testedChannels, channel.Name)
				testedModels = append(testedModels, testModel)
				if channel.Name == "primary" {
					return testResult{newAPIError: types.NewError(errors.New("upstream down"), types.ErrorCodeBadResponse)}
				}
				return testResult{}
			}
			t.Cleanup(func() {
				common.StatusCheckGroups = previousGroups
				common.StatusCheckFlexibleMode = previousFlexibleMode
				common.MemoryCacheEnabled = previousMemoryCache
				common.RetryTimes = previousRetryTimes
				runStatusCheckProbeChannelTest = previousProbeTest
			})

			require.NoError(t, db.Create(&model.User{
				Username: "status-probe-root",
				Password: "unused",
				Role:     common.RoleRootUser,
				Status:   common.UserStatusEnabled,
				Group:    tt.group,
			}).Error)
			for _, spec := range []struct {
				name     string
				priority int64
			}{{name: "primary", priority: 10}, {name: "backup", priority: 0}} {
				channel := &model.Channel{
					Name:     spec.name,
					Key:      spec.name + "-key",
					Group:    tt.group,
					Models:   "probe-model",
					Status:   common.ChannelStatusEnabled,
					Priority: lo.ToPtr(spec.priority),
				}
				require.NoError(t, db.Create(channel).Error)
				require.NoError(t, db.Create(&model.Ability{Group: tt.group, Model: "probe-model", ChannelId: channel.Id, Enabled: true, Priority: lo.ToPtr(spec.priority)}).Error)
			}
			require.NoError(t, db.Create(&model.StatusCheckProbeState{
				Group:       tt.group,
				LastProbeAt: time.Now().Add(-2 * time.Minute).Unix(),
			}).Error)

			summary, err := runStatusCheckProbeTask(context.Background())
			require.NoError(t, err)
			assert.Equal(t, 1, summary.Tested)
			assert.Equal(t, 1, summary.Succeeded)
			assert.Equal(t, []string{"primary", "backup"}, testedChannels)
			assert.Equal(t, []string{tt.probeModel, tt.probeModel}, testedModels)

			result, err := perfmetrics.QueryStatus(24, []string{tt.group}, nil)
			require.NoError(t, err)
			require.Len(t, result.Groups, 1)
			assert.Equal(t, int64(1), result.Groups[0].RequestCount)
			assert.Equal(t, 100.0, result.Groups[0].Availability)
		})
	}
}

func TestStatusCheckFlexibleProbeGroupsSkipsDegradationGroups(t *testing.T) {
	previousGroups := common.StatusCheckGroups
	previousFlexibleMode := common.StatusCheckFlexibleMode
	previousDegradationMode := common.StatusCheckDegradationMode
	t.Cleanup(func() {
		common.StatusCheckGroups = previousGroups
		common.StatusCheckFlexibleMode = previousFlexibleMode
		common.StatusCheckDegradationMode = previousDegradationMode
	})

	common.StatusCheckGroups = `["default"]`
	common.StatusCheckFlexibleMode = `{"groups":{"default":{"enabled":true,"idle_minutes":5,"max_consecutive_probes":3}}}`
	common.StatusCheckDegradationMode = `{"groups":{"default":{"enabled":true,"models":[{"model":"gpt-5","effort":"high"}]}}}`
	assert.Empty(t, statusCheckFlexibleProbeGroups())

	common.StatusCheckDegradationMode = `{"groups":{"default":{"enabled":false,"models":[{"model":"gpt-5","effort":"high"}]}}}`
	assert.Len(t, statusCheckFlexibleProbeGroups(), 1)
}
