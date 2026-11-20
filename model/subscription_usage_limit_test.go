package model

import (
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func prepareSubscriptionUsageLimitTest(t *testing.T) {
	t.Helper()
	truncateTables(t)
	require.NoError(t, DB.AutoMigrate(&SubscriptionPreConsumeRecord{}))
	require.NoError(t, DB.Exec("DELETE FROM subscription_pre_consume_records").Error)
}

func TestSubscriptionPlanNormalizeDefaultsClearsBenefitsOnlyUsageLimits(t *testing.T) {
	plan := SubscriptionPlan{
		TotalAmount:   -42,
		FiveHourLimit: 100,
		WeeklyLimit:   200,
		MonthlyLimit:  300,
	}

	plan.NormalizeDefaults()

	require.EqualValues(t, -1, plan.TotalAmount)
	require.Zero(t, plan.FiveHourLimit)
	require.Zero(t, plan.WeeklyLimit)
	require.Zero(t, plan.MonthlyLimit)
}

func createSubscriptionUsageLimitFixture(t *testing.T, plan *SubscriptionPlan) (*User, *UserSubscription) {
	t.Helper()
	user := &User{
		Username: "subscription-usage-limit-" + common.GetRandomString(8),
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
	}
	require.NoError(t, DB.Create(user).Error)
	require.NoError(t, DB.Create(plan).Error)
	InvalidateSubscriptionPlanCache(plan.Id)

	var subscription *UserSubscription
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		subscription, err = CreateUserSubscriptionFromPlanTx(tx, user.Id, plan, "test")
		return err
	}))
	return user, subscription
}

func TestSubscriptionUsageLimitsAreEnforcedAtomically(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{
		Title:         "Usage caps",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		FiveHourLimit: 100,
		WeeklyLimit:   150,
		MonthlyLimit:  200,
	}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)

	_, err := PreConsumeUserSubscription("subscription-usage-first", user.Id, "test", 0, 60)
	require.NoError(t, err)

	var afterFirst UserSubscription
	require.NoError(t, DB.First(&afterFirst, subscription.Id).Error)
	assert.EqualValues(t, 60, afterFirst.AmountUsed)
	assert.EqualValues(t, 60, afterFirst.FiveHourUsed)
	assert.EqualValues(t, 60, afterFirst.WeeklyUsed)
	assert.EqualValues(t, 60, afterFirst.MonthlyUsed)

	_, err = PreConsumeUserSubscription("subscription-usage-over-five-hours", user.Id, "test", 0, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription quota insufficient")

	var afterRejected UserSubscription
	require.NoError(t, DB.First(&afterRejected, subscription.Id).Error)
	assert.EqualValues(t, 60, afterRejected.AmountUsed)
	assert.EqualValues(t, 60, afterRejected.FiveHourUsed)
	assert.EqualValues(t, 60, afterRejected.WeeklyUsed)
	assert.EqualValues(t, 60, afterRejected.MonthlyUsed)

	require.NoError(t, PostConsumeUserSubscriptionDelta(subscription.Id, 40))
	assert.Error(t, PostConsumeUserSubscriptionDelta(subscription.Id, 1))

	var afterSettlement UserSubscription
	require.NoError(t, DB.First(&afterSettlement, subscription.Id).Error)
	assert.EqualValues(t, 100, afterSettlement.AmountUsed)
	assert.EqualValues(t, 100, afterSettlement.FiveHourUsed)
	assert.EqualValues(t, 100, afterSettlement.WeeklyUsed)
	assert.EqualValues(t, 100, afterSettlement.MonthlyUsed)
}

func TestFiveHourWindowStartsAtFirstConsumption(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{
		Title:         "First-use 5-hour window",
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		FiveHourLimit: 100,
	}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)
	firstUseBase := GetDBTimestamp() - 3600
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).Updates(map[string]interface{}{
		"start_time":                firstUseBase,
		"five_hour_used":            0,
		"five_hour_last_reset_time": 0,
		"five_hour_next_reset_time": 0,
	}).Error)

	before := GetDBTimestamp()
	_, err := PreConsumeUserSubscription("subscription-first-use-window", user.Id, "test", 0, 10)
	after := GetDBTimestamp()
	require.NoError(t, err)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
	assert.GreaterOrEqual(t, reloaded.FiveHourLastResetTime, before)
	assert.LessOrEqual(t, reloaded.FiveHourLastResetTime, after)
	assert.Equal(t, reloaded.FiveHourLastResetTime+int64((5*time.Hour).Seconds()), reloaded.FiveHourNextResetTime)
	assert.NotEqual(t, firstUseBase+int64((5*time.Hour).Seconds()), reloaded.FiveHourNextResetTime)
}

func TestSubscriptionUsageLimitPeriodsAreIndependentlyEnforced(t *testing.T) {
	tests := []struct {
		name string
		plan SubscriptionPlan
		used func(UserSubscription) int64
	}{
		{
			name: "five-hour",
			plan: SubscriptionPlan{FiveHourLimit: 10},
			used: func(sub UserSubscription) int64 { return sub.FiveHourUsed },
		},
		{
			name: "weekly",
			plan: SubscriptionPlan{WeeklyLimit: 10},
			used: func(sub UserSubscription) int64 { return sub.WeeklyUsed },
		},
		{
			name: "monthly",
			plan: SubscriptionPlan{MonthlyLimit: 10},
			used: func(sub UserSubscription) int64 { return sub.MonthlyUsed },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepareSubscriptionUsageLimitTest(t)
			tt.plan.Title = "Independent " + tt.name
			tt.plan.DurationUnit = SubscriptionDurationMonth
			tt.plan.DurationValue = 1
			user, subscription := createSubscriptionUsageLimitFixture(t, &tt.plan)

			_, err := PreConsumeUserSubscription("subscription-usage-"+tt.name+"-first", user.Id, "test", 0, 10)
			require.NoError(t, err)
			_, err = PreConsumeUserSubscription("subscription-usage-"+tt.name+"-over", user.Id, "test", 0, 1)
			require.Error(t, err)

			var reloaded UserSubscription
			require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
			assert.EqualValues(t, 10, tt.used(reloaded))
		})
	}
}

func TestSubscriptionUsageLimitsResetDuringPreConsume(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{
		Title:         "Resettable usage caps",
		DurationUnit:  SubscriptionDurationYear,
		DurationValue: 1,
		FiveHourLimit: 10,
		WeeklyLimit:   10,
		MonthlyLimit:  10,
	}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)
	now := GetDBTimestamp()
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).Updates(map[string]interface{}{
		"five_hour_used":            7,
		"five_hour_last_reset_time": now - int64((5 * time.Hour).Seconds()),
		"five_hour_next_reset_time": now - 1,
		"weekly_used":               7,
		"weekly_last_reset_time":    now - 8*24*3600,
		"weekly_next_reset_time":    now - 1,
		"monthly_used":              7,
		"monthly_last_reset_time":   now - 32*24*3600,
		"monthly_next_reset_time":   now - 1,
	}).Error)

	_, err := PreConsumeUserSubscription("subscription-usage-reset", user.Id, "test", 0, 1)
	require.NoError(t, err)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
	assert.EqualValues(t, 1, reloaded.FiveHourUsed)
	assert.EqualValues(t, 1, reloaded.WeeklyUsed)
	assert.EqualValues(t, 1, reloaded.MonthlyUsed)
	assert.Greater(t, reloaded.FiveHourNextResetTime, now)
	assert.Greater(t, reloaded.WeeklyNextResetTime, now)
	assert.Greater(t, reloaded.MonthlyNextResetTime, now)
}

func TestSubscriptionUsageLimitKeepsTerminalWindowUsage(t *testing.T) {
	now := int64(1_700_000_000)
	used := int64(7)
	lastResetTime := now - 60
	nextResetTime := int64(0)

	changed := maybeResetSubscriptionUsageLimit(
		&used,
		&lastResetTime,
		&nextResetTime,
		10,
		SubscriptionResetFiveHour,
		now-60,
		now+60,
		now,
	)

	assert.False(t, changed)
	assert.EqualValues(t, 7, used)
	assert.Equal(t, now-60, lastResetTime)
	assert.Zero(t, nextResetTime)
}

func TestResetDueSubscriptionsResetsIndependentUsageLimits(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Title:         "Scheduled usage caps",
		DurationUnit:  SubscriptionDurationYear,
		DurationValue: 1,
		FiveHourLimit: 10,
	}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)
	require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).Updates(map[string]interface{}{
		"five_hour_used":            7,
		"five_hour_last_reset_time": now - int64((5 * time.Hour).Seconds()),
		"five_hour_next_reset_time": now - 1,
	}).Error)

	resetCount, err := ResetDueSubscriptions(10)
	require.NoError(t, err)
	assert.Equal(t, 1, resetCount)

	var reloaded UserSubscription
	require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
	assert.Equal(t, user.Id, reloaded.UserId)
	assert.Zero(t, reloaded.FiveHourUsed)
	assert.Greater(t, reloaded.FiveHourNextResetTime, now)
}

func TestPreConsumeSubscriptionResultCarriesReceiptIdentity(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{Title: "Receipt identity", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)
	requestId := "receipt-identity-preconsume"
	result, err := PreConsumeUserSubscription(requestId, user.Id, "test", 0, 10)
	require.NoError(t, err)
	require.Equal(t, subscription.Id, result.UserSubscriptionId)
	var receipt SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestId).First(&receipt).Error)
	require.NotZero(t, receipt.Id)
	assert.Equal(t, receipt.Id, result.ReceiptId)
	replayed, err := PreConsumeUserSubscription(requestId, user.Id, "test", 0, 10)
	require.NoError(t, err)
	require.True(t, replayed.Reused)
	assert.Equal(t, receipt.Id, replayed.ReceiptId)
}

func TestPreConsumeSubscriptionDuplicateInsertReturnsStoredReceipt(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{Title: "Receipt race", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
	user, subscription := createSubscriptionUsageLimitFixture(t, plan)
	requestId := "receipt-identity-duplicate"
	injected := false
	var injectErr error
	name := "test:competing_pre_consume_receipt"
	require.NoError(t, DB.Callback().Create().Before("gorm:create").Register(name, func(tx *gorm.DB) {
		r, ok := tx.Statement.Dest.(*SubscriptionPreConsumeRecord)
		if !ok || injected || r.RequestId != requestId {
			return
		}
		injected = true
		competing := SubscriptionPreConsumeRecord{RequestId: requestId, UserId: user.Id, UserSubscriptionId: subscription.Id, PreConsumed: r.PreConsumed, Allocations: r.Allocations, ReservedTotal: r.ReservedTotal, FundingMode: r.FundingMode, Durable: true, Status: "consumed"}
		injectErr = tx.Session(&gorm.Session{NewDB: true}).Create(&competing).Error
	}))
	t.Cleanup(func() { _ = DB.Callback().Create().Remove(name) })
	result, err := PreConsumeUserSubscription(requestId, user.Id, "test", 0, 10)
	require.True(t, injected)
	require.NoError(t, injectErr)
	require.NoError(t, err)
	require.True(t, result.Reused)
	var stored SubscriptionPreConsumeRecord
	require.NoError(t, DB.Where("request_id = ?", requestId).First(&stored).Error)
	assert.Equal(t, stored.Id, result.ReceiptId)
}

func TestPreConsumeSubscriptionLargeAllocationDatabaseMatrix(t *testing.T) {
	oldRatios := ratio_setting.GroupRatio2JSONString()
	require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":1}`))
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios)) })
	for _, dialect := range []common.DatabaseType{common.DatabaseTypeSQLite, common.DatabaseTypeMySQL, common.DatabaseTypePostgreSQL} {
		t.Run(string(dialect), func(t *testing.T) {
			dsn := "local"
			if dialect == common.DatabaseTypeMySQL {
				dsn = os.Getenv("TEST_MYSQL_DSN")
			}
			if dialect == common.DatabaseTypePostgreSQL {
				dsn = os.Getenv("TEST_POSTGRES_DSN")
			}
			if dsn == "" {
				t.Skip("isolated DSN not configured")
			}
			oldPath := common.SQLitePath
			common.SQLitePath = filepath.Join(t.TempDir(), "allocations.db")
			t.Cleanup(func() { common.SQLitePath = oldPath })
			t.Setenv("ALLOCATION_MATRIX_DSN", dsn)
			db, _, err := chooseDB("ALLOCATION_MATRIX_DSN", false)
			require.NoError(t, err)
			oldDB, oldLog, oldType := DB, LOG_DB, common.MainDatabaseType()
			DB, LOG_DB = db, db
			common.SetMainDatabaseType(dialect)
			initCol()
			conn, err := db.DB()
			require.NoError(t, err)
			t.Cleanup(func() {
				DB, LOG_DB = oldDB, oldLog
				common.SetMainDatabaseType(oldType)
				initCol()
				require.NoError(t, conn.Close())
			})
			models := []any{&User{}, &UserSubscription{}, &SubscriptionPlan{}, &SubscriptionPreConsumeRecord{}, &SubscriptionOrder{}, &SystemTask{}, &Option{}, &Task{}}
			require.NoError(t, db.AutoMigrate(models...))
			t.Cleanup(func() { require.NoError(t, db.Migrator().DropTable(models...)) })
			user := createReserveTestUser(t, 100)
			plan := SubscriptionPlan{Title: "Large allocations", SubscriptionGroup: "vip", TotalAmount: 1}
			require.NoError(t, db.Create(&plan).Error)
			InvalidateSubscriptionPlanCache(plan.Id)
			rows := make([]UserSubscription, 600)
			for i := range rows {
				rows[i] = UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: GetDBTimestamp() - 1, EndTime: GetDBTimestamp() + 3600, AmountTotal: 1}
			}
			rows[0].ExtraCreditsTotal = 1
			rows[0].ExtraCreditsGroups = `["vip"]`
			rows[0].ExtraCreditsScopedGroups = `["vip"]`
			require.NoError(t, db.CreateInBatches(&rows, 50).Error)
			result, err := PreConsumeUserSubscriptionForGroup("large-allocation-matrix", user.Id, "model", 0, 600, "vip")
			require.NoError(t, err)
			var receipt SubscriptionPreConsumeRecord
			require.NoError(t, db.First(&receipt, result.ReceiptId).Error)
			require.Greater(t, len(receipt.Allocations), 65_535)
			require.NoError(t, RefundSubscriptionPreConsume(receipt.RequestId))
			var used int64
			require.NoError(t, db.Model(&UserSubscription{}).Where("user_id = ?", user.Id).Select("SUM(amount_used)").Scan(&used).Error)
			require.Zero(t, used)
		})
	}
}

func TestPreConsumeSubscriptionSeesCreditsAcquiredBeforeAccountLock(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{Title: "Concurrent extra purchase", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 10}
	user, sub := createSubscriptionUsageLimitFixture(t, plan)
	injected := false
	name := "test:purchase_before_account_lock"
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
		rows, ok := tx.Statement.Dest.(*[]UserSubscription)
		if !ok || len(*rows) != 0 || injected {
			return
		}
		injected = true
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(sub).Updates(map[string]any{"amount_used": 10, "extra_credits_total": 20, "extra_credits_groups": `["default"]`}).Error)
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(name) })
	result, err := PreConsumeUserSubscriptionForGroup("credits-before-account-lock", user.Id, "test", 0, 10, "default")
	require.True(t, injected)
	require.NoError(t, err)
	require.NotZero(t, result.ReceiptId)
	require.NoError(t, DB.First(sub, sub.Id).Error)
	require.EqualValues(t, 10, sub.ExtraCreditsUsed)
}

func TestPreConsumeSubscriptionDefaultGroupChangesBeforeLock(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{Title: "Default group change", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 5}
	user, _ := createSubscriptionUsageLimitFixture(t, plan)
	scoped := SubscriptionPlan{Title: "B", SubscriptionGroup: "group-b"}
	require.NoError(t, DB.Create(&scoped).Error)
	extra := UserSubscription{UserId: user.Id, PlanId: scoped.Id, Status: "expired", ExtraCreditsTotal: 5, ExtraCreditsGroups: `["group-b"]`, ExtraCreditsScopedGroups: `["group-b"]`}
	require.NoError(t, DB.Create(&extra).Error)
	changed := false
	const hook = "test:change_default_group_after_lookup"
	require.NoError(t, DB.Callback().Query().After("gorm:query").Register(hook, func(tx *gorm.DB) {
		g, ok := tx.Statement.Dest.(*string)
		if !ok || *g != "default" || changed {
			return
		}
		changed = true
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(user).UpdateColumn("group", "group-b").Error)
	}))
	t.Cleanup(func() { _ = DB.Callback().Query().Remove(hook) })
	r, err := PreConsumeUserSubscription("group-at-lock", user.Id, "model", 0, 5)
	require.True(t, changed)
	require.NoError(t, err)
	require.Equal(t, extra.Id, r.UserSubscriptionId)
	require.NoError(t, DB.First(&extra, extra.Id).Error)
	require.EqualValues(t, 5, extra.ExtraCreditsUsed)
}

func TestPreConsumeSubscriptionOverflowUsesNextCandidate(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{Title: "Overflow fallback", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
	user, first := createSubscriptionUsageLimitFixture(t, plan)
	require.NoError(t, DB.Model(first).UpdateColumn("funding_reserved", int64(math.MaxInt64)).Error)
	second := *first
	second.Id = 0
	second.EndTime++
	second.FundingReserved = 0
	require.NoError(t, DB.Create(&second).Error)
	result, err := PreConsumeUserSubscriptionForGroup("overflow-fallback", user.Id, "test", 0, 1, "default")
	require.NoError(t, err)
	require.Equal(t, second.Id, result.UserSubscriptionId)
	require.NoError(t, DB.First(first, first.Id).Error)
	require.Zero(t, first.AmountUsed)
	require.EqualValues(t, math.MaxInt64, first.FundingReserved)
}

func TestPreConsumeSubscriptionDuplicateFoundRowsRejectsOtherOwner(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{Title: "Found rows collision", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
	user, sub := createSubscriptionUsageLimitFixture(t, plan)
	id := "found-rows-other-owner"
	inserted := false
	name := "test:found_rows_collision"
	require.NoError(t, DB.Callback().Create().After("gorm:create").Register(name, func(tx *gorm.DB) {
		r, ok := tx.Statement.Dest.(*SubscriptionPreConsumeRecord)
		if !ok || r.RequestId != id || inserted {
			return
		}
		inserted = true
		require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(&SubscriptionPreConsumeRecord{}).Where("id = ?", r.Id).UpdateColumn("user_id", user.Id+1).Error)
		tx.RowsAffected = 1
	}))
	t.Cleanup(func() { _ = DB.Callback().Create().Remove(name) })
	_, err := PreConsumeUserSubscriptionForGroup(id, user.Id, "test", 0, 5, "default")
	require.True(t, inserted)
	require.Error(t, err, "driver affected row count cannot establish receipt ownership")
	require.NoError(t, DB.First(sub, sub.Id).Error)
	require.Zero(t, sub.AmountUsed)
	require.Zero(t, sub.FundingReserved)
}

func TestPreConsumeSubscriptionRejectsFundingReservationOverflow(t *testing.T) {
	tests := []struct {
		name     string
		reserved int64
		wantErr  bool
		want     int64
	}{
		{"fills", math.MaxInt64 - 5, false, math.MaxInt64},
		{"overflows", math.MaxInt64 - 4, true, math.MaxInt64 - 4},
		{"negative", -1, true, -1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepareSubscriptionUsageLimitTest(t)
			plan := &SubscriptionPlan{Title: "Reservation overflow", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
			user, subscription := createSubscriptionUsageLimitFixture(t, plan)
			// An admin total reset clears usage but keeps in-flight reservations.
			require.NoError(t, DB.Model(&UserSubscription{}).Where("id = ?", subscription.Id).UpdateColumn("funding_reserved", tt.reserved).Error)
			requestId := "reservation-overflow-" + tt.name
			_, err := PreConsumeUserSubscription(requestId, user.Id, "test", 0, 5)
			var reloaded UserSubscription
			require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
			var receipts int64
			require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", requestId).Count(&receipts).Error)
			if tt.wantErr {
				require.Error(t, err)
				assert.Zero(t, reloaded.AmountUsed)
				assert.Zero(t, receipts)
			} else {
				require.NoError(t, err)
				assert.EqualValues(t, 5, reloaded.AmountUsed)
				assert.EqualValues(t, 1, receipts)
			}
			assert.Equal(t, tt.want, reloaded.FundingReserved)
		})
	}
}

func TestCleanupSubscriptionPreConsumeIgnoresCompletedOldTask(t *testing.T) {
	prepareSubscriptionUsageLimitTest(t)
	plan := &SubscriptionPlan{Title: "Reused request cleanup", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
	user, sub := createSubscriptionUsageLimitFixture(t, plan)
	id := "cleanup-same-key-new-reservation"
	old, err := PreConsumeUserSubscription(id, user.Id, "test", 0, 5)
	require.NoError(t, err)
	require.NoError(t, RefundSubscriptionPreConsume(id))
	require.NoError(t, DB.Where("id = ?", old.ReceiptId).Delete(&SubscriptionPreConsumeRecord{}).Error)
	task := Task{UserId: user.Id, Status: TaskStatusFailure, PrivateData: TaskPrivateData{BillingSource: "subscription", SubscriptionId: sub.Id, SubscriptionRequestID: id, SubscriptionReceiptId: old.ReceiptId, SubscriptionFinancialComplete: true}}
	require.NoError(t, DB.Create(&task).Error)
	newer, err := PreConsumeUserSubscription(id, user.Id, "test", 0, 5)
	require.NoError(t, err)
	require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("id = ?", newer.ReceiptId).UpdateColumn("updated_at", GetDBTimestamp()-1000).Error)
	removed, err := CleanupSubscriptionPreConsumeRecords(500)
	require.NoError(t, err)
	require.EqualValues(t, 1, removed)
	require.NoError(t, DB.First(sub, sub.Id).Error)
	require.Zero(t, sub.FundingReserved)
}

func TestCleanupSubscriptionPreConsumeRecordsKeepsPendingBillingLog(t *testing.T) {
	for _, scenario := range []string{"selection", "recheck"} {
		t.Run(scenario, func(t *testing.T) {
			prepareSubscriptionUsageLimitTest(t)
			plan := &SubscriptionPlan{Title: "Pending log", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
			user, subscription := createSubscriptionUsageLimitFixture(t, plan)
			requestId := "cleanup-pending-log-" + scenario
			_, err := PreConsumeUserSubscription(requestId, user.Id, "test", 0, 5)
			require.NoError(t, err)
			raw, err := common.Marshal([]Log{{UserId: user.Id, RequestId: requestId, Type: LogTypeConsume, Quota: 5}})
			require.NoError(t, err)
			cols := map[string]any{"billing_log": string(raw), "updated_at": GetDBTimestamp() - 1000}
			if scenario == "selection" {
				cols["billing_log_pending"] = true
			}
			require.NoError(t, DB.Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", requestId).UpdateColumns(cols).Error)
			if scenario == "recheck" {
				// The log becomes pending after candidate selection, before the row lock.
				name := "test:pending_before_receipt_lock"
				selected := false
				require.NoError(t, DB.Callback().Query().After("gorm:query").Register(name, func(tx *gorm.DB) {
					if _, ok := tx.Statement.Dest.(*[]SubscriptionPreConsumeRecord); !ok || selected {
						return
					}
					selected = true
					require.NoError(t, tx.Session(&gorm.Session{NewDB: true}).Model(&SubscriptionPreConsumeRecord{}).Where("request_id = ?", requestId).UpdateColumn("billing_log_pending", true).Error)
				}))
				t.Cleanup(func() { _ = DB.Callback().Query().Remove(name) })
			}
			removed, err := CleanupSubscriptionPreConsumeRecords(500)
			require.NoError(t, err)
			assert.Zero(t, removed)
			var kept SubscriptionPreConsumeRecord
			require.NoError(t, DB.Where("request_id = ?", requestId).First(&kept).Error)
			assert.True(t, kept.BillingLogPending)
			var reloaded UserSubscription
			require.NoError(t, DB.First(&reloaded, subscription.Id).Error)
			assert.EqualValues(t, 5, reloaded.FundingReserved)
		})
	}
}
