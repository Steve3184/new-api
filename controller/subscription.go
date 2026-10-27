package controller

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ---- Shared types ----

type SubscriptionPlanDTO struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

type BillingPreferenceRequest struct {
	BillingPreference string `json:"billing_preference"`
}

type SubscriptionBalancePayRequest struct {
	PlanId     int  `json:"plan_id"`
	Multiplier int  `json:"multiplier"`
	AutoRenew  bool `json:"auto_renew"`
}

type SubscriptionRenewalRequest struct {
	AutoRenew       bool `json:"auto_renew"`
	RenewMultiplier int  `json:"renew_multiplier"`
}

type SubscriptionUpgradeRequest struct {
	SubscriptionId int `json:"subscription_id"`
	PlanId         int `json:"plan_id"`
	Multiplier     int `json:"multiplier"`
}

func normalizeSubscriptionPlanInput(plan *model.SubscriptionPlan) error {
	if plan == nil {
		return fmt.Errorf("套餐不能为空")
	}
	if plan.MaxActivePerUser < 0 || plan.MaxActivePerUser > 2147483647 {
		return fmt.Errorf("同时生效数量上限必须在 0 到 2147483647 之间")
	}
	plan.Currency = strings.TrimSpace(plan.Currency)
	if plan.Currency == "" {
		plan.Currency = "USD"
	}
	plan.WalletOnlyGroupsMode = model.NormalizeWalletOnlyGroupsMode(plan.WalletOnlyGroupsMode)
	plan.WalletOnlyGroups = model.NormalizeWalletOnlyGroups(plan.WalletOnlyGroups)
	if plan.TotalAmount < 0 {
		plan.TotalAmount = -1
		plan.FiveHourLimit = 0
		plan.WeeklyLimit = 0
		plan.MonthlyLimit = 0
	} else {
		if plan.FiveHourLimit < 0 || plan.FiveHourLimit > int64(common.MaxQuota) {
			return fmt.Errorf("5小时限额必须介于 0 和 %d 之间", common.MaxQuota)
		}
		if plan.WeeklyLimit < 0 || plan.WeeklyLimit > int64(common.MaxQuota) {
			return fmt.Errorf("周限额必须介于 0 和 %d 之间", common.MaxQuota)
		}
		if plan.MonthlyLimit < 0 || plan.MonthlyLimit > int64(common.MaxQuota) {
			return fmt.Errorf("月限额必须介于 0 和 %d 之间", common.MaxQuota)
		}
	}
	_, rateLimits, err := model.NormalizeSubscriptionRateLimitGroups(plan.RateLimitGroups)
	if err != nil {
		return err
	}
	groups := ratio_setting.GetGroupRatioCopy()
	filteredRateLimits := make([]model.SubscriptionRateLimitGroup, 0, len(rateLimits))
	for _, rateLimit := range rateLimits {
		if _, ok := groups[rateLimit.Group]; ok {
			filteredRateLimits = append(filteredRateLimits, rateLimit)
		}
	}
	encoded, encodeErr := common.Marshal(filteredRateLimits)
	if encodeErr != nil {
		return encodeErr
	}
	plan.RateLimitGroups = string(encoded)
	walletGroups := make([]string, 0)
	seenWallet := make(map[string]struct{})
	for _, group := range strings.Split(plan.WalletOnlyGroups, ",") {
		group = strings.TrimSpace(group)
		if group == "" {
			continue
		}
		if _, ok := groups[group]; !ok {
			continue
		}
		if _, exists := seenWallet[group]; exists {
			continue
		}
		seenWallet[group] = struct{}{}
		walletGroups = append(walletGroups, group)
	}
	plan.WalletOnlyGroups = strings.Join(walletGroups, ",")
	usageTiers, _, err := model.NormalizeSubscriptionUsageTiers(plan.UsageTiers)
	if err != nil {
		return err
	}
	plan.UsageTiers = usageTiers
	if !plan.SubscriptionUsageTiersFitQuota() {
		return common.NewMessage("Usage tier quota exceeds the maximum allowed quota")
	}
	upgradePlanIds := model.ParseSubscriptionPlanIds(plan.UpgradePlanIds, plan.Id)
	existingPlanIds := make([]int, 0, len(upgradePlanIds))
	if len(upgradePlanIds) > 0 {
		if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id IN ?", upgradePlanIds).Pluck("id", &existingPlanIds).Error; err != nil {
			return err
		}
	}
	upgradeIds := make([]string, 0, len(upgradePlanIds))
	for _, id := range upgradePlanIds {
		if slices.Contains(existingPlanIds, id) {
			upgradeIds = append(upgradeIds, strconv.Itoa(id))
		}
	}
	plan.UpgradePlanIds = strings.Join(upgradeIds, ",")
	if plan.AllowBalancePay != nil && !*plan.AllowBalancePay {
		plan.AllowAutoRenew = false
	}
	return nil
}

// ---- User APIs ----

func GetSubscriptionPlans(c *gin.Context) {
	if !operation_setting.IsPaymentComplianceConfirmed() {
		common.ApiSuccess(c, []SubscriptionPlanDTO{})
		return
	}

	var plans []model.SubscriptionPlan
	if err := model.DB.Where("enabled = ?", true).Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		p.NormalizeDefaults()
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
}

func GetSubscriptionSelf(c *gin.Context) {
	userId := c.GetInt("id")
	settingMap, _ := model.GetUserSetting(userId, false)
	pref := common.NormalizeBillingPreference(settingMap.BillingPreference)

	// Get all subscriptions (including expired)
	allSubscriptions, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		allSubscriptions = []model.SubscriptionSummary{}
	}

	// Get active subscriptions for backward compatibility
	activeSubscriptions, err := model.GetAllActiveUserSubscriptions(userId)
	if err != nil {
		activeSubscriptions = []model.SubscriptionSummary{}
	}

	common.ApiSuccess(c, gin.H{
		"billing_preference": pref,
		"subscriptions":      activeSubscriptions, // all active subscriptions
		"all_subscriptions":  allSubscriptions,    // all subscriptions including expired
	})
}

func UpdateSubscriptionPreference(c *gin.Context) {
	userId := c.GetInt("id")
	var req BillingPreferenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	pref := common.NormalizeBillingPreference(req.BillingPreference)

	user, err := model.GetUserById(userId, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	current := user.GetSetting()
	current.BillingPreference = pref
	if err := model.UpdateUserSetting(user.Id, current); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{"billing_preference": pref})
}

func SubscriptionRequestBalancePay(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	userId := c.GetInt("id")
	var req SubscriptionBalancePayRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}

	if err := model.PurchaseSubscriptionWithBalance(userId, req.PlanId, req.Multiplier, req.AutoRenew); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func UpdateSubscriptionRenewal(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	subscriptionId, _ := strconv.Atoi(c.Param("id"))
	var req SubscriptionRenewalRequest
	if err := c.ShouldBindJSON(&req); err != nil || subscriptionId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := model.UpdateUserSubscriptionRenewal(c.GetInt("id"), subscriptionId, req.AutoRenew, req.RenewMultiplier); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

func SubscriptionUpgradeQuote(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req SubscriptionUpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SubscriptionId <= 0 || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	quote, err := model.QuoteSubscriptionUpgrade(c.GetInt("id"), req.SubscriptionId, req.PlanId, req.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, quote)
}

func SubscriptionUpgrade(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}
	var req SubscriptionUpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.SubscriptionId <= 0 || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	quote, err := model.UpgradeSubscriptionWithBalance(c.GetInt("id"), req.SubscriptionId, req.PlanId, req.Multiplier)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, quote)
}

// ---- Admin APIs ----

func AdminListSubscriptionPlans(c *gin.Context) {
	var plans []model.SubscriptionPlan
	if err := model.DB.Order("sort_order desc, id desc").Find(&plans).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	result := make([]SubscriptionPlanDTO, 0, len(plans))
	for _, p := range plans {
		p.NormalizeDefaults()
		result = append(result, SubscriptionPlanDTO{
			Plan: p,
		})
	}
	common.ApiSuccess(c, result)
}

type AdminUpsertSubscriptionPlanRequest struct {
	Plan model.SubscriptionPlan `json:"plan"`
}

func AdminCreateSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	req.Plan.Id = 0
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorT(c, "Subscription plan title cannot be empty")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorT(c, "Price cannot be negative")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorT(c, "Price cannot exceed 9999")
		return
	}
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	if err := normalizeSubscriptionPlanInput(&req.Plan); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Plan.AllowBalancePay == nil {
		req.Plan.AllowBalancePay = common.GetPointer(true)
	}
	if req.Plan.AllowWalletOverflow == nil {
		req.Plan.AllowWalletOverflow = common.GetPointer(true)
	}
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorT(c, "Purchase limit cannot be negative")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorT(c, "Total quota cannot be negative")
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorT(c, "Upgrade group does not exist")
			return
		}
	}
	req.Plan.DowngradeGroup = strings.TrimSpace(req.Plan.DowngradeGroup)
	if req.Plan.DowngradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.DowngradeGroup]; !ok {
			common.ApiErrorT(c, "Downgrade group does not exist")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorT(c, "Custom reset cycle must be greater than 0 seconds")
		return
	}
	err := model.DB.Create(&req.Plan).Error
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(req.Plan.Id)
	common.ApiSuccess(c, req.Plan)
}

func AdminUpdateSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorT(c, "Invalid ID")
		return
	}
	var req AdminUpsertSubscriptionPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if strings.TrimSpace(req.Plan.Title) == "" {
		common.ApiErrorT(c, "Subscription plan title cannot be empty")
		return
	}
	if req.Plan.PriceAmount < 0 {
		common.ApiErrorT(c, "Price cannot be negative")
		return
	}
	if req.Plan.PriceAmount > 9999 {
		common.ApiErrorT(c, "Price cannot exceed 9999")
		return
	}
	req.Plan.Id = id
	if req.Plan.Currency == "" {
		req.Plan.Currency = "USD"
	}
	if err := normalizeSubscriptionPlanInput(&req.Plan); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.Plan.DurationUnit == "" {
		req.Plan.DurationUnit = model.SubscriptionDurationMonth
	}
	if req.Plan.DurationValue <= 0 && req.Plan.DurationUnit != model.SubscriptionDurationCustom {
		req.Plan.DurationValue = 1
	}
	if req.Plan.MaxPurchasePerUser < 0 {
		common.ApiErrorT(c, "Purchase limit cannot be negative")
		return
	}
	if req.Plan.TotalAmount < 0 {
		common.ApiErrorT(c, "Total quota cannot be negative")
		return
	}
	req.Plan.UpgradeGroup = strings.TrimSpace(req.Plan.UpgradeGroup)
	if req.Plan.UpgradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.UpgradeGroup]; !ok {
			common.ApiErrorT(c, "Upgrade group does not exist")
			return
		}
	}
	req.Plan.DowngradeGroup = strings.TrimSpace(req.Plan.DowngradeGroup)
	if req.Plan.DowngradeGroup != "" {
		if _, ok := ratio_setting.GetGroupRatioCopy()[req.Plan.DowngradeGroup]; !ok {
			common.ApiErrorT(c, "Downgrade group does not exist")
			return
		}
	}
	req.Plan.QuotaResetPeriod = model.NormalizeResetPeriod(req.Plan.QuotaResetPeriod)
	if req.Plan.QuotaResetPeriod == model.SubscriptionResetCustom && req.Plan.QuotaResetCustomSeconds <= 0 {
		common.ApiErrorT(c, "Custom reset cycle must be greater than 0 seconds")
		return
	}

	err := model.DB.Transaction(func(tx *gorm.DB) error {
		// update plan (allow zero values updates with map)
		updateMap := map[string]any{
			"title":                      req.Plan.Title,
			"subtitle":                   req.Plan.Subtitle,
			"price_amount":               req.Plan.PriceAmount,
			"currency":                   req.Plan.Currency,
			"duration_unit":              req.Plan.DurationUnit,
			"duration_value":             req.Plan.DurationValue,
			"custom_seconds":             req.Plan.CustomSeconds,
			"enabled":                    req.Plan.Enabled,
			"sort_order":                 req.Plan.SortOrder,
			"stripe_price_id":            req.Plan.StripePriceId,
			"creem_product_id":           req.Plan.CreemProductId,
			"waffo_pancake_product_id":   req.Plan.WaffoPancakeProductId,
			"max_purchase_per_user":      req.Plan.MaxPurchasePerUser,
			"max_active_per_user":        req.Plan.MaxActivePerUser,
			"total_amount":               req.Plan.TotalAmount,
			"five_hour_limit":            req.Plan.FiveHourLimit,
			"weekly_limit":               req.Plan.WeeklyLimit,
			"monthly_limit":              req.Plan.MonthlyLimit,
			"upgrade_group":              req.Plan.UpgradeGroup,
			"downgrade_group":            req.Plan.DowngradeGroup,
			"wallet_only_groups_enabled": req.Plan.WalletOnlyGroupsEnabled,
			"wallet_only_groups_mode":    req.Plan.WalletOnlyGroupsMode,
			"wallet_only_groups":         req.Plan.WalletOnlyGroups,
			"rate_limit_groups":          req.Plan.RateLimitGroups,
			"quota_reset_period":         req.Plan.QuotaResetPeriod,
			"quota_reset_custom_seconds": req.Plan.QuotaResetCustomSeconds,
			"usage_tiers":                req.Plan.UsageTiers,
			"allow_tier_switch":          req.Plan.AllowTierSwitch,
			"upgrade_plan_ids":           req.Plan.UpgradePlanIds,
			"allow_auto_renew":           req.Plan.AllowAutoRenew,
			"updated_at":                 common.GetTimestamp(),
		}
		if req.Plan.AllowBalancePay != nil {
			updateMap["allow_balance_pay"] = *req.Plan.AllowBalancePay
		}
		if req.Plan.AllowWalletOverflow != nil {
			updateMap["allow_wallet_overflow"] = *req.Plan.AllowWalletOverflow
		}
		if err := tx.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Updates(updateMap).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

type AdminUpdateSubscriptionPlanStatusRequest struct {
	Enabled *bool `json:"enabled"`
}

func AdminUpdateSubscriptionPlanStatus(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorT(c, "Invalid ID")
		return
	}
	var req AdminUpdateSubscriptionPlanStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.Enabled == nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if err := model.DB.Model(&model.SubscriptionPlan{}).Where("id = ?", id).Update("enabled", *req.Enabled).Error; err != nil {
		common.ApiError(c, err)
		return
	}
	model.InvalidateSubscriptionPlanCache(id)
	common.ApiSuccess(c, nil)
}

func AdminDeleteSubscriptionPlan(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		common.ApiErrorMsg(c, "无效的ID")
		return
	}
	if err := model.DeleteSubscriptionPlan(id); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

type AdminBindSubscriptionRequest struct {
	UserId     int `json:"user_id"`
	PlanId     int `json:"plan_id"`
	Multiplier int `json:"multiplier"`
}

func AdminBindSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	var req AdminBindSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.UserId <= 0 || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	msg, err := model.AdminBindSubscription(req.UserId, req.PlanId, req.Multiplier, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	subscriptionChangeSuccess(c, msg)
}

// subscriptionChangeSuccess replies to an admin subscription change. notice
// tells the admin how the user's group changes, when it does.
func subscriptionChangeSuccess(c *gin.Context, notice *common.Message) {
	if notice == nil {
		common.ApiSuccess(c, nil)
		return
	}
	data := notice.Fields()
	data["message"] = notice.Error()
	common.ApiSuccess(c, data)
}

// ---- Admin: user subscription management ----

func AdminListUserSubscriptions(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorT(c, "Invalid user ID")
		return
	}
	subs, err := model.GetAllUserSubscriptions(userId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, subs)
}

type AdminCreateUserSubscriptionRequest struct {
	PlanId     int `json:"plan_id"`
	Multiplier int `json:"multiplier"`
}

type AdminResetSubscriptionRequest struct {
	PlanId           int   `json:"plan_id"`
	AdvanceResetTime *bool `json:"advance_reset_time"`
	ResetTotal       *bool `json:"reset_total"`
	ResetFiveHour    *bool `json:"reset_five_hour"`
	ResetWeekly      *bool `json:"reset_weekly"`
}

func resolveAdvanceResetTime(value *bool) bool {
	if value == nil {
		return true
	}
	return *value
}

func resolveSubscriptionResetOptions(req AdminResetSubscriptionRequest) model.SubscriptionResetOptions {
	if req.ResetTotal == nil && req.ResetFiveHour == nil && req.ResetWeekly == nil {
		return model.SubscriptionResetOptions{
			ResetTotal:    true,
			ResetFiveHour: true,
			ResetWeekly:   true,
			ResetMonthly:  true,
		}
	}
	resetTotal := req.ResetTotal != nil && *req.ResetTotal
	return model.SubscriptionResetOptions{
		ResetTotal:    resetTotal,
		ResetFiveHour: req.ResetFiveHour != nil && *req.ResetFiveHour,
		ResetWeekly:   req.ResetWeekly != nil && *req.ResetWeekly,
		// Monthly usage belongs to the total billing-cycle reset in the
		// administrator UI, while legacy omitted options still reset it.
		ResetMonthly: resetTotal,
	}
}

func recordSubscriptionResetUserLogs(c *gin.Context, result *model.SubscriptionResetResult, adminInfo *model.AuditAdminInfo) {
	if result == nil || result.ResetCount == 0 {
		return
	}
	const action = "subscription.user_quota_reset"
	params := map[string]any{"plan_id": result.PlanId, "plan_title": result.PlanTitle}
	operation := model.AuditOperation{Action: action, Params: params}
	content := auditContentEN(action, params)
	for _, userId := range result.AffectedUserIds {
		model.RecordLogWithAdminInfo(userId, model.LogTypeManage, content, adminInfo, &operation, c)
	}
}

// AdminCreateUserSubscription creates a new user subscription from a plan (no payment).
func AdminCreateUserSubscription(c *gin.Context) {
	if !requirePaymentCompliance(c) {
		return
	}

	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorT(c, "Invalid user ID")
		return
	}
	var req AdminCreateUserSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil || req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	msg, err := model.AdminBindSubscription(userId, req.PlanId, req.Multiplier, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	subscriptionChangeSuccess(c, msg)
}

func AdminResetUserSubscriptionsByPlan(c *gin.Context) {
	userId, _ := strconv.Atoi(c.Param("id"))
	if userId <= 0 {
		common.ApiErrorT(c, "Invalid user ID")
		return
	}
	var req AdminResetSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	if req.PlanId <= 0 {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	advanceResetTime := resolveAdvanceResetTime(req.AdvanceResetTime)
	options := resolveSubscriptionResetOptions(req)
	result, err := model.AdminResetUserSubscriptionsByPlanWithOptions(userId, req.PlanId, advanceResetTime, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubscriptionResetUserLogs(c, result, auditOperatorInfo(c))
	recordManageAuditFor(c, userId, "subscription.user_plan_reset", map[string]interface{}{
		"target_user_id":     userId,
		"plan_id":            result.PlanId,
		"plan_title":         result.PlanTitle,
		"reset_count":        result.ResetCount,
		"user_count":         result.UserCount,
		"advance_reset_time": result.AdvanceResetTime,
		"reset_total":        options.ResetTotal,
		"reset_five_hour":    options.ResetFiveHour,
		"reset_weekly":       options.ResetWeekly,
	})
	common.ApiSuccess(c, result)
}

func AdminResetPlanSubscriptions(c *gin.Context) {
	planId, _ := strconv.Atoi(c.Param("id"))
	if planId <= 0 {
		common.ApiErrorT(c, "Invalid ID")
		return
	}
	var req AdminResetSubscriptionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiErrorT(c, "Invalid parameters")
		return
	}
	advanceResetTime := resolveAdvanceResetTime(req.AdvanceResetTime)
	options := resolveSubscriptionResetOptions(req)
	result, err := model.AdminResetPlanSubscriptionsWithOptions(planId, advanceResetTime, options)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	recordSubscriptionResetUserLogs(c, result, auditOperatorInfo(c))
	common.SysLog(common.LogText("admin reset subscription plan %d quota: reset_count=%d user_count=%d advance_reset_time=%t",
		result.PlanId, result.ResetCount, result.UserCount, result.AdvanceResetTime))
	recordManageAudit(c, "subscription.plan_reset", map[string]interface{}{
		"plan_id":            result.PlanId,
		"plan_title":         result.PlanTitle,
		"reset_count":        result.ResetCount,
		"user_count":         result.UserCount,
		"advance_reset_time": result.AdvanceResetTime,
		"reset_total":        options.ResetTotal,
		"reset_five_hour":    options.ResetFiveHour,
		"reset_weekly":       options.ResetWeekly,
	})
	common.ApiSuccess(c, result)
}

// AdminInvalidateUserSubscription cancels a user subscription immediately.
func AdminInvalidateUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorT(c, "Invalid subscription ID")
		return
	}
	msg, err := model.AdminInvalidateUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	subscriptionChangeSuccess(c, msg)
}

// AdminDeleteUserSubscription hard-deletes a user subscription.
func AdminDeleteUserSubscription(c *gin.Context) {
	subId, _ := strconv.Atoi(c.Param("id"))
	if subId <= 0 {
		common.ApiErrorT(c, "Invalid subscription ID")
		return
	}
	msg, err := model.AdminDeleteUserSubscription(subId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	subscriptionChangeSuccess(c, msg)
}
