package model

import (
	"database/sql"
	"errors"
	"slices"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"gorm.io/gorm"
)

const retiredSubscriptionGroupsOptionKey = "SubscriptionGroups"
const subscriptionBindingMigrationName = "derive_groups_from_plan_bindings_v1"

// Internal database migration state, not a group setting or API option.
type SubscriptionMigration struct {
	Name string `gorm:"primaryKey;size:96"`
}

var errSubscriptionBindingMigrationPending = errors.New("subscription group migration is pending; bring up the upgraded master before this node")

// Compare byte length rather than collation equality. MySQL can collate
// nonempty names as empty, while SQLite's text length stops at a NUL byte.
func subscriptionGroupByteLength() string {
	if common.UsingMainDatabase(common.DatabaseTypeSQLite) {
		return "LENGTH(CAST(subscription_group AS BLOB))"
	}
	return "OCTET_LENGTH(subscription_group)"
}

// Move the old group-level auto-key setting to each exactly matching plan.
// Retire the old option and record completion in the same transaction.
func MigrateSubscriptionGroupAutoCreateKeys() error {
	return migrateSubscriptionGroupAutoCreateKeys(DB)
}

// Non-master nodes may not run schema migrations. They must not accept
// checkouts or callbacks until the master has committed the entitlement copy.
func RequireSubscriptionBindingMigration() error {
	return requireSubscriptionBindingMigration(DB)
}

func requireSubscriptionBindingMigration(db *gorm.DB) error {
	if err := db.Where(&SubscriptionMigration{Name: subscriptionBindingMigrationName}).Take(&SubscriptionMigration{}).Error; errors.Is(err, gorm.ErrRecordNotFound) {
		return errSubscriptionBindingMigrationPending
	} else {
		return err
	}
}

func migrateSubscriptionGroupAutoCreateKeys(db *gorm.DB) error {
	return subscriptionPlanTransaction(db, func(tx *gorm.DB) error {
		var option Option
		err := lockForUpdate(tx).Where(&Option{Key: retiredSubscriptionGroupsOptionKey}).First(&option).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		if err := requireSubscriptionBindingMigration(tx); !errors.Is(err, errSubscriptionBindingMigrationPending) {
			return err
		}
		var old map[string]struct {
			AutoCreateKey bool `json:"auto_create_key"`
		}
		if option.Value != "" {
			if err := common.UnmarshalJsonStr(option.Value, &old); err != nil {
				return err
			}
		}
		var plans []SubscriptionPlan
		if err := tx.Select("id", "subscription_group").Where(subscriptionGroupByteLength() + " > 0").Find(&plans).Error; err != nil {
			return err
		}
		var enableKeys, disableKeys []int
		newGroups := make(map[string]bool)
		for _, plan := range plans {
			group := plan.SubscriptionGroup
			if group == "" || group == "auto" {
				continue
			}
			config, wasScoped := old[group]
			if !wasScoped {
				newGroups[group] = true
			}
			if config.AutoCreateKey {
				enableKeys = append(enableKeys, plan.Id)
			} else {
				disableKeys = append(disableKeys, plan.Id)
			}
		}
		for _, batch := range []struct {
			ids     []int
			enabled bool
		}{{enableKeys, true}, {disableKeys, false}} {
			for start := 0; start < len(batch.ids); start += 500 {
				if err := tx.Model(&SubscriptionPlan{}).Where("id IN ?", batch.ids[start:min(start+500, len(batch.ids))]).UpdateColumn("auto_create_key", batch.enabled).Error; err != nil {
					return err
				}
			}
		}
		if err := snapshotLegacySubscriptionGroupsTx(tx, newGroups); err != nil {
			return err
		}
		if err := tx.Model(&UserSubscription{}).Where("legacy_cycle_groups IS NULL").UpdateColumn("legacy_cycle_groups", "{}").Error; err != nil {
			return err
		}
		if err := tx.Model(&SubscriptionOrder{}).Where("legacy_cycle_groups IS NULL AND status = ?", common.TopUpStatusPending).UpdateColumn("legacy_cycle_groups", "{}").Error; err != nil {
			return err
		}
		if option.Key != "" {
			if err := tx.Delete(&option).Error; err != nil {
				return err
			}
		}
		return tx.Create(&SubscriptionMigration{Name: subscriptionBindingMigrationName}).Error
	})
}

// SubscriptionGroupNames derives membership from saved bindings, including
// disabled plans: unlisting a plan does not revoke existing entitlements.
func SubscriptionGroupNames() ([]string, error) {
	return subscriptionGroupNamesTx(DB)
}

func subscriptionGroupNamesTx(db *gorm.DB) ([]string, error) {
	if db == nil {
		return nil, errors.New("subscription plan database is unavailable")
	}
	var bindings []string
	if err := db.Model(&SubscriptionPlan{}).Where(subscriptionGroupByteLength()+" > 0").Pluck("subscription_group", &bindings).Error; err != nil {
		return nil, err
	}
	bindings = slices.DeleteFunc(bindings, func(group string) bool { return group == "" || group == "auto" })
	slices.Sort(bindings)
	return slices.Compact(bindings), nil
}

func IsSubscriptionGroup(group string) (bool, error) {
	return isSubscriptionGroupTx(DB, group)
}

func isSubscriptionGroupTx(tx *gorm.DB, group string) (bool, error) {
	if group == "" || group == "auto" {
		return false, nil
	}
	if tx == nil {
		return false, errors.New("subscription plan database is unavailable")
	}
	var bindings []string
	if err := tx.Model(&SubscriptionPlan{}).Where("subscription_group = ?", group).Pluck("subscription_group", &bindings).Error; err != nil {
		return false, err
	}
	// SQL can match case or accent variants; only the exact name is bound.
	return slices.Contains(bindings, group), nil
}

// syncSubscriptionPlanEntitlementTx serializes a new order, grant, or renewal
// with structural changes to the plan. PostgreSQL needs a new row version,
// not just a lock: waiting repeatable-read transactions must not validate
// against a snapshot that predates a newly committed order or grant.
func syncSubscriptionPlanEntitlementTx(tx *gorm.DB, planID int) (*SubscriptionPlan, error) {
	var plan SubscriptionPlan
	if err := lockForUpdate(tx).First(&plan, planID).Error; err != nil {
		return nil, err
	}
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		if err := tx.Model(&plan).UpdateColumn("subscription_group", gorm.Expr("subscription_group")).Error; err != nil {
			return nil, err
		}
	}
	plan.NormalizeDefaults()
	return &plan, nil
}

// Structural edits lock the same ordered set before examining bindings or
// entitlement rows. Buyers only lock their own plan, then their order/cycle.
func lockSubscriptionBindingSetTx(tx *gorm.DB) error {
	if err := lockSubscriptionCatalogRevisionTx(tx); err != nil {
		return err
	}
	var ids []int
	return lockForUpdate(tx).Model(&SubscriptionPlan{}).Order("id").Pluck("id", &ids).Error
}

// Lock the catalog separately from its rows: READ COMMITTED does not prevent
// an insert just because the transaction locked all existing plans.
func lockSubscriptionCatalogRevisionTx(tx *gorm.DB) error {
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return requireSubscriptionBindingMigration(lockForUpdate(tx))
	}
	if !common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return nil
	}
	// Write a row version so older repeatable-read snapshots abort.
	result := tx.Model(&SubscriptionMigration{}).Where("name = ?", subscriptionBindingMigrationName).
		UpdateColumn("name", subscriptionBindingMigrationName)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errSubscriptionBindingMigrationPending
	}
	return nil
}

// Binding locks plans before reading user identities. On MySQL, committed
// reads keep those identities current without reversing checkout's lock order.
func subscriptionPlanTransaction(db *gorm.DB, fn func(*gorm.DB) error) error {
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return db.Transaction(fn, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	}
	return db.Transaction(fn)
}

func UpdateSubscriptionPlanTransaction(fn func(*gorm.DB) error) error {
	return subscriptionPlanTransaction(DB, fn)
}

// CheckSubscriptionPlanGroupChangeTx returns the locked plan's current group.
// A nil target preserves it; a non-nil target may explicitly unbind it.
func CheckSubscriptionPlanGroupChangeTx(tx *gorm.DB, planID int, target *string) (string, error) {
	if target != nil && *target != "" {
		var current SubscriptionPlan
		if err := tx.Select("id", "subscription_group").First(&current, planID).Error; err != nil {
			return "", err
		}
		if current.SubscriptionGroup == *target {
			// A normal edit of an already-bound plan only needs its own lock.
			if err := lockForUpdate(tx).Select("id", "subscription_group").First(&current, planID).Error; err != nil {
				return "", err
			}
			if current.SubscriptionGroup != *target {
				return "", common.NewMessage("The subscription plan changed; reload and try again")
			}
			return current.SubscriptionGroup, nil
		}
		if err := lockSubscriptionBindingSetTx(tx); err != nil {
			return "", err
		}
	}
	var plan SubscriptionPlan
	if err := lockForUpdate(tx).Select("id", "subscription_group").First(&plan, planID).Error; err != nil {
		return "", err
	}
	if target != nil && *target != plan.SubscriptionGroup {
		if err := checkSubscriptionPlanRetainEntitlementsTx(tx, planID); err != nil {
			return "", err
		}
		if err := retainLegacyAccessBeforeBindingTx(tx, *target); err != nil {
			return "", err
		}
	}
	return plan.SubscriptionGroup, nil
}

func checkSubscriptionPlanRetainEntitlementsTx(tx *gorm.DB, planID int) error {
	// MySQL locking reads see current records even if the caller already
	// established an older repeatable-read snapshot. Checkout and settlement
	// acquire the plan lock before locking or creating the protected rows.
	read := func() *gorm.DB {
		if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
			return lockForUpdate(tx)
		}
		return tx
	}
	var active []int
	if err := read().Model(&UserSubscription{}).Where("plan_id = ? AND status = ? AND end_time > ?", planID, "active", common.GetTimestamp()).Limit(1).Pluck("id", &active).Error; err != nil {
		return err
	}
	if len(active) > 0 {
		return common.NewMessage("Cannot change or delete a subscription plan with active subscriptions")
	}
	var pending []int
	if err := read().Model(&SubscriptionOrder{}).Where("plan_id = ? AND status = ?", planID, common.TopUpStatusPending).Limit(1).Pluck("id", &pending).Error; err != nil {
		return err
	}
	if len(pending) > 0 {
		return common.NewMessage("Cannot change or delete a subscription plan with pending orders")
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
	if group == "" || group == "auto" || !plan.AutoCreateKey {
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
