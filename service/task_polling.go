package service

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	taskdto "github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/MoeForge/moego/ojson"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/samber/lo"
)

// TaskPollingAdaptor 定义轮询所需的最小适配器接口，避免 service -> relay 的循环依赖
type TaskPollingAdaptor interface {
	Init(info *relaycommon.RelayInfo)
	// AdjustBillingOnComplete 在任务到达终态（成功/失败）时由轮询循环调用。
	// 返回正数触发差额结算（补扣/退还），返回 0 保持预扣费金额不变。
	AdjustBillingOnComplete(task *model.Task, taskResult *relaycommon.TaskInfo) int
}

type taskAudioDurationProvider interface {
	ProbeAudioDuration(context.Context, *model.Task) (float64, error)
}

type legacyTaskPollingFetch interface {
	FetchTask(baseURL, key string, body map[string]any, proxy string) (*http.Response, error)
}

type modernTaskPollingFetch interface {
	FetchTask(baseURL, key string, task *model.Task, proxy string) (*http.Response, error)
}

type legacyTaskPollingParse interface {
	ParseTaskResult(body []byte) (*relaycommon.TaskInfo, error)
}

type modernTaskPollingParse interface {
	ParseTaskResult(task *model.Task, resp *http.Response, body []byte) (*relaycommon.TaskInfo, error)
}

func fetchTaskStatus(adaptor TaskPollingAdaptor, baseURL, key string, task *model.Task, body map[string]any, proxy string) (*http.Response, error) {
	if modern, ok := adaptor.(modernTaskPollingFetch); ok {
		return modern.FetchTask(baseURL, key, task, proxy)
	}
	if legacy, ok := adaptor.(legacyTaskPollingFetch); ok {
		return legacy.FetchTask(baseURL, key, body, proxy)
	}
	return nil, fmt.Errorf("task adaptor does not implement FetchTask")
}

func parseTaskStatus(adaptor TaskPollingAdaptor, task *model.Task, resp *http.Response, body []byte) (*relaycommon.TaskInfo, error) {
	if modern, ok := adaptor.(modernTaskPollingParse); ok {
		return modern.ParseTaskResult(task, resp, body)
	}
	if legacy, ok := adaptor.(legacyTaskPollingParse); ok {
		return legacy.ParseTaskResult(body)
	}
	return nil, fmt.Errorf("task adaptor does not implement ParseTaskResult")
}

type BatchTaskPollingAdaptor interface {
	TaskPollingAdaptor
	FetchMode() string
}

type legacyBatchTaskPollingFetch interface {
	FetchBatchTasks(baseURL, key string, taskIDs []string, proxy string) (*http.Response, error)
}

type modernBatchTaskPollingFetch interface {
	FetchBatchTasks(baseURL, key string, tasks []*model.Task, proxy string) (*http.Response, error)
}

type legacyBatchTaskPollingParse interface {
	ParseBatchResult(body []byte) (map[string]*BatchTaskResult, error)
}

type modernBatchTaskPollingParse interface {
	ParseBatchResult(tasks []*model.Task, resp *http.Response, body []byte) (map[string]*BatchTaskResult, error)
}

func fetchBatchTaskStatuses(adaptor BatchTaskPollingAdaptor, baseURL, key string, tasks []*model.Task, taskIDs []string, proxy string) (*http.Response, error) {
	if modern, ok := adaptor.(modernBatchTaskPollingFetch); ok {
		return modern.FetchBatchTasks(baseURL, key, tasks, proxy)
	}
	if legacy, ok := adaptor.(legacyBatchTaskPollingFetch); ok {
		return legacy.FetchBatchTasks(baseURL, key, taskIDs, proxy)
	}
	return nil, fmt.Errorf("batch task adaptor does not implement FetchBatchTasks")
}

func parseBatchTaskStatuses(adaptor BatchTaskPollingAdaptor, tasks []*model.Task, resp *http.Response, body []byte) (map[string]*BatchTaskResult, error) {
	if modern, ok := adaptor.(modernBatchTaskPollingParse); ok {
		return modern.ParseBatchResult(tasks, resp, body)
	}
	if legacy, ok := adaptor.(legacyBatchTaskPollingParse); ok {
		return legacy.ParseBatchResult(body)
	}
	return nil, fmt.Errorf("batch task adaptor does not implement ParseBatchResult")
}

const (
	pollClassOK           = "ok"
	pollClassOtherClient  = "other_client"
	pollClassNotFound     = "not_found"
	pollClassAuth         = "auth"
	pollClassTransient    = "transient"
	pollClassUnrecognized = "unrecognized"
	pollClassHookError    = "hook_error"
	pollClassTransport    = "transport_error"
)

type BatchTaskResult struct {
	TaskInfo   relaycommon.TaskInfo
	Action     string
	SubmitTime int64
	StartTime  int64
	FinishTime int64
	Data       any
}

// GetTaskAdaptorFunc 由 main 包注入，用于获取指定平台的任务适配器。
// 打破 service -> relay -> relay/channel -> service 的循环依赖。
var GetTaskAdaptorFunc func(platform constant.TaskPlatform) TaskPollingAdaptor

// ReportTaskFunc is injected by main: it hands a task that reached its final
// state to the request plugins' report hooks (requestplugin.ReportTask).
var ReportTaskFunc func(task *model.Task)

// reportFinalTask reports a task once, after its final state is stored and
// settled. A task no instance may report costs nothing.
func reportFinalTask(task *model.Task) {
	if ReportTaskFunc != nil && task.PrivateData.RequestPlugins != nil {
		ReportTaskFunc(task)
	}
}

func pinLocalSubscriptionRefund(task *model.Task) {
	if task.PrivateData.SubscriptionRequestID != "" {
		task.PrivateData.SubscriptionLocalRefund = true
		task.PrivateData.SubscriptionFinalBilling = &model.SubscriptionTaskFinalBilling{Quota: 0,
			Log: &model.RecordTaskBillingLogParams{ModelName: taskModelName(task), Other: taskBillingOther(task)},
		}
	}
}

// reportFailedTasks fails only tasks for which this caller wins the status transition.
func reportFailedTasks(tasks []*model.Task, reason string) {
	now := time.Now().Unix()
	for _, task := range tasks {
		if task == nil {
			continue
		}
		oldStatus := task.Status
		if oldStatus == model.TaskStatusSuccess || oldStatus == model.TaskStatusFailure {
			continue
		}
		task.Status, task.Progress, task.FailReason = model.TaskStatusFailure, "100%", reason
		pinLocalSubscriptionRefund(task)
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		won, err := task.UpdateWithStatus(oldStatus)
		if err != nil || !won {
			continue
		}
		if task.PrivateData.SubscriptionRequestID != "" {
			finalizeTerminalTask(context.Background(), nil, task, &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Reason: reason})
			continue
		}
		reportFinalTask(task)
	}
}

// sweepTimedOutTasks 在主轮询之前独立清理超时任务。
// 每次最多处理 100 条，剩余的下个周期继续处理。
// 使用 per-task CAS (UpdateWithStatus) 防止覆盖被正常轮询已推进的任务。
func sweepTimedOutTasks(ctx context.Context) {
	if constant.TaskTimeoutMinutes <= 0 {
		return
	}
	cutoff := time.Now().Unix() - int64(constant.TaskTimeoutMinutes)*60
	tasks := model.GetTimedOutUnfinishedTasks(cutoff, 100)
	if len(tasks) == 0 {
		return
	}

	reason := "Task timed out"
	legacyReason := "Task timed out (a task left by the old system is not refunded; contact the administrator)"
	now := time.Now().Unix()
	timedOutCount := 0

	for _, task := range tasks {
		isLegacy := task.PrivateData.SubscriptionRequestID == "" && task.SubmitTime > 0 && task.SubmitTime < model.TaskRefundLegacyCutoff

		oldStatus := task.Status
		task.Status = model.TaskStatusFailure
		task.Progress = "100%"
		task.FinishTime = now
		if isLegacy {
			task.FailReason = legacyReason
			// 旧系统任务明确不退款，随终态 CAS 一并清掉 quota，
			// 避免留下可再次退款的计费状态。
			task.Quota = 0
		} else {
			task.FailReason = reason
			pinLocalSubscriptionRefund(task)
		}

		var won bool
		var err error
		if isLegacy {
			won, err = task.UpdateWithStatusRetiringQuota(oldStatus)
		} else {
			won, err = task.UpdateWithStatus(oldStatus)
		}
		if err != nil {
			logger.LogError(ctx, common.LogText("sweepTimedOutTasks CAS update error for task %s: %v", task.TaskID, err))
			continue
		}
		if !won {
			logger.LogInfo(ctx, common.LogText("sweepTimedOutTasks: task %s already transitioned, skip", task.TaskID))
			continue
		}
		timedOutCount++
		if task.PrivateData.SubscriptionRequestID != "" {
			finalizeTerminalTask(ctx, nil, task, &relaycommon.TaskInfo{Status: model.TaskStatusFailure, Reason: reason})
			continue
		}
		if !isLegacy && (task.Quota != 0 || task.PrivateData.SubscriptionRequestID != "") {
			RefundTaskQuota(ctx, task, reason)
		}
		reportFinalTask(task)
	}

	if timedOutCount > 0 {
		logger.LogInfo(ctx, common.LogText("sweepTimedOutTasks: timed out %d tasks", timedOutCount))
	}
}

// TaskPollSummary is the result recorded on an async_task_poll system task row,
// summarizing one polling pass.
type TaskPollSummary struct {
	UnfinishedTasks  int `json:"unfinished_tasks"`
	PlatformsScanned int `json:"platforms_scanned"`
	NullTasksFailed  int `json:"null_tasks_failed"`
}

// subscriptionBillingRetryCursor continues the receipt retry window after the
// rows one polling pass already attempted.
var subscriptionBillingRetryCursor atomic.Int64

// A finite snapshot boundary lets the next sweep revisit failed older rows
// even while new terminal tasks keep arriving.
var subscriptionBillingRetryUpper atomic.Int64

// subscriptionBillingRetryWindow is how many pending receipts one pass serves.
const subscriptionBillingRetryWindow = 100

const pendingTerminalCondition = "(subscription_billing_pending = ? OR subscription_report_pending = ?) AND status IN ?"

// outstandingTerminalReceipts returns the retry window of terminal tasks whose
// subscription receipt is still pending, and the cursor the next pass resumes
// from. Half of the window is always the oldest pending rows, so a receipt
// whose settlement keeps failing is retried on every pass instead of staying
// buried under newer ones. The other half pages through the rest of the queue
// after cursor and never repeats a head row. When that page comes back short,
// the end of the queue was reached and next returns to 0, so newer rows
// arriving on every pass cannot keep the cursor from revisiting older ones.
func outstandingTerminalReceipts(cursor int64, limit int, upper ...int64) (pending []model.Task, next int64, err error) {
	statuses := []string{model.TaskStatusSuccess, model.TaskStatusFailure}
	headLimit := max(limit/2, 1)
	tailLimit := limit - headLimit

	if err = model.DB.Where(pendingTerminalCondition, true, true, statuses).Order("id").Limit(headLimit).Find(&pending).Error; err != nil {
		return nil, cursor, err
	}
	if len(pending) < headLimit || tailLimit <= 0 {
		return pending, 0, nil
	}
	after := max(cursor, pending[len(pending)-1].ID)
	var tail []model.Task
	tailQuery := model.DB.Where(pendingTerminalCondition, true, true, statuses).Where("id > ?", after)
	if len(upper) > 0 {
		tailQuery = tailQuery.Where("id <= ?", upper[0])
	}
	if err = tailQuery.Order("id").Limit(tailLimit).Find(&tail).Error; err != nil {
		return nil, cursor, err
	}
	if len(tail) == tailLimit {
		next = tail[len(tail)-1].ID
	}
	return append(pending, tail...), next, nil
}

// RunTaskPollingOnce performs one async-task (Suno/video) polling pass
// synchronously. It honors ctx cancellation (the system-task runner cancels it
// when the lease is lost) and, when report is non-nil, reports progress as
// (processedPlatforms, totalPlatforms). It returns immediately if the task
// adaptor factory has not been wired yet, to avoid a nil call during startup.
func RunTaskPollingOnce(ctx context.Context, report func(processed, total int)) TaskPollSummary {
	summary := TaskPollSummary{}
	if ctx == nil {
		ctx = context.Background()
	}

	common.SysLog(common.LogText("task progress polling started"))
	// Settlement and the subsequent report claim commit independently.
	cursor := subscriptionBillingRetryCursor.Load()
	if cursor == 0 {
		var last model.Task
		if err := model.DB.Select("id").Where(pendingTerminalCondition, true, true, []string{model.TaskStatusSuccess, model.TaskStatusFailure}).Order("id desc").Limit(1).Find(&last).Error; err != nil {
			return summary
		}
		subscriptionBillingRetryUpper.Store(last.ID)
	}
	upper := subscriptionBillingRetryUpper.Load()
	pendingBilling, nextCursor, err := outstandingTerminalReceipts(cursor, subscriptionBillingRetryWindow, upper)
	if nextCursor >= upper {
		nextCursor = 0
	}
	if err != nil {
		logger.LogWarn(ctx, common.LogText("failed to finalize subscription request receipt: %s", err))
	} else {
		subscriptionBillingRetryCursor.Store(nextCursor)
		for i := range pendingBilling {
			task := &pendingBilling[i]
			if ctx.Err() != nil {
				break
			}
			if task.PrivateData.SubscriptionRequestID == "" {
				continue
			}
			result := relaycommon.TaskInfo{Status: string(task.Status)}
			if len(task.PrivateData.SubscriptionFinalResult) > 0 {
				if err := common.Unmarshal(task.PrivateData.SubscriptionFinalResult, &result); err != nil {
					continue
				}
			}
			// A per-call, free, tiered or already-aligned final amount is settled
			// from the receipt alone, so an unavailable platform adaptor cannot
			// keep the funding reservation open.
			var adaptor TaskPollingAdaptor
			if GetTaskAdaptorFunc != nil {
				adaptor = GetTaskAdaptorFunc(task.Platform)
			}
			finalizeTerminalTask(ctx, adaptor, task, &result)
		}
	}
	if err := model.FlushSubscriptionTaskBillingLogs(); err != nil {
		logger.LogWarn(ctx, common.LogText("failed to record task billing log: %s", err))
	}
	sweepTimedOutTasks(ctx)
	if GetTaskAdaptorFunc == nil {
		return summary
	}
	allTasks := model.GetAllUnFinishSyncTasks(constant.TaskQueryLimit)
	summary.UnfinishedTasks = len(allTasks)
	platformTask := make(map[constant.TaskPlatform][]*model.Task)
	for _, t := range allTasks {
		platformTask[t.Platform] = append(platformTask[t.Platform], t)
	}

	totalPlatforms := len(platformTask)
	processedPlatforms := 0
	for platform, tasks := range platformTask {
		if ctx.Err() != nil {
			break
		}
		if report != nil {
			report(processedPlatforms, totalPlatforms)
		}
		processedPlatforms++
		if len(tasks) == 0 {
			continue
		}
		summary.PlatformsScanned++
		taskChannelM := make(map[int][]string)
		tasksByChannel := make(map[int]map[string]*model.Task)
		nullTaskIds := make([]int64, 0)
		var nullTasks []*model.Task
		for _, task := range tasks {
			upstreamID := task.GetUpstreamTaskID()
			if upstreamID == "" {
				// 统计失败的未完成任务
				nullTaskIds = append(nullTaskIds, task.ID)
				nullTasks = append(nullTasks, task)
				continue
			}
			if tasksByChannel[task.ChannelId] == nil {
				tasksByChannel[task.ChannelId] = make(map[string]*model.Task)
			}
			tasksByChannel[task.ChannelId][upstreamID] = task
			taskChannelM[task.ChannelId] = append(taskChannelM[task.ChannelId], upstreamID)
		}
		if len(nullTaskIds) > 0 {
			summary.NullTasksFailed += len(nullTaskIds)
			reportFailedTasks(nullTasks, "")
		}
		if len(taskChannelM) == 0 {
			continue
		}

		// Upstream identifiers are only unique within a channel. Keep each
		// channel's task map private while polling channels concurrently.
		var channels sync.WaitGroup
		for channelID, ids := range taskChannelM {
			channelTasks := tasksByChannel[channelID]
			channels.Add(1)
			gopool.Go(func() {
				defer channels.Done()
				DispatchPlatformUpdate(ctx, platform, map[int][]string{channelID: ids}, channelTasks)
			})
		}
		channels.Wait()
	}
	if report != nil && ctx.Err() == nil {
		report(totalPlatforms, totalPlatforms)
	}
	common.SysLog(common.LogText("task progress polling finished"))
	return summary
}

// DispatchPlatformUpdate 按平台分发轮询更新
func DispatchPlatformUpdate(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) {
	if ctx == nil {
		ctx = context.Background()
	}
	if platform == constant.TaskPlatformMidjourney {
		// MJ 轮询由其自身处理，这里预留入口
		return
	}
	adaptor := GetTaskAdaptorFunc(platform)
	if batchAdaptor, ok := adaptor.(BatchTaskPollingAdaptor); ok && batchAdaptor.FetchMode() == "batch" {
		if err := UpdateBatchTasks(ctx, batchAdaptor, taskChannelM, taskM); err != nil {
			common.SysLog(common.LogText("UpdateBatchTasks fail: %s", err))
		}
		return
	}
	if err := UpdateVideoTasks(ctx, platform, taskChannelM, taskM); err != nil {
		common.SysLog(common.LogText("UpdateVideoTasks fail: %s", err))
	}
}

func UpdateBatchTasks(ctx context.Context, adaptor BatchTaskPollingAdaptor, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	for channelId, taskIds := range taskChannelM {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		err := updateBatchTasks(ctx, adaptor, channelId, taskIds, taskM)
		if err != nil {
			logger.LogError(ctx, common.LogText("channel #%d failed to update async tasks: %s", channelId, err.Error()))
		}
	}
	return nil
}

func updateBatchTasks(ctx context.Context, adaptor BatchTaskPollingAdaptor, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, common.LogText("channel #%d has %d unfinished tasks", channelId, len(taskIds)))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(taskIds) == 0 {
		return nil
	}
	ch, err := model.CacheGetChannel(channelId)
	if err != nil {
		common.SysLog(common.LogText("CacheGetChannel: %v", err))
		reportFailedTasks(bulkFailedTasks(taskIds, taskM), "Failed to get the channel information. Please contact the administrator")
		return err
	}
	proxy := ch.GetSetting().Proxy
	baseURL := ch.GetBaseURL()
	if baseURL == "" {
		baseURL = constant.GetChannelBaseURL(ch.Type)
	}
	tasks := make([]*model.Task, 0, len(taskIds))
	for _, upstreamID := range taskIds {
		if task := taskM[upstreamID]; task != nil {
			tasks = append(tasks, task)
		}
	}
	// The channel type tells plugin adaptors whether the upstream is another
	// New API gateway, the same signal submission derives from the request.
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:     ch.Type,
		ChannelId:       ch.Id,
		ChannelBaseUrl:  baseURL,
		ChannelSetting:  ch.GetSetting(),
		HeadersOverride: ch.GetHeaderOverride(),
	}
	info.ApiKey = ch.Key
	adaptor.Init(info)
	var accounts []string
	batches := make(map[string][]*model.Task)
	for _, task := range tasks {
		account := task.PrivateData.CredentialAccount
		if _, ok := batches[account]; !ok {
			accounts = append(accounts, account)
		}
		batches[account] = append(batches[account], task)
	}
	var errs []error
	for _, account := range accounts {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		batchTasks := batches[account]
		batchIDs := make([]string, 0, len(batchTasks))
		for _, task := range batchTasks {
			batchIDs = append(batchIDs, task.GetUpstreamTaskID())
		}
		resp, err := fetchBatchTaskStatuses(adaptor, baseURL, ch.Key, batchTasks, batchIDs, proxy)
		if err != nil {
			common.SysLog(common.LogText("Get Task Do req error: %v", err))
			errs = append(errs, recordPollFailureForTasks(ctx, adaptor, batchTasks, pollClassTransport, 0, err.Error()))
			continue
		}
		if resp == nil || resp.Body == nil {
			errs = append(errs, recordPollFailureForTasks(ctx, adaptor, batchTasks, pollClassTransport, 0, "empty response"))
			continue
		}
		if err := processBatchTaskResponse(ctx, adaptor, channelId, batchTasks, resp); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func processBatchTaskResponse(ctx context.Context, adaptor BatchTaskPollingAdaptor, channelId int, tasks []*model.Task, resp *http.Response) error {
	if resp == nil || resp.Body == nil {
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassTransport, 0, "empty response")
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		common.SysLog(common.LogText("Get Suno Task parse body error: %v", err))
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassTransport, resp.StatusCode, err.Error())
	}
	switch classifyPollHTTP(resp.StatusCode) {
	case pollClassNotFound:
		for _, task := range tasks {
			if task != nil {
				logger.LogWarn(ctx, common.LogText("task %s poll %s (failures=%d, http=%d): %s", task.TaskID, pollClassNotFound, task.PrivateData.PollFailures, resp.StatusCode, unrecognizedPollDetail("", responseBody)))
			}
		}
		return failTasksFromPoll(ctx, adaptor, tasks, "Upstream task not found")
	case pollClassAuth:
		logger.LogWarn(ctx, common.LogText("task poll auth failure channel_id=%d http=%d", channelId, resp.StatusCode))
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassAuth, resp.StatusCode, "")
	case pollClassTransient:
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassTransient, resp.StatusCode, "")
	}
	responseItems, err := parseBatchTaskStatuses(adaptor, tasks, resp, responseBody)
	if err != nil {
		return recordPollFailureForTasks(ctx, adaptor, tasks, pollClassHookError, resp.StatusCode, err.Error())
	}
	batch := make(map[string]*model.Task, len(tasks))
	for _, task := range tasks {
		batch[task.GetUpstreamTaskID()] = task
	}
	for upstreamID, responseItem := range responseItems {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		task := batch[upstreamID]
		if task == nil {
			logger.LogWarn(ctx, common.LogText("Batch task response ignored: unknown task_id=%s", upstreamID))
			continue
		}
		snap := task.Snapshot()
		httpClass := classifyPollHTTP(resp.StatusCode)
		parsedStatus := model.TaskStatus(responseItem.TaskInfo.Status)
		if parsedStatus == model.TaskStatusUnknown || parsedStatus == "" || !knownPollStatus(parsedStatus) {
			if err := recordPollFailure(ctx, adaptor, task, snap.Status, pollClassUnrecognized, resp.StatusCode, responseItem.TaskInfo.Reason); err != nil {
				common.SysLog(common.LogText("UpdateSunoTask task error: %s", err.Error()))
			}
			continue
		}
		if httpClass == pollClassOtherClient && isNonTerminalPollStatus(parsedStatus) {
			if err := recordPollFailure(ctx, adaptor, task, snap.Status, pollClassUnrecognized, resp.StatusCode, responseItem.TaskInfo.Reason); err != nil {
				common.SysLog(common.LogText("UpdateSunoTask task error: %s", err.Error()))
			}
			continue
		}
		if isNonTerminalPollStatus(parsedStatus) {
			task.PrivateData.PollFailures = 0
		}
		if len(responseItem.TaskInfo.PluginState) > 0 {
			task.PrivateData.PluginState = responseItem.TaskInfo.PluginState
		}
		task.Status = lo.If(parsedStatus != "", parsedStatus).Else(task.Status)
		task.FailReason = lo.If(responseItem.TaskInfo.Reason != "", responseItem.TaskInfo.Reason).Else(task.FailReason)
		task.SubmitTime = lo.If(responseItem.SubmitTime != 0, responseItem.SubmitTime).Else(task.SubmitTime)
		task.StartTime = lo.If(responseItem.StartTime != 0, responseItem.StartTime).Else(task.StartTime)
		task.FinishTime = lo.If(responseItem.FinishTime != 0, responseItem.FinishTime).Else(task.FinishTime)
		if responseItem.TaskInfo.Progress != "" {
			task.Progress = responseItem.TaskInfo.Progress
		}
		if responseItem.TaskInfo.Reason != "" || task.Status == model.TaskStatusFailure {
			logger.LogInfo(ctx, common.LogText("%s failed: %s", task.TaskID, task.FailReason))
			task.Status = model.TaskStatusFailure
			task.Progress = "100%"
		}
		if responseItem.TaskInfo.Status == model.TaskStatusSuccess {
			task.Progress = "100%"
		}
		if responseItem.Data != nil {
			task.SetData(responseItem.Data)
		} else if task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure {
			logger.LogWarn(ctx, common.LogText(
				"Batch task %s reached terminal status without data; preserving existing task data",
				task.TaskID,
			))
		}
		if responseItem.TaskInfo.Url != "" {
			task.PrivateData.ResultURL = responseItem.TaskInfo.Url
		}

		isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
		terminalTransition := isDone && snap.Status != task.Status
		if terminalTransition && task.PrivateData.SubscriptionRequestID != "" {
			if err := StoreTaskSettlementFacts(task, &responseItem.TaskInfo); err != nil {
				common.SysLog(common.LogText("UpdateSunoTask task error: %s", err.Error()))
				continue
			}
		}
		won, updateErr := task.UpdateWithStatus(snap.Status)
		if updateErr != nil {
			common.SysLog(common.LogText("UpdateSunoTask task error: %s", updateErr.Error()))
			continue
		}
		if !won {
			logger.LogWarn(ctx, common.LogText("Batch task %s already transitioned by another process, skip billing", task.TaskID))
			continue
		}
		if terminalTransition {
			finalizeTerminalTask(ctx, adaptor, task, &responseItem.TaskInfo)
		}
	}
	return nil
}

// UpdateVideoTasks 按渠道更新所有视频任务
func UpdateVideoTasks(ctx context.Context, platform constant.TaskPlatform, taskChannelM map[int][]string, taskM map[string]*model.Task) error {
	channelIDs := make([]int, 0, len(taskChannelM))
	for channelID := range taskChannelM {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)

	var wg sync.WaitGroup
	for _, channelId := range channelIDs {
		taskIds := taskChannelM[channelId]
		if len(taskIds) == 0 {
			continue
		}
		taskIds = append([]string(nil), taskIds...)

		wg.Add(1)
		gopool.Go(func() {
			defer wg.Done()
			if err := updateVideoTasks(ctx, platform, channelId, taskIds, taskM); err != nil {
				logger.LogError(ctx, common.LogText("Channel #%d failed to update video async tasks: %s", channelId, err.Error()))
			}
		})
	}
	wg.Wait()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return nil
}

func updateVideoTasks(ctx context.Context, platform constant.TaskPlatform, channelId int, taskIds []string, taskM map[string]*model.Task) error {
	logger.LogInfo(ctx, common.LogText("Channel #%d pending video tasks: %d", channelId, len(taskIds)))
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if len(taskIds) == 0 {
		return nil
	}
	cacheGetChannel, err := model.CacheGetChannel(channelId)
	if err != nil {
		reportFailedTasks(bulkFailedTasks(taskIds, taskM), "Failed to get the channel information. Please contact the administrator")
		return fmt.Errorf("CacheGetChannel failed: %w", err)
	}
	adaptor := GetTaskAdaptorFunc(platform)
	if adaptor == nil {
		return fmt.Errorf("video adaptor not found")
	}
	info := &relaycommon.RelayInfo{}
	info.ChannelMeta = &relaycommon.ChannelMeta{
		ChannelType:     cacheGetChannel.Type,
		ChannelId:       cacheGetChannel.Id,
		ChannelBaseUrl:  cacheGetChannel.GetBaseURL(),
		ChannelSetting:  cacheGetChannel.GetSetting(),
		HeadersOverride: cacheGetChannel.GetHeaderOverride(),
	}
	info.ChannelSetting.Proxy = cacheGetChannel.GetSetting().Proxy
	info.ApiKey = cacheGetChannel.Key
	adaptor.Init(info)
	disablePollingSleep := cacheGetChannel.GetOtherSettings().DisableTaskPollingSleep
	for i, taskId := range taskIds {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err := updateVideoSingleTask(ctx, adaptor, cacheGetChannel, taskId, taskM); err != nil {
			logger.LogError(ctx, common.LogText("Failed to update video task %s: %s", taskId, err.Error()))
		}
		if disablePollingSleep || i == len(taskIds)-1 {
			continue
		}

		// sleep 1 second between tasks for this channel only.
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(1 * time.Second):
		}
	}
	return nil
}

func updateVideoSingleTask(ctx context.Context, adaptor TaskPollingAdaptor, ch *model.Channel, taskId string, taskM map[string]*model.Task) error {
	if ctx.Err() != nil {
		return ctx.Err()
	}
	baseURL := constant.GetChannelBaseURL(ch.Type)
	if ch.GetBaseURL() != "" {
		baseURL = ch.GetBaseURL()
	}
	proxy := ch.GetSetting().Proxy

	task := taskM[taskId]
	if task == nil {
		logger.LogError(ctx, common.LogText("Task %s not found in taskM", taskId))
		return fmt.Errorf("task %s not found", taskId)
	}
	key := ch.Key

	privateData := task.PrivateData
	if privateData.Key != "" {
		key = privateData.Key
	} else if ch.Type == constant.ChannelTypeAgnes && ch.ChannelInfo.IsMultiKey {
		keys := ch.GetKeys()
		requestID := ""
		if privateData.Execution != nil {
			requestID = privateData.Execution.RequestID
		}
		keyIndex, found, indexErr := model.GetLogMultiKeyIndex(requestID, ch.Id)
		if indexErr != nil {
			logger.LogWarn(ctx, fmt.Sprintf("recover Agnes polling key index for task %s: %v", taskId, indexErr))
		}
		if found && keyIndex >= 0 && keyIndex < len(keys) {
			key = keys[keyIndex]
		} else {
			// Old task logs may not include an index. Still select one enabled key
			// instead of sending the newline-delimited channel payload.
			selectedKey, _, keyErr := ch.GetNextEnabledKey()
			if keyErr != nil {
				return fmt.Errorf("get Agnes polling key for task %s: %w", taskId, keyErr)
			}
			key = selectedKey
		}
	}
	fetchBody := map[string]any{
		"task_id": task.GetUpstreamTaskID(),
		"action":  constant.NormalizeTaskAction(task.Action),
	}
	// Agnes v2.0 status queries require video_id, while older gateway builds
	// persisted the response's internal task_id. Preserve the original submit
	// response for the Agnes adaptor so it can recover video_id for those tasks.
	if ch.Type == constant.ChannelTypeAgnes {
		fetchBody["task_data"] = task.Data
	}
	// Providers that require the upstream model on status queries (such as
	// Agnes 2.5) need the model persisted with the task. Prefer the resolved
	// upstream name and fall back to the original request for older tasks.
	if modelName := task.Properties.UpstreamModelName; modelName != "" {
		fetchBody["model"] = modelName
	} else if modelName := task.Properties.OriginModelName; modelName != "" {
		fetchBody["model"] = modelName
	}
	snap := task.Snapshot()
	resp, err := fetchTaskStatus(adaptor, baseURL, key, task, fetchBody, proxy)
	if err != nil {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassTransport, 0, err.Error())
	}
	if resp == nil || resp.Body == nil {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassTransport, 0, "empty response")
	}
	defer resp.Body.Close()
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassTransport, resp.StatusCode, err.Error())
	}

	logger.LogDebug(ctx, "updateVideoSingleTask response: %s", responseBody)

	switch classifyPollHTTP(resp.StatusCode) {
	case pollClassNotFound:
		logger.LogWarn(ctx, common.LogText("task %s poll %s (failures=%d, http=%d): %s", task.TaskID, pollClassNotFound, task.PrivateData.PollFailures, resp.StatusCode, unrecognizedPollDetail("", responseBody)))
		return failTaskFromPoll(ctx, adaptor, task, snap.Status, "Upstream task not found")
	case pollClassAuth:
		logger.LogWarn(ctx, common.LogText("task poll auth failure channel_id=%d task=%s http=%d", ch.Id, task.TaskID, resp.StatusCode))
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassAuth, resp.StatusCode, "")
	case pollClassTransient:
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassTransient, resp.StatusCode, "")
	}

	taskResult := &relaycommon.TaskInfo{}
	// try parse as New API response format
	var responseItems taskdto.TaskResponse[model.Task]
	if err = common.Unmarshal(responseBody, &responseItems); err == nil && responseItems.IsSuccess() {
		logger.LogDebug(ctx, "updateVideoSingleTask parsed as new api response format: %+v", responseItems)
		t := responseItems.Data
		taskResult.TaskID = t.TaskID
		taskResult.Status = string(t.Status)
		taskResult.Url = t.GetResultURL()
		taskResult.Progress = t.Progress
		taskResult.Reason = t.FailReason
		task.Data = t.Data
	} else if taskResult, err = parseTaskStatus(adaptor, task, resp, responseBody); err != nil {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassHookError, resp.StatusCode, err.Error())
	}

	logger.LogDebug(ctx, "updateVideoSingleTask taskResult: %+v", taskResult)

	parsedStatus := model.TaskStatus(taskResult.Status)
	if parsedStatus == model.TaskStatusUnknown || parsedStatus == "" || !knownPollStatus(parsedStatus) {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassUnrecognized, resp.StatusCode, unrecognizedPollDetail(taskResult.Reason, responseBody))
	}
	if classifyPollHTTP(resp.StatusCode) == pollClassOtherClient && isNonTerminalPollStatus(parsedStatus) {
		return recordPollFailure(ctx, adaptor, task, snap.Status, pollClassUnrecognized, resp.StatusCode, unrecognizedPollDetail(taskResult.Reason, responseBody))
	}

	task.Data = redactVideoResponseBody(responseBody)
	if len(taskResult.PluginState) > 0 {
		task.PrivateData.PluginState = taskResult.PluginState
	}
	if isNonTerminalPollStatus(parsedStatus) {
		task.PrivateData.PollFailures = 0
	}

	now := time.Now().Unix()
	shouldFinalizeBilling := false

	task.Status = parsedStatus
	switch parsedStatus {
	case model.TaskStatusSubmitted:
		task.Progress = taskcommon.ProgressSubmitted
	case model.TaskStatusQueued:
		task.Progress = taskcommon.ProgressQueued
	case model.TaskStatusInProgress:
		task.Progress = taskcommon.ProgressInProgress
		if task.StartTime == 0 {
			task.StartTime = now
		}
	case model.TaskStatusSuccess:
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		if strings.HasPrefix(taskResult.Url, "data:") {
			// data: URI (e.g. Vertex base64 encoded video) — keep in Data, not in ResultURL
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		} else if taskResult.Url != "" {
			// Direct upstream URL (e.g. Kling, Ali, Doubao, etc.)
			task.PrivateData.ResultURL = taskResult.Url
		} else {
			// No URL from adaptor — construct proxy URL using public task ID
			task.PrivateData.ResultURL = taskcommon.BuildProxyURL(task.TaskID)
		}
		if durationProvider, ok := adaptor.(taskAudioDurationProvider); ok {
			if duration, err := durationProvider.ProbeAudioDuration(ctx, task); err == nil && (duration > 0 || math.IsNaN(duration)) {
				// A failed measurement cannot displace valid provider usage. If it
				// is the only measurement, the expression must retain the reservation.
				_, valid := settlementFact("seconds", taskResult.UsageFacts["seconds"])
				if duration <= relaycommon.MaxTaskDurationSeconds || !valid {
					if taskResult.UsageFacts == nil {
						taskResult.UsageFacts = make(map[string]any)
					}
					taskResult.UsageFacts["seconds"] = duration
				}
			}
		}
		shouldFinalizeBilling = true
	case model.TaskStatusFailure:
		logger.LogJson(ctx, common.LogText("Task %s failed", taskId), task)
		task.Status = model.TaskStatusFailure
		task.Progress = taskcommon.ProgressComplete
		if task.FinishTime == 0 {
			task.FinishTime = now
		}
		task.FailReason = taskResult.Reason
		logger.LogInfo(ctx, common.LogText("Task %s failed: %s", task.TaskID, task.FailReason))
		taskResult.Progress = taskcommon.ProgressComplete
		shouldFinalizeBilling = true
	}
	if taskResult.Progress != "" {
		task.Progress = taskResult.Progress
	}

	isDone := task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure
	if isDone && snap.Status != task.Status {
		if task.PrivateData.SubscriptionRequestID != "" {
			if err := StoreTaskSettlementFacts(task, taskResult); err != nil {
				return err
			}
		}
		won, err := task.UpdateWithStatus(snap.Status)
		if err != nil {
			logger.LogError(ctx, common.LogText("UpdateWithStatus failed for task %s: %s", task.TaskID, err.Error()))
			shouldFinalizeBilling = false
		} else if !won {
			logger.LogWarn(ctx, common.LogText("Task %s CAS lost or no-op update, skip billing", task.TaskID))
			shouldFinalizeBilling = false
		}
	} else if !snap.Equal(task.Snapshot()) {
		if _, err := task.UpdateWithStatus(snap.Status); err != nil {
			logger.LogError(ctx, common.LogText("Failed to update task %s: %s", task.TaskID, err.Error()))
		}
	} else {
		// No changes, skip update
		logger.LogDebug(ctx, "No update needed for task %s", task.TaskID)
	}

	if shouldFinalizeBilling {
		finalizeTerminalTask(ctx, adaptor, task, taskResult)
	}

	return nil
}

// finalizeTerminalTask 终态统一收尾（状态 CAS 赢家调用，恰好一次）：采样 + 结算 + 失败兜底退款。
func finalizeTerminalTask(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) {
	if !task.PrivateData.SubscriptionFinancialComplete {
		billingSettled := settleTaskBillingOnComplete(ctx, adaptor, task, taskResult)
		if task.Status == model.TaskStatusFailure && !billingSettled && (task.Quota != 0 || task.PrivateData.SubscriptionRequestID != "") {
			RefundTaskQuota(ctx, task, task.FailReason)
		}
	}
	// 新订阅任务只在终结收据落账后上报一次：资金步骤失败时保留上报资格给重试，
	// 采样与上报都不能先于扣款发生，也不能被并发的第二个终结路径重复。
	if task.PrivateData.SubscriptionRequestID != "" {
		if task.SubscriptionBillingPending {
			return
		}
		claimed, err := model.ClaimSubscriptionTaskReport(task)
		if err != nil {
			logger.LogWarn(ctx, common.LogText("failed to claim the task report: %s", err))
			return
		}
		if !claimed {
			return
		}
	}
	perfmetrics.RecordTaskResult(task, taskResult)
	reportFinalTask(task)
}

// bulkFailedTasks are the tasks of upstream ids a bulk update failed.
func bulkFailedTasks(upstreamIDs []string, tasks map[string]*model.Task) []*model.Task {
	failed := make([]*model.Task, 0, len(upstreamIDs))
	for _, upstreamID := range upstreamIDs {
		failed = append(failed, tasks[upstreamID])
	}
	return failed
}

func redactVideoResponseBody(body []byte) []byte {
	// Only the redacted members change; the rest keeps the upstream bytes.
	if !ojson.ValidBytes(body) || !ojson.GetBytesNoCopy(body, "@this").IsObject() {
		return body
	}
	doc := ojson.NewDoc(body)
	if resp := ojson.GetBytesNoCopy(body, "response"); resp.IsObject() {
		_ = doc.Delete("response.bytesBase64Encoded")
		if video := resp.Get("video"); video.Type == ojson.String {
			_ = doc.Set("response.video", truncateBase64(video.Str))
		}
		if videos := resp.Get("videos"); videos.IsArray() {
			for i, video := range videos.Array() {
				if video.IsObject() {
					_ = doc.Delete(fmt.Sprintf("response.videos.%d.bytesBase64Encoded", i))
				}
			}
		}
	}
	if steps := ojson.GetBytesNoCopy(body, "steps"); steps.IsArray() {
		for i, step := range steps.Array() {
			parts := step.Get("content")
			if !parts.IsArray() {
				continue
			}
			for j, part := range parts.Array() {
				if part.Get("type").String() == "video" {
					_ = doc.Delete(fmt.Sprintf("steps.%d.content.%d.data", i, j))
				}
			}
		}
	}
	return doc.Bytes()
}

func truncateBase64(s string) string {
	const maxKeep = 256
	if len(s) <= maxKeep {
		return s
	}
	return s[:maxKeep] + "..."
}

// settleTaskBillingOnComplete 任务完成时的统一计费调整。
// 返回 true 表示用量结算路径已接管最终计费；失败任务仅在返回 false 时补做全额退款。
// 优先级：1. tiered snapshot → 2. adaptor 调整 → 3. token 重算。
//
// 表达式求值失败会保留预扣额度，因此也视为已接管，避免错误全退。
func settleTaskBillingOnComplete(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, taskResult *relaycommon.TaskInfo) bool {
	// 已固定的最终金额是上一次终结尝试做出的决定：重试只执行它，不按当前
	// 快照重新推导金额；扣款失败也不回落到全额退款，避免同一任务两次不同结论。
	if pinned := task.PrivateData.SubscriptionFinalBilling; pinned != nil {
		if _, err := model.SettleSubscriptionTaskQuota(task, pinned.Quota); err != nil {
			logger.LogWarn(ctx, common.LogText("failed to settle the pinned final quota of task %s: %s", task.TaskID, err))
		}
		return true
	}
	if bc := task.PrivateData.BillingContext; bc != nil && bc.BillingFree {
		if task.Quota != 0 || task.PrivateData.SubscriptionRequestID != "" {
			RecalculateTaskQuota(ctx, task, 0, common.NewMessage("Model price is below the free threshold"), nil)
		}
		return true
	}
	if bc := task.PrivateData.BillingContext; bc != nil && bc.TieredSnapshot != nil {
		// 用量表达式结算只适用于成功任务；失败任务由调用方全额退款。
		if task.Status == model.TaskStatusFailure {
			return false
		}
		if task.PrivateData.SubscriptionInvalidUsage {
			logger.LogWarn(ctx, common.LogText("task %s expression settlement failed, keeping the pre-consumed quota: %v", task.TaskID, "invalid completion usage"))
			finishTaskSubscriptionFunding(ctx, task)
			return true
		}
		result, usageFacts, err := EvaluateTaskCompletionUsage(bc.TieredSnapshot, taskResult.UsageFacts)
		if err != nil {
			logger.LogWarn(ctx, common.LogText("task %s expression settlement failed, keeping the pre-consumed quota: %v", task.TaskID, err))
			if task.PrivateData.SubscriptionRequestID != "" {
				actual := task.Quota
				if task.PrivateData.SubscriptionSubmitQuota != nil {
					actual = *task.PrivateData.SubscriptionSubmitQuota
				}
				RecalculateTaskQuota(ctx, task, actual, common.NewMessage("Pre-consumed"))
			}
			return true
		}
		if result.Clamp != nil {
			logger.LogWarn(ctx, common.LogText("task %s expression settlement quota was clamped: %+v", task.TaskID, result.Clamp))
		}
		bc.TieredSnapshot.UsageFacts = usageFacts
		bc.TieredSnapshot.EstimatedTier = result.MatchedTier
		RecalculateTaskQuota(ctx, task, result.ActualQuotaAfterGroup, common.NewMessage("Settled by the task usage expression"), result.Clamp)
		return true
	}
	// 按次计费的成功任务保持预扣；失败任务由调用方全额退款。
	if bc := task.PrivateData.BillingContext; bc != nil && bc.PerCallBilling {
		if task.Status == model.TaskStatusSuccess {
			if task.PrivateData.SubscriptionSubmitQuota != nil && task.PrivateData.SubscriptionRequestID != "" {
				RecalculateTaskQuota(ctx, task, *task.PrivateData.SubscriptionSubmitQuota, common.NewMessage("Pre-consumed"))
			} else {
				finishTaskSubscriptionFunding(ctx, task)
			}
		}
		logger.LogInfo(ctx, common.LogText("task %s is billed per request, skipping difference settlement", task.TaskID))
		return false
	}
	// 优先让 adaptor 决定最终额度。平台适配器在重启后可能不可用，
	// 只按已确定的金额（按次/免费/阶梯/已对齐）终结的收据不依赖它。
	if task.PrivateData.SubscriptionRequestID != "" && adaptor == nil {
		return true
	}
	if adaptor != nil {
		if actualQuota := adaptor.AdjustBillingOnComplete(task, taskResult); actualQuota > 0 {
			RecalculateTaskQuota(ctx, task, actualQuota, common.NewMessage("Adjusted by the channel adaptor"))
			return true
		}
	}
	// 回退到 token 重算。
	tokens := taskResult.TotalTokens
	if tokens == 0 && taskResult.CompletionTokens > 0 {
		tokens = taskResult.CompletionTokens
	}
	if tokens > 0 && RecalculateTaskQuotaByTokens(ctx, task, tokens) {
		return true
	}
	if task.Status == model.TaskStatusSuccess {
		finishTaskSubscriptionFunding(ctx, task)
	}
	return false
}

func classifyPollHTTP(statusCode int) string {
	switch {
	case statusCode >= 200 && statusCode < 300:
		return pollClassOK
	case statusCode == http.StatusNotFound || statusCode == http.StatusGone:
		return pollClassNotFound
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusForbidden:
		return pollClassAuth
	case statusCode == http.StatusTooManyRequests || statusCode >= 500:
		return pollClassTransient
	case statusCode >= 400 && statusCode < 500:
		return pollClassOtherClient
	default:
		return pollClassTransient
	}
}

func knownPollStatus(status model.TaskStatus) bool {
	switch status {
	case model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusInProgress, model.TaskStatusSuccess, model.TaskStatusFailure:
		return true
	default:
		return false
	}
}

func isNonTerminalPollStatus(status model.TaskStatus) bool {
	switch status {
	case model.TaskStatusNotStart, model.TaskStatusSubmitted, model.TaskStatusQueued, model.TaskStatusInProgress:
		return true
	default:
		return false
	}
}

// unrecognizedPollDetail pairs the plugin's reason with a bounded copy of the
// upstream body so the WARN line is enough to diagnose a parser gap.
func unrecognizedPollDetail(reason string, body []byte) string {
	const maxBodyChars = 512
	redacted := string(redactVideoResponseBody(body))
	if len(redacted) > maxBodyChars {
		redacted = redacted[:maxBodyChars] + "…"
	}
	if strings.TrimSpace(reason) == "" {
		return "body=" + redacted
	}
	return reason + "; body=" + redacted
}

func recordPollFailure(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, fromStatus model.TaskStatus, class string, statusCode int, detail string) error {
	task.PrivateData.PollFailures++
	// TASK_POLL_MAX_FAILURES <= 0 disables the consecutive-failure cutoff, matching
	// TASK_TIMEOUT_MINUTES semantics; the 72h sweep (TASK_TIMEOUT_MINUTES default
	// 4320) remains the only backstop.
	giveUp := constant.TaskPollMaxFailures > 0 && task.PrivateData.PollFailures >= constant.TaskPollMaxFailures
	if giveUp || class == pollClassUnrecognized || class == pollClassHookError {
		// The redacted body is intentionally not persisted to Task.Data on these
		// paths, and the fail reason of a task that is given up is a fixed
		// sentence, so the WARN line is the only operator-visible copy of the
		// class, the status and what the plugin could not interpret.
		logger.LogWarn(ctx, common.LogText("task %s poll %s (failures=%d, http=%d): %s", task.TaskID, class, task.PrivateData.PollFailures, statusCode, detail))
	}
	if giveUp {
		return failTaskFromPoll(ctx, adaptor, task, fromStatus, "Failed to query the upstream task status")
	}
	if _, err := task.UpdateWithStatus(fromStatus); err != nil {
		return err
	}
	return nil
}

func recordPollFailureForTasks(ctx context.Context, adaptor TaskPollingAdaptor, tasks []*model.Task, class string, statusCode int, detail string) error {
	var firstErr error
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if err := recordPollFailure(ctx, adaptor, task, task.Status, class, statusCode, detail); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func failTaskFromPoll(ctx context.Context, adaptor TaskPollingAdaptor, task *model.Task, fromStatus model.TaskStatus, reason string) error {
	now := time.Now().Unix()
	task.Status = model.TaskStatusFailure
	task.Progress = taskcommon.ProgressComplete
	if task.FinishTime == 0 {
		task.FinishTime = now
	}
	task.FailReason = reason
	pinLocalSubscriptionRefund(task)
	won, err := task.UpdateWithStatus(fromStatus)
	if err != nil {
		return err
	}
	if !won {
		return nil
	}
	finalizeTerminalTask(ctx, adaptor, task, relaycommon.FailTaskInfo(reason))
	return nil
}

func failTasksFromPoll(ctx context.Context, adaptor TaskPollingAdaptor, tasks []*model.Task, reason string) error {
	var firstErr error
	for _, task := range tasks {
		if task == nil {
			continue
		}
		if err := failTaskFromPoll(ctx, adaptor, task, task.Status, reason); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
