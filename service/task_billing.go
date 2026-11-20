package service

import (
	"context"
	"encoding/base64"
	"fmt"
	"maps"
	"math"
	"strings"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/billingexpr"
	"github.com/QuantumNous/new-api/pkg/jsplugin"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/expr-lang/expr/ast"
	"github.com/gin-gonic/gin"
)

// LogTaskConsumption 记录任务消费日志和统计信息（仅记录，不涉及实际扣费）。
// 实际扣费已由 BillingSession（PreConsumeBilling + SettleBilling）完成。
func LogTaskConsumption(c *gin.Context, info *relaycommon.RelayInfo, task *model.Task) {
	tokenName := c.GetString("token_name")
	logContent := []*common.Message{common.NewMessage("Action {{action}}", map[string]any{"action": info.Action})}
	// 支持任务仅按次计费
	if common.StringsContains(constant.TaskPricePatches, info.OriginModelName) {
		logContent = append(logContent, common.NewMessage("Billed per request"))
	} else {
		var contents []string
		if otherRatios := info.PriceData.OtherRatios(); len(otherRatios) > 0 {
			for key, ra := range otherRatios {
				if 1.0 != ra {
					contents = append(contents, fmt.Sprintf("%s: %.2f", key, ra))
				}
			}
		}
		if snap := info.TieredBillingSnapshot; snap != nil {
			for key, value := range snap.UsageFacts {
				contents = append(contents, fmt.Sprintf("%s: %v", key, value))
			}
		}
		if len(contents) > 0 {
			logContent = append(logContent, common.NewMessage("Billing parameters: {{params}}", map[string]any{"params": strings.Join(contents, ", ")}))
		}
	}
	other := model.NewLogOther()
	other.SetPublic("is_task", true)
	other.SetPublic("request_path", c.Request.URL.Path)
	if taskDeliveredInline(c, task) {
		other.SetPublic("task_sync", true)
	}
	other.SetPublic("model_price", info.PriceData.ModelPrice)
	if info.PriceData.ModelRatio > 0 {
		other.SetPublic("model_ratio", info.PriceData.ModelRatio)
	}
	other.SetPublic("group_ratio", info.PriceData.GroupRatioInfo.GroupRatio)
	if info.PriceData.GroupRatioInfo.HasSpecialRatio {
		other.SetPublic("user_group_ratio", info.PriceData.GroupRatioInfo.GroupSpecialRatio)
	}
	if info.IsModelMapped {
		other.SetPublic("is_model_mapped", true)
		other.SetPublic("upstream_model_name", info.UpstreamModelName)
	}
	if snap := info.TieredBillingSnapshot; snap != nil {
		other.SetPublic("billing_mode", "tiered_expr")
		other.SetPublic("expr_b64", base64.StdEncoding.EncodeToString([]byte(snap.ExprString)))
		if snap.UsingGroup != "" {
			other.SetPublic("billing_group", snap.UsingGroup)
		}
		other.SetPublic("matched_tier", snap.EstimatedTier)
		if len(snap.UsageFacts) > 0 {
			other.SetPublic("usage_facts", snap.UsageFacts)
		}
		setTaskImageCount(other, snap.UsageFacts["image_count"])
	} else {
		setTaskImageCount(other, info.PriceData.OtherRatios()["image_count"])
	}
	appendTaskLogInfo(task, other)
	attachQuotaSaturation(c, info, other)
	if len(info.RequestPluginSteps) > 0 {
		other.SetAdmin("request_plugins", info.RequestPluginSteps)
	}
	appendRequestPluginRecord(info, other)
	consumeParams := model.RecordConsumeLogParams{
		ChannelId:    info.ChannelId,
		PromptTokens: info.GetEstimatePromptTokens(),
		ModelName:    info.OriginModelName,
		TokenName:    tokenName,
		Quota:        info.PriceData.Quota,
		Content:      logContent,
		TokenId:      info.TokenId,
		Group:        info.UsingGroup,
		Other:        other,
	}
	if task.PrivateData.SubscriptionRequestID != "" && task.PrivateData.SubscriptionSubmitQuota != nil {
		// Settlement writes this consume log after the request has ended, so it
		// keeps what RecordConsumeLog reads from the request: report hooks get
		// the consume params whether or not consume logs are kept, and the log
		// keeps the request correlation and the IP the user chose to log.
		common.SetContextKey(c, constant.ContextKeyConsumeLog, &consumeParams)
		submitLog := &model.RecordTaskBillingLogParams{
			UserId:            info.UserId,
			LogType:           model.LogTypeConsume,
			ChannelId:         consumeParams.ChannelId,
			ModelName:         consumeParams.ModelName,
			Quota:             consumeParams.Quota,
			Content:           consumeParams.Content,
			TokenId:           consumeParams.TokenId,
			Group:             consumeParams.Group,
			Other:             consumeParams.Other,
			Execution:         task.PrivateData.Execution,
			NodeName:          task.PrivateData.NodeName,
			RequestId:         c.GetString(common.RequestIdKey),
			UpstreamRequestId: c.GetString(common.UpstreamRequestIdKey),
			PromptTokens:      consumeParams.PromptTokens,
			TokenName:         consumeParams.TokenName,
		}
		if common.LogConsumeEnabled {
			if settingMap, err := model.GetUserSetting(info.UserId, false); err == nil && settingMap.RecordIpLog {
				submitLog.Ip = c.ClientIP()
			}
		}
		task.PrivateData.SubscriptionSubmitLog = submitLog
		return
	}
	model.RecordConsumeLog(c, info.UserId, consumeParams)
	model.UpdateUserUsedQuotaAndRequestCount(info.UserId, info.PriceData.Quota)
	model.UpdateChannelUsedQuota(info.ChannelId, info.PriceData.Quota)
}

// ---------------------------------------------------------------------------
// 异步任务计费辅助函数
// ---------------------------------------------------------------------------

// resolveTokenKey 通过 TokenId 运行时获取令牌 Key（用于 Redis 缓存操作）。
// 如果令牌已被删除或查询失败，返回空字符串。
func resolveTokenKey(ctx context.Context, tokenId int, taskID string) string {
	token, err := model.GetTokenById(tokenId)
	if err != nil {
		logger.LogWarn(ctx, common.LogText("failed to get token key (tokenId=%d, task=%s): %s", tokenId, taskID, err.Error()))
		return ""
	}
	return token.Key
}

// taskIsSubscription 判断任务是否通过订阅计费。
func taskIsSubscription(task *model.Task) bool {
	return task.PrivateData.BillingSource == BillingSourceSubscription && task.PrivateData.SubscriptionId > 0
}

func finishTaskSubscriptionFunding(ctx context.Context, task *model.Task) {
	if !taskIsSubscription(task) || task.PrivateData.SubscriptionRequestID == "" {
		return
	}
	actual := task.Quota
	if target := task.PrivateData.SubscriptionSubmitQuota; target != nil {
		actual = *target // Submission was persisted but its charge was not aligned yet.
	}
	if _, err := model.SettleSubscriptionTaskQuota(task, actual); err != nil {
		logger.LogWarn(ctx, common.LogText("failed to finalize subscription request receipt: %s", err))
	}
}

// taskAdjustFunding 调整任务的资金来源（钱包或订阅），delta > 0 表示扣费，delta < 0 表示退还。
func taskAdjustFunding(task *model.Task, delta int) error {
	if taskIsSubscription(task) {
		if task.PrivateData.SubscriptionRequestID == "" {
			return model.PostConsumeUserSubscriptionDelta(task.PrivateData.SubscriptionId, int64(delta))
		}
		return adjustSubscriptionFunding(task.PrivateData.SubscriptionId, task.PrivateData.SubscriptionRequestID, int64(delta), true)
	}
	if delta > 0 {
		return model.DecreaseUserQuota(task.UserId, delta, false)
	}
	return model.IncreaseUserQuota(task.UserId, -delta, false)
}

// taskAdjustTokenQuota 调整任务的令牌额度，delta > 0 表示扣费，delta < 0 表示退还。
// 需要通过 resolveTokenKey 运行时获取 key（不从 PrivateData 中读取）。
func taskAdjustTokenQuota(ctx context.Context, task *model.Task, delta int) {
	if task.PrivateData.TokenId <= 0 || delta == 0 {
		return
	}
	tokenKey := resolveTokenKey(ctx, task.PrivateData.TokenId, task.TaskID)
	if tokenKey == "" {
		return
	}
	var err error
	if delta > 0 {
		err = model.DecreaseTokenQuota(task.PrivateData.TokenId, tokenKey, delta)
	} else {
		err = model.IncreaseTokenQuota(task.PrivateData.TokenId, tokenKey, -delta)
	}
	if err != nil {
		logger.LogWarn(ctx, common.LogText("failed to adjust token quota (delta=%d, task=%s): %s", delta, task.TaskID, err.Error()))
	}
}

// taskBillingOther 从 task 的 BillingContext 构建日志 Other 字段。
func taskBillingOther(task *model.Task) *model.LogOther {
	other := model.NewLogOther()
	// 前端列表按 is_task 选择任务档位渲染分支,缺失会把差额/退款日志误判成普通 relay 动态计费
	other.SetPublic("is_task", true)
	if bc := task.PrivateData.BillingContext; bc != nil {
		other.SetPublic("model_price", bc.ModelPrice)
		if bc.ModelRatio > 0 {
			other.SetPublic("model_ratio", bc.ModelRatio)
		}
		other.SetPublic("group_ratio", bc.GroupRatio)
		if priceData := taskBillingContextPriceData(bc); priceData != nil {
			for k, v := range priceData.OtherRatios() {
				if !other.SetPublic(k, v) {
					common.SysError(common.LogText("task billing other ratio key rejected: %s", k))
				}
			}
		}
		if snap := bc.TieredSnapshot; snap != nil {
			other.SetPublic("billing_mode", "tiered_expr")
			other.SetPublic("expr_b64", base64.StdEncoding.EncodeToString([]byte(snap.ExprString)))
			if snap.UsingGroup != "" {
				other.SetPublic("billing_group", snap.UsingGroup)
			}
			other.SetPublic("matched_tier", snap.EstimatedTier)
			if len(snap.UsageFacts) > 0 {
				other.SetPublic("usage_facts", snap.UsageFacts)
			}
			setTaskImageCount(other, snap.UsageFacts["image_count"])
		} else if priceData := taskBillingContextPriceData(bc); priceData != nil {
			setTaskImageCount(other, priceData.OtherRatios()["image_count"])
		}
	}
	props := task.Properties
	if props.UpstreamModelName != "" && props.UpstreamModelName != props.OriginModelName {
		other.SetPublic("is_model_mapped", true)
		other.SetPublic("upstream_model_name", props.UpstreamModelName)
	}
	appendTaskLogInfo(task, other)
	return other
}

// setTaskImageCount publishes the billed image quantity of an image task as
// other.image_count, the same field the HTTP image relay writes, so the log
// detail shows one "billable image count" regardless of the serving path. The
// value is a host-validated count fact (at most dto.MaxImageN), never a quota.
func setTaskImageCount(other *model.LogOther, value any) {
	count, ok := value.(float64)
	if !ok || count < 0 || count > float64(dto.MaxImageN) {
		return
	}
	other.SetPublic("image_count", common.QuotaRound(count))
}

// taskDeliveredInline reports whether the submitting HTTP request itself
// returns the task deliverable: the upstream completed immediately, or the
// request came through the synchronous OpenAI Images protocol, where the host
// waits for an asynchronous upstream task before answering. The usage log
// presents such requests as synchronous instead of as asynchronous jobs.
func taskDeliveredInline(c *gin.Context, task *model.Task) bool {
	if task != nil && (task.Status == model.TaskStatusSuccess || task.Status == model.TaskStatusFailure) {
		return true
	}
	pinnedValue, exists := c.Get(jsplugin.ContextKeyPinnedEndpoint)
	if !exists {
		return false
	}
	pinned, ok := pinnedValue.(jsplugin.PinnedEndpoint)
	return ok && pinned.Protocol == jsplugin.ProtocolOpenAIImage
}

func appendTaskLogInfo(task *model.Task, other *model.LogOther) {
	if task == nil || other == nil {
		return
	}
	if task.TaskID != "" {
		other.SetPublic("task_id", task.TaskID)
	}
	if task.PrivateData.ResultDiscarded {
		// The result was delivered inline and the upstream snapshot was not
		// persisted, so no artifact can be retrieved for this task.
		other.SetPublic("result_discarded", true)
	}
	if task.PrivateData.Execution != nil {
		AppendTaskPluginAuditInfo(other, task.PrivateData.Execution.TaskPlugin)
	}
	if task.PrivateData.UpstreamTaskID == "" && task.PrivateData.NodeName == "" {
		return
	}
	if task.PrivateData.UpstreamTaskID != "" {
		other.SetRoot("upstream_task_id", task.PrivateData.UpstreamTaskID)
	}
	if task.PrivateData.NodeName != "" {
		other.SetRoot("node_name", task.PrivateData.NodeName)
	}
}

func taskBillingContextPriceData(bc *model.TaskBillingContext) *types.PriceData {
	if bc == nil || len(bc.OtherRatios) == 0 {
		return nil
	}
	priceData := &types.PriceData{}
	if !priceData.ReplaceOtherRatios(bc.OtherRatios) {
		return nil
	}
	return priceData
}

// taskModelName 从 BillingContext 或 Properties 中获取模型名称。
func taskModelName(task *model.Task) string {
	if bc := task.PrivateData.BillingContext; bc != nil && bc.OriginModelName != "" {
		return bc.OriginModelName
	}
	return task.Properties.OriginModelName
}

// RefundTaskQuota 统一的任务失败退款逻辑。
// 当异步任务失败时，退还资金与令牌额度，并回减用户和渠道用量。
// 返回资金来源是否已成功退还；失败时保留 quota，供显式重试或人工对账。
func RefundTaskQuota(ctx context.Context, task *model.Task, reason string) bool {
	quota := task.Quota
	if quota == 0 {
		if taskIsSubscription(task) && task.PrivateData.SubscriptionRequestID != "" {
			_, err := model.SettleSubscriptionTaskQuota(task, 0)
			return err == nil
		}
		return true
	}

	// 1. 退还资金来源（钱包或订阅）
	fundingErr := error(nil)
	if taskIsSubscription(task) && task.PrivateData.SubscriptionRequestID != "" {
		var delta int
		other := taskBillingOther(task)
		other.SetPublic("reason", reason)
		delta, fundingErr = model.SettleSubscriptionTaskQuota(task, 0, model.RecordTaskBillingLogParams{ModelName: taskModelName(task), Other: other, Content: nil})
		quota = -delta
	} else {
		fundingErr = taskAdjustFunding(task, -quota)
	}
	if err := fundingErr; err != nil {
		logger.LogWarn(ctx, common.LogText("failed to refund the funding source, task %s: %s", task.TaskID, err.Error()))
		return false
	}

	if quota == 0 {
		return true
	}
	if taskIsSubscription(task) && task.PrivateData.SubscriptionRequestID != "" {
		if err := model.FlushSubscriptionTaskBillingLogs(); err != nil {
			logger.LogWarn(ctx, common.LogText("failed to record task billing log: %s", err))
		}
		return true
	}
	finishTaskSubscriptionFunding(ctx, task)
	// 2. 退还令牌额度
	if task.PrivateData.SubscriptionRequestID == "" {
		taskAdjustTokenQuota(ctx, task, -quota)
	}

	// 3. 回减预扣时累计的用户和渠道用量，请求次数保持不变
	model.UpdateUserUsedQuota(task.UserId, -quota)
	model.UpdateChannelUsedQuota(task.ChannelId, -quota)

	// 4. 记录日志
	other := taskBillingOther(task)
	other.SetPublic("task_id", task.TaskID)
	other.SetPublic("reason", reason)
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		Execution: task.PrivateData.Execution,
		UserId:    task.UserId,
		LogType:   model.LogTypeRefund,
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     quota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
	})

	// 5. 资金退款完成后再清除持久化标记。
	// 回写失败必须显式告警，避免漏掉潜在的重复退款风险。
	task.Quota = 0
	if err := task.UpdateQuota(); err != nil {
		logger.LogError(ctx, common.LogText("refund succeeded but clearing the task quota failed, task %s: %s", task.TaskID, err.Error()))
	}
	return true
}

// RecalculateTaskQuota 通用的异步差额结算。
// actualQuota 是任务完成后的实际应扣额度，与预扣额度 (task.Quota) 做差额结算。
// reason 写入日志内容（例如按 token 重算或 adaptor 调整）。
// clamps 可选：若计算 actualQuota 时发生额度饱和，将其记入日志 admin_info（仅管理员可见）。
func RecalculateTaskQuota(ctx context.Context, task *model.Task, actualQuota int, reason *common.Message, clamps ...*common.QuotaClamp) {
	if actualQuota < 0 {
		return
	}
	preConsumedQuota := task.Quota
	quotaDelta := actualQuota - preConsumedQuota

	if quotaDelta == 0 {
		if taskIsSubscription(task) && task.PrivateData.SubscriptionRequestID != "" {
			if _, err := model.SettleSubscriptionTaskQuota(task, actualQuota); err != nil {
				logger.LogError(ctx, common.LogText("failed to settle subscription request: %s", err))
				return
			}
		}
		finishTaskSubscriptionFunding(ctx, task)
		logger.LogInfo(ctx, common.LogText("task %s pre-consumed quota matches the actual quota (%s, %s)",
			task.TaskID, logger.LogQuota(actualQuota), reason.Error()))
		return
	}

	logger.LogInfo(ctx, common.LogText("task %s settles the difference: delta=%s (actual %s, pre-consumed %s, %s)",
		task.TaskID,
		logger.LogQuota(quotaDelta),
		logger.LogQuota(actualQuota),
		logger.LogQuota(preConsumedQuota),
		reason.Error(),
	))

	// 调整资金来源
	fundingErr := error(nil)
	if taskIsSubscription(task) && task.PrivateData.SubscriptionRequestID != "" {
		other := taskBillingOther(task)
		for _, clamp := range clamps {
			attachQuotaSaturationToOther(other, clamp)
		}
		quotaDelta, fundingErr = model.SettleSubscriptionTaskQuota(task, actualQuota, model.RecordTaskBillingLogParams{ModelName: taskModelName(task), Other: other, Content: []*common.Message{reason}})
	} else {
		fundingErr = taskAdjustFunding(task, quotaDelta)
	}
	if err := fundingErr; err != nil {
		logger.LogError(ctx, common.LogText("failed to adjust funding for the difference, task %s: %s", task.TaskID, err.Error()))
		return
	}

	if quotaDelta == 0 {
		return
	}
	if taskIsSubscription(task) && task.PrivateData.SubscriptionRequestID != "" {
		if err := model.FlushSubscriptionTaskBillingLogs(); err != nil {
			logger.LogWarn(ctx, common.LogText("failed to record task billing log: %s", err))
		}
		return
	}
	finishTaskSubscriptionFunding(ctx, task)
	// 调整令牌额度
	if task.PrivateData.SubscriptionRequestID == "" {
		taskAdjustTokenQuota(ctx, task, quotaDelta)
	}

	task.Quota = actualQuota
	if err := task.UpdateQuota(); err != nil {
		logger.LogError(ctx, common.LogText("failed to write back the settled task quota, task %s: %s", task.TaskID, err.Error()))
	}

	// 提交阶段已经累计过一次请求；结算阶段只调整最终用量。
	model.UpdateUserUsedQuota(task.UserId, quotaDelta)
	model.UpdateChannelUsedQuota(task.ChannelId, quotaDelta)

	var logType int
	var logQuota int
	if quotaDelta > 0 {
		logType = model.LogTypeConsume
		logQuota = quotaDelta
	} else {
		logType = model.LogTypeRefund
		logQuota = -quotaDelta
	}
	other := taskBillingOther(task)
	other.SetPublic("task_id", task.TaskID)
	other.SetPublic("pre_consumed_quota", preConsumedQuota)
	other.SetPublic("actual_quota", actualQuota)
	for _, clamp := range clamps {
		attachQuotaSaturationToOther(other, clamp)
	}
	model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
		Execution: task.PrivateData.Execution,
		UserId:    task.UserId,
		LogType:   logType,
		Content:   []*common.Message{reason},
		ChannelId: task.ChannelId,
		ModelName: taskModelName(task),
		Quota:     logQuota,
		TokenId:   task.PrivateData.TokenId,
		Group:     task.Group,
		Other:     other,
		NodeName:  task.PrivateData.NodeName,
	})
}

// RecalculateTaskQuotaByTokens 根据实际 token 消耗重新计费（异步差额结算）。
// 当任务成功且返回了 totalTokens 时，根据模型倍率和分组倍率重新计算实际扣费额度，
// 与预扣费的差额进行补扣或退还。支持钱包和订阅计费来源。
func RecalculateTaskQuotaByTokens(ctx context.Context, task *model.Task, totalTokens int) bool {
	if totalTokens <= 0 {
		return false
	}

	modelRatio, finalGroupRatio, err := taskTokenRatios(task)
	if err != nil {
		logger.LogError(ctx, common.LogText("failed to resolve task billing group: %s", err))
		return task.Status == model.TaskStatusSuccess && task.PrivateData.SubscriptionRequestID != ""
	}
	if modelRatio <= 0 {
		return false
	}

	// 计算 OtherRatios 乘积（视频折扣、时长等）
	otherMultiplier := 1.0
	if priceData := taskBillingContextPriceData(task.PrivateData.BillingContext); priceData != nil {
		otherMultiplier = priceData.OtherRatioMultiplier()
	}

	// 计算实际应扣费额度: totalTokens * modelRatio * groupRatio * otherMultiplier（饱和转换，防止溢出成负数）
	actualQuota, clamp := common.QuotaFromFloatChecked(float64(totalTokens) * modelRatio * finalGroupRatio * otherMultiplier)

	reason := common.NewMessage("Recalculated from tokens: tokens {{tokens}}, model ratio {{model_ratio}}, group ratio {{group_ratio}}, other multiplier {{other_multiplier}}", map[string]any{
		"tokens":           totalTokens,
		"model_ratio":      fmt.Sprintf("%.2f", modelRatio),
		"group_ratio":      fmt.Sprintf("%.2f", finalGroupRatio),
		"other_multiplier": fmt.Sprintf("%.4f", otherMultiplier),
	})
	RecalculateTaskQuota(ctx, task, actualQuota, reason, clamp)
	return true
}

func taskTokenRatios(task *model.Task) (float64, float64, error) {
	// Receipt-backed submissions already persist both effective ratios. Never
	// price their recorded tokens against settings changed after acceptance.
	if bc := task.PrivateData.BillingContext; task.PrivateData.SubscriptionRequestID != "" && bc != nil {
		return bc.ModelRatio, bc.GroupRatio, nil
	}
	ratio, ok, _ := ratio_setting.GetModelRatio(taskModelName(task))
	if !ok || ratio <= 0 {
		return 0, 0, nil
	}
	group := task.Group
	if group == "" {
		if user, err := model.GetUserById(task.UserId, false); err == nil {
			group = user.Group
		}
	}
	if group == "" {
		return 0, 0, fmt.Errorf("task billing group is missing")
	}
	groupRatio, _, err := GetUserGroupRatio(group, group)
	return ratio, groupRatio, err
}

// StoreTaskSettlementFacts records only the bounded inputs needed after a
// restart. The result URL, provider payload and plugin state live on the task.
func StoreTaskSettlementFacts(task *model.Task, result *relaycommon.TaskInfo) error {
	var expr string
	if bc := task.PrivateData.BillingContext; bc != nil && bc.TieredSnapshot != nil {
		expr = bc.TieredSnapshot.ExprString
	}
	facts, invalid := boundedCompletionFacts(expr, result.UsageFacts)
	snapshot := relaycommon.TaskInfo{Status: result.Status, TotalTokens: result.TotalTokens, CompletionTokens: result.CompletionTokens, UsageFacts: facts}
	raw, err := common.Marshal(snapshot)
	if err != nil {
		return err
	}
	task.PrivateData.SubscriptionFinalResult = raw
	task.PrivateData.SubscriptionInvalidUsage = invalid
	result.UsageFacts = facts // The CAS winner uses the same facts as recovery.
	return nil
}

func boundedCompletionFacts(expr string, input map[string]any) (map[string]any, bool) {
	const maxFacts = 128
	if len(input) == 0 {
		return nil, false
	}
	facts := make(map[string]any, min(len(input), maxFacts))
	needed := billingexpr.UsedUsageKeys(expr)
	oversized := len(input) > maxFacts
	dropped, invalid := false, false
	for key, value := range input {
		// For oversized sets, retain the expression's inputs, not a random
		// prefix of the adaptor's map. Never replay a partial tariff.
		if oversized && !needed[key] {
			dropped = true
			continue
		}
		v, ok := settlementFact(key, value)
		if !ok || len(facts) == maxFacts {
			dropped = true
			invalid = invalid || needed[key]
			continue
		}
		facts[key] = v
	}
	// Literal keys are known at compile time. With a dynamic u(...) lookup,
	// an omitted value cannot safely be assumed irrelevant.
	if dropped && !invalid && expr != "" {
		prog, err := billingexpr.CompileFromCache(expr)
		invalid = err != nil || ast.Find(prog.Node(), func(node ast.Node) bool {
			call, ok := node.(*ast.CallNode)
			if !ok || len(call.Arguments) != 1 {
				return false
			}
			id, ok := call.Callee.(*ast.IdentifierNode)
			_, literal := call.Arguments[0].(*ast.StringNode)
			return ok && id.Value == "u" && !literal
		}) != nil
	}
	return facts, invalid
}

func settlementFact(key string, value any) (any, bool) {
	if len(key) == 0 || len(key) > 256 || !utf8.ValidString(key) {
		return nil, false
	}
	var number float64
	switch v := value.(type) {
	case string:
		return v, len(v) <= 1024 && utf8.ValidString(v)
	case bool:
		return v, true
	case float64:
		number = v
	case int:
		number = float64(v)
	case int64:
		number = float64(v)
	default:
		return nil, false
	}
	limit := float64(common.MaxQuota)
	if key == "seconds" {
		limit = relaycommon.MaxTaskDurationSeconds
	}
	return number, number >= 0 && number <= limit
}

// EvaluateTaskCompletionUsage evaluates actual facts against the frozen task
// expression. It neither mutates the snapshot nor moves funds: synchronous
// submission and polling have different persistence and settlement barriers.
func EvaluateTaskCompletionUsage(snap *billingexpr.BillingSnapshot, facts map[string]any) (billingexpr.TieredResult, map[string]any, error) {
	if snap == nil {
		return billingexpr.TieredResult{}, nil, fmt.Errorf("task billing snapshot is missing")
	}
	facts, invalid := boundedCompletionFacts(snap.ExprString, facts)
	if invalid {
		return billingexpr.TieredResult{}, nil, fmt.Errorf("invalid completion usage")
	}
	usage := make(map[string]any, len(snap.UsageFacts)+len(facts))
	maps.Copy(usage, snap.UsageFacts)
	maps.Copy(usage, facts)
	result, err := billingexpr.ComputeTieredQuotaWithRequest(snap, billingexpr.TokenParams{}, billingexpr.RequestInput{Usage: usage})
	if err == nil && (result.ActualQuotaBeforeGroup < 0 || math.IsNaN(result.ActualQuotaBeforeGroup)) {
		err = fmt.Errorf("task completion expression produced an invalid cost")
	}
	return result, usage, err
}
