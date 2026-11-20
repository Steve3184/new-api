package model

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedSubscriptionResetPlan(t *testing.T, plan *SubscriptionPlan) {
	t.Helper()
	require.NoError(t, DB.Create(plan).Error)
}

func seedSubscriptionResetSub(t *testing.T, sub *UserSubscription) {
	t.Helper()
	require.NoError(t, DB.Create(sub).Error)
}

func getSubscriptionResetSub(t *testing.T, id int) UserSubscription {
	t.Helper()
	var sub UserSubscription
	require.NoError(t, DB.Where("id = ?", id).First(&sub).Error)
	return sub
}

func TestAdminResetUserSubscriptionsByPlanResetsAllActiveMatchesAndAdvancesTime(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9101,
		Title:            "Pro",
		PriceAmount:      10,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      1000,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	otherPlan := &SubscriptionPlan{
		Id:               9102,
		Title:            "Basic",
		PriceAmount:      1,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      100,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	seedSubscriptionResetPlan(t, plan)
	seedSubscriptionResetPlan(t, otherPlan)

	activeEnd := now + 30*24*3600
	expiredEnd := now - 1
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9201, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 300, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9202, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 500, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9203, UserId: 101, PlanId: otherPlan.Id, AmountTotal: 100, AmountUsed: 60, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9204, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 700, StartTime: now - 7200, EndTime: expiredEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now - 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9205, UserId: 102, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 800, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 120})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9206, UserId: 101, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 900, StartTime: now - 3600, EndTime: activeEnd, Status: "cancelled", LastResetTime: now - 3600, NextResetTime: now + 120})

	beforeReset := GetDBTimestamp()
	result, err := AdminResetUserSubscriptionsByPlan(101, plan.Id, true)
	afterReset := GetDBTimestamp()

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, plan.Id, result.PlanId)
	assert.Equal(t, 2, result.MatchedCount)
	assert.Equal(t, 2, result.ResetCount)
	assert.Equal(t, 1, result.UserCount)
	assert.Equal(t, []int{101}, result.AffectedUserIds)
	assert.True(t, result.AdvanceResetTime)

	for _, id := range []int{9201, 9202} {
		sub := getSubscriptionResetSub(t, id)
		assert.Zero(t, sub.AmountUsed)
		assert.GreaterOrEqual(t, sub.LastResetTime, beforeReset)
		assert.LessOrEqual(t, sub.LastResetTime, afterReset)
		assert.Equal(t, calcNextResetTime(time.Unix(sub.LastResetTime, 0), plan, sub.EndTime), sub.NextResetTime)
	}
	assert.EqualValues(t, 60, getSubscriptionResetSub(t, 9203).AmountUsed)
	assert.EqualValues(t, 700, getSubscriptionResetSub(t, 9204).AmountUsed)
	assert.EqualValues(t, 800, getSubscriptionResetSub(t, 9205).AmountUsed)
	assert.EqualValues(t, 900, getSubscriptionResetSub(t, 9206).AmountUsed)
}

func TestAdminResetUserSubscriptionsByPlanKeepsResetTimes(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9301,
		Title:            "Team",
		PriceAmount:      20,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      2000,
		QuotaResetPeriod: SubscriptionResetMonthly,
	}
	seedSubscriptionResetPlan(t, plan)

	lastReset := now - 86400
	nextReset := now + 86400
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9302, UserId: 201, PlanId: plan.Id, AmountTotal: 2000, AmountUsed: 1200, StartTime: now - 172800, EndTime: now + 30*24*3600, Status: "active", LastResetTime: lastReset, NextResetTime: nextReset})

	result, err := AdminResetUserSubscriptionsByPlan(201, plan.Id, false)

	require.NoError(t, err)
	assert.False(t, result.AdvanceResetTime)
	sub := getSubscriptionResetSub(t, 9302)
	assert.Zero(t, sub.AmountUsed)
	assert.Equal(t, lastReset, sub.LastResetTime)
	assert.Equal(t, nextReset, sub.NextResetTime)
}

func TestAdminResetUserSubscriptionsByPlanHonorsSelectedUsageBuckets(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9351,
		Title:            "Selective reset",
		PriceAmount:      20,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      2000,
		FiveHourLimit:    500,
		WeeklyLimit:      700,
		MonthlyLimit:     900,
		QuotaResetPeriod: SubscriptionResetDaily,
	}
	seedSubscriptionResetPlan(t, plan)
	seedSubscriptionResetSub(t, &UserSubscription{
		Id: 9352, UserId: 235, PlanId: plan.Id, AmountTotal: 2000, AmountUsed: 1200,
		FiveHourLimit: 500, FiveHourUsed: 200, FiveHourLastResetTime: now - 100, FiveHourNextResetTime: now + 100,
		WeeklyLimit: 700, WeeklyUsed: 300, WeeklyLastResetTime: now - 100, WeeklyNextResetTime: now + 100,
		MonthlyLimit: 900, MonthlyUsed: 400, MonthlyLastResetTime: now - 100, MonthlyNextResetTime: now + 100,
		StartTime: now - 3600, EndTime: now + 30*24*3600, Status: "active", LastResetTime: now - 100, NextResetTime: now + 100,
	})

	_, err := AdminResetUserSubscriptionsByPlanWithOptions(235, plan.Id, true, SubscriptionResetOptions{
		ResetFiveHour: true,
	})
	require.NoError(t, err)

	sub := getSubscriptionResetSub(t, 9352)
	assert.Zero(t, sub.FiveHourUsed)
	assert.EqualValues(t, 1200, sub.AmountUsed)
	assert.EqualValues(t, 300, sub.WeeklyUsed)
	assert.EqualValues(t, 400, sub.MonthlyUsed)
	assert.Equal(t, now, sub.FiveHourLastResetTime)
	assert.Equal(t, sub.FiveHourLastResetTime+int64((5*time.Hour).Seconds()), sub.FiveHourNextResetTime)
	assert.Equal(t, now-100, sub.LastResetTime)
}

func TestAdminResetUserSubscriptionsByPlanNoActiveMatchReturnsError(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:            9401,
		Title:         "Expired",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   1000,
	}
	seedSubscriptionResetPlan(t, plan)
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9402, UserId: 301, PlanId: plan.Id, AmountTotal: 1000, AmountUsed: 500, StartTime: now - 7200, EndTime: now - 1, Status: "active"})

	result, err := AdminResetUserSubscriptionsByPlan(301, plan.Id, true)

	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, strings.Contains(err.Error(), "This user has no active subscription to this plan"))
}

func TestAdminResetPlanSubscriptionsResetsAllActiveUsers(t *testing.T) {
	truncateTables(t)

	now := GetDBTimestamp()
	plan := &SubscriptionPlan{
		Id:               9501,
		Title:            "Business",
		PriceAmount:      30,
		DurationUnit:     SubscriptionDurationMonth,
		DurationValue:    1,
		TotalAmount:      3000,
		QuotaResetPeriod: SubscriptionResetNever,
	}
	seedSubscriptionResetPlan(t, plan)

	activeEnd := now + 30*24*3600
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9502, UserId: 401, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1000, StartTime: now - 3600, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9503, UserId: 401, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1100, StartTime: now - 3500, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9504, UserId: 402, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1200, StartTime: now - 3400, EndTime: activeEnd, Status: "active", LastResetTime: now - 3600, NextResetTime: now + 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9505, UserId: 403, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1300, StartTime: now - 7200, EndTime: now - 1, Status: "active", LastResetTime: now - 3600, NextResetTime: now - 10})
	seedSubscriptionResetSub(t, &UserSubscription{Id: 9506, UserId: 404, PlanId: plan.Id, AmountTotal: 3000, AmountUsed: 1400, StartTime: now - 3600, EndTime: activeEnd, Status: "cancelled", LastResetTime: now - 3600, NextResetTime: now + 10})

	result, err := AdminResetPlanSubscriptions(plan.Id, true)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 3, result.MatchedCount)
	assert.Equal(t, 3, result.ResetCount)
	assert.Equal(t, 2, result.UserCount)
	assert.Equal(t, []int{401, 402}, result.AffectedUserIds)
	for _, id := range []int{9502, 9503, 9504} {
		sub := getSubscriptionResetSub(t, id)
		assert.Zero(t, sub.AmountUsed)
		assert.Zero(t, sub.LastResetTime)
		assert.Zero(t, sub.NextResetTime)
	}
	assert.EqualValues(t, 1300, getSubscriptionResetSub(t, 9505).AmountUsed)
	assert.EqualValues(t, 1400, getSubscriptionResetSub(t, 9506).AmountUsed)
}

func TestAdminResetPlanSubscriptionsNoMatchSucceeds(t *testing.T) {
	truncateTables(t)

	plan := &SubscriptionPlan{
		Id:            9601,
		Title:         "Empty",
		PriceAmount:   10,
		DurationUnit:  SubscriptionDurationMonth,
		DurationValue: 1,
		TotalAmount:   1000,
	}
	seedSubscriptionResetPlan(t, plan)

	result, err := AdminResetPlanSubscriptions(plan.Id, true)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Zero(t, result.MatchedCount)
	assert.Zero(t, result.ResetCount)
	assert.Zero(t, result.UserCount)
	assert.Empty(t, result.AffectedUserIds)
}

// recordSubscriptionLockOrder collects the order of the statements a rollback
// path issues, marking the ones that take row locks: FOR UPDATE reads and every
// write. Refund paths lock the account row and then all of its subscription
// rows, so a rollback that locks in the opposite order deadlocks against them
// on MySQL and PostgreSQL.
func recordSubscriptionLockOrder(t *testing.T) *[]string {
	t.Helper()
	order := &[]string{}
	record := func(kind string, locking bool) func(*gorm.DB) {
		return func(tx *gorm.DB) {
			table := ""
			if tx.Statement != nil {
				table = tx.Statement.Table
			}
			mark := "read"
			if locking {
				mark = "lock"
			}
			*order = append(*order, kind+"|"+mark+"|"+table)
		}
	}
	queryName := "test:record_rollback_query_order"
	updateName := "test:record_rollback_update_order"
	deleteName := "test:record_rollback_delete_order"
	require.NoError(t, DB.Callback().Query().Before("gorm:query").Register(queryName, func(tx *gorm.DB) {
		_, locking := tx.Statement.Clauses["FOR"]
		record("query", locking)(tx)
	}))
	require.NoError(t, DB.Callback().Update().Before("gorm:update").Register(updateName, record("update", true)))
	require.NoError(t, DB.Callback().Delete().Before("gorm:delete").Register(deleteName, record("delete", true)))
	t.Cleanup(func() {
		require.NoError(t, DB.Callback().Query().Remove(queryName))
		require.NoError(t, DB.Callback().Update().Remove(updateName))
		require.NoError(t, DB.Callback().Delete().Remove(deleteName))
	})
	return order
}

func TestSubscriptionResetLocksAccountBeforeFundingRows(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprint(all), func(t *testing.T) {
			truncateTables(t)
			plan := &SubscriptionPlan{Title: "Reset lock order", DurationUnit: SubscriptionDurationMonth, DurationValue: 1, TotalAmount: 100}
			user, sub := createSubscriptionUsageLimitFixture(t, plan)
			order := recordSubscriptionLockOrder(t)
			var err error
			if all {
				_, err = AdminResetPlanSubscriptions(plan.Id, false)
			} else {
				_, err = AdminResetUserSubscriptionsByPlan(user.Id, plan.Id, false)
			}
			require.NoError(t, err)
			firstAccount, firstSub := -1, -1
			for i, entry := range *order {
				if firstAccount < 0 && strings.HasSuffix(entry, "|users") {
					firstAccount = i
				}
				if firstSub < 0 && strings.HasSuffix(entry, "|user_subscriptions") && (strings.Contains(entry, "|lock|") || strings.HasPrefix(entry, "update|")) {
					firstSub = i
				}
			}
			require.GreaterOrEqual(t, firstAccount, 0, "reset must serialize with funding release on the account")
			require.Less(t, firstAccount, firstSub)
			require.NoError(t, DB.First(sub, sub.Id).Error)
			require.Zero(t, sub.AmountUsed)
		})
	}
}

func TestSubscriptionGroupRollbackLocksAccountBeforeSubscriptions(t *testing.T) {
	tests := []struct {
		name string
		run  func(subId int) error
	}{
		{"expire", func(subId int) error {
			_, err := ExpireDueSubscriptions(10)
			return err
		}},
		{"invalidate", func(subId int) error {
			_, err := AdminInvalidateUserSubscription(subId)
			return err
		}},
		{"delete", func(subId int) error {
			_, err := AdminDeleteUserSubscription(subId)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			truncateTables(t)
			now := GetDBTimestamp()
			user := &User{
				Username: "rollback-lock-" + common.GetRandomString(8),
				Password: "password",
				Status:   common.UserStatusEnabled,
				Group:    "svip",
			}
			require.NoError(t, DB.Create(user).Error)
			endTime := now + 3600
			if tt.name == "expire" {
				endTime = now - 1
			}
			sub := UserSubscription{
				UserId: user.Id, Status: "active", StartTime: now - 7200, EndTime: endTime,
				AmountTotal: 100, UpgradeGroup: "svip", PrevUserGroup: "default",
			}
			require.NoError(t, DB.Create(&sub).Error)

			order := recordSubscriptionLockOrder(t)
			require.NoError(t, tt.run(sub.Id))

			var reloaded User
			require.NoError(t, DB.First(&reloaded, user.Id).Error)
			require.Equal(t, "default", reloaded.Group, "the group rollback runs")

			firstLock := func(table string) int {
				for i, entry := range *order {
					parts := strings.SplitN(entry, "|", 3)
					if parts[2] == table && (parts[1] == "lock" || table == "users" && common.UsingMainDatabase(common.DatabaseTypeSQLite)) {
						return i
					}
				}
				return -1
			}
			t.Logf("order: %v", *order)
			accountLock := firstLock("users")
			subscriptionLock := firstLock("user_subscriptions")
			require.GreaterOrEqual(t, accountLock, 0, "the rollback must lock the account row")
			require.GreaterOrEqual(t, subscriptionLock, 0, "the rollback locks its subscription row")
			assert.Less(t, accountLock, subscriptionLock, "locks must follow the account-then-subscriptions order")
		})
	}
}
