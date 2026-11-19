package model

import (
	"database/sql"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestSubscriptionPlanWalletOnlyGroupPolicy(t *testing.T) {
	tests := []struct {
		name  string
		plan  SubscriptionPlan
		group string
		want  bool
	}{
		{name: "disabled", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: false, WalletOnlyGroups: "vip"}, group: "vip", want: false},
		{name: "blacklist match", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "blacklist", WalletOnlyGroups: "vip,team"}, group: "vip", want: true},
		{name: "blacklist miss", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "blacklist", WalletOnlyGroups: "vip,team"}, group: "default", want: false},
		{name: "whitelist match", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip,team"}, group: "team", want: false},
		{name: "whitelist miss", plan: SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip,team"}, group: "default", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.plan.IsWalletOnlyForGroup(tt.group))
		})
	}
}

func TestSubscriptionPlanBoundGroupDoesNotChangeLegacyPlans(t *testing.T) {
	require.NoError(t, ratio_setting.UpdateSubscriptionGroups(`{"subscriber":{}}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateSubscriptionGroups(`{}`)) })
	var scoped SubscriptionPlan
	require.NoError(t, common.UnmarshalJsonStr(`{"subscription_group":"subscriber","wallet_only_groups_enabled":true,"wallet_only_groups_mode":"blacklist","wallet_only_groups":"subscriber"}`, &scoped))
	legacy := SubscriptionPlan{WalletOnlyGroupsEnabled: true, WalletOnlyGroups: "blocked"}

	assert.False(t, scoped.IsWalletOnlyForGroup("subscriber"), "explicit binding takes precedence over legacy wallet-group rules")
	assert.True(t, scoped.IsWalletOnlyForGroup("default"), "a bound plan must not pay for an ordinary group")
	assert.False(t, legacy.IsWalletOnlyForGroup("default"), "an unbound plan still covers its legacy groups")
	assert.True(t, legacy.IsWalletOnlyForGroup("blocked"), "an unbound plan still respects its old exclusions")
}

func TestSubscriptionPurchaseCapacityDatabaseMatrix(t *testing.T) {
	for _, engine := range []string{"sqlite", "mysql", "postgres"} {
		t.Run(engine, func(t *testing.T) {
			var driver gorm.Dialector
			switch engine {
			case "sqlite":
				driver = sqlite.Open(":memory:")
			case "mysql":
				dsn := os.Getenv("TEST_MYSQL_DSN")
				if dsn == "" {
					t.Skip("TEST_MYSQL_DSN not configured")
				}
				driver = mysql.Open(dsn)
			case "postgres":
				dsn := os.Getenv("TEST_POSTGRES_DSN")
				if dsn == "" {
					t.Skip("TEST_POSTGRES_DSN not configured")
				}
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			prefix := fmt.Sprintf("subcap_%d_", time.Now().UnixNano())
			if engine == "sqlite" {
				prefix = ""
			}
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
			require.NoError(t, err)
			var version string
			versionQuery := "SELECT version()"
			if engine == "sqlite" {
				versionQuery = "SELECT sqlite_version()"
			}
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("Database version: %s", version)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			if engine == "sqlite" {
				sqlDB.SetMaxOpenConns(1)
			}
			oldDB, oldLogDB, oldType := DB, LOG_DB, common.MainDatabaseType()
			DB, LOG_DB = db, db
			common.SetMainDatabaseType(common.DatabaseType(engine))
			initCol()
			t.Cleanup(func() {
				DB, LOG_DB = oldDB, oldLogDB
				common.SetMainDatabaseType(oldType)
				initCol()
				require.NoError(t, db.Migrator().DropTable(&SubscriptionOrder{}, &UserSubscription{}, &SubscriptionPlan{}, &User{}, &SystemTask{}))
				require.NoError(t, sqlDB.Close())
			})
			for range 2 {
				if engine == "sqlite" {
					require.NoError(t, ensureSubscriptionPlanTableSQLite())
				}
				require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}, &User{}, &UserSubscription{}, &SubscriptionOrder{}, &SystemTask{}))
			}
			// Recreate the released plan schema: its simultaneous purchase limit
			// already existed, but its subscription group binding did not.
			require.NoError(t, db.Migrator().DropColumn(&SubscriptionPlan{}, "subscription_group"))
			legacy := &SubscriptionPlan{Title: "Retained fork plan", PriceAmount: 12, MaxPurchasePerUser: 9, MaxActivePerUser: 2, WaffoPancakeProductId: "legacy-product", WeeklyLimit: 1234, DurationUnit: SubscriptionDurationMonth, DurationValue: 1, Enabled: true, UpgradeGroup: "vip", DowngradeGroup: "default", WalletOnlyGroupsEnabled: true, WalletOnlyGroups: "team"}
			require.NoError(t, db.Omit("SubscriptionGroup").Create(legacy).Error)
			for range 2 {
				if engine == "sqlite" {
					require.NoError(t, ensureSubscriptionPlanTableSQLite())
				}
				require.NoError(t, db.AutoMigrate(&SubscriptionPlan{}))
			}
			var plan SubscriptionPlan
			require.NoError(t, db.First(&plan, legacy.Id).Error)
			assert.Equal(t, legacy.Title, plan.Title)
			assert.Equal(t, legacy.WaffoPancakeProductId, plan.WaffoPancakeProductId)
			assert.Equal(t, legacy.WeeklyLimit, plan.WeeklyLimit)
			assert.Equal(t, 9, plan.MaxPurchasePerUser)
			assert.Equal(t, legacy.MaxActivePerUser, plan.MaxActivePerUser)
			assert.Empty(t, plan.SubscriptionGroup)
			assert.Equal(t, legacy.UpgradeGroup, plan.UpgradeGroup)
			assert.Equal(t, legacy.DowngradeGroup, plan.DowngradeGroup)
			assert.True(t, plan.IsWalletOnlyForGroup("team"))
			plan.MaxActivePerUser = 1
			require.NoError(t, db.Model(&plan).Update("max_active_per_user", 1).Error)
			InvalidateSubscriptionPlanCache(plan.Id)
			user := &User{Username: "capacity-test", Group: "default", Quota: 99999999}
			require.NoError(t, db.Create(user).Error)
			now := getDBTimestamp(db)
			require.NoError(t, db.Create(&UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 100, EndTime: now - 1}).Error)
			require.NoError(t, db.Create(&UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "cancelled", StartTime: now - 100, EndTime: now + 3600}).Error)
			// Concurrent grants serialize on the same user row.
			var wg sync.WaitGroup
			results := make(chan error, 2)
			for range 2 {
				wg.Go(func() {
					results <- db.Transaction(func(tx *gorm.DB) error {
						_, err := CreateUserSubscriptionFromPlanTx(tx, user.Id, &plan, "test")
						return err
					})
				})
			}
			wg.Wait()
			close(results)
			successes := 0
			for err := range results {
				if err == nil {
					successes++
				}
			}
			assert.Equal(t, 1, successes)
			require.Error(t, CheckSubscriptionPurchaseCapacity(db, user.Id, &plan))
			require.Error(t, PurchaseSubscriptionWithBalance(user.Id, plan.Id, 1, false))
			var unchangedUser User
			require.NoError(t, db.First(&unchangedUser, user.Id).Error)
			assert.Equal(t, user.Quota, unchangedUser.Quota)
			require.Error(t, (&SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, TradeNo: "blocked-active", Status: common.TopUpStatusPending}).Insert())
			require.NoError(t, db.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Update("end_time", now-1).Error)
			require.NoError(t, CheckSubscriptionPurchaseCapacity(db, user.Id, &plan))
			// Editing back to zero removes the simultaneous limit.
			plan.MaxActivePerUser = 0
			require.NoError(t, db.Model(&plan).Update("max_active_per_user", 0).Error)
			require.NoError(t, CheckSubscriptionPurchaseCapacity(db, user.Id, &plan))
			for range 2 {
				require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
					_, err := CreateUserSubscriptionFromPlanTx(tx, user.Id, &plan, "test")
					return err
				}))
			}
			plan.MaxPurchasePerUser = 1
			require.Error(t, CheckSubscriptionPurchaseCapacity(db, user.Id, &plan))
		})
	}
}

func TestSubscriptionGroupBindingSurvivesStaleSnapshot(t *testing.T) {
	for _, engine := range []struct {
		dialect common.DatabaseType
		dsnEnv  string
	}{
		{common.DatabaseTypeMySQL, "TEST_MYSQL_DSN"},
		{common.DatabaseTypePostgreSQL, "TEST_POSTGRES_DSN"},
	} {
		t.Run(string(engine.dialect), func(t *testing.T) {
			dsn := os.Getenv(engine.dsnEnv)
			if dsn == "" {
				t.Skip(engine.dsnEnv + " not configured")
			}
			var driver gorm.Dialector = mysql.Open(dsn)
			if engine.dialect == common.DatabaseTypePostgreSQL {
				driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
			}
			prefix := fmt.Sprintf("subbind_%d_", time.Now().UnixNano())
			db, err := gorm.Open(driver, &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
			require.NoError(t, err)
			conn, err := db.DB()
			require.NoError(t, err)
			conn.SetMaxOpenConns(2)
			oldMain, oldLog := common.MainDatabaseType(), common.LogDatabaseType()
			common.SetDatabaseTypes(engine.dialect, oldLog)
			initCol()
			tables := []any{&Option{}, &SubscriptionPlan{}, &UserSubscription{}}
			t.Cleanup(func() {
				require.NoError(t, db.Migrator().DropTable(tables...))
				require.NoError(t, conn.Close())
				common.SetDatabaseTypes(oldMain, oldLog)
				initCol()
			})
			require.NoError(t, db.AutoMigrate(tables...))
			require.NoError(t, db.Create(&Option{Key: ratio_setting.SubscriptionGroupsOptionKey, Value: `{"vip":{}}`}).Error)
			plans := []SubscriptionPlan{{Title: "First", SubscriptionGroup: "vip"}, {Title: "Last", SubscriptionGroup: "vip"}}
			require.NoError(t, db.Create(&plans).Error)
			target := ""
			unbind := func(tx *gorm.DB, id int) error {
				if err := CheckSubscriptionPlanGroupChangeTx(tx, id, &target); err != nil {
					return err
				}
				return tx.Model(&SubscriptionPlan{}).Where("id = ?", id).Update("subscription_group", target).Error
			}
			err = db.Transaction(func(stale *gorm.DB) error {
				var snapshotCount int64
				require.NoError(t, stale.Model(&SubscriptionPlan{}).Where("subscription_group = ?", "vip").Count(&snapshotCount).Error)
				require.EqualValues(t, 2, snapshotCount)
				require.NoError(t, db.Transaction(func(fresh *gorm.DB) error { return unbind(fresh, plans[0].Id) }))
				return unbind(stale, plans[1].Id)
			}, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
			require.Error(t, err, "a stale snapshot must not allow the last unbind to commit")
			var count int64
			require.NoError(t, db.Model(&SubscriptionPlan{}).Where("subscription_group = ?", "vip").Count(&count).Error)
			assert.EqualValues(t, 1, count)
		})
	}
}

func TestDeleteSubscriptionPlanRequiresNoActiveSubscriptions(t *testing.T) {
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&Option{}))
	plan := &SubscriptionPlan{
		Id:            9901,
		Title:         "Deletable",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
	}
	require.NoError(t, DB.Create(plan).Error)
	now := common.GetTimestamp()
	sub := &UserSubscription{
		Id:        9902,
		UserId:    9903,
		PlanId:    plan.Id,
		Status:    "active",
		StartTime: now - 60,
		EndTime:   now + 3600,
	}
	require.NoError(t, DB.Create(sub).Error)

	require.Error(t, DeleteSubscriptionPlan(plan.Id))
	require.NoError(t, DB.Model(sub).Update("end_time", now-1).Error)
	require.NoError(t, DeleteSubscriptionPlan(plan.Id))

	var deleted SubscriptionPlan
	err := DB.First(&deleted, plan.Id).Error
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}
