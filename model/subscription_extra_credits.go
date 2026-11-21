package model

import (
	"context"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/go-redis/redis/v8"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// SubscriptionExtraCreditPurchase is an immutable receipt for a wallet exchange.
type SubscriptionExtraCreditPurchase struct {
	Id             int     `json:"id"`
	UserId         int     `json:"user_id" gorm:"index"`
	SubscriptionId int     `json:"subscription_id" gorm:"index"`
	RequestId      string  `json:"request_id" gorm:"size:64;uniqueIndex"`
	WalletQuota    int64   `json:"wallet_quota" gorm:"type:bigint;not null"`
	Credits        int64   `json:"credits" gorm:"type:bigint;not null"`
	Rate           float64 `json:"rate" gorm:"type:decimal(18,6);not null"`
	CreatedAt      int64   `json:"created_at" gorm:"bigint"`
	LogPending     bool    `json:"-" gorm:"not null;default:false;index"`
}

type subscriptionQuotaAllocation struct {
	SubscriptionId  int   `json:"subscription_id"`
	Regular         int64 `json:"regular"`
	Extra           int64 `json:"extra"`
	PeriodReset     int64 `json:"period_reset"`
	FiveHourReset   int64 `json:"five_hour_reset"`
	WeeklyReset     int64 `json:"weekly_reset"`
	RegularEligible bool  `json:"regular_eligible,omitempty"`
	MonthlyReset    int64 `json:"monthly_reset"`
	PeriodVersion   int64 `json:"period_version,omitempty"`
	FiveHourVersion int64 `json:"five_hour_version,omitempty"`
	WeeklyVersion   int64 `json:"weekly_version,omitempty"`
	MonthlyVersion  int64 `json:"monthly_version,omitempty"`
}

// A pending hold survives a worker crash, keyed by the SQL idempotency key.
// Retrying under the SQL user lock reuses the hold; SQL rollback releases it.
func reserveExtraCreditWallet(userID int, amount int64, requestID string) (cacheQuotaResult, error) {
	const script = `
if tonumber(redis.call('HGET',KEYS[1],'Id') or '0') ~= tonumber(ARGV[2])
 or tonumber(redis.call('HGET',KEYS[1],'CacheSchema') or '0') ~= tonumber(ARGV[3]) then return -1 end
if redis.call('HGET',KEYS[1],ARGV[4]) then
 if redis.call('HGET',KEYS[1],ARGV[4]) ~= ARGV[1] then return -1 end
 return 1
end
local quota=tonumber(redis.call('HGET',KEYS[1],'Quota') or '-1')
if quota < tonumber(ARGV[1]) then return 0 end
redis.call('ZADD',KEYS[2],ARGV[5],ARGV[6])
redis.call('HINCRBY',KEYS[1],'Quota',-tonumber(ARGV[1]))
redis.call('HSET',KEYS[1],ARGV[4],ARGV[1])
return 1`
	result, err := common.RDB.Eval(context.Background(), script, []string{getUserCacheKey(userID), "extra-credit-wallet-holds"}, amount, userID, userCacheSchemaVersion, "extra-credit-hold:"+requestID, common.GetTimestamp()+120, fmt.Sprintf("%d:%s", userID, requestID)).Int()
	return quotaResultFromLua(result, err)
}

func finishExtraCreditWallet(userID int, amount int64, requestID string, refund bool) error {
	const script = `
local held=redis.call('HGET',KEYS[1],ARGV[3])
if held and ARGV[1] ~= '0' and held ~= ARGV[1] then return 0 end
if held and ARGV[2] == '1' then redis.call('DEL',KEYS[1]) end
redis.call('HDEL',KEYS[1],ARGV[3])
redis.call('ZREM',KEYS[2],ARGV[4])
return 1`
	flag := 0
	if refund {
		flag = 1
	}
	return common.RDB.Eval(context.Background(), script, []string{getUserCacheKey(userID), "extra-credit-wallet-holds"}, amount, flag, "extra-credit-hold:"+requestID, fmt.Sprintf("%d:%s", userID, requestID)).Err()
}

// Reconcile under the same SQL lock as purchase, including commit-ack failures.
// A committed receipt clears its hold without refund; a rolled-back one releases it.
func reconcileExtraCreditWallet(userID int, requestID string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.RDB.ZRem(context.Background(), "extra-credit-wallet-holds", fmt.Sprintf("%d:%s", userID, requestID)).Err()
			}
			return err
		}
		var receipt SubscriptionExtraCreditPurchase
		err := tx.Where("request_id = ? AND user_id = ?", requestID, userID).Take(&receipt).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		return finishExtraCreditWallet(userID, 0, requestID, errors.Is(err, gorm.ErrRecordNotFound) || receipt.RequestId != requestID)
	})
}

// Recover abandoned holds even when the browser never retries the purchase.
func ReconcileExtraCreditWalletHolds() error {
	if !common.RedisEnabled {
		return nil
	}
	members, err := common.RDB.ZRangeByScore(context.Background(), "extra-credit-wallet-holds", &redis.ZRangeBy{Min: "-inf", Max: strconv.FormatInt(common.GetTimestamp(), 10), Count: 300}).Result()
	if err != nil {
		return err
	}
	var failures []error
	for _, member := range members {
		userText, requestID, ok := strings.Cut(member, ":")
		userID, err := strconv.Atoi(userText)
		if !ok || err != nil || userID <= 0 || strconv.Itoa(userID) != userText || !validExtraCreditRequestID(requestID) {
			failures = append(failures, errors.New("invalid extra credit wallet hold member"), common.RDB.ZRem(context.Background(), "extra-credit-wallet-holds", member).Err())
			continue
		}
		if err := reconcileExtraCreditWallet(userID, requestID); err != nil {
			failures = append(failures, fmt.Errorf("reconcile wallet hold for user %d: %w", userID, err))
		}
	}
	return errors.Join(failures...)
}

// Keep balances and refundable in-flight allocations tied to their funding row.
func checkSubscriptionExtraCreditsTx(tx *gorm.DB, sub *UserSubscription) error {
	if sub.ExtraCreditsTotal > sub.ExtraCreditsUsed || sub.FundingReserved > 0 {
		return common.NewMessage("Subscription still has unspent extra credits")
	}
	return nil
}

func extraCreditStatus(status string) bool {
	return status == "active" || status == "expired" || status == SubscriptionStatusUpgraded
}

func (s *UserSubscription) extraCreditGroups() ([]string, error) {
	if s.ExtraCreditsGroups == "" {
		return nil, nil
	}
	var groups []string
	err := common.UnmarshalJsonStr(s.ExtraCreditsGroups, &groups)
	return groups, err
}

func subscriptionExtraCreditPurchaseGroupsTx(tx *gorm.DB, user *User, sub *UserSubscription, plan *SubscriptionPlan) ([]string, error) {
	groups := []string{}
	if plan.SubscriptionGroup != "" {
		groups = append(groups, plan.SubscriptionGroup)
	} else {
		scoped, err := subscriptionGroupNamesTx(tx)
		if err != nil {
			return nil, err
		}
		for group := range OrdinaryUserUsableGroups(user.Group) {
			if group == "" || group == "auto" || !ratio_setting.ContainsGroupRatio(group) {
				continue
			}
			if slices.Contains(scoped, group) {
				if slices.Contains(sub.legacyCycleGrant().Quota, group) {
					groups = append(groups, group)
				}
			} else if !plan.IsWalletOnlyForGroup(group, false) {
				groups = append(groups, group)
			}
		}
	}
	slices.Sort(groups)
	return slices.Compact(groups), nil
}

func CanPurchaseSubscriptionExtraCredits(userID int, sub *UserSubscription) bool {
	allowed, _ := SubscriptionExtraCreditPurchaseEligibility(userID, sub)
	return allowed
}

func SubscriptionExtraCreditPurchaseEligibility(userID int, sub *UserSubscription) (bool, error) {
	if sub == nil || sub.UserId != userID || sub.Status != "active" || sub.EndTime <= GetDBTimestamp() {
		return false, nil
	}
	plan, err := GetSubscriptionPlanById(sub.PlanId)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if !plan.Enabled || !plan.AllowExtraCredits || plan.ExtraCreditsRate <= 0 {
		return false, nil
	}
	var user User
	if err := DB.Select("id", "group").First(&user, userID).Error; err != nil {
		return false, err
	}
	groups, err := subscriptionExtraCreditPurchaseGroupsTx(DB, &user, sub, plan)
	if err != nil {
		return false, err
	}
	if len(groups) == 0 {
		return false, nil
	}
	prior, err := sub.extraCreditGroups()
	if err != nil {
		return false, err
	}
	slices.Sort(prior)
	scoped, err := subscriptionGroupNamesTx(DB)
	if err != nil {
		return false, err
	}
	var currentScoped, priorScoped []string
	for _, g := range groups {
		if slices.Contains(scoped, g) {
			currentScoped = append(currentScoped, g)
		}
	}
	if sub.ExtraCreditsScopedGroups != "" {
		if err := common.UnmarshalJsonStr(sub.ExtraCreditsScopedGroups, &priorScoped); err != nil {
			return false, err
		}
	}
	slices.Sort(priorScoped)
	unchanged := slices.Equal(prior, groups) && slices.Equal(priorScoped, currentScoped)
	if sub.Status == "expired" || sub.EndTime <= GetDBTimestamp() {
		return len(prior) > 0 && unchanged, nil
	}
	return unchanged || sub.ExtraCreditsTotal == 0 || checkSubscriptionExtraCreditsTx(DB, sub) == nil, nil
}

func validExtraCreditRequestID(id string) bool {
	return len(id) >= 16 && len(id) <= 64 && strings.TrimSpace(id) == id
}

func PurchaseSubscriptionExtraCredits(userID, subscriptionID int, walletQuota int64, requestID string, expectedRate float64) (*SubscriptionExtraCreditPurchase, error) {
	if userID <= 0 || subscriptionID <= 0 || walletQuota <= 0 || walletQuota > common.MaxWalletQuota || !validExtraCreditRequestID(requestID) {
		return nil, common.NewMessage("Invalid extra credits amount")
	}
	var receipt SubscriptionExtraCreditPurchase
	if common.RedisEnabled {
		if _, err := GetUserCache(userID); err != nil {
			return nil, err
		}
	}
	err := subscriptionPlanTransaction(DB, func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		err := tx.Where("request_id = ?", requestID).Take(&receipt).Error

		if err == nil {
			if receipt.UserId != userID || receipt.SubscriptionId != subscriptionID || receipt.WalletQuota != walletQuota || receipt.RequestId != requestID || receipt.Rate != expectedRate {
				return common.NewMessage("Extra credits purchase request conflicts with an earlier purchase")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		// A prior worker may have died after reserving Redis but before SQL
		// committed. Its user-row lock is now ours, so release that orphan
		// before validating and reserving this retry again.
		if common.RedisEnabled {
			if err := finishExtraCreditWallet(userID, 0, requestID, true); err != nil {
				return err
			}
			if err := populateUserCache(user); err != nil {
				return err
			}
		}
		var sub UserSubscription
		if err := tx.Where("id = ? AND user_id = ?", subscriptionID, userID).First(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return common.NewMessage("Subscription not found")
			}
			return err
		}
		plan, err := syncSubscriptionPlanEntitlementTx(tx, sub.PlanId)
		if err != nil {
			return err
		}
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", subscriptionID, userID).First(&sub).Error; err != nil {
			return err
		}
		if sub.PlanId != plan.Id || sub.Status != "active" || sub.EndTime <= getDBTimestamp(tx) {
			return common.NewMessage("Cannot purchase extra credits for this subscription")
		}
		if !plan.Enabled || !plan.AllowExtraCredits {
			return common.NewMessage("Extra credits are not enabled for this plan")
		}
		rate := plan.ExtraCreditsRate
		if math.IsNaN(rate) || math.IsInf(rate, 0) || rate < 0.000001 || rate > 1000000 {
			return common.NewMessage("Invalid extra credits exchange rate")
		}
		if expectedRate != rate {
			return common.NewMessage("Extra credits exchange rate changed; reload and try again")
		}
		if !decimal.NewFromFloat(rate).Equal(decimal.NewFromFloat(rate).Round(6)) {
			return common.NewMessage("Invalid extra credits exchange rate")
		}
		credits, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(walletQuota).Mul(decimal.NewFromFloat(rate)).Floor())
		if err != nil || credits <= 0 || sub.ExtraCreditsTotal < 0 || sub.ExtraCreditsUsed < 0 || sub.ExtraCreditsUsed > sub.ExtraCreditsTotal || int64(credits) > common.MaxWalletQuota-sub.ExtraCreditsTotal {
			return common.NewMessage("Invalid extra credits amount")
		}
		groups, err := subscriptionExtraCreditPurchaseGroupsTx(tx, &user, &sub, plan)
		if err != nil {
			return err
		}
		if len(groups) == 0 {
			return common.NewMessage("Cannot purchase extra credits for this subscription")
		}
		slices.Sort(groups)
		groups = slices.Compact(groups)
		prior, err := sub.extraCreditGroups()
		if err != nil {
			return err
		}
		slices.Sort(prior)
		if !slices.Equal(prior, groups) {
			if sub.Status == "expired" || sub.EndTime <= getDBTimestamp(tx) {
				return common.NewMessage("Cannot purchase extra credits for this subscription")
			}
			if sub.ExtraCreditsTotal > 0 {
				if err := checkSubscriptionExtraCreditsTx(tx, &sub); err != nil {
					return err
				}
			}
		}
		scopedNames, err := subscriptionGroupNamesTx(tx)
		if err != nil {
			return err
		}
		var scopedPurchased []string
		for _, g := range groups {
			if slices.Contains(scopedNames, g) {
				scopedPurchased = append(scopedPurchased, g)
			}
		}
		var priorScoped []string
		if sub.ExtraCreditsScopedGroups != "" {
			if err := common.UnmarshalJsonStr(sub.ExtraCreditsScopedGroups, &priorScoped); err != nil {
				return err
			}
		}
		slices.Sort(priorScoped)
		if !slices.Equal(priorScoped, scopedPurchased) {
			if sub.Status == "expired" || sub.EndTime <= getDBTimestamp(tx) {
				return common.NewMessage("Cannot purchase extra credits for this subscription")
			}
			if sub.ExtraCreditsTotal > 0 {
				if err := checkSubscriptionExtraCreditsTx(tx, &sub); err != nil {
					return err
				}
			}
		}
		scopedRaw, err := common.Marshal(scopedPurchased)
		if err != nil {
			return err
		}
		raw, err := common.Marshal(groups)
		if err != nil {
			return err
		}
		// Reserve the cache balance as well as SQL so concurrent wallet requests
		// and a retried exchange observe the same spendable funds.
		if common.RedisEnabled {
			result, err := reserveExtraCreditWallet(userID, walletQuota, requestID)
			if err != nil {
				return err
			}
			if result == cacheQuotaMiss || result == cacheQuotaInsufficient && int64(user.Quota) >= walletQuota {
				if result == cacheQuotaInsufficient {
					if err := invalidateUserCache(userID); err != nil {
						return err
					}
				}
				if err := populateUserCache(user); err != nil {
					return err
				}
				result, err = reserveExtraCreditWallet(userID, walletQuota, requestID)
				if err != nil {
					return err
				}
			}
			if result == cacheQuotaMiss {
				return errors.New("wallet quota cache unavailable")
			}
			if result != cacheQuotaOK {
				return common.NewMessage("Insufficient balance")
			}
		}
		result := tx.Model(&User{}).Where("id = ? AND quota >= ?", userID, walletQuota).UpdateColumn("quota", gorm.Expr("quota - ?", walletQuota))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			if common.RedisEnabled {
				if err := invalidateUserCache(userID); err != nil {
					return err
				}
			}
			return common.NewMessage("Insufficient balance")
		}
		if err := tx.Model(&sub).Updates(map[string]any{"extra_credits_total": sub.ExtraCreditsTotal + int64(credits), "extra_credits_groups": string(raw), "extra_credits_scoped_groups": string(scopedRaw)}).Error; err != nil {
			return err
		}
		receipt = SubscriptionExtraCreditPurchase{UserId: userID, SubscriptionId: subscriptionID, RequestId: requestID, WalletQuota: walletQuota, Credits: int64(credits), Rate: rate, CreatedAt: common.GetTimestamp(), LogPending: true}
		return tx.Create(&receipt).Error
	})
	if err != nil {
		if common.RedisEnabled {
			if refundErr := reconcileExtraCreditWallet(userID, requestID); refundErr != nil {
				common.SysError(fmt.Sprintf("extra credits wallet compensation failed for user %d: %s", userID, refundErr))
			}
		}
		return nil, err
	}
	if common.RedisEnabled {
		if finishErr := reconcileExtraCreditWallet(userID, requestID); finishErr != nil {
			common.SysError(fmt.Sprintf("extra credit hold finalization failed: %s", finishErr))
		}
	}
	if receipt.LogPending {
		if err := deliverExtraCreditPurchaseLog(receipt.Id); err != nil {
			common.SysError(fmt.Sprintf("extra credits purchase log pending for receipt %d: %s", receipt.Id, err))
		} else {
			receipt.LogPending = false
		}
	}
	return &receipt, nil
}

func deliverExtraCreditPurchaseLog(id int) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var current SubscriptionExtraCreditPurchase
		if err := lockForUpdate(tx).First(&current, id).Error; err != nil || !current.LogPending {
			return err
		}
		var username string
		if err := tx.Unscoped().Model(&User{}).Where("id = ?", current.UserId).Pluck("username", &username).Error; err != nil {
			return err
		}
		other := NewLogOther()
		message := common.NewMessage("Extra credits purchased: {{credits}}; wallet quota: {{quota}}; subscription: {{subscription}}", map[string]any{"credits": current.Credits, "quota": current.WalletQuota, "subscription": current.SubscriptionId})
		entry := Log{UserId: current.UserId, Username: username, CreatedAt: current.CreatedAt, Type: LogTypeTopup, Content: other.setContent([]*common.Message{message}), Other: other.JSONString(), SubscriptionEventId: fmt.Sprintf("extra-purchase:%d", current.Id)}
		entry.RequestId = current.RequestId
		if err := deliverSubscriptionLog(tx, entry); err != nil {
			return err
		}
		return tx.Model(&current).UpdateColumn("log_pending", false).Error
	})
}

func FlushExtraCreditPurchaseLogs() error {
	var ceiling int
	if err := DB.Model(&SubscriptionExtraCreditPurchase{}).Where("log_pending = ?", true).Select("COALESCE(MAX(id),0)").Scan(&ceiling).Error; err != nil {
		return err
	}
	var failures []error
	var batch []SubscriptionExtraCreditPurchase
	err := DB.Select("id").Where("log_pending = ? AND id <= ?", true, ceiling).FindInBatches(&batch, 100, func(_ *gorm.DB, _ int) error {
		for _, r := range batch {
			if err := deliverExtraCreditPurchaseLog(r.Id); err != nil {
				failures = append(failures, fmt.Errorf("purchase receipt %d: %w", r.Id, err))
			}
		}
		return nil
	}).Error
	return errors.Join(append(failures, err)...)
}

func subscriptionReservedTotal(r *SubscriptionPreConsumeRecord) int64 {
	if r.Allocations != "" {
		return r.ReservedTotal
	}
	return r.PreConsumed
}

func preConsumeSubscriptionCredits(requestID string, userID int, amount int64, group string) (*SubscriptionPreConsumeResult, error) {
	result := &SubscriptionPreConsumeResult{}
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		if group == "" {
			group = user.Group
		}
		var r SubscriptionPreConsumeRecord
		err := tx.Where("request_id = ?", requestID).First(&r).Error
		if err == nil {
			if r.RequestId != requestID || r.UserId != userID || r.RequestGroup != "" && r.RequestGroup != group || r.Status == "refunded" || r.Status == "settled" || r.PreConsumed != amount || r.Allocations != "" && r.ReservedTotal != r.PreConsumed {
				return errors.New("subscription pre-consume request conflicts")
			}
			result.ReceiptId = r.Id
			result.UserSubscriptionId = r.UserSubscriptionId
			result.PreConsumed = r.PreConsumed
			result.Reused = true
			var sub UserSubscription
			if err := tx.First(&sub, r.UserSubscriptionId).Error; err != nil {
				return err
			}
			result.AmountTotal = sub.AmountTotal
			result.AmountUsedBefore = sub.AmountUsed
			result.AmountUsedAfter = sub.AmountUsed
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		r = SubscriptionPreConsumeRecord{RequestId: requestID, UserId: userID, RequestGroup: group, Status: "consumed", Allocations: "[]", PreConsumed: amount, FundingMode: "credits", Durable: true}
		if err := adjustSubscriptionCreditsTx(tx, &r, amount); err != nil {
			return err
		}
		if err := tx.Create(&r).Error; err != nil {
			return err
		}
		result.ReceiptId = r.Id
		result.UserSubscriptionId = r.UserSubscriptionId
		result.PreConsumed = amount
		var sub UserSubscription
		if err := tx.First(&sub, r.UserSubscriptionId).Error; err != nil {
			return err
		}
		result.AmountTotal = sub.AmountTotal
		result.AmountUsedAfter = sub.AmountUsed
		var used []subscriptionQuotaAllocation
		if err := common.UnmarshalJsonStr(r.Allocations, &used); err != nil {
			return err
		}
		result.AmountUsedBefore = sub.AmountUsed
		for _, a := range used {
			if a.SubscriptionId == sub.Id {
				result.AmountUsedBefore -= a.Regular
			}
		}
		return nil
	})
	return result, err
}

func PreserveSubscriptionRequest(requestID string, expected ...SubscriptionPreConsumeRecord) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var r SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ? AND status = ?", requestID, "consumed").First(&r).Error; err != nil {
			return err
		}
		if r.RequestId != requestID {
			return errors.New("subscription request id conflicts")
		}
		if err := validateSubscriptionReceiptSnapshot(&r, expected); err != nil {
			return err
		}
		return tx.Model(&r).UpdateColumns(map[string]any{"durable": true, "updated_at": getDBTimestamp(tx)}).Error
	})
}

func FinishSubscriptionRequest(requestID string) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var r SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ?", requestID).First(&r).Error; err != nil {
			return err
		}
		if r.RequestId != requestID {
			return errors.New("subscription request id conflicts")
		}
		if r.Status == "settled" || r.Status == "refunded" {
			return nil
		}
		if err := releaseSubscriptionFundingTx(tx, &r); err != nil {
			return err
		}
		r.Status = "settled"
		r.Durable = false
		return tx.Save(&r).Error
	})
}

func releaseSubscriptionFundingTx(tx *gorm.DB, r *SubscriptionPreConsumeRecord) error {
	var account User
	if err := lockForUpdate(tx.Unscoped()).Select("id").Where("id = ?", r.UserId).Find(&account).Error; err != nil {
		return err
	}
	if r.Allocations == "" {
		return nil
	}
	var allocations []subscriptionQuotaAllocation
	if err := common.UnmarshalJsonStr(r.Allocations, &allocations); err != nil {
		return err
	}
	slices.SortFunc(allocations, func(a, b subscriptionQuotaAllocation) int { return a.SubscriptionId - b.SubscriptionId })
	for _, a := range allocations {
		amount := a.Regular + a.Extra
		if amount == 0 {
			continue
		}
		var sub UserSubscription
		if err := lockForUpdate(tx).First(&sub, a.SubscriptionId).Error; err != nil {
			return err
		}
		sub.FundingReserved = max(0, sub.FundingReserved-amount)
		if err := tx.Model(&sub).UpdateColumn("funding_reserved", sub.FundingReserved).Error; err != nil {
			return err
		}
	}
	return nil
}

// SubscriptionTaskFinalBilling retains a decided final amount across financial
// rollbacks. New multipliers or an older caller cannot change its retry amount.
type SubscriptionTaskFinalBilling struct {
	Quota int                         `json:"quota"`
	Log   *RecordTaskBillingLogParams `json:"log,omitempty"`
}

func PinSubscriptionTaskFinalQuota(task *Task, actual int, logs ...RecordTaskBillingLogParams) error {
	if actual < 0 {
		return errors.New("invalid subscription final quota")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var stored Task
		if err := lockForUpdate(tx).First(&stored, task.ID).Error; err != nil {
			return err
		}
		if stored.UserId != task.UserId || stored.PrivateData.SubscriptionRequestID != task.PrivateData.SubscriptionRequestID || stored.PrivateData.SubscriptionId != task.PrivateData.SubscriptionId {
			return errors.New("task funding identity conflicts")
		}
		if stored.PrivateData.SubscriptionFinalBilling == nil {
			stored.PrivateData.SubscriptionFinalBilling = &SubscriptionTaskFinalBilling{Quota: actual}
			if len(logs) > 0 {
				p := logs[0]
				stored.PrivateData.SubscriptionFinalBilling.Log = &p
			}
			if err := tx.Model(&stored).UpdateColumn("private_data", stored.PrivateData).Error; err != nil {
				return err
			}
		}
		task.PrivateData = stored.PrivateData
		task.Quota = stored.Quota
		task.SubscriptionBillingPending = stored.SubscriptionBillingPending
		return nil
	})
}

// ClaimSubscriptionTaskReport provides at-most-once terminal reporting after the
// financial commit. A failed financial attempt leaves reporting unclaimed.
func ClaimSubscriptionTaskReport(task *Task) (bool, error) {
	var claimedData TaskPrivateData
	err := DB.Transaction(func(tx *gorm.DB) error {
		var stored Task
		if err := lockForUpdate(tx).First(&stored, task.ID).Error; err != nil {
			return err
		}
		if stored.SubscriptionBillingPending || !stored.PrivateData.SubscriptionFinancialComplete || stored.PrivateData.SubscriptionResultReported {
			return nil
		}
		stored.PrivateData.SubscriptionResultReported = true
		if err := tx.Model(&stored).UpdateColumns(map[string]any{"private_data": stored.PrivateData, "subscription_report_pending": false}).Error; err != nil {
			return err
		}
		claimedData = stored.PrivateData
		return nil
	})
	if err != nil || !claimedData.SubscriptionResultReported {
		return false, err
	}
	task.PrivateData = claimedData
	task.SubscriptionReportPending = false
	return true, nil
}

// SettleSubscriptionTaskQuota commits the allocation receipt and task quota together.
func SettleSubscriptionTaskQuota(task *Task, actual int, logs ...RecordTaskBillingLogParams) (int, error) {
	if err := PinSubscriptionTaskFinalQuota(task, actual, logs...); err != nil {
		return 0, err
	}
	return settleSubscriptionTaskQuota(task, actual, true, logs...)
}

var ErrSubscriptionTaskNotFound = fmt.Errorf("subscription task: %w", gorm.ErrRecordNotFound)

// PrepareSubscriptionTaskQuota atomically aligns a persisted task with its
// submission charge while keeping its allocation open for terminal settlement.
func PrepareSubscriptionTaskQuota(task *Task, actual int, logs ...RecordTaskBillingLogParams) error {
	_, err := settleSubscriptionTaskQuota(task, actual, false, logs...)
	return err
}

func settleSubscriptionTaskQuota(task *Task, actual int, final bool, logs ...RecordTaskBillingLogParams) (int, error) {
	var delta int
	var committed Task
	err := DB.Transaction(func(tx *gorm.DB) error {
		var stored Task
		if err := lockForUpdate(tx).First(&stored, task.ID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return ErrSubscriptionTaskNotFound
			}
			return err
		}
		if stored.PrivateData.SubscriptionRequestID != task.PrivateData.SubscriptionRequestID || stored.UserId != task.UserId {
			return errors.New("task funding identity conflicts")
		}
		if stored.PrivateData.SubscriptionFinancialComplete {
			committed = stored
			return nil
		}
		var r SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ?", task.PrivateData.SubscriptionRequestID).First(&r).Error; err != nil {
			return err
		}
		if r.RequestId != task.PrivateData.SubscriptionRequestID || r.UserId != task.UserId || r.UserSubscriptionId != task.PrivateData.SubscriptionId || stored.PrivateData.SubscriptionReceiptId > 0 && r.Id != stored.PrivateData.SubscriptionReceiptId {
			return errors.New("task funding receipt conflicts")
		}
		if final && stored.PrivateData.SubscriptionFinalBilling != nil {
			actual = stored.PrivateData.SubscriptionFinalBilling.Quota
			logs = nil
			if stored.PrivateData.SubscriptionFinalBilling.Log != nil {
				logs = []RecordTaskBillingLogParams{*stored.PrivateData.SubscriptionFinalBilling.Log}
			}
		}
		if !final {
			if stored.PrivateData.SubscriptionSubmitQuota == nil {
				committed = stored
				return nil
			}
			actual = *stored.PrivateData.SubscriptionSubmitQuota
		}
		if actual < 0 {
			return errors.New("invalid subscription final quota")
		}
		if r.Status == "settled" || r.Status == "refunded" {
			if stored.Quota != actual {
				return errors.New("completed subscription task quota conflicts")
			}
			if subscriptionReservedTotal(&r) != int64(actual) {
				return errors.New("subscription final quota conflicts")
			}
		} else {
			if err := adjustSubscriptionCreditsTx(tx, &r, int64(actual)-subscriptionReservedTotal(&r)); err != nil {
				return err
			}
			if final {
				if err := releaseSubscriptionFundingTx(tx, &r); err != nil {
					return err
				}
				r.Status = "settled"
				if actual == 0 {
					r.Status = "refunded"
				}
				r.Durable = false
			}
			if err := tx.Save(&r).Error; err != nil {
				return err
			}
		}
		delta = actual - stored.Quota
		if stored.PrivateData.TokenId > 0 && delta != 0 {
			var token Token
			tokenErr := lockForUpdate(tx).Where("id = ? AND user_id = ?", stored.PrivateData.TokenId, stored.UserId).First(&token).Error
			if tokenErr != nil && !errors.Is(tokenErr, gorm.ErrRecordNotFound) {
				return tokenErr
			}
			if tokenErr == nil {
				if err := invalidateTokenCacheForMutation(token.Key); err != nil {
					return err
				}
				if err := tx.Model(&token).Updates(map[string]any{"remain_quota": gorm.Expr("remain_quota - ?", delta), "used_quota": gorm.Expr("used_quota + ?", delta), "accessed_time": common.GetTimestamp()}).Error; err != nil {
					return err
				}
			}
		}
		initial := stored.PrivateData.SubscriptionSubmitLog != nil
		submitAmount := actual
		if final && initial {
			submitAmount = stored.Quota
			if target := stored.PrivateData.SubscriptionSubmitQuota; target != nil {
				submitAmount = min(submitAmount, *target)
			}
		}
		billingDelta := delta
		if final && initial {
			billingDelta = actual - submitAmount
		}
		statDelta := 0
		if final {
			statDelta = delta
			if initial {
				statDelta = actual
			}
		} else if initial {
			statDelta = actual
		}
		if statDelta != 0 || initial {
			updates := map[string]any{"used_quota": gorm.Expr("used_quota + ?", statDelta)}
			if initial {
				updates["request_count"] = gorm.Expr("request_count + 1")
			}
			if err := tx.Unscoped().Model(&User{}).Where("id = ?", stored.UserId).Updates(updates).Error; err != nil {
				return err
			}
			if stored.ChannelId > 0 {
				if err := tx.Model(&Channel{}).Where("id = ?", stored.ChannelId).UpdateColumn("used_quota", gorm.Expr("used_quota + ?", statDelta)).Error; err != nil {
					return err
				}
			}
		}
		if initial && common.LogConsumeEnabled {
			params := *stored.PrivateData.SubscriptionSubmitLog
			if err := queueSubscriptionTaskBillingLogTx(tx, &r, &stored, params, submitAmount, LogTypeConsume); err != nil {
				return err
			}
		}
		if final && billingDelta != 0 && (billingDelta < 0 || common.LogConsumeEnabled) {
			params := RecordTaskBillingLogParams{}
			if len(logs) > 0 {
				params = logs[0]
			}
			if params.Other == nil {
				params.Other = NewLogOther()
			}
			params.Other.SetPublic("pre_consumed_quota", actual-billingDelta)
			params.Other.SetPublic("actual_quota", actual)
			kind, amount := LogTypeConsume, billingDelta
			if billingDelta < 0 {
				kind = LogTypeRefund
				amount = -billingDelta
				if stored.PrivateData.SubscriptionLocalRefund && stored.FailReason != "" {
					params.Other.SetPublic("reason", stored.FailReason)
				}
			}
			if err := queueSubscriptionTaskBillingLogTx(tx, &r, &stored, params, amount, kind); err != nil {
				return err
			}
		}
		updates := map[string]any{"quota": actual, "subscription_billing_pending": !final}
		stored.PrivateData.SubscriptionSubmitQuota = nil
		stored.PrivateData.SubscriptionSubmitLog = nil
		if final {
			stored.PrivateData.SubscriptionFinancialComplete = true
			stored.SubscriptionReportPending = !stored.PrivateData.SubscriptionResultReported
			updates["subscription_report_pending"] = stored.SubscriptionReportPending
			if stored.PrivateData.SubscriptionFinalBilling != nil {
				stored.PrivateData.SubscriptionFinalBilling.Log = nil
			}
		}
		updates["private_data"] = stored.PrivateData
		if err := tx.Model(&stored).Updates(updates).Error; err != nil {
			return err
		}
		stored.Quota = actual
		stored.SubscriptionBillingPending = !final
		committed = stored
		return nil
	})
	if err == nil {
		task.Quota = committed.Quota
		task.SubscriptionBillingPending = committed.SubscriptionBillingPending
		task.SubscriptionReportPending = committed.SubscriptionReportPending
		task.PrivateData = committed.PrivateData
	}
	return delta, err
}

// Each export is committed in the primary DB transaction that clears its
// receipt queue. Separate log DB delivery remains idempotent by event ID.
type subscriptionTaskBillingEvent struct {
	EventId      string `json:"subscription_event_id,omitempty"`
	LogDelivered bool   `json:"subscription_log_delivered,omitempty"`
	Log
	Export *QuotaData `json:"subscription_export,omitempty"`
	// Retain the export for audit if its sibling log still needs delivery.
	// An older worker also sees a nil Export and will not reinsert it.
	DeliveredExport *QuotaData `json:"subscription_delivered_export,omitempty"`
}

// decodeSubscriptionTaskBillingLogs also accepts a single legacy pending log.
func decodeSubscriptionTaskBillingLogs(raw string) ([]subscriptionTaskBillingEvent, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var entries []subscriptionTaskBillingEvent
	if strings.HasPrefix(strings.TrimSpace(raw), "[") {
		err := common.UnmarshalJsonStr(raw, &entries)
		return entries, err
	}
	var entry subscriptionTaskBillingEvent
	if err := common.UnmarshalJsonStr(raw, &entry); err != nil {
		return nil, err
	}
	return []subscriptionTaskBillingEvent{entry}, nil
}

// The receipt queue and dashboard event commit with the charge; retries cannot
// replace an undelivered submission log with the terminal difference.
func queueSubscriptionTaskBillingLogTx(tx *gorm.DB, r *SubscriptionPreConsumeRecord, task *Task, params RecordTaskBillingLogParams, quota, kind int) error {
	var user User
	if err := tx.Unscoped().Select("username").Where("id = ?", task.UserId).Find(&user).Error; err != nil {
		return err
	}
	tokenName := params.TokenName
	if task.PrivateData.TokenId > 0 && tokenName == "" {
		var token Token
		if err := tx.Unscoped().Select("name").Where("id = ?", task.PrivateData.TokenId).Find(&token).Error; err != nil {
			return err
		}
		tokenName = token.Name
	}
	other := params.Other
	if other == nil {
		other = NewLogOther()
	}
	other.SetPublic("task_id", task.TaskID)
	other.SetPublic("is_task", true)
	modelName := params.ModelName
	if modelName == "" {
		modelName = task.Properties.OriginModelName
	}
	entry := Log{RequestId: params.RequestId, UpstreamRequestId: params.UpstreamRequestId, PromptTokens: params.PromptTokens, Ip: params.Ip, UserId: task.UserId, Username: user.Username, TokenName: tokenName, CreatedAt: common.GetTimestamp(), Type: kind, Content: other.setContent(params.Content), ModelName: modelName, Quota: quota, ChannelId: task.ChannelId, TokenId: task.PrivateData.TokenId, Group: task.Group, Other: other.JSONString()}
	if task.PrivateData.Execution != nil {
		entry.UserAgent = NormalizeLogUserAgent(task.PrivateData.Execution.UserAgent)
	}
	ensureLogRequestId(&entry)
	var entries []subscriptionTaskBillingEvent
	if r.BillingLogPending {
		var err error
		entries, err = decodeSubscriptionTaskBillingLogs(r.BillingLog)
		if err != nil {
			return err
		}
	}
	event := subscriptionTaskBillingEvent{Log: entry, EventId: common.NewRequestId()}
	if kind == LogTypeConsume && common.DataExportEnabled {
		node := params.NodeName
		if node == "" {
			node = task.PrivateData.NodeName
		}
		if node == "" {
			node = common.NodeName
		}
		data := QuotaData{UserID: task.UserId, Username: user.Username, ModelName: modelName, TokenUsed: params.PromptTokens, Quota: quota, Count: 1, CreatedAt: entry.CreatedAt - entry.CreatedAt%3600, UseGroup: task.Group, TokenID: task.PrivateData.TokenId, ChannelID: task.ChannelId, NodeName: node}
		event.Export = &data
	}
	entries = append(entries, event)
	raw, err := common.Marshal(entries)
	if err != nil {
		return err
	}
	r.BillingLog = string(raw)
	r.BillingLogPending = true
	return tx.Model(r).Updates(map[string]any{"billing_log": r.BillingLog, "billing_log_pending": true}).Error
}

// The caller holds the source receipt lock. A separate log database commits
// before the receipt ack, so a retry must recognize the original event.
func deliverSubscriptionLog(tx *gorm.DB, entry Log) error {
	if entry.RequestId == "" {
		return errors.New("subscription billing log has no identity")
	}
	logDB := LOG_DB
	if logDB == DB {
		logDB = tx
	}
	query := logDB.Model(&Log{})
	if entry.SubscriptionEventId != "" {
		query = query.Where("subscription_event_id = ? AND type = ?", entry.SubscriptionEventId, entry.Type)
	} else {
		query = query.Where("request_id = ? AND user_id = ? AND channel_id = ? AND type = ? AND quota = ?", entry.RequestId, entry.UserId, entry.ChannelId, entry.Type, entry.Quota)
	}
	var count int64
	if err := query.Count(&count).Error; err != nil || count > 0 {
		return err
	}
	entry.Id = 0
	return logDB.Create(&entry).Error
}

var subscriptionLogCursor atomic.Int64
var subscriptionLogCeiling atomic.Int64

// MySQL and PostgreSQL use bounded indexed identifiers; SQLite and the
// ClickHouse log table store these fields without varchar width limits.
func oversizedBillingIdentifiers(db *gorm.DB, fields ...string) bool {
	if db.Dialector.Name() != "mysql" && db.Dialector.Name() != "postgres" {
		return false
	}
	return slices.ContainsFunc(fields, func(s string) bool { return utf8.RuneCountInString(s) > 191 })
}

func deliverSubscriptionEventLog(tx *gorm.DB, receiptID int, event subscriptionTaskBillingEvent) error {
	if oversizedBillingIdentifiers(LOG_DB, event.RequestId, event.UpstreamRequestId) ||
		LOG_DB.Dialector.Name() == "mysql" && oversizedBillingIdentifiers(LOG_DB, event.ModelName) {
		return quarantineSubscriptionEvent(tx, receiptID, event, "log")
	}
	// Long text may compress and fit a PostgreSQL index. Only quarantine an
	// actual index-row size rejection, after rolling back the failed insert.
	if LOG_DB.Dialector.Name() == "postgres" && utf8.RuneCountInString(event.ModelName) > 191 {
		err := tx.Transaction(func(sp *gorm.DB) error { return deliverSubscriptionLog(sp, event.Log) })
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "54000" && (strings.HasPrefix(pgErr.Message, "index row requires ") || strings.HasPrefix(pgErr.Message, "index row size ")) {
			return quarantineSubscriptionEvent(tx, receiptID, event, "log")
		}
		return err
	}
	return deliverSubscriptionLog(tx, event.Log)
}

// The primary log table also exists with a separate log DB. This root-only
// diagnostic and the destination ack commit together, independently of receipt
// retention. Keep the payload out of indexed columns and user-visible content.
func quarantineSubscriptionEvent(tx *gorm.DB, receiptID int, event subscriptionTaskBillingEvent, destination string) error {
	if event.Export == nil {
		event.Export, event.DeliveredExport = event.DeliveredExport, nil
	}
	other := NewLogOther()
	other.SetRoot("subscription_delivery", map[string]any{"receipt_id": receiptID, "destination": destination, "event": event})
	payload, err := common.Marshal(other)
	if err != nil {
		return err
	}
	entry := Log{Type: LogTypeSystem, CreatedAt: common.GetTimestamp(), Content: "Subscription billing delivery quarantined: indexed identifier too long (" + destination + ")", Other: string(payload), SubscriptionEventId: event.EventId}
	ensureLogRequestId(&entry)
	return tx.Create(&entry).Error
}

func FlushSubscriptionTaskBillingLogs() error {
	cursor, ceiling := subscriptionLogCursor.Load(), subscriptionLogCeiling.Load()
	if ceiling == 0 {
		if err := DB.Model(&SubscriptionPreConsumeRecord{}).Where("billing_log_pending = ?", true).Select("COALESCE(MAX(id),0)").Scan(&ceiling).Error; err != nil {
			return err
		}
		subscriptionLogCeiling.Store(ceiling)
	}
	var pending []SubscriptionPreConsumeRecord
	if err := DB.Select("id").Where("billing_log_pending = ? AND id > ? AND id <= ?", true, cursor, ceiling).Order("id").Limit(100).Find(&pending).Error; err != nil {
		return err
	}
	if len(pending) < 100 || len(pending) > 0 && int64(pending[len(pending)-1].Id) >= ceiling {
		subscriptionLogCursor.Store(0)
		subscriptionLogCeiling.Store(0)
	} else {
		subscriptionLogCursor.Store(int64(pending[len(pending)-1].Id))
	}
	var failures []error
	for _, candidate := range pending {
		if err := DB.Transaction(func(tx *gorm.DB) error {
			var receipt SubscriptionPreConsumeRecord
			if err := lockForUpdate(tx).First(&receipt, candidate.Id).Error; err != nil {
				return err
			}
			if !receipt.BillingLogPending {
				return nil
			}
			entries, err := decodeSubscriptionTaskBillingLogs(receipt.BillingLog)
			if err != nil {
				return err
			}
			remaining := entries[:0]
			for _, event := range entries {
				archiveExport := event.Export != nil && oversizedBillingIdentifiers(tx, event.Export.NodeName, event.Export.ModelName)
				// A rejected statement poisons a PostgreSQL transaction until rollback.
				// Give each destination a savepoint; ack only its successful payload.
				if !event.LogDelivered {
					event.Log.SubscriptionEventId = event.EventId
					if err := tx.Transaction(func(sp *gorm.DB) error {
						return deliverSubscriptionEventLog(sp, receipt.Id, event)
					}); err != nil {
						failures = append(failures, err)
					} else {
						event.LogDelivered = true
						if !archiveExport {
							event.Log = Log{}
						}
					}
				}
				if event.Export != nil {
					data := *event.Export
					data.Id = 0
					if err := tx.Transaction(func(sp *gorm.DB) error {
						if archiveExport {
							return quarantineSubscriptionEvent(sp, receipt.Id, event, "export")
						}
						return sp.Create(&data).Error
					}); err != nil {
						failures = append(failures, err)
					} else {
						event.DeliveredExport, event.Export = event.Export, nil
					}
				}
				if !event.LogDelivered || event.Export != nil {
					remaining = append(remaining, event)
				}
			}
			var raw []byte
			if len(remaining) > 0 {
				raw, err = common.Marshal(remaining)
				if err != nil {
					return err
				}
			}
			return tx.Model(&receipt).Updates(map[string]any{"billing_log_pending": len(remaining) > 0, "billing_log": string(raw)}).Error
		}); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// SettleSubscriptionRequestQuota applies an absolute final amount once.
// Retrying after a task write-back failure observes the same settled receipt.
func SettleSubscriptionRequestQuota(requestID string, actual int64, expected ...SubscriptionPreConsumeRecord) error {
	if actual < 0 {
		return errors.New("invalid subscription final quota")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var r SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ?", requestID).First(&r).Error; err != nil {
			return err
		}
		if r.RequestId != requestID {
			return errors.New("subscription request id conflicts")
		}
		if err := validateSubscriptionReceiptSnapshot(&r, expected); err != nil {
			return err
		}
		if r.Status == "settled" || r.Status == "refunded" {
			if subscriptionReservedTotal(&r) == actual {
				return nil
			}
			return errors.New("subscription final quota conflicts")
		}
		if err := adjustSubscriptionCreditsTx(tx, &r, actual-subscriptionReservedTotal(&r)); err != nil {
			return err
		}
		if err := releaseSubscriptionFundingTx(tx, &r); err != nil {
			return err
		}
		r.Status = "settled"
		if actual == 0 {
			r.Status = "refunded"
		}
		r.Durable = false
		return tx.Save(&r).Error
	})
}

func AdjustSubscriptionRequestQuota(requestID string, delta int64) error {
	_, err := AdjustSubscriptionRequestQuotaWithResult(requestID, delta)
	return err
}

// AdjustSubscriptionRequestQuotaWithResult returns the original funding row's
// ordinary usage from the same transaction as the allocation adjustment.
func AdjustSubscriptionRequestQuotaWithResult(requestID string, delta int64, expected ...SubscriptionPreConsumeRecord) (*SubscriptionPreConsumeResult, error) {
	if requestID == "" {
		return nil, errors.New("subscription request id is empty")
	}
	var result SubscriptionPreConsumeResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		var r SubscriptionPreConsumeRecord
		if err := lockForUpdate(tx).Where("request_id = ?", requestID).First(&r).Error; err != nil {
			return err
		}
		if r.RequestId != requestID {
			return errors.New("subscription request id conflicts")
		}
		if err := validateSubscriptionReceiptSnapshot(&r, expected); err != nil {
			return err
		}
		if r.Status == "refunded" || r.Status == "settled" {
			return errors.New("subscription pre-consume already completed")
		}
		if err := adjustSubscriptionCreditsTx(tx, &r, delta); err != nil {
			return err
		}
		if err := tx.Save(&r).Error; err != nil {
			return err
		}
		var sub UserSubscription
		if err := tx.First(&sub, r.UserSubscriptionId).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		result = SubscriptionPreConsumeResult{ReceiptId: r.Id, UserSubscriptionId: sub.Id, PreConsumed: r.ReservedTotal, AmountTotal: sub.AmountTotal, AmountUsedAfter: sub.AmountUsed}
		return nil
	})
	return &result, err
}

// A nonnegative ReservedTotal also pins the allocation snapshot used to compute
// a delta. Identity-only session guards use -1 with the receipt ID returned by reserve.
func validateSubscriptionReceiptSnapshot(r *SubscriptionPreConsumeRecord, expected []SubscriptionPreConsumeRecord) error {
	for _, prior := range expected {
		if r.Id != prior.Id || r.UserId != prior.UserId || r.UserSubscriptionId != prior.UserSubscriptionId || r.RequestId != prior.RequestId {
			return errors.New("subscription funding receipt changed")
		}
		if prior.ReservedTotal >= 0 && (subscriptionReservedTotal(r) != subscriptionReservedTotal(&prior) || r.Allocations != prior.Allocations || r.Status != prior.Status) {
			return errors.New("subscription funding snapshot changed")
		}
	}
	return nil
}

// All callers hold the request row; all funding rows lock in stable ID order.
// Extra refunds are taken from this request's allocation, never another one.
func adjustSubscriptionCreditsTx(tx *gorm.DB, r *SubscriptionPreConsumeRecord, delta int64) error {
	if delta == 0 {
		return nil
	}
	if delta == math.MinInt64 {
		return errors.New("invalid subscription quota delta")
	}
	var allocations []subscriptionQuotaAllocation
	legacy := r.Allocations == ""
	// Financial mutations always lock the account before subscription rows.
	var account User
	if err := lockForUpdate(tx.Unscoped()).Select("id", "group", "deleted_at").Where("id = ?", r.UserId).Find(&account).Error; err != nil {
		return err
	}
	if delta > 0 && account.Id == 0 {
		return gorm.ErrRecordNotFound
	}

	if legacy {
		allocations = []subscriptionQuotaAllocation{{SubscriptionId: r.UserSubscriptionId, Regular: r.PreConsumed}}
		r.ReservedTotal = r.PreConsumed
	} else if err := common.UnmarshalJsonStr(r.Allocations, &allocations); err != nil {
		return err
	}
	if r.ReservedTotal < 0 || delta < 0 && -delta > r.ReservedTotal || delta > 0 && delta > math.MaxInt64-r.ReservedTotal {
		return errors.New("subscription quota delta exceeds reservation")
	}
	var subs []UserSubscription
	if err := lockForUpdate(tx).Where("user_id = ?", r.UserId).Order("id asc").Find(&subs).Error; err != nil {
		return err
	}
	byID := map[int]*UserSubscription{}
	for i := range subs {
		byID[subs[i].Id] = &subs[i]
	}
	if legacy {
		if sub := byID[r.UserSubscriptionId]; sub != nil {
			allocations[0].PeriodReset = sub.LastResetTime
			allocations[0].FiveHourReset = sub.FiveHourLastResetTime
			allocations[0].WeeklyReset = sub.WeeklyLastResetTime
			allocations[0].MonthlyReset = sub.MonthlyLastResetTime
			allocations[0].PeriodVersion = sub.UsageResetVersion
			allocations[0].FiveHourVersion = sub.FiveHourResetVersion
			allocations[0].WeeklyVersion = sub.WeeklyResetVersion
			allocations[0].MonthlyVersion = sub.MonthlyResetVersion
			if sub.FundingReserved < 0 || r.PreConsumed < 0 || r.PreConsumed > math.MaxInt64-sub.FundingReserved {
				return errors.New("subscription reservation overflow")
			}
			sub.FundingReserved += r.PreConsumed
		}
	}
	if delta < 0 {
		remaining := -delta
		for _, extra := range []bool{true, false} {
			for i := len(allocations) - 1; i >= 0 && remaining > 0; i-- {
				a := &allocations[i]
				sub := byID[a.SubscriptionId]
				available := a.Regular
				if extra {
					available = a.Extra
				}
				n := min(remaining, available)
				if n == 0 {
					continue
				}
				if sub == nil {
					return errors.New("subscription funding row missing")
				}
				sub.FundingReserved = max(0, sub.FundingReserved-n)
				if extra {
					if sub.ExtraCreditsUsed < n {
						return errors.New("extra credit refund exceeds usage")
					}
					sub.ExtraCreditsUsed -= n
					a.Extra -= n
				} else {
					// A late refund must not return quota used by a different
					// request after a billing window rolled over.
					if legacy || a.sameUsagePeriod(sub) {
						sub.AmountUsed = decreaseSubscriptionUsage(sub.AmountUsed, -n)
					}
					if legacy || a.FiveHourReset == sub.FiveHourLastResetTime && a.FiveHourVersion == sub.FiveHourResetVersion {
						sub.FiveHourUsed = decreaseSubscriptionUsage(sub.FiveHourUsed, -n)
					}
					if legacy || a.WeeklyReset == sub.WeeklyLastResetTime && a.WeeklyVersion == sub.WeeklyResetVersion {
						sub.WeeklyUsed = decreaseSubscriptionUsage(sub.WeeklyUsed, -n)
					}
					if legacy || a.MonthlyReset == sub.MonthlyLastResetTime && a.MonthlyVersion == sub.MonthlyResetVersion {
						sub.MonthlyUsed = decreaseSubscriptionUsage(sub.MonthlyUsed, -n)
					}
					a.Regular -= n
				}
				remaining -= n
			}
		}
		if remaining != 0 {
			return errors.New("subscription allocation is incomplete")
		}
	} else {
		remaining := delta
		group := r.RequestGroup
		if group == "" {
			group = account.Group
		}
		spendableExtras := map[int]bool{}
		if !account.DeletedAt.Valid {
			for i := range subs {
				sub := &subs[i]
				if sub.ExtraCreditsUsed < 0 || sub.ExtraCreditsTotal <= sub.ExtraCreditsUsed || sub.FundingReserved < 0 || sub.FundingReserved == math.MaxInt64 || !extraCreditStatus(sub.Status) {
					continue
				}
				groups, err := extraCreditAuthorizedGroupsTx(tx, sub)
				if err != nil {
					return err
				}
				if slices.Contains(groups, group) {
					spendableExtras[sub.Id] = true
				}
			}
		}
		hasGroupExtras := len(spendableExtras) > 0
		if hasGroupExtras {
			r.FundingMode = "credits"
		} else if r.FundingMode == "" {
			r.FundingMode = "legacy"
		}
		scoped, err := isSubscriptionGroupTx(tx, group)
		if err != nil {
			return err
		}
		checkLegacy := newLegacyCycleAccessCheck(tx, r.UserId, group)
		slices.SortFunc(subs, func(a, b UserSubscription) int {
			if a.EndTime < b.EndTime {
				return -1
			}
			if a.EndTime > b.EndTime {
				return 1
			}
			return a.Id - b.Id
		})
		// Rebuild pointers after sorting the owning slice.
		for i := range subs {
			byID[subs[i].Id] = &subs[i]
		}
		for _, extra := range []bool{false, true} {
			for i := range subs {
				sub := &subs[i]
				if remaining == 0 {
					break
				}
				if (r.FundingMode == "legacy" || !extra && !hasGroupExtras) && r.UserSubscriptionId > 0 && sub.Id != r.UserSubscriptionId {
					continue
				}
				available := int64(0)
				regularEligible := false
				if extra {
					if !spendableExtras[sub.Id] {
						continue
					}
					if sub.ExtraCreditsUsed < 0 || sub.ExtraCreditsTotal < sub.ExtraCreditsUsed {
						return errors.New("invalid extra credits balance")
					}
					available = sub.ExtraCreditsTotal - sub.ExtraCreditsUsed
					if sub.Status == "active" && sub.EndTime > getDBTimestamp(tx) && sub.AmountTotal >= 0 {
						plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
						if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
							return err
						}
						if err == nil && !sub.isWalletOnlyForGroup(plan, group, scoped) {
							regularEligible = true
							if scoped && plan.SubscriptionGroup == "" {
								regularEligible, err = checkLegacy()
								if err != nil {
									return err
								}
							}
						}
					}
				} else {
					plan, err := getSubscriptionPlanByIdTx(tx, sub.PlanId)
					if errors.Is(err, gorm.ErrRecordNotFound) {
						continue
					}
					if err != nil {
						return err
					}
					if sub.Status == "active" && sub.EndTime > getDBTimestamp(tx) {
						if err := maybeResetUserSubscriptionWithPlanTx(tx, sub, plan, getDBTimestamp(tx)); err != nil {
							return err
						}
					}
					continuing := slices.ContainsFunc(allocations, func(a subscriptionQuotaAllocation) bool {
						return a.SubscriptionId == sub.Id && (a.Regular > 0 || a.RegularEligible || r.FundingMode == "legacy" && sub.Id == r.UserSubscriptionId) && a.sameUsagePeriod(sub)
					})
					if account.DeletedAt.Valid && !continuing {
						continue
					}
					if sub.AmountTotal < 0 || (!continuing && (sub.Status != "active" || sub.EndTime <= getDBTimestamp(tx))) || sub.Status == "cancelled" {
						continue
					}
					if !continuing && sub.isWalletOnlyForGroup(plan, group, scoped) {
						continue
					}
					if !continuing && scoped && plan.SubscriptionGroup == "" {
						allowed, err := checkLegacy()
						if err != nil {
							return err
						}
						if !allowed {
							continue
						}
					}
					regularEligible = true
					if sub.AmountUsed < 0 {
						continue
					}
					available = min(remaining, math.MaxInt64-sub.AmountUsed)
					for _, cap := range [][2]int64{{sub.AmountTotal, sub.AmountUsed}, {sub.FiveHourLimit, sub.FiveHourUsed}, {sub.WeeklyLimit, sub.WeeklyUsed}, {sub.MonthlyLimit, sub.MonthlyUsed}} {
						if cap[0] > 0 {
							if cap[1] < 0 {
								available = 0
								break
							}
							available = min(available, max(cap[0]-cap[1], 0))
						}
					}
				}
				if sub.FundingReserved < 0 {
					return errors.New("invalid subscription reservation")
				}
				available = min(available, math.MaxInt64-sub.FundingReserved)
				if !extra && !hasGroupExtras && available < remaining {
					continue
				}
				n := min(remaining, available)
				if n <= 0 {
					continue
				}
				index := slices.IndexFunc(allocations, func(a subscriptionQuotaAllocation) bool {
					return a.SubscriptionId == sub.Id && a.PeriodReset == sub.LastResetTime && a.PeriodVersion == sub.UsageResetVersion && a.FiveHourReset == sub.FiveHourLastResetTime && a.FiveHourVersion == sub.FiveHourResetVersion && a.WeeklyReset == sub.WeeklyLastResetTime && a.WeeklyVersion == sub.WeeklyResetVersion && a.MonthlyReset == sub.MonthlyLastResetTime && a.MonthlyVersion == sub.MonthlyResetVersion
				})
				if index < 0 {
					allocations = append(allocations, subscriptionQuotaAllocation{SubscriptionId: sub.Id, PeriodReset: sub.LastResetTime, FiveHourReset: sub.FiveHourLastResetTime, WeeklyReset: sub.WeeklyLastResetTime, MonthlyReset: sub.MonthlyLastResetTime, PeriodVersion: sub.UsageResetVersion, FiveHourVersion: sub.FiveHourResetVersion, WeeklyVersion: sub.WeeklyResetVersion, MonthlyVersion: sub.MonthlyResetVersion, RegularEligible: regularEligible})
					index = len(allocations) - 1
				}
				sub.FundingReserved += n
				if extra {
					allocations[index].RegularEligible = allocations[index].RegularEligible || regularEligible
					sub.ExtraCreditsUsed += n
					allocations[index].Extra += n
				} else {
					before := sub.FiveHourUsed
					if err := sub.applyUsageDelta(n); err != nil {
						return err
					}
					initializeSubscriptionUsageWindow(before, sub.FiveHourUsed, &sub.FiveHourLastResetTime, &sub.FiveHourNextResetTime, sub.FiveHourLimit, SubscriptionResetFiveHour, sub.EndTime, getDBTimestamp(tx))
					allocations[index].RegularEligible = true
					allocations[index].Regular += n
					allocations[index].PeriodReset = sub.LastResetTime
					allocations[index].FiveHourReset = sub.FiveHourLastResetTime
					allocations[index].WeeklyReset = sub.WeeklyLastResetTime
					allocations[index].MonthlyReset = sub.MonthlyLastResetTime
					allocations[index].PeriodVersion = sub.UsageResetVersion
					allocations[index].FiveHourVersion = sub.FiveHourResetVersion
					allocations[index].WeeklyVersion = sub.WeeklyResetVersion
					allocations[index].MonthlyVersion = sub.MonthlyResetVersion
				}
				if r.UserSubscriptionId == 0 {
					r.UserSubscriptionId = sub.Id
				}
				remaining -= n
			}
		}
		if remaining > 0 {
			return fmt.Errorf("subscription quota insufficient, need=%d", delta)
		}
	}
	for _, a := range allocations {
		if sub := byID[a.SubscriptionId]; sub != nil {
			if err := tx.Save(sub).Error; err != nil {
				return err
			}
		}
	}
	raw, err := common.Marshal(allocations)
	if err != nil {
		return err
	}
	r.Allocations = string(raw)
	r.ReservedTotal += delta
	r.Durable = true
	return nil
}

// Schedule initialization anchors to StartTime without clearing usage. Keep
// comparing historical timestamps for receipts predating versioned natural resets.
func (a subscriptionQuotaAllocation) sameUsagePeriod(sub *UserSubscription) bool {
	return a.PeriodVersion == sub.UsageResetVersion && (a.PeriodReset == sub.LastResetTime || a.PeriodReset == 0 && sub.LastResetTime == sub.StartTime)
}

func extraCreditAuthorizedGroupsTx(tx *gorm.DB, sub *UserSubscription) ([]string, error) {
	groups, err := sub.extraCreditGroups()
	if err != nil {
		return nil, err
	}
	var pinnedScoped []string
	if sub.ExtraCreditsScopedGroups != "" {
		if err := common.UnmarshalJsonStr(sub.ExtraCreditsScopedGroups, &pinnedScoped); err != nil {
			return nil, err
		}
	}
	scoped, err := subscriptionGroupNamesTx(tx)
	if err != nil {
		return nil, err
	}
	userGroup, err := getUserGroupByIdTx(tx, sub.UserId)
	if err != nil {
		return nil, err
	}
	ordinary := OrdinaryUserUsableGroups(userGroup)
	return slices.DeleteFunc(groups, func(g string) bool {
		if slices.Contains(scoped, g) {
			return !slices.Contains(pinnedScoped, g)
		}
		_, ok := ordinary[g]
		return !ok
	}), nil
}

func GetUserExtraCreditGroups(userID int) ([]string, error) {
	if userID <= 0 {
		return nil, nil
	}
	var subs []UserSubscription
	if err := DB.Where("user_id = ? AND extra_credits_total > extra_credits_used AND status IN ?", userID, []string{"active", "expired", SubscriptionStatusUpgraded}).Find(&subs).Error; err != nil {
		return nil, err
	}
	var groups []string
	for _, sub := range subs {
		names, err := extraCreditAuthorizedGroupsTx(DB, &sub)
		if err != nil {
			return nil, err
		}
		groups = append(groups, names...)
	}
	slices.Sort(groups)
	return slices.Compact(groups), nil
}
