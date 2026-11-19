package model

import (
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
)

type legacyCycleGrant struct {
	Access   []string                `json:"access,omitempty"`
	Quota    []string                `json:"quota,omitempty"`
	Checkout *legacyCheckoutIdentity `json:"checkout,omitempty"`
}

type legacyCheckoutIdentity struct {
	Group    string `json:"group"`
	Previous string `json:"previous"`
	Upgrade  string `json:"upgrade"`
}

var errLegacySubscriptionGroupsUnavailable = common.NewMessage("This legacy plan no longer has an available subscription group")

func CreateSubscriptionPlanWithBinding(plan *SubscriptionPlan) error {
	return createSubscriptionPlanWithBinding(DB, plan)
}

func createSubscriptionPlanWithBinding(db *gorm.DB, plan *SubscriptionPlan) error {
	return subscriptionPlanTransaction(db, func(tx *gorm.DB) error {
		if plan.SubscriptionGroup != "" {
			if err := lockSubscriptionBindingSetTx(tx); err != nil {
				return err
			}
		} else if err := lockSubscriptionCatalogRevisionTx(tx); err != nil {
			return err
		}
		if err := retainLegacyAccessBeforeBindingTx(tx, plan.SubscriptionGroup); err != nil {
			return err
		}
		return tx.Create(plan).Error
	})
}

func retainLegacyAccessBeforeBindingTx(tx *gorm.DB, group string) error {
	if group == "" || group == "auto" {
		return nil
	}
	current := tx
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		current = lockForUpdate(tx)
	}
	bound, err := isSubscriptionGroupTx(current, group)
	if err != nil || bound {
		return err
	}
	// The structural operation already locked the plan catalog in ID order.
	// PostgreSQL also needs new row versions so a pre-binding repeatable-read
	// checkout cannot acquire an unchanged legacy plan and use a stale catalog.
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		if err := tx.Model(&SubscriptionPlan{}).Where(subscriptionGroupByteLength()+" = 0").UpdateColumn("subscription_group", gorm.Expr("subscription_group")).Error; err != nil {
			return err
		}
	}
	return snapshotLegacySubscriptionGroupsTx(tx, map[string]bool{group: true})
}

// Snapshot access and funding separately. Only cycles and unsettled checkouts
// that already existed when the group was ordinary get a grant. Checkout,
// completion, and renewal all lock the affected plan before its orders/cycles.
func snapshotLegacySubscriptionGroupsTx(tx *gorm.DB, groups map[string]bool) error {
	if len(groups) == 0 {
		return nil
	}
	var plans []SubscriptionPlan
	if err := lockForUpdate(tx).Where(subscriptionGroupByteLength() + " = 0").Order("id").Find(&plans).Error; err != nil {
		return err
	}

	ratios := ratio_setting.GetGroupRatioCopy()
	now := common.GetTimestamp()
	userGroups := make(map[int]string)
	identityHistory := make(map[int][]UserSubscription)
	grantFor := func(raw string, userID int, grantedGroup, prevGroup string, subscriptionID int, checkoutAt *int64, plan *SubscriptionPlan, amount int64) (string, error) {
		userGroup, known := userGroups[userID]
		if !known {
			var user User
			if err := tx.Select("id", "group").First(&user, userID).Error; err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return "", err
			}
			userGroup = user.Group
			userGroups[userID] = userGroup
		}
		if userGroup == "" {
			return raw, nil // No current account to preserve.
		}
		grant := decodeLegacyCycleGrant(raw)
		var chain map[int]bool
		if checkoutAt != nil && grant.Checkout != nil {
			userGroup, grantedGroup, prevGroup = grant.Checkout.Group, grant.Checkout.Upgrade, grant.Checkout.Previous
		} else if checkoutAt != nil || grantedGroup != "" {
			history, loaded := identityHistory[userID]
			if !loaded {
				if err := tx.Select("id", "start_time", "end_time", "status", "upgrade_group", "prev_user_group").Where("user_id = ? AND LENGTH(prev_user_group) > 0 AND status IN ?", userID, []string{"active", "expired"}).Order("start_time DESC, id DESC").Find(&history).Error; err != nil {
					return "", err
				}
				identityHistory[userID] = history
			}
			if checkoutAt != nil {
				// Old pending orders have no identity snapshot. Walk back only
				// through recorded transitions that postdate the checkout.
				for _, period := range history {
					if period.Status == "active" && period.EndTime > now && period.StartTime > *checkoutAt && period.UpgradeGroup == userGroup {
						userGroup = period.PrevUserGroup
					}
				}
				var err error
				prevGroup, err = legacyPurchasePreviousGroup(tx, userID, plan.Id, userGroup, grantedGroup, *checkoutAt)
				if err != nil {
					return "", err
				}
				if grantedGroup != "" {
					userGroup = grantedGroup
				}
			} else {
				chain = make(map[int]bool)
				identity := userGroup
				for _, period := range history {
					if period.Status == "active" && period.EndTime > now && period.UpgradeGroup == identity {
						chain[period.Id] = true
						identity = period.PrevUserGroup
					}
				}
			}
		}
		// A previously unbound group may have changed policy. Recompute just
		// this binding's groups; keep unrelated grants for the same period.
		oldCount := len(grant.Access) + len(grant.Quota)
		grant.Access = slices.DeleteFunc(grant.Access, func(group string) bool { return groups[group] })
		grant.Quota = slices.DeleteFunc(grant.Quota, func(group string) bool { return groups[group] })
		changed := oldCount != len(grant.Access)+len(grant.Quota)
		ordinary := OrdinaryUserUsableGroups(userGroup)
		for group := range groups {
			// Mutable conditions are checked when exercising the grant.
			if _, allowed := ordinary[group]; !allowed {
				continue
			}
			if _, configured := ratios[group]; !configured {
				continue
			}
			quota := amount >= 0 && !plan.IsWalletOnlyForGroup(group, false)
			identity := userGroup
			if chain[subscriptionID] {
				identity = grantedGroup
			}
			preservedDirectAccess := checkoutAt == nil && grantedGroup != "" && grantedGroup == identity && grantedGroup == group
			_, upgradeAllows := OrdinaryUserUsableGroups(grantedGroup)[group]
			if !quota && !preservedDirectAccess && !(upgradeAllows && legacyUpgradeGrantsGroup(grantedGroup, prevGroup, identity, group)) {
				continue // An unrelated paid plan did not grant wallet access.
			}
			grant.Access = append(grant.Access, group)
			if quota {
				grant.Quota = append(grant.Quota, group)
			}
			changed = true
		}
		if !changed {
			return raw, nil
		}
		slices.Sort(grant.Access)
		grant.Access = slices.Compact(grant.Access)
		slices.Sort(grant.Quota)
		grant.Quota = slices.Compact(grant.Quota)
		encoded, err := common.Marshal(grant)
		return string(encoded), err
	}
	for _, plan := range plans {
		plan.NormalizeDefaults()
		query := lockForUpdate(tx).Select("id", "user_id", "amount_total", "price_amount", "upgrade_group", "prev_user_group", "legacy_cycle_groups").Where("plan_id = ? AND status = ? AND end_time > ?", plan.Id, "active", now).
			Where("price_amount > 0 OR (price_amount IS NULL AND source IN ?)", []string{"order", PaymentMethodBalance, SubscriptionSourceUpgrade})

		var records []UserSubscription
		if err := query.Order("id").Find(&records).Error; err != nil {
			return err
		}
		paid := make(map[int]map[int]bool)
		for _, sub := range records {
			if sub.PriceAmount == nil {
				if _, loaded := paid[sub.UserId]; !loaded {
					matched, err := matchHistoricalSubscriptionPayments(tx, sub.UserId, plan.Id)
					if err != nil {
						return err
					}
					paid[sub.UserId] = matched
				}
				if !paid[sub.UserId][sub.Id] {
					continue
				}
			}
			encoded, err := grantFor(sub.LegacyCycleGroups, sub.UserId, strings.TrimSpace(sub.UpgradeGroup), sub.PrevUserGroup, sub.Id, nil, &plan, sub.AmountTotal)
			if err != nil {
				return err
			}
			if encoded != sub.LegacyCycleGroups {
				if err := tx.Model(&sub).UpdateColumn("legacy_cycle_groups", encoded).Error; err != nil {
					return err
				}
			}
		}
		orders := lockForUpdate(tx).Select("id", "user_id", "create_time", "legacy_cycle_groups").Where("plan_id = ? AND status = ?", plan.Id, common.TopUpStatusPending)

		var pending []SubscriptionOrder
		if err := orders.Order("id").Find(&pending).Error; err != nil {
			return err
		}
		for _, order := range pending {
			encoded, err := grantFor(order.LegacyCycleGroups, order.UserId, strings.TrimSpace(plan.UpgradeGroup), "", 0, &order.CreateTime, &plan, plan.TotalAmount)
			if err != nil {
				return err
			}
			if encoded != order.LegacyCycleGroups {
				if err := tx.Model(&order).UpdateColumn("legacy_cycle_groups", encoded).Error; err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Historical rows have no order ID. Include free orders and inactive cycles
// so they can consume their own match. Only retain an unpriced cycle if every
// maximum one-to-one matching assigns it a positive payment.
func matchHistoricalSubscriptionPayments(tx *gorm.DB, userID, planID int) (map[int]bool, error) {
	var cycles []UserSubscription
	if err := tx.Select("id", "source", "start_time", "end_time", "created_at", "price_amount", "status").
		Where("user_id = ? AND plan_id = ? AND source IN ?", userID, planID, []string{"order", PaymentMethodBalance, SubscriptionSourceUpgrade}).Order("id").Find(&cycles).Error; err != nil {
		return nil, err
	}
	var orders []SubscriptionOrder
	if err := tx.Select("id", "create_time", "complete_time", "money", "payment_method", "order_type").
		Where("user_id = ? AND plan_id = ? AND status = ? AND complete_time > 0", userID, planID, common.TopUpStatusSuccess).Order("complete_time, id").Find(&orders).Error; err != nil {
		return nil, err
	}
	links := make([][]int, len(cycles))
	for i, cycle := range cycles {
		created := cycle.CreatedAt
		if created <= 0 {
			created = cycle.StartTime
		}
		for j, order := range orders {
			if order.CompleteTime < created || order.CompleteTime > cycle.EndTime {
				continue
			}
			switch cycle.Source {
			case "order":
				if order.PaymentMethod == PaymentMethodBalance || order.CreateTime > created {
					continue
				}
			case PaymentMethodBalance:
				if order.PaymentMethod != PaymentMethodBalance || order.OrderType != SubscriptionOrderTypePurchase {
					continue
				}
			case SubscriptionSourceUpgrade:
				if order.PaymentMethod != PaymentMethodBalance || order.OrderType != SubscriptionOrderTypeUpgrade {
					continue
				}
			}
			if cycle.PriceAmount != nil && cycle.Source != SubscriptionSourceUpgrade && (*cycle.PriceAmount > 0) != (order.Money > 0) {
				continue
			}
			links[i] = append(links[i], j)
		}
	}
	// Start with one maximum matching. For each cycle, remove its positive
	// edges and check whether an alternating path can restore that matching's
	// size. If it can, the positive payment is not attributable to that cycle.
	owner := make([]int, len(orders))
	for j := range owner {
		owner[j] = -1
	}
	var augment func(int, int, []int, []bool) bool
	augment = func(i, omitPaidFor int, matching []int, seen []bool) bool {
		for _, j := range links[i] {
			if seen[j] || (i == omitPaidFor && orders[j].Money > 0) {
				continue
			}
			seen[j] = true
			if matching[j] < 0 || augment(matching[j], omitPaidFor, matching, seen) {
				matching[j] = i
				return true
			}
		}
		return false
	}
	for i := range cycles {
		augment(i, -1, owner, make([]bool, len(orders)))
	}
	matched := make([]bool, len(cycles))
	for _, i := range owner {
		if i >= 0 {
			matched[i] = true
		}
	}
	paid := make(map[int]bool)
	for j, i := range owner {
		if i < 0 || cycles[i].PriceAmount != nil || cycles[i].Status != "active" || orders[j].Money <= 0 {
			continue
		}
		trial := slices.Clone(owner)
		trial[j] = -1
		possible := false
		for k := range cycles {
			if (k == i || !matched[k]) && augment(k, i, trial, make([]bool, len(orders))) {
				possible = true
				break
			}
		}
		paid[cycles[i].Id] = !possible
	}
	return paid, nil
}

func decodeLegacyCycleGrant(raw string) legacyCycleGrant {
	var grant legacyCycleGrant
	if raw == "" || common.UnmarshalJsonStr(raw, &grant) != nil {
		return legacyCycleGrant{}
	}
	return grant
}

func (s UserSubscription) legacyCycleGrant() legacyCycleGrant {
	return decodeLegacyCycleGrant(s.LegacyCycleGroups)
}

func (s UserSubscription) isWalletOnlyForGroup(plan *SubscriptionPlan, group string, scoped bool) bool {
	if scoped && plan.SubscriptionGroup == "" && slices.Contains(s.legacyCycleGrant().Quota, group) {
		return false
	}
	return plan.IsWalletOnlyForGroup(group, scoped)
}

// Storage records the original entitlement; live rules still govern its use.
func newLegacyCycleAccessCheck(db *gorm.DB, userID int, group string) func() (bool, error) {
	return sync.OnceValues(func() (bool, error) {
		var user User
		if err := db.Select("group").First(&user, userID).Error; err != nil {
			return false, err
		}
		return NewOrdinaryGroupAccessCheck(db)(userID, user.Group, group)
	})
}

func checkNewLegacySubscriptionPurchaseTx(tx *gorm.DB, plan *SubscriptionPlan, userID int) error {
	if plan.SubscriptionGroup != "" {
		return nil
	}
	identity, err := legacyPurchaseIdentity(tx, plan, userID)
	if err != nil {
		return err
	}
	return checkNewLegacySubscriptionPeriodTx(tx, plan, identity.Group, identity.Upgrade, identity.Previous)
}

func (o *SubscriptionOrder) prepareLegacyCheckoutTx(tx *gorm.DB, plan *SubscriptionPlan) error {
	o.LegacyCycleGroups = "{}"
	if plan.SubscriptionGroup != "" {
		return nil
	}
	identity, err := legacyPurchaseIdentity(tx, plan, o.UserId)
	if err != nil {
		return err
	}
	if err := checkNewLegacySubscriptionPeriodTx(tx, plan, identity.Group, identity.Upgrade, identity.Previous); err != nil {
		return err
	}
	raw, err := common.Marshal(legacyCycleGrant{Checkout: &identity})
	if err == nil {
		o.LegacyCycleGroups = string(raw)
	}
	return err
}

func legacyPurchaseIdentity(tx *gorm.DB, plan *SubscriptionPlan, userID int) (legacyCheckoutIdentity, error) {
	var user User
	if err := tx.Select("group").First(&user, userID).Error; err != nil {
		return legacyCheckoutIdentity{}, err
	}
	upgradeGroup := strings.TrimSpace(plan.UpgradeGroup)
	group := user.Group
	if upgradeGroup != "" {
		group = upgradeGroup
	}
	previous, err := legacyPurchasePreviousGroup(tx, userID, plan.Id, user.Group, upgradeGroup, common.GetTimestamp())
	return legacyCheckoutIdentity{Group: group, Upgrade: upgradeGroup, Previous: previous}, err
}

func legacyPurchasePreviousGroup(db *gorm.DB, userID, planID int, currentGroup, upgradeGroup string, at int64) (string, error) {
	if upgradeGroup == "" || currentGroup != upgradeGroup {
		return currentGroup, nil
	}
	var current UserSubscription
	err := db.Select("prev_user_group").Where("user_id = ? AND plan_id = ? AND upgrade_group = ? AND status IN ? AND start_time <= ? AND end_time > ? AND LENGTH(prev_user_group) > 0",
		userID, planID, upgradeGroup, []string{"active", "expired"}, at, at).
		Order("end_time DESC, id DESC").Take(&current).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return currentGroup, nil
	}
	return current.PrevUserGroup, err
}

func legacyUpgradeGrantsGroup(upgrade, previous, current, group string) bool {
	if upgrade == "" || upgrade != current || previous == "" || previous == upgrade {
		return false
	}
	if upgrade == group {
		return true
	}

	_, alreadyAvailable := OrdinaryUserUsableGroups(previous)[group]
	return !alreadyAvailable
}

// Purchase may change the user's identity group. Renewal uses the user's
// existing identity. Neither may charge for quota lost to a plan binding.
func checkNewLegacySubscriptionPeriodTx(tx *gorm.DB, plan *SubscriptionPlan, userGroup, upgradeGroup, previousGroup string) error {
	if plan.SubscriptionGroup != "" {
		return nil
	}
	bound, err := subscriptionGroupNamesTx(tx)
	if err != nil || len(bound) == 0 {
		return err
	}
	lostBenefit := false
	for group := range OrdinaryUserUsableGroups(userGroup) {
		if group == "" || group == "auto" || !ratio_setting.ContainsGroupRatio(group) {
			continue
		}
		quota := plan.TotalAmount >= 0 && !plan.IsWalletOnlyForGroup(group, false)
		if !quota && !legacyUpgradeGrantsGroup(upgradeGroup, previousGroup, userGroup, group) {
			continue
		}

		if !slices.Contains(bound, group) {
			return nil
		}
		lostBenefit = true
	}
	if lostBenefit {
		return errLegacySubscriptionGroupsUnavailable
	}
	return nil
}

// Retained cycles preserve access, subject to the current ordinary group rules.
func GetRetainedLegacySubscriptionGroups(userID int, previouslyUsable map[string]string) ([]string, error) {
	if userID <= 0 || len(previouslyUsable) == 0 {
		return nil, nil
	}
	var rows []UserSubscription
	if err := DB.Select("legacy_cycle_groups").Where("user_id = ? AND status = ? AND end_time > ? AND legacy_cycle_groups IS NOT NULL AND legacy_cycle_groups <> ?", userID, "active", common.GetTimestamp(), "{}").Find(&rows).Error; err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var groups []string
	for _, row := range rows {
		for _, group := range row.legacyCycleGrant().Access {
			if _, allowed := previouslyUsable[group]; allowed && !seen[group] {
				groups = append(groups, group)
				seen[group] = true
			}
		}
	}
	return groups, nil
}
