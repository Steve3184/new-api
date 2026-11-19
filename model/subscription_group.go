package model

import (
	"slices"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// validateSubscriptionGroupsTx requires an existing plan binding independent of
// its price tiers. Plans can be bound before the group is enabled.
func validateSubscriptionGroupsTx(tx *gorm.DB, value string) error {
	groups, err := ratio_setting.ParseSubscriptionGroups(value)
	if err != nil || len(groups) == 0 {
		return err
	}
	names := make([]string, 0, len(groups))
	for group := range groups {
		if !ratio_setting.ContainsGroupRatio(group) {
			return common.NewMessage("Subscription group does not exist: {{group}}", map[string]any{"group": group})
		}
		names = append(names, group)
	}
	var bound []string
	if err := lockForUpdate(tx).Model(&SubscriptionPlan{}).Where("subscription_group IN ?", names).Pluck("subscription_group", &bound).Error; err != nil {
		return err
	}
	for _, group := range bound {
		delete(groups, group)
	}
	for group := range groups {
		return common.NewMessage("Bind at least one subscription plan to group {{group}}", map[string]any{"group": group})
	}
	return nil
}

func lockAndValidateSubscriptionGroupOptionTx(tx *gorm.DB, values map[string]string) error {
	value, ok := values[ratio_setting.SubscriptionGroupsOptionKey]
	if !ok {
		return nil
	}
	if _, err := lockSubscriptionGroupOptionTx(tx); err != nil {
		return err
	}
	return validateSubscriptionGroupsTx(tx, value)
}

// Take the same database lock for configuration edits and binding removal so
// two concurrent admin changes cannot leave an enabled group without a plan.
func lockSubscriptionGroupOptionTx(tx *gorm.DB) (Option, error) {
	option := Option{Key: ratio_setting.SubscriptionGroupsOptionKey, Value: "{}"}
	if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&option).Error; err != nil {
		return option, err
	}
	if err := lockForUpdate(tx).Where(&Option{Key: option.Key}).First(&option).Error; err != nil {
		return option, err
	}
	// PostgreSQL's repeatable-read snapshot is not refreshed by a lock.
	// Create a new row version so a concurrent writer with an older snapshot
	// must abort rather than validate bindings against stale plan data.
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return option, tx.Model(&option).UpdateColumn("value", option.Value).Error
	}
	return option, nil
}

// CheckSubscriptionPlanGroupChangeTx must run in the transaction that updates
// or deletes the plan. A nil target represents deletion.
func CheckSubscriptionPlanGroupChangeTx(tx *gorm.DB, planID int, target *string) error {
	option, err := lockSubscriptionGroupOptionTx(tx)
	if err != nil {
		return err
	}
	var plan SubscriptionPlan
	if err := lockForUpdate(tx).Select("id", "subscription_group").First(&plan, planID).Error; err != nil {
		return err
	}
	group := plan.SubscriptionGroup
	if group == "" || (target != nil && *target == group) {
		return nil
	}
	groups, err := ratio_setting.ParseSubscriptionGroups(option.Value)
	if err != nil {
		return err
	}
	if _, enabled := groups[group]; !enabled {
		return nil
	}
	var active []int
	if err := lockForUpdate(tx).Model(&UserSubscription{}).Where("plan_id = ? AND status = ? AND end_time > ?", planID, "active", common.GetTimestamp()).Limit(1).Pluck("id", &active).Error; err != nil {
		return err
	}
	if len(active) > 0 {
		return common.NewMessage("Cannot change the group of a plan with active subscriptions")
	}
	var alternatives []string
	// MySQL locking reads use current rows even under repeatable read.
	if err := lockForUpdate(tx).Model(&SubscriptionPlan{}).Where("id <> ? AND subscription_group = ?", planID, group).Pluck("subscription_group", &alternatives).Error; err != nil {
		return err
	}
	if !slices.Contains(alternatives, group) {
		return common.NewMessage("Bind at least one subscription plan to group {{group}}", map[string]any{"group": group})
	}
	return nil
}

// GetActiveUserSubscriptionGroups intentionally excludes legacy plans, even
// when a legacy plan changed the user's ordinary group to the same name.
func GetActiveUserSubscriptionGroups(userID int, groups []string) ([]string, error) {
	if userID <= 0 || len(groups) == 0 {
		return nil, nil
	}
	activePlanIDs := DB.Model(&UserSubscription{}).Select("plan_id").
		Where("user_id = ? AND status = ? AND end_time > ?", userID, "active", common.GetTimestamp())
	var result []string
	err := DB.Model(&SubscriptionPlan{}).
		Where("subscription_group IN ? AND id IN (?)", groups, activePlanIDs).
		Pluck("subscription_group", &result).Error
	if err != nil {
		return nil, err
	}
	allowed := make(map[string]bool, len(groups))
	for _, group := range groups {
		allowed[group] = true
	}
	// SQL may match case/accent variants under the database collation.
	// Authorize only exact names from this request's enabled-group snapshot.
	result = slices.DeleteFunc(result, func(group string) bool {
		if !allowed[group] {
			return true
		}
		delete(allowed, group)
		return false
	})
	slices.Sort(result)
	return result, nil
}

func ensureSubscriptionGroupKeyTx(tx *gorm.DB, userID int, plan *SubscriptionPlan) error {
	group := plan.SubscriptionGroup
	config, scoped := ratio_setting.GetSubscriptionGroup(group)
	if !scoped || !config.AutoCreateKey {
		return nil
	}
	var user User
	if err := lockForUpdate(tx).Select("id", "group", "setting").First(&user, userID).Error; err != nil {
		return err
	}
	var tokens []Token
	if err := tx.Select("id", "group", "auto_groups", "status", "expired_time", "unlimited_quota", "remain_quota").Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return err
	}
	now := common.GetTimestamp()
	for _, token := range tokens {
		if token.Status != common.TokenStatusEnabled || (token.ExpiredTime != -1 && token.ExpiredTime < now) || (!token.UnlimitedQuota && token.RemainQuota <= 0) {
			continue
		}
		if token.Group == group || (token.Group == "" && user.Group == group) {
			return nil
		}
		if token.Group == "auto" {
			groups, err := token.GetAutoGroups()
			if err != nil {
				continue
			}
			if len(groups) == 0 {
				groups = setting.GetAutoGroups()
			}
			if slices.Contains(groups, group) {
				return nil
			}
		}
	}
	// Never make a completed payment unusable because the user hit the key cap.
	if len(tokens) >= operation_setting.GetMaxUserTokens() {
		return nil
	}
	key, err := common.GenerateKey()
	if err != nil {
		return err
	}
	name := i18n.Translate(user.GetSetting().Language, i18n.MsgSubscriptionTokenName, map[string]any{"Name": plan.Title})
	for len(name) > 50 {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return tx.Create(&Token{UserId: userID, Name: name, Key: key, Group: group,
		Status: common.TokenStatusEnabled, UnlimitedQuota: true, ExpiredTime: -1,
		CreatedTime: now, AccessedTime: now}).Error
}
