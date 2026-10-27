package model

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	// MaxSubscriptionUsageMultiplier bounds the quota multiplier of a usage tier.
	MaxSubscriptionUsageMultiplier = 100
	MaxSubscriptionUsageTiers      = 10
)

const (
	SubscriptionOrderTypePurchase = "purchase"
	SubscriptionOrderTypeRenew    = "renew"
	SubscriptionOrderTypeUpgrade  = "upgrade"
)

const (
	SubscriptionStatusUpgraded = "upgraded"
	SubscriptionSourceUpgrade  = "upgrade"
)

// SubscriptionUsageTier is a purchasable usage multiplier of a plan. The plan
// itself is the 1x tier; a tier scales every quota limit by Multiplier and
// carries its own price and payment gateway product ids.
type SubscriptionUsageTier struct {
	Multiplier            int     `json:"multiplier"`
	PriceAmount           float64 `json:"price_amount"`
	StripePriceId         string  `json:"stripe_price_id,omitempty"`
	CreemProductId        string  `json:"creem_product_id,omitempty"`
	WaffoPancakeProductId string  `json:"waffo_pancake_product_id,omitempty"`
}

// NormalizeSubscriptionUsageTiers validates the JSON tiers of a plan and
// returns them sorted by multiplier.
func NormalizeSubscriptionUsageTiers(raw string) (string, []SubscriptionUsageTier, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "null" {
		return "[]", []SubscriptionUsageTier{}, nil
	}
	var tiers []SubscriptionUsageTier
	if err := common.UnmarshalJsonStr(raw, &tiers); err != nil {
		return "", nil, common.NewMessage("Usage tiers are invalid")
	}
	if len(tiers) > MaxSubscriptionUsageTiers {
		return "", nil, common.NewMessage("At most {{count}} usage tiers are allowed", map[string]any{"count": MaxSubscriptionUsageTiers})
	}
	seen := make(map[int]struct{}, len(tiers))
	for i := range tiers {
		tier := &tiers[i]
		if tier.Multiplier < 2 || tier.Multiplier > MaxSubscriptionUsageMultiplier {
			return "", nil, common.NewMessage("Usage multiplier must be between 2 and {{max}}", map[string]any{"max": MaxSubscriptionUsageMultiplier})
		}
		if _, ok := seen[tier.Multiplier]; ok {
			return "", nil, common.NewMessage("Usage multipliers must be unique")
		}
		seen[tier.Multiplier] = struct{}{}
		if tier.PriceAmount < 0 || tier.PriceAmount > 9999 || math.IsNaN(tier.PriceAmount) {
			return "", nil, common.NewMessage("Usage tier price must be between 0 and 9999")
		}
		tier.StripePriceId = strings.TrimSpace(tier.StripePriceId)
		tier.CreemProductId = strings.TrimSpace(tier.CreemProductId)
		tier.WaffoPancakeProductId = strings.TrimSpace(tier.WaffoPancakeProductId)
	}
	slices.SortFunc(tiers, func(a, b SubscriptionUsageTier) int { return a.Multiplier - b.Multiplier })
	encoded, err := common.Marshal(tiers)
	if err != nil {
		return "", nil, err
	}
	return string(encoded), tiers, nil
}

// UsageTierOptions returns every purchasable tier, starting with the 1x plan.
func (p *SubscriptionPlan) UsageTierOptions() []SubscriptionUsageTier {
	options := []SubscriptionUsageTier{{
		Multiplier:            1,
		PriceAmount:           p.PriceAmount,
		StripePriceId:         p.StripePriceId,
		CreemProductId:        p.CreemProductId,
		WaffoPancakeProductId: p.WaffoPancakeProductId,
	}}
	_, tiers, err := NormalizeSubscriptionUsageTiers(p.UsageTiers)
	if err != nil {
		return options
	}
	return append(options, tiers...)
}

// ForUsageTier returns a copy of the plan priced and sized for the usage tier
// multiplier (0 selects the 1x plan).
func (p *SubscriptionPlan) ForUsageTier(multiplier int) (*SubscriptionPlan, error) {
	multiplier = max(multiplier, 1)
	for _, tier := range p.UsageTierOptions() {
		if tier.Multiplier != multiplier {
			continue
		}
		view := p.withUsageMultiplier(multiplier)
		view.PriceAmount = tier.PriceAmount
		view.StripePriceId = tier.StripePriceId
		view.CreemProductId = tier.CreemProductId
		view.WaffoPancakeProductId = tier.WaffoPancakeProductId
		return view, nil
	}
	return nil, common.NewMessage("This usage tier is not available")
}

// withUsageMultiplier scales the quota limits of the plan without touching
// its price.
func (p *SubscriptionPlan) withUsageMultiplier(multiplier int) *SubscriptionPlan {
	view := *p
	view.UsageMultiplier = max(multiplier, 1)
	view.TotalAmount = scaleSubscriptionQuota(p.TotalAmount, view.UsageMultiplier)
	view.FiveHourLimit = scaleSubscriptionQuota(p.FiveHourLimit, view.UsageMultiplier)
	view.WeeklyLimit = scaleSubscriptionQuota(p.WeeklyLimit, view.UsageMultiplier)
	view.MonthlyLimit = scaleSubscriptionQuota(p.MonthlyLimit, view.UsageMultiplier)
	return &view
}

// scaleSubscriptionQuota multiplies a quota limit, keeping 0 (unlimited) and
// -1 (benefits only) as they are and saturating at the wallet quota bound.
func scaleSubscriptionQuota(value int64, multiplier int) int64 {
	if value <= 0 || multiplier <= 1 {
		return value
	}
	if value > common.MaxWalletQuota/int64(multiplier) {
		common.SysError(common.LogText("subscription quota %d saturated at usage multiplier %d", value, multiplier))
		return common.MaxWalletQuota
	}
	return value * int64(multiplier)
}

// SubscriptionUsageTiersFitQuota reports whether every tier keeps the scaled
// quota limits of the plan within the wallet quota bound.
func (p *SubscriptionPlan) SubscriptionUsageTiersFitQuota() bool {
	for _, tier := range p.UsageTierOptions() {
		for _, value := range []int64{p.TotalAmount, p.FiveHourLimit, p.WeeklyLimit, p.MonthlyLimit} {
			if value > 0 && value > common.MaxWalletQuota/int64(tier.Multiplier) {
				return false
			}
		}
	}
	return true
}

// ParseSubscriptionPlanIds parses comma separated plan ids, dropping invalid,
// duplicate and excluded ids.
func ParseSubscriptionPlanIds(raw string, excludeId int) []int {
	ids := make([]int, 0)
	for value := range strings.SplitSeq(raw, ",") {
		id, err := strconv.Atoi(strings.TrimSpace(value))
		if err != nil || id <= 0 || id == excludeId || slices.Contains(ids, id) {
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// creditPrice is the list price that values the unused part of the current
// billing period. Rows created before prices were recorded fall back to the
// plan price when they were paid for.
func (s *UserSubscription) creditPrice(plan *SubscriptionPlan) float64 {
	if s.PriceAmount != nil {
		return *s.PriceAmount
	}
	if s.Source != "order" && s.Source != PaymentMethodBalance {
		return 0
	}
	tierPlan, err := plan.ForUsageTier(s.Multiplier)
	if err != nil {
		return 0
	}
	return tierPlan.PriceAmount
}

// upgradeCreditQuota values the unused part of the current billing period in
// wallet quota. Plans whose total quota never resets also cap the credit by
// the unused share of that quota, so a drained subscription earns no credit.
func (s *UserSubscription) upgradeCreditQuota(plan *SubscriptionPlan, now int64) (int, error) {
	price := s.creditPrice(plan)
	period := s.EndTime - s.StartTime
	if price <= 0 || period <= 0 || s.EndTime <= now || common.QuotaPerUnit <= 0 {
		return 0, nil
	}
	remaining := min(s.EndTime-now, period)
	fraction := decimal.NewFromInt(remaining).Div(decimal.NewFromInt(period))
	if NormalizeResetPeriod(plan.QuotaResetPeriod) == SubscriptionResetNever && s.AmountTotal > 0 {
		unused := decimal.NewFromInt(max(s.AmountTotal-s.AmountUsed, 0)).Div(decimal.NewFromInt(s.AmountTotal))
		fraction = decimal.Min(fraction, unused)
	}
	credit := decimal.NewFromFloat(price).
		Mul(decimal.NewFromFloat(common.QuotaPerUnit)).
		Mul(fraction).
		Floor()
	return common.WalletQuotaFromDecimalStrict(credit)
}

// SubscriptionUpgradeQuote is the wallet charge of an upgrade: the full price
// of the target tier minus the credit for the unused current period.
type SubscriptionUpgradeQuote struct {
	SubscriptionId int `json:"subscription_id"`
	PlanId         int `json:"plan_id"`
	Multiplier     int `json:"multiplier"`
	PriceQuota     int `json:"price_quota"`
	CreditQuota    int `json:"credit_quota"`
	ChargeQuota    int `json:"charge_quota"`
}

type subscriptionUpgrade struct {
	current    UserSubscription
	sourcePlan *SubscriptionPlan
	target     *SubscriptionPlan
	samePlan   bool
	quote      SubscriptionUpgradeQuote
}

// prepareSubscriptionUpgradeTx locks the subscription and validates that it
// may move to the target plan and tier.
func prepareSubscriptionUpgradeTx(tx *gorm.DB, userId int, subscriptionId int, planId int, multiplier int) (*subscriptionUpgrade, error) {
	if userId <= 0 || subscriptionId <= 0 || planId <= 0 {
		return nil, common.NewMessage("Invalid parameters")
	}
	upgrade := &subscriptionUpgrade{}
	if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", subscriptionId, userId).First(&upgrade.current).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, common.NewMessage("Subscription not found")
		}
		return nil, err
	}
	now := getDBTimestamp(tx)
	current := &upgrade.current
	if current.Status != "active" || current.EndTime <= now {
		return nil, common.NewMessage("Only active subscriptions can be upgraded")
	}
	sourcePlan, err := getSubscriptionPlanByIdTx(tx, current.PlanId)
	if err != nil {
		return nil, err
	}
	upgrade.sourcePlan = sourcePlan
	targetPlan := sourcePlan
	upgrade.samePlan = planId == current.PlanId
	if !upgrade.samePlan {
		if !slices.Contains(ParseSubscriptionPlanIds(sourcePlan.UpgradePlanIds, sourcePlan.Id), planId) {
			return nil, common.NewMessage("This plan cannot be upgraded to the selected plan")
		}
		if targetPlan, err = getSubscriptionPlanByIdTx(tx, planId); err != nil {
			return nil, err
		}
	} else {
		if !sourcePlan.AllowTierSwitch {
			return nil, common.NewMessage("This plan does not allow switching usage tiers")
		}
		if max(multiplier, 1) <= max(current.Multiplier, 1) {
			return nil, common.NewMessage("Only a higher usage tier can be switched to now; lower tiers apply at the next renewal")
		}
	}
	if !targetPlan.Enabled {
		return nil, common.NewMessage("Subscription plan is not enabled")
	}
	if !*targetPlan.AllowBalancePay {
		return nil, common.NewMessage("Upgrades are paid with balance, which the target plan does not allow")
	}
	if upgrade.target, err = targetPlan.ForUsageTier(multiplier); err != nil {
		return nil, err
	}
	priceQuota, err := calcSubscriptionBalanceQuota(upgrade.target.PriceAmount)
	if err != nil {
		return nil, err
	}
	creditQuota, err := current.upgradeCreditQuota(sourcePlan, now)
	if err != nil {
		return nil, err
	}
	upgrade.quote = SubscriptionUpgradeQuote{
		SubscriptionId: current.Id,
		PlanId:         upgrade.target.Id,
		Multiplier:     upgrade.target.UsageMultiplier,
		PriceQuota:     priceQuota,
		CreditQuota:    creditQuota,
		ChargeQuota:    max(priceQuota-creditQuota, 0),
	}
	return upgrade, nil
}

// QuoteSubscriptionUpgrade prices an upgrade without changing anything.
func QuoteSubscriptionUpgrade(userId int, subscriptionId int, planId int, multiplier int) (*SubscriptionUpgradeQuote, error) {
	var quote SubscriptionUpgradeQuote
	err := DB.Transaction(func(tx *gorm.DB) error {
		upgrade, err := prepareSubscriptionUpgradeTx(tx, userId, subscriptionId, planId, multiplier)
		if err != nil {
			return err
		}
		quote = upgrade.quote
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &quote, nil
}

// UpgradeSubscriptionWithBalance replaces an active subscription with a new
// billing period of the target plan and tier, charging the wallet the quoted
// difference. The replaced subscription ends now with status upgraded.
func UpgradeSubscriptionWithBalance(userId int, subscriptionId int, planId int, multiplier int) (*SubscriptionUpgradeQuote, error) {
	var upgrade *subscriptionUpgrade
	groupChanged := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		var err error
		upgrade, err = prepareSubscriptionUpgradeTx(tx, userId, subscriptionId, planId, multiplier)
		if err != nil {
			return err
		}
		charge := upgrade.quote.ChargeQuota
		if user.Quota < charge {
			return common.NewMessage("Insufficient balance")
		}
		if charge > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("quota", gorm.Expr("quota - ?", charge)).Error; err != nil {
				return err
			}
		}
		now := getDBTimestamp(tx)
		current := &upgrade.current
		// Updates writes the map back into current, so read the renewal
		// choice before ending the subscription.
		carryAutoRenew := current.AutoRenew && upgrade.target.AllowAutoRenew
		if err := tx.Model(current).Updates(map[string]any{
			"status":     SubscriptionStatusUpgraded,
			"end_time":   now,
			"auto_renew": false,
		}).Error; err != nil {
			return err
		}
		reverted, err := downgradeUserGroupForSubscriptionTx(tx, current, now)
		if err != nil {
			return err
		}
		next, err := createUserSubscriptionTx(tx, userId, upgrade.target, SubscriptionSourceUpgrade, !upgrade.samePlan)
		if err != nil {
			return err
		}
		groupChanged = reverted != "" || next.PrevUserGroup != ""
		if carryAutoRenew {
			if err := tx.Model(next).Update("auto_renew", true).Error; err != nil {
				return err
			}
		}
		money := 0.0
		if common.QuotaPerUnit > 0 {
			money = float64(charge) / common.QuotaPerUnit
		}
		payload := fmt.Sprintf("charged_quota=%d;credit_quota=%d;from_subscription=%d", charge, upgrade.quote.CreditQuota, current.Id)
		return createBalanceSubscriptionOrderTx(tx, userId, upgrade.target, SubscriptionOrderTypeUpgrade, money, payload)
	})
	if err != nil {
		return nil, err
	}
	if upgrade.quote.ChargeQuota > 0 {
		if err := cacheDecrUserQuota(userId, int64(upgrade.quote.ChargeQuota)); err != nil {
			common.SysLog(common.LogText("failed to decrease user quota cache after subscription upgrade: %s", err.Error()))
		}
	}
	InvalidateUserSubscriptionRateLimitCache(userId)
	if groupChanged {
		refreshSubscriptionUserGroupCache(userId, common.LogText("subscription upgrade"))
	}
	RecordLog(userId, LogTypeTopup, common.NewMessage("Subscription upgraded with balance from {{from}} to {{plan}} ({{multiplier}}x), credit: {{credit}}, quota deducted: {{quota}}", map[string]any{
		"from":       upgrade.sourcePlan.Title,
		"plan":       upgrade.target.Title,
		"multiplier": upgrade.target.UsageMultiplier,
		"credit":     upgrade.quote.CreditQuota,
		"quota":      upgrade.quote.ChargeQuota,
	}))
	return &upgrade.quote, nil
}

// UpdateUserSubscriptionRenewal turns balance auto-renewal on or off and sets
// the usage tier of the next renewal (0 keeps the current tier).
func UpdateUserSubscriptionRenewal(userId int, subscriptionId int, autoRenew bool, renewMultiplier int) error {
	if userId <= 0 || subscriptionId <= 0 || renewMultiplier < 0 {
		return common.NewMessage("Invalid parameters")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var sub UserSubscription
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", subscriptionId, userId).First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.NewMessage("Subscription not found")
			}
			return err
		}
		if sub.Status != "active" || sub.EndTime <= getDBTimestamp(tx) {
			return common.NewMessage("Only active subscriptions can be renewed")
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		if autoRenew && (!plan.Enabled || !plan.AllowAutoRenew) {
			return common.NewMessage("This plan does not support auto-renewal")
		}
		if renewMultiplier == max(sub.Multiplier, 1) {
			renewMultiplier = 0
		}
		if renewMultiplier > 0 {
			if !plan.AllowTierSwitch {
				return common.NewMessage("This plan does not allow switching usage tiers")
			}
			if _, err := plan.ForUsageTier(renewMultiplier); err != nil {
				return err
			}
		}
		return tx.Model(&sub).Updates(map[string]any{
			"auto_renew":       autoRenew,
			"renew_multiplier": renewMultiplier,
		}).Error
	})
}

type subscriptionRenewal struct {
	userId       int
	planTitle    string
	multiplier   int
	chargedQuota int
	stopReason   string
}

// RenewDueSubscriptions renews auto-renewing subscriptions that end before
// now+lead from wallet balance. The new period starts when the current one
// ends, so renewal leaves no gap. A renewal that cannot be paid turns
// auto-renewal off and lets the subscription expire.
func RenewDueSubscriptions(limit int, lead time.Duration) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	dueBefore := GetDBTimestamp() + int64(lead/time.Second)
	renewed := 0
	lastId := 0
	for {
		var subs []UserSubscription
		if err := DB.Select("id", "user_id").
			Where("id > ? AND status = ? AND auto_renew = ? AND end_time > 0 AND end_time <= ?", lastId, "active", true, dueBefore).
			Order("id asc").
			Limit(limit).
			Find(&subs).Error; err != nil {
			return renewed, err
		}
		for _, sub := range subs {
			lastId = sub.Id
			renewal, err := renewUserSubscription(sub.UserId, sub.Id, dueBefore)
			if err != nil {
				common.SysError(common.LogText("failed to renew subscription %d: %v", sub.Id, err))
				continue
			}
			if renewal == nil {
				continue
			}
			finalizeSubscriptionRenewal(renewal)
			if renewal.stopReason == "" {
				renewed++
			}
		}
		if len(subs) < limit {
			return renewed, nil
		}
	}
}

func renewUserSubscription(userId int, subscriptionId int, dueBefore int64) (*subscriptionRenewal, error) {
	var renewal *subscriptionRenewal
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Where("id = ?", userId).First(&user).Error; err != nil {
			return err
		}
		var sub UserSubscription
		err := lockForUpdate(tx).
			Where("id = ? AND user_id = ? AND status = ? AND auto_renew = ? AND end_time > 0 AND end_time <= ?", subscriptionId, userId, "active", true, dueBefore).
			First(&sub).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		renewal = &subscriptionRenewal{userId: userId, planTitle: plan.Title}
		stopRenewal := func(reason string) error {
			renewal.stopReason = reason
			return tx.Model(&sub).Updates(map[string]any{"auto_renew": false, "renew_multiplier": 0}).Error
		}
		if !plan.Enabled || !plan.AllowAutoRenew {
			return stopRenewal("unsupported")
		}
		multiplier := sub.Multiplier
		if sub.RenewMultiplier > 0 {
			multiplier = sub.RenewMultiplier
		}
		tierPlan, err := plan.ForUsageTier(multiplier)
		if err != nil {
			return stopRenewal("unsupported")
		}
		requiredQuota, err := calcSubscriptionBalanceQuota(tierPlan.PriceAmount)
		if err != nil {
			return err
		}
		if user.Quota < requiredQuota {
			return stopRenewal("insufficient_balance")
		}
		if requiredQuota > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userId).
				Update("quota", gorm.Expr("quota - ?", requiredQuota)).Error; err != nil {
				return err
			}
		}
		start := time.Unix(max(sub.EndTime, getDBTimestamp(tx)), 0)
		if err := sub.startBillingPeriod(tierPlan, start, tierPlan.PriceAmount); err != nil {
			return err
		}
		sub.RenewMultiplier = 0
		if err := tx.Save(&sub).Error; err != nil {
			return err
		}
		renewal.multiplier = sub.Multiplier
		renewal.chargedQuota = requiredQuota
		payload := fmt.Sprintf("charged_quota=%d;subscription=%d", requiredQuota, sub.Id)
		return createBalanceSubscriptionOrderTx(tx, userId, tierPlan, SubscriptionOrderTypeRenew, tierPlan.PriceAmount, payload)
	})
	if err != nil {
		return nil, err
	}
	return renewal, nil
}

func finalizeSubscriptionRenewal(renewal *subscriptionRenewal) {
	switch renewal.stopReason {
	case "insufficient_balance":
		RecordLog(renewal.userId, LogTypeSystem, common.NewMessage("Auto-renewal of {{plan}} failed: insufficient balance. Auto-renewal has been turned off", map[string]any{"plan": renewal.planTitle}))
		return
	case "unsupported":
		RecordLog(renewal.userId, LogTypeSystem, common.NewMessage("Auto-renewal of {{plan}} stopped: the plan no longer supports it", map[string]any{"plan": renewal.planTitle}))
		return
	}
	if renewal.chargedQuota > 0 {
		if err := cacheDecrUserQuota(renewal.userId, int64(renewal.chargedQuota)); err != nil {
			common.SysLog(common.LogText("failed to decrease user quota cache after subscription renewal: %s", err.Error()))
		}
	}
	InvalidateUserSubscriptionRateLimitCache(renewal.userId)
	RecordLog(renewal.userId, LogTypeTopup, common.NewMessage("Subscription renewed automatically, plan: {{plan}} ({{multiplier}}x), quota deducted: {{quota}}", map[string]any{
		"plan":       renewal.planTitle,
		"multiplier": renewal.multiplier,
		"quota":      renewal.chargedQuota,
	}))
}

// GetSubscriptionPlanTier loads a plan priced and sized for the usage tier
// multiplier, for checkout flows that charge the tier price.
func GetSubscriptionPlanTier(planId int, multiplier int) (*SubscriptionPlan, error) {
	plan, err := GetSubscriptionPlanById(planId)
	if err != nil {
		return nil, err
	}
	return plan.ForUsageTier(multiplier)
}
