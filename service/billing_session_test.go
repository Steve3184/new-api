package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestSubscriptionExtraCreditsDurableSubmissionLog(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.QuotaData{}))
	user := model.User{Username: "submit-log-user", AffCode: "submit-log-user", Group: "default", Quota: 100, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Submission log", SubscriptionGroup: "vip", TotalAmount: 20}
	require.NoError(t, model.DB.Create(&plan).Error)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: common.GetTimestamp() + 1000, AmountTotal: 20}
	require.NoError(t, model.DB.Create(&sub).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/videos", nil)
	info := &relaycommon.RelayInfo{TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: "generate"}, ChannelMeta: &relaycommon.ChannelMeta{}, RequestId: "durable-initial-log", UserId: user.Id, UsingGroup: "vip", OriginModelName: "test", IsPlayground: true, ForcePreConsume: true, UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
	session, apiErr := NewBillingSession(ctx, info, 10)
	require.Nil(t, apiErr)
	task := &model.Task{UserId: user.Id, Quota: 3, PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: sub.Id, SubscriptionRequestID: info.RequestId}}
	session.BindTask(task)
	LogTaskConsumption(ctx, info, task)
	require.NotNil(t, task.PrivateData.SubscriptionSubmitLog)
	require.NoError(t, model.DB.Create(task).Error)
	require.NoError(t, session.Settle(3))
	require.NoError(t, model.DB.First(&user, user.Id).Error)
	require.Equal(t, 3, user.UsedQuota)
	require.Equal(t, 1, user.RequestCount)
	RecalculateTaskQuota(t.Context(), task, 5, common.NewMessage("token adjustment"), &common.QuotaClamp{Op: "QuotaRound", Kind: common.QuotaClampOverflow})
	require.NoError(t, model.FlushSubscriptionTaskBillingLogs())
	var entry model.Log
	require.NoError(t, model.LOG_DB.Where("user_id = ? AND type = ?", user.Id, model.LogTypeConsume).Order("id DESC").First(&entry).Error)
	require.Contains(t, entry.Other, "quota_saturation")
}

func TestSubscriptionExtraCreditsBillingSession(t *testing.T) {
	truncate(t)
	for _, expired := range []bool{false, true} {
		name := fmt.Sprintf("extra-billing-%t", expired)
		user := &model.User{Username: name, AffCode: name, Group: "default", Status: common.UserStatusEnabled, Quota: 100}
		require.NoError(t, model.DB.Create(user).Error)
		plan := &model.SubscriptionPlan{Title: name, SubscriptionGroup: "vip", TotalAmount: 5, AllowWalletOverflow: common.GetPointer(false)}
		require.NoError(t, model.DB.Create(plan).Error)
		now := common.GetTimestamp()
		sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 5}
		if expired {
			sub.Status = "expired"
			sub.EndTime = now - 1
		}
		sub.ExtraCreditsTotal = 30
		sub.ExtraCreditsGroups = `["vip"]`
		sub.ExtraCreditsScopedGroups = `["vip"]`
		require.NoError(t, model.DB.Create(&sub).Error)
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
		info := &relaycommon.RelayInfo{RequestId: name, UserId: user.Id, UsingGroup: "vip", OriginModelName: "test", IsPlayground: true, ForcePreConsume: true, UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
		session, apiErr := NewBillingSession(ctx, info, 10)
		require.Nil(t, apiErr, "expired subscriptions must still fund requests with purchased credits")
		require.NoError(t, session.ReserveBeforeSubmit(15))
		wantLoggedRegular := int64(5)
		if expired {
			wantLoggedRegular = 0
		}
		require.Equal(t, wantLoggedRegular, info.SubscriptionAmountUsedAfterPreConsume, "extra reservations do not increase ordinary quota usage in billing logs")
		require.NoError(t, session.Settle(8))
		require.NoError(t, session.Settle(8))
		var saved model.UserSubscription
		require.NoError(t, model.DB.First(&saved, sub.Id).Error)
		wantRegular := int64(5)
		if expired {
			wantRegular = 0
		}
		require.Equal(t, wantRegular, saved.AmountUsed)
		raw, err := common.Marshal(saved)
		require.NoError(t, err)
		var fields map[string]any
		require.NoError(t, common.Unmarshal(raw, &fields))
		require.Equal(t, float64(8-wantRegular), fields["extra_credits_used"])
		var savedUser model.User
		require.NoError(t, model.DB.First(&savedUser, user.Id).Error)
		require.Equal(t, 100, savedUser.Quota)
		// A task's refund must return its own extra-credit reservation without
		// undoing the previously settled request or returning wallet balance.
		requestID := name + "-task"
		info.RequestId = requestID
		taskSession, taskErr := NewBillingSession(ctx, info, 6)
		require.Nil(t, taskErr)
		require.NoError(t, taskSession.ReserveBeforeSubmit(9))
		funding := taskSession.funding.(*SubscriptionFunding)
		task := &model.Task{UserId: user.Id, PrivateData: model.TaskPrivateData{
			BillingSource: BillingSourceSubscription, SubscriptionId: funding.subscriptionId,
			SubscriptionRequestID: requestID,
			Execution:             &model.TaskExecutionSnapshot{RequestID: requestID},
		}}
		require.NoError(t, taskAdjustFunding(task, 1))
		require.NoError(t, taskAdjustFunding(task, -10))
		require.NoError(t, model.DB.First(&saved, sub.Id).Error)
		require.Equal(t, wantRegular, saved.AmountUsed)
		require.EqualValues(t, 8-wantRegular, saved.ExtraCreditsUsed)
		info.RequestId = name + "-failed"
		failedSession, failedErr := NewBillingSession(ctx, info, 4)
		require.Nil(t, failedErr)
		require.NoError(t, failedSession.ReserveBeforeSubmit(7))
		require.NoError(t, failedSession.funding.Refund())
		require.NoError(t, failedSession.funding.Refund())
		require.NoError(t, model.DB.First(&saved, sub.Id).Error)
		require.Equal(t, wantRegular, saved.AmountUsed)
		require.EqualValues(t, 8-wantRegular, saved.ExtraCreditsUsed)
	}
}

func TestSubscriptionExtraCreditsLegacyAdjustments(t *testing.T) {
	truncate(t)
	user := &model.User{Username: "extra-adjustment-user", AffCode: "extra-adjustment-user", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(user).Error)
	plan := &model.SubscriptionPlan{Title: "Extra adjustments", SubscriptionGroup: "vip", TotalAmount: 10}
	require.NoError(t, model.DB.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: common.GetTimestamp() - 1, EndTime: common.GetTimestamp() + 3600, AmountTotal: 10}
	require.NoError(t, model.DB.Create(&sub).Error)
	other := sub
	other.Id = 0
	other.AmountTotal = 100
	require.NoError(t, model.DB.Create(&other).Error)
	_, err := model.PreConsumeUserSubscriptionForGroup("extra-legacy-adjustment", user.Id, "model", 0, 4, "vip")
	require.NoError(t, err)
	require.NoError(t, model.AdjustSubscriptionRequestQuota("extra-legacy-adjustment", 4))
	require.Error(t, model.AdjustSubscriptionRequestQuota("extra-legacy-adjustment", 3), "ordinary requests stay on their original subscription")
	require.NoError(t, model.DB.Model(&sub).Updates(map[string]any{"extra_credits_total": 20, "extra_credits_groups": `["vip"]`, "extra_credits_scoped_groups": `["vip"]`}).Error)
	require.NoError(t, model.AdjustSubscriptionRequestQuota("extra-legacy-adjustment", 3), "a newly purchased balance can fund the existing request")
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 10, sub.AmountUsed)
	require.NoError(t, model.DB.First(&other, other.Id).Error)
	require.EqualValues(t, 1, other.AmountUsed)
	record := model.SubscriptionPreConsumeRecord{RequestId: "old-task-added-reserve", UserId: user.Id, UserSubscriptionId: other.Id, PreConsumed: 2, Status: "consumed"}
	require.NoError(t, model.DB.Create(&record).Error)
	require.NoError(t, model.PostConsumeUserSubscriptionDelta(other.Id, 7))
	require.NoError(t, adjustSubscriptionFunding(other.Id, record.RequestId, -7))
	require.NoError(t, model.DB.First(&other, other.Id).Error)
	require.EqualValues(t, 1, other.AmountUsed)
}

func TestSubscriptionExtraCreditsReservationBoundaries(t *testing.T) {
	truncate(t)
	user := model.User{Username: "extra-boundaries", AffCode: "extra-boundaries", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Boundaries", SubscriptionGroup: "vip", TotalAmount: 100}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: common.GetTimestamp() - 100, EndTime: common.GetTimestamp() + 3600, AmountTotal: 100, LastResetTime: 10, ExtraCreditsTotal: 20, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	_, err := model.PreConsumeUserSubscriptionForGroup("boundary-resumed", user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
	require.NoError(t, model.AdjustSubscriptionRequestQuota("boundary-resumed", 5))
	_, err = model.PreConsumeUserSubscriptionForGroup("boundary-resumed", user.Id, "model", 0, 10, "vip")
	require.Error(t, err, "a reconstructed session must not reserve its additional quota twice")
	require.NoError(t, model.RefundSubscriptionPreConsume("boundary-resumed"))
	// Ordinary requests also retain their original window before buying extras.
	require.NoError(t, model.DB.Model(&sub).Updates(map[string]any{"extra_credits_total": 0, "extra_credits_used": 0}).Error)
	_, err = model.PreConsumeUserSubscriptionForGroup("boundary-old-window", user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&sub).Updates(map[string]any{"last_reset_time": 20, "amount_used": 7, "extra_credits_total": 20}).Error)
	require.NoError(t, model.RefundSubscriptionPreConsume("boundary-old-window"))
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	require.EqualValues(t, 7, sub.AmountUsed, "old-period refund must not remove new-period usage")
	_, err = model.PreConsumeUserSubscriptionForGroup("boundary-complete-after-expiry", user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
	require.NoError(t, model.DB.Model(&sub).Updates(map[string]any{"status": "expired", "end_time": common.GetTimestamp() - 1, "extra_credits_total": 0}).Error)
	require.NoError(t, model.AdjustSubscriptionRequestQuota("boundary-complete-after-expiry", 5), "an existing request may settle against its original period after expiry")
}

func TestSubscriptionExtraCreditsTaskReceipts(t *testing.T) {
	truncate(t)
	user := model.User{Username: "extra-task-receipt", AffCode: "extra-task-receipt", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Extra task receipt", SubscriptionGroup: "vip", TotalAmount: 1}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "expired", StartTime: common.GetTimestamp() - 100, EndTime: common.GetTimestamp() - 1, AmountTotal: 1, ExtraCreditsTotal: 10, ExtraCreditsGroups: `["vip"]`, ExtraCreditsScopedGroups: `["vip"]`}
	require.NoError(t, model.DB.Create(&sub).Error)
	id := "task-receipt-finalization"
	_, err := model.PreConsumeUserSubscriptionForGroup(id, user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
	task := model.Task{UserId: user.Id, TaskID: id, Status: model.TaskStatusSuccess, Quota: 10, PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: sub.Id, SubscriptionRequestID: id}}
	require.NoError(t, model.DB.Create(&task).Error)
	require.NoError(t, model.DB.Model(&sub).UpdateColumn("extra_credits_total", 20).Error)
	require.NoError(t, model.DB.Exec(`CREATE TRIGGER reject_extra_task_quota BEFORE UPDATE OF quota ON tasks BEGIN SELECT RAISE(ABORT, 'injected task quota write failure'); END`).Error)
	RecalculateTaskQuota(context.Background(), &task, 15, common.NewMessage("test rollback"))
	var rolledBack model.UserSubscription
	require.NoError(t, model.DB.First(&rolledBack, sub.Id).Error)
	require.EqualValues(t, 10, rolledBack.ExtraCreditsUsed, "failed task quota write rolls back the extra debit")
	require.NoError(t, model.DB.Exec("DROP TRIGGER reject_extra_task_quota").Error)
	require.NoError(t, model.DB.Model(&sub).UpdateColumn("extra_credits_total", 10).Error)
	RecalculateTaskQuota(context.Background(), &task, 11, common.NewMessage("test settlement"))
	var record model.SubscriptionPreConsumeRecord
	require.NoError(t, model.DB.Where("request_id = ?", id).First(&record).Error)
	require.True(t, record.Durable, "failed final charge keeps receipt")
	_, err = model.AdminDeleteUserSubscription(sub.Id)
	require.Error(t, err, "in-flight extras protected")
	require.NoError(t, model.DB.Model(&sub).UpdateColumn("extra_credits_total", 15).Error)
	RecalculateTaskQuota(context.Background(), &task, 10, common.NewMessage("test settlement"))
	require.Equal(t, 15, task.Quota, "retry settles the first persisted amount even when caller supplies a different one")
	require.NoError(t, model.DB.Where("request_id = ?", id).First(&record).Error)
	require.False(t, record.Durable, "successful settlement completes receipt")
	require.Error(t, model.RefundSubscriptionPreConsume(id), "a completed request cannot be refunded by an older session")
	require.Error(t, model.AdjustSubscriptionRequestQuota(id, -1), "completed allocations are immutable")
	_, err = model.AdminDeleteUserSubscription(sub.Id)
	require.NoError(t, err, "fully consumed final credits do not block management forever")
	task.PrivateData.SubscriptionRequestID = "missing-new-task-receipt"
	require.Error(t, taskAdjustFunding(&task, -1), "new tasks never fall back to ordinary quota if allocation is missing")
}

func TestSubscriptionOnlyBillingDatabaseMatrix(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, dialect := range []struct {
		name common.DatabaseType
		env  string
	}{
		{common.DatabaseTypeSQLite, ""},
		{common.DatabaseTypeMySQL, "TEST_MYSQL_DSN"},
		{common.DatabaseTypePostgreSQL, "TEST_POSTGRES_DSN"},
	} {
		t.Run(string(dialect.name), func(t *testing.T) {
			var driver gorm.Dialector = sqlite.Open(":memory:")
			if dialect.env != "" {
				dsn := os.Getenv(dialect.env)
				if dsn == "" {
					t.Skip(dialect.env + " is not configured")
				}
				if dialect.name == common.DatabaseTypeMySQL {
					driver = mysql.Open(dsn)
				} else {
					driver = postgres.New(postgres.Config{DSN: dsn, PreferSimpleProtocol: true})
				}
			}
			db, err := gorm.Open(driver, &gorm.Config{})
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)
			sqlDB.SetMaxOpenConns(1)
			t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
			oldDB := model.DB
			oldMainType, oldLogType := common.MainDatabaseType(), common.LogDatabaseType()
			model.DB = db
			common.SetDatabaseTypes(dialect.name, oldLogType)
			t.Cleanup(func() {
				model.DB = oldDB
				common.SetDatabaseTypes(oldMainType, oldLogType)
				// The cached group bindings came from this test database's plans.
				_ = model.PurgeSubscriptionPlanCache()
			})
			require.NoError(t, db.AutoMigrate(&model.User{}, &model.SubscriptionPlan{}, &model.UserSubscription{}, &model.SubscriptionPreConsumeRecord{}, &model.Option{}, &model.SystemTask{}))
			versionQuery := "select version()"
			if dialect.name == common.DatabaseTypeSQLite {
				versionQuery = "select sqlite_version()"
			}
			var version string
			require.NoError(t, db.Raw(versionQuery).Scan(&version).Error)
			t.Logf("database: %s", version)

			for _, tc := range []struct {
				name       string
				status     string
				expired    bool
				total      int64
				used       int64
				wallet     int
				wantSource string
				wantError  bool
			}{
				{name: "no subscription uses wallet", wallet: 100, wantSource: BillingSourceWallet},
				{name: "expired subscription uses wallet", status: "active", expired: true, total: 100, wallet: 100, wantSource: BillingSourceWallet},
				{name: "cancelled subscription uses wallet", status: "cancelled", total: 100, wallet: 100, wantSource: BillingSourceWallet},
				{name: "benefits only subscription uses wallet", status: "active", total: -1, wallet: 100, wantSource: BillingSourceWallet},
				{name: "active subscription funds request", status: "active", total: 100, wallet: 100, wantSource: BillingSourceSubscription},
				{name: "exhausted active subscription rejects despite wallet balance", status: "active", total: 100, used: 100, wallet: 100, wantError: true},
				{name: "no subscription and insufficient wallet rejects", wallet: 5, wantError: true},
			} {
				t.Run(tc.name, func(t *testing.T) {
					user := &model.User{Username: tc.name, Status: common.UserStatusEnabled, Group: "default", Quota: tc.wallet}
					require.NoError(t, db.Create(user).Error)
					t.Cleanup(func() { require.NoError(t, db.Unscoped().Delete(user).Error) })
					var subscription model.UserSubscription
					if tc.status != "" {
						allowOverflow := true
						plan := &model.SubscriptionPlan{Title: tc.name, TotalAmount: tc.total, AllowWalletOverflow: &allowOverflow}
						require.NoError(t, db.Create(plan).Error)
						model.InvalidateSubscriptionPlanCache(plan.Id)
						t.Cleanup(func() {
							require.NoError(t, db.Delete(plan).Error)
							model.InvalidateSubscriptionPlanCache(plan.Id)
						})
						end := time.Now().Add(time.Hour).Unix()
						if tc.expired {
							end = time.Now().Add(-time.Hour).Unix()
						}
						subscription = model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: tc.status, StartTime: time.Now().Add(-2 * time.Hour).Unix(), EndTime: end, AmountTotal: tc.total, AmountUsed: tc.used}
						require.NoError(t, db.Create(&subscription).Error)
						t.Cleanup(func() {
							require.NoError(t, db.Where("user_subscription_id = ?", subscription.Id).Delete(&model.SubscriptionPreConsumeRecord{}).Error)
							require.NoError(t, db.Delete(&subscription).Error)
						})
					}
					ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
					ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
					info := &relaycommon.RelayInfo{
						RequestId: fmt.Sprintf("subscription-only-%d", user.Id), UserId: user.Id, UsingGroup: "default", OriginModelName: "test-model",
						IsPlayground: true, ForcePreConsume: true,
						UserSetting: dto.UserSetting{BillingPreference: "subscription_only"},
					}
					session, apiErr := NewBillingSession(ctx, info, 10)
					if tc.wantError {
						require.NotNil(t, apiErr)
						assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
						assert.Nil(t, session)
					} else {
						require.Nil(t, apiErr)
						require.NotNil(t, session)
						assert.Equal(t, tc.wantSource, info.BillingSource)
						require.NoError(t, session.Settle(15))
					}
					assert.Equal(t, "subscription_only", info.UserSetting.BillingPreference)
					var reloaded model.User
					require.NoError(t, db.First(&reloaded, user.Id).Error)
					wantWallet := tc.wallet
					if tc.wantSource == BillingSourceWallet {
						wantWallet -= 15
					}
					assert.Equal(t, wantWallet, reloaded.Quota)
					if subscription.Id > 0 {
						require.NoError(t, db.First(&subscription, subscription.Id).Error)
						wantUsed := tc.used
						if tc.wantSource == BillingSourceSubscription {
							wantUsed += 15
						}
						assert.Equal(t, wantUsed, subscription.AmountUsed)
					}
				})
			}
			t.Run("scoped_and_legacy_subscriptions_coexist", verifyCoexistingSubscriptions)
		})
	}
}

func verifyCoexistingSubscriptions(t *testing.T) {
	user := &model.User{Username: "coexisting-subscriptions", Password: "password", Status: common.UserStatusEnabled, Group: "default", Quota: 100}
	require.NoError(t, model.DB.Create(user).Error)
	legacy := &model.SubscriptionPlan{Title: "Legacy", TotalAmount: 100, AllowWalletOverflow: common.GetPointer(true)}
	scoped := &model.SubscriptionPlan{Title: "Scoped", TotalAmount: 100, UpgradeGroup: "vip", AllowWalletOverflow: common.GetPointer(false)}
	require.NoError(t, common.UnmarshalJsonStr(`{"subscription_group":"vip"}`, scoped))
	for _, plan := range []*model.SubscriptionPlan{legacy, scoped} {
		require.NoError(t, model.DB.Create(plan).Error)
		model.InvalidateSubscriptionPlanCache(plan.Id)
	}
	now := common.GetTimestamp()
	legacySub := model.UserSubscription{UserId: user.Id, PlanId: legacy.Id, Status: "active", StartTime: now - 60, EndTime: now + 120, AmountTotal: 100, AllowWalletOverflow: true}
	scopedSub := model.UserSubscription{UserId: user.Id, PlanId: scoped.Id, Status: "active", StartTime: now - 60, EndTime: now + 240, AmountTotal: 100}
	require.NoError(t, model.DB.Create(&legacySub).Error)
	require.NoError(t, model.DB.Create(&scopedSub).Error)
	_, eligible, err := model.GetActiveSubscriptionAvailabilityForGroup(user.Id, "vip")
	require.NoError(t, err)
	assert.True(t, eligible, "the bound plan covers its group without a second activation step")
	require.NoError(t, model.DB.Model(&legacySub).Update("status", "cancelled").Error)
	_, eligible, err = model.GetActiveSubscriptionAvailabilityForGroup(user.Id, "vip")
	require.NoError(t, err)
	assert.True(t, eligible, "the bound plan works without a legacy subscription")
	require.NoError(t, model.DB.Model(&legacySub).Update("status", "active").Error)
	t.Run("ignores_ordinary_ratio_for_automatically_bound_group", func(t *testing.T) {
		oldRatios := ratio_setting.GroupRatio2JSONString()
		t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(oldRatios)) })
		require.NoError(t, ratio_setting.UpdateGroupRatioByJSONString(`{"default":1,"vip":2}`))
		ratio, special, err := GetUserGroupRatio(user.Group, "vip")
		require.NoError(t, err)
		assert.Equal(t, 1.0, ratio)
		assert.False(t, special)
	})
	require.NoError(t, model.DB.Model(&scopedSub).Update("status", "cancelled").Error)
	_, allowed := GetUserUsableGroupsForUser(user.Id, user.Group)["vip"]
	assert.False(t, allowed, "a legacy subscription alone does not grant access to a scoped group")
	require.NoError(t, model.DB.Model(&scopedSub).Update("status", "active").Error)
	_, allowed = GetUserUsableGroupsForUser(user.Id, user.Group)["vip"]
	assert.True(t, allowed, "holding a scoped subscription grants access without replacing the user's group")
	bill := func(group, preference, requestID string) (*relaycommon.RelayInfo, *types.NewAPIError) {
		ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
		ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
		info := &relaycommon.RelayInfo{RequestId: requestID, UserId: user.Id, UserGroup: user.Group, UsingGroup: group, IsPlayground: true, ForcePreConsume: true, UserSetting: dto.UserSetting{BillingPreference: preference}}
		session, apiErr := NewBillingSession(ctx, info, 10)
		if apiErr == nil {
			require.NoError(t, session.Settle(15))
		}
		return info, apiErr
	}
	ordinary, apiErr := bill("default", "subscription_first", "legacy-normal-group")
	require.Nil(t, apiErr)
	assert.Equal(t, legacySub.Id, ordinary.SubscriptionId)
	vip, apiErr := bill("vip", "wallet_first", "scoped-subscription-group")
	require.Nil(t, apiErr)
	assert.Equal(t, scopedSub.Id, vip.SubscriptionId, "a subscription group must not consume from the wallet or the legacy plan first")
	for _, sub := range []*model.UserSubscription{&legacySub, &scopedSub} {
		require.NoError(t, model.DB.First(sub, sub.Id).Error)
		assert.EqualValues(t, 15, sub.AmountUsed)
	}
	require.NoError(t, model.DB.Model(&model.UserSubscription{}).Where("id = ?", scopedSub.Id).Update("amount_used", 100).Error)
	_, apiErr = bill("vip", "wallet_only", "scoped-wallet-denied")
	require.NotNil(t, apiErr, "without wallet overflow, neither the wallet preference nor the legacy plan may fund a subscription group")
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	var reloaded model.User
	require.NoError(t, model.DB.First(&reloaded, user.Id).Error)
	assert.Equal(t, 100, reloaded.Quota)

	require.NoError(t, model.DB.Model(scoped).Update("allow_wallet_overflow", true).Error)
	model.InvalidateSubscriptionPlanCache(scoped.Id)
	_, apiErr = bill("vip", "subscription_only", "scoped-wallet-opt-out")
	require.NotNil(t, apiErr)
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	vip, apiErr = bill("vip", "subscription_first", "scoped-wallet-permitted")
	require.Nil(t, apiErr)
	assert.Equal(t, BillingSourceWallet, vip.BillingSource)
	assert.Zero(t, vip.SubscriptionId)

	require.NoError(t, model.DB.Model(&scopedSub).Update("amount_total", -1).Error)
	vip, apiErr = bill("vip", "subscription_first", "scoped-benefits-wallet")
	require.Nil(t, apiErr)
	assert.Equal(t, BillingSourceWallet, vip.BillingSource)

	require.NoError(t, model.DB.Model(&scopedSub).Update("end_time", now-1).Error)
	_, apiErr = bill("vip", "wallet_first", "scoped-entitlement-expired")
	require.NotNil(t, apiErr, "an expired subscription does not authorize wallet fallback")
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	require.NoError(t, model.DB.First(&reloaded, user.Id).Error)
	assert.Equal(t, 70, reloaded.Quota)
}

func TestNewBillingSessionUsesEffectiveRequestGroupForSubscriptionWhitelist(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)

	user := &model.User{
		Username: "subscription-whitelist",
		Password: "password",
		Status:   common.UserStatusEnabled,
		Group:    "default",
		Quota:    100,
	}
	require.NoError(t, model.DB.Create(user).Error)
	plan := &model.SubscriptionPlan{
		Title:                   "VIP subscription",
		DurationUnit:            model.SubscriptionDurationMonth,
		DurationValue:           1,
		TotalAmount:             100,
		WalletOnlyGroupsEnabled: true,
		WalletOnlyGroupsMode:    "whitelist",
		WalletOnlyGroups:        "vip",
	}
	require.NoError(t, model.DB.Create(plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	subscription := &model.UserSubscription{
		UserId:      user.Id,
		PlanId:      plan.Id,
		AmountTotal: plan.TotalAmount,
		Status:      "active",
		StartTime:   time.Now().Add(-time.Hour).Unix(),
		EndTime:     time.Now().Add(time.Hour).Unix(),
	}
	require.NoError(t, model.DB.Create(subscription).Error)

	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{
		RequestId:       "subscription-whitelist-request",
		UserId:          user.Id,
		UserGroup:       user.Group,
		UsingGroup:      "vip",
		OriginModelName: "test-model",
		IsPlayground:    true,
		UserSetting: dto.UserSetting{
			BillingPreference: "subscription_first",
		},
	}

	session, apiErr := NewBillingSession(ctx, info, 10)
	require.Nil(t, apiErr)
	require.NotNil(t, session)
	assert.Equal(t, BillingSourceSubscription, info.BillingSource)
	assert.Equal(t, subscription.Id, info.SubscriptionId)

	var reloadedUser model.User
	require.NoError(t, model.DB.Select("quota").First(&reloadedUser, user.Id).Error)
	assert.Equal(t, 100, reloadedUser.Quota)
	var reloadedSubscription model.UserSubscription
	require.NoError(t, model.DB.Select("amount_used").First(&reloadedSubscription, subscription.Id).Error)
	assert.EqualValues(t, 10, reloadedSubscription.AmountUsed)
}

func TestSubscriptionPurchaseLogMaintenance(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.SubscriptionExtraCreditPurchase{}))
	var receipts []model.SubscriptionExtraCreditPurchase
	for i := range 2 {
		id := fmt.Sprintf("maintenance-purchase-%d", i)
		user := model.User{Username: id, AffCode: id, Group: "default", Quota: 10}
		require.NoError(t, model.DB.Create(&user).Error)
		plan := model.SubscriptionPlan{Title: id, SubscriptionGroup: "vip", AllowExtraCredits: true, ExtraCreditsRate: 1}
		require.NoError(t, model.DB.Create(&plan).Error)
		sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", EndTime: common.GetTimestamp() + 3600}
		require.NoError(t, model.DB.Create(&sub).Error)
		if i == 0 {
			require.NoError(t, model.DB.Exec("CREATE TRIGGER offline_purchases BEFORE INSERT ON logs BEGIN SELECT RAISE(ABORT,'offline'); END").Error)
			t.Cleanup(func() { model.DB.Exec("DROP TRIGGER IF EXISTS offline_purchases") })
		}
		r, err := model.PurchaseSubscriptionExtraCredits(user.Id, sub.Id, 1, id, 1)
		require.NoError(t, err)
		receipts = append(receipts, *r)
	}
	t.Cleanup(func() {
		model.DB.Where("request_id IN ?", []string{"maintenance-purchase-0", "maintenance-purchase-1"}).Delete(&model.SubscriptionExtraCreditPurchase{})
	})
	require.NoError(t, model.DB.Exec("DROP TRIGGER offline_purchases").Error)
	require.NoError(t, model.DB.Exec(fmt.Sprintf("CREATE TRIGGER offline_purchases BEFORE INSERT ON logs WHEN NEW.user_id=%d BEGIN SELECT RAISE(ABORT,'offline'); END", receipts[0].UserId)).Error)
	runSubscriptionQuotaResetOnce()
	var pending []int
	require.NoError(t, model.DB.Model(&model.SubscriptionExtraCreditPurchase{}).Where("log_pending = ?", true).Order("id").Pluck("id", &pending).Error)
	require.Equal(t, []int{receipts[0].Id}, pending)
	require.NoError(t, model.DB.Exec("DROP TRIGGER offline_purchases").Error)
	runSubscriptionQuotaResetOnce()
	runSubscriptionQuotaResetOnce()
	for _, r := range receipts {
		var count int64
		require.NoError(t, model.DB.Model(&model.Log{}).Where("subscription_event_id = ? AND user_id = ?", fmt.Sprintf("extra-purchase:%d", r.Id), r.UserId).Count(&count).Error)
		require.EqualValues(t, 1, count)
	}
}

// A task route reserves absolutely (ForcePreConsume), but a task row that never
// persisted can no longer settle later: the session must close the receipt now.
func TestSubscriptionTaskWithoutPersistedRowSettlesAbsolutely(t *testing.T) {
	for _, tc := range []struct {
		name   string
		actual int
	}{
		{name: "final_amount_differs", actual: 15},
		{name: "final_amount_matches_reservation", actual: 10},
		{name: "commit_rollback", actual: 15},
	} {
		t.Run(tc.name, func(t *testing.T) {
			truncate(t)
			user := model.User{Username: tc.name, AffCode: tc.name, Group: "default", Quota: 100, Status: common.UserStatusEnabled}
			require.NoError(t, model.DB.Create(&user).Error)
			plan := model.SubscriptionPlan{Title: tc.name, SubscriptionGroup: "vip", TotalAmount: 100}
			require.NoError(t, model.DB.Create(&plan).Error)
			model.InvalidateSubscriptionPlanCache(plan.Id)
			now := common.GetTimestamp()
			sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100}
			require.NoError(t, model.DB.Create(&sub).Error)
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			ctx.Request = httptest.NewRequest("POST", "/v1/videos", nil)
			info := &relaycommon.RelayInfo{RequestId: tc.name, UserId: user.Id, UserGroup: user.Group, UsingGroup: "vip", OriginModelName: "test-model", IsPlayground: true, ForcePreConsume: true, UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
			session, apiErr := NewBillingSession(ctx, info, 10)
			require.Nil(t, apiErr)
			// The response was written before the task insert failed, so the bound
			// task never received a row and never owns the settlement.
			task := &model.Task{UserId: user.Id, PrivateData: model.TaskPrivateData{BillingSource: BillingSourceSubscription, SubscriptionId: sub.Id, SubscriptionRequestID: info.RequestId}}
			session.BindTask(task)
			if tc.name == "commit_rollback" {
				require.NoError(t, model.DB.Exec("PRAGMA foreign_keys=ON").Error)
				t.Cleanup(func() { model.DB.Exec("PRAGMA foreign_keys=OFF") })
				require.NoError(t, model.DB.Exec("CREATE TABLE commit_probe (owner INTEGER REFERENCES users(id) DEFERRABLE INITIALLY DEFERRED)").Error)
				t.Cleanup(func() { model.DB.Exec("DROP TABLE commit_probe") })
				err := model.DB.Transaction(func(tx *gorm.DB) error {
					if err := tx.Create(task).Error; err != nil {
						return err
					}
					return tx.Exec("INSERT INTO commit_probe VALUES (-1)").Error
				})
				require.ErrorContains(t, err, "FOREIGN KEY constraint failed")
				require.Positive(t, task.ID)
				// SQLite keeps a failed deferred COMMIT open until explicitly rolled back.
				require.NoError(t, model.DB.Exec("ROLLBACK").Error)
				require.ErrorIs(t, model.DB.First(&model.Task{}, task.ID).Error, gorm.ErrRecordNotFound)
				require.NoError(t, model.DB.Migrator().RenameTable("tasks", "tasks_unavailable"))
				err = session.Settle(tc.actual)
				require.NoError(t, model.DB.Migrator().RenameTable("tasks_unavailable", "tasks"))
				require.ErrorContains(t, err, "no such table")
				require.False(t, session.fundingSettled)
			}
			require.NoError(t, session.Settle(tc.actual))
			var record model.SubscriptionPreConsumeRecord
			require.NoError(t, model.DB.Where("request_id = ?", info.RequestId).First(&record).Error)
			require.Equal(t, "settled", record.Status, "an orphan task must not leave the receipt open for a poller")
			require.False(t, record.Durable)
			require.EqualValues(t, tc.actual, record.ReservedTotal)
			var saved model.UserSubscription
			require.NoError(t, model.DB.First(&saved, sub.Id).Error)
			require.EqualValues(t, tc.actual, saved.AmountUsed)
			require.Zero(t, saved.FundingReserved, "absolute settlement releases the whole reservation")
			// Retrying the same settlement stays idempotent.
			require.NoError(t, session.Settle(tc.actual))
			require.NoError(t, model.SettleSubscriptionRequestQuota(info.RequestId, int64(tc.actual)))
		})
	}
}

func TestSubscriptionGroupActivationPreservesCurrentLegacyCycle(t *testing.T) {
	truncate(t)
	user := model.User{Username: "legacy-current-cycle", Group: "vip", Status: common.UserStatusEnabled, Quota: 100}
	require.NoError(t, model.DB.Create(&user).Error)
	legacy := model.SubscriptionPlan{Title: "Old", TotalAmount: 100, WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip"}
	require.NoError(t, model.DB.Create(&legacy).Error)
	model.InvalidateSubscriptionPlanCache(legacy.Id)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: legacy.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100, PriceAmount: common.GetPointer(2.0)}
	require.NoError(t, model.DB.Create(&sub).Error)
	binding := model.SubscriptionPlan{Title: "New", SubscriptionGroup: "vip"}
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&binding))
	assert.Contains(t, GetUserUsableGroupsForUser(user.Id, user.Group), "vip")
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{RequestId: "grandfather-real-charge", UserId: user.Id, UserGroup: user.Group, UsingGroup: "vip", IsPlayground: true, ForcePreConsume: true, UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
	session, apiErr := NewBillingSession(ctx, info, 10)
	require.Nil(t, apiErr)
	require.NoError(t, session.Settle(15))
	assert.Equal(t, sub.Id, info.SubscriptionId)
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	assert.EqualValues(t, 15, sub.AmountUsed)
	require.NoError(t, model.DB.Model(&sub).Update("end_time", now-1).Error)
	assert.NotContains(t, GetUserUsableGroupsForUser(user.Id, user.Group), "vip")
	newSub := model.UserSubscription{UserId: user.Id, PlanId: legacy.Id, Status: "active", StartTime: now, EndTime: now + 3600, AmountTotal: 100, PriceAmount: common.GetPointer(2.0)}
	require.NoError(t, model.DB.Create(&newSub).Error)
	assert.NotContains(t, GetUserUsableGroupsForUser(user.Id, user.Group), "vip", "a later legacy purchase does not grant subscription access")
}

func TestSubscriptionBindingRetainsLegacyAccessWithoutExpandingFunding(t *testing.T) {
	truncate(t)
	oldGroups := setting.UserUsableGroups2JSONString()
	t.Cleanup(func() { require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(oldGroups)) })
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	now := common.GetTimestamp()
	var users []*model.User
	for _, fixture := range []struct {
		name, group, upgradeGroup string
		quota                     int64
	}{
		{"already-assigned", "vip", "vip", -1},
		{"not-previously-authorized", "default", "", 100},
		{"unrelated-benefits-only", "vip", "", -1},
	} {
		u := &model.User{Username: fixture.name, AffCode: fixture.name, Group: fixture.group, Status: common.UserStatusEnabled, Quota: 100}
		require.NoError(t, model.DB.Create(u).Error)
		users = append(users, u)
		plan := &model.SubscriptionPlan{Title: fixture.name, TotalAmount: fixture.quota, UpgradeGroup: fixture.upgradeGroup}
		require.NoError(t, model.DB.Create(plan).Error)
		model.InvalidateSubscriptionPlanCache(plan.Id)
		sub := &model.UserSubscription{UserId: u.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: fixture.quota, PriceAmount: common.GetPointer(2.0), UpgradeGroup: fixture.upgradeGroup}
		require.NoError(t, model.DB.Create(sub).Error)
	}
	binding := &model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "vip", TotalAmount: 100}
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(binding))
	assert.Contains(t, GetUserUsableGroupsForUser(users[0].Id, users[0].Group), "vip", "paid benefits-only cycle retains ordinary access")
	allow, err := model.UserActiveSubscriptionsAllowWalletOverflowForGroup(users[0].Id, "vip")
	require.NoError(t, err)
	assert.True(t, allow, "old wallet-only access still uses the wallet")
	assert.NotContains(t, GetUserUsableGroupsForUser(users[1].Id, users[1].Group), "vip")
	_, eligible, err := model.GetActiveSubscriptionAvailabilityForGroup(users[1].Id, "vip")
	require.NoError(t, err)
	assert.False(t, eligible, "the legacy cycle must not fund a group the user could not previously use")
	assert.NotContains(t, GetUserUsableGroupsForUser(users[2].Id, users[2].Group), "vip", "an unrelated paid plan cannot preserve ordinary group access")
	allow, err = model.UserActiveSubscriptionsAllowWalletOverflowForGroup(users[2].Id, "vip")
	require.NoError(t, err)
	assert.False(t, allow, "an unrelated paid plan cannot authorize wallet fallback")
}

func TestSubscriptionBindingRetainsPaidHistoricalPeriodAfterPriceChange(t *testing.T) {
	truncate(t)
	user := model.User{Username: "legacy-paid-before-price-change", AffCode: "legacy-old-price", Group: "vip", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	legacy := model.SubscriptionPlan{Title: "Now free", PriceAmount: 0, TotalAmount: 100, WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip"}
	require.NoError(t, model.DB.Create(&legacy).Error)
	model.InvalidateSubscriptionPlanCache(legacy.Id)
	now := common.GetTimestamp()
	paid := &model.UserSubscription{UserId: user.Id, PlanId: legacy.Id, Status: "active", Source: "order", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100}
	free := &model.UserSubscription{UserId: user.Id, PlanId: legacy.Id, Status: "active", Source: "order", StartTime: now - 120, EndTime: now + 3600, AmountTotal: 100}
	require.NoError(t, model.DB.Create(paid).Error)
	require.NoError(t, model.DB.Create(free).Error)
	require.NoError(t, model.DB.Model(paid).UpdateColumn("created_at", paid.StartTime).Error)
	require.NoError(t, model.DB.Model(free).UpdateColumn("created_at", free.StartTime).Error)
	require.NoError(t, model.DB.Create(&model.SubscriptionOrder{UserId: user.Id, PlanId: legacy.Id, TradeNo: "already-paid-before-price-change", Money: 2, Status: common.TopUpStatusSuccess, CreateTime: paid.StartTime - 10, CompleteTime: paid.StartTime + 10}).Error)
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "New scoped plan", SubscriptionGroup: "vip"}))
	require.NoError(t, model.DB.First(paid, paid.Id).Error)
	require.NoError(t, model.DB.First(free, free.Id).Error)
	assert.JSONEq(t, `{"access":["vip"],"quota":["vip"]}`, paid.LegacyCycleGroups)
	assert.Empty(t, free.LegacyCycleGroups, "a different period for the same user and plan must not inherit the payment")
}

func TestSubscriptionBindingRetainsBothPaidPeriodsCreatedInSameSecond(t *testing.T) {
	testBindingRetainsNearbyPaidPeriods(t, 0)
}

func TestSubscriptionBindingRetainsPaidPeriodWhenNextStartsAtSettlement(t *testing.T) {
	testBindingRetainsNearbyPaidPeriods(t, 2)
}

func testBindingRetainsNearbyPaidPeriods(t *testing.T, spacing int64) {
	t.Helper()
	truncate(t)
	withOrdinaryGroupSettings(t, `{"default":"Default","vip":"VIP"}`, `{"default":1,"vip":1}`, `[]`, nil)
	user := model.User{Username: "legacy-same-second", AffCode: "legacy-same-second", Group: "vip"}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Historical orders", TotalAmount: 100}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	var cycles []model.UserSubscription
	for i, tradeNo := range []string{"legacy-same-second-1", "legacy-same-second-2"} {
		cycle := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Source: "order", Status: "active", StartTime: now - 60 + int64(i)*spacing, EndTime: now + 3600, AmountTotal: 100}
		require.NoError(t, model.DB.Create(&cycle).Error)
		require.NoError(t, model.DB.Model(&cycle).UpdateColumn("created_at", cycle.StartTime).Error)
		require.NoError(t, model.DB.Create(&model.SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, TradeNo: tradeNo, Status: common.TopUpStatusSuccess, Money: 2, CreateTime: now - 120, CompleteTime: cycle.StartTime + 2}).Error)
		cycles = append(cycles, cycle)
	}
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "vip"}))
	for _, cycle := range cycles {
		require.NoError(t, model.DB.First(&cycle, cycle.Id).Error)
		assert.JSONEq(t, `{"access":["vip"],"quota":["vip"]}`, cycle.LegacyCycleGroups)
	}
}

func TestSubscriptionBindingPreservesPendingLegacyCheckout(t *testing.T) {
	truncate(t)
	user := model.User{Username: "legacy-pending-checkout", AffCode: "legacy-pending", Group: "default", Status: common.UserStatusEnabled, Quota: 100}
	require.NoError(t, model.DB.Create(&user).Error)
	legacy := model.SubscriptionPlan{Title: "Old checkout", Enabled: true, PriceAmount: 2, TotalAmount: 100, UpgradeGroup: "vip", WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip"}
	require.NoError(t, model.DB.Create(&legacy).Error)
	model.InvalidateSubscriptionPlanCache(legacy.Id)
	order := &model.SubscriptionOrder{UserId: user.Id, PlanId: legacy.Id, TradeNo: "paid-legacy-after-first-binding", Status: common.TopUpStatusPending, Money: 2, PaymentCurrency: "USD", PaymentProvider: model.PaymentProviderStripe}
	require.NoError(t, order.Insert())
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "New scoped plan", SubscriptionGroup: "vip"}))
	blocked := &model.SubscriptionOrder{UserId: user.Id, PlanId: legacy.Id, TradeNo: "new-checkout-after-first-binding", Status: common.TopUpStatusPending}
	require.ErrorContains(t, blocked.Insert(), "no longer has an available subscription group")
	require.NoError(t, model.CompleteSubscriptionOrder(order.TradeNo, "", model.PaymentProviderStripe, model.PaymentMethodStripe))
	assert.Contains(t, GetUserUsableGroupsForUser(user.Id, "vip"), "vip")
	_, covers, err := model.GetActiveSubscriptionAvailabilityForGroup(user.Id, "vip")
	require.NoError(t, err)
	assert.True(t, covers, "the checkout kept its original quota rights")
}

func TestSubscriptionBindingRetainsRepeatedPendingBenefitsCheckout(t *testing.T) {
	truncate(t)
	withOrdinaryGroupSettings(t, `{"default":"Default"}`, `{"default":1,"vip":1,"special":1}`, `[]`, map[string]map[string]string{"vip": {"+:special": "Special"}})
	user := model.User{Username: "repeat-benefits-checkout", AffCode: "repeat-benefits", Group: "vip", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Benefits", PriceAmount: 2, Enabled: true, TotalAmount: -1, UpgradeGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := common.GetTimestamp()
	first := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 60, EndTime: now + 3600, Status: "active", AmountTotal: -1, PriceAmount: common.GetPointer(2.0), UpgradeGroup: "vip", PrevUserGroup: "default"}
	require.NoError(t, model.DB.Create(&first).Error)
	order := model.SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, Money: 2, Status: common.TopUpStatusPending, TradeNo: "repeat-benefits-checkout", PaymentProvider: model.PaymentProviderStripe}
	require.NoError(t, order.Insert())
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "special"}))
	require.NoError(t, model.DB.Model(&first).Updates(map[string]any{"status": "expired", "end_time": now - 1}).Error)
	require.NoError(t, model.CompleteSubscriptionOrder(order.TradeNo, "", model.PaymentProviderStripe, model.PaymentMethodStripe))
	assert.Contains(t, GetUserUsableGroupsForUser(user.Id, "vip"), "special")
}

func TestSubscriptionBindingRetainsCheckoutBeforeAnotherPlanChangesIdentity(t *testing.T) {
	truncate(t)
	withOrdinaryGroupSettings(t, `{"default":"Default"}`, `{"default":1,"vip":1,"special":1}`, `[]`, map[string]map[string]string{"vip": {"+:special": "Special"}})
	user := model.User{Username: "checkout-identity", AffCode: "checkout-identity", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Benefits", PriceAmount: 2, Enabled: true, TotalAmount: -1, UpgradeGroup: "vip"}
	other := plan
	other.Title = "Other benefits"
	require.NoError(t, model.DB.Create(&plan).Error)
	require.NoError(t, model.DB.Create(&other).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := common.GetTimestamp()
	order := model.SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, Money: 2, CreateTime: now - 120, Status: common.TopUpStatusPending, TradeNo: "checkout-identity", PaymentProvider: model.PaymentProviderStripe}
	require.NoError(t, order.Insert())
	first := model.UserSubscription{UserId: user.Id, PlanId: other.Id, StartTime: now - 60, EndTime: now + 3600, Status: "active", AmountTotal: -1, PriceAmount: common.GetPointer(2.0), UpgradeGroup: "vip", PrevUserGroup: "default"}
	require.NoError(t, model.DB.Create(&first).Error)
	require.NoError(t, model.DB.Model(&user).UpdateColumn("group", "vip").Error)
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "special"}))
	require.NoError(t, model.DB.Model(&first).Updates(map[string]any{"status": "expired", "end_time": now - 1}).Error)
	require.NoError(t, model.DB.Model(&user).UpdateColumn("group", "default").Error)
	require.NoError(t, model.CompleteSubscriptionOrder(order.TradeNo, "", model.PaymentProviderStripe, model.PaymentMethodStripe))
	assert.Contains(t, GetUserUsableGroupsForUser(user.Id, "vip"), "special")
}

func TestSubscriptionBindingRetainsAccessAcrossIdentityUpgrades(t *testing.T) {
	truncate(t)
	withOrdinaryGroupSettings(t, `{"default":"Default"}`, `{"default":1,"vip":1,"pro":1,"special":1}`, `[]`, map[string]map[string]string{
		"vip": {"+:special": "Special"}, "pro": {"+:special": "Special"},
	})
	user := model.User{Username: "upgrade-chain", AffCode: "upgrade-chain", Group: "pro"}
	require.NoError(t, model.DB.Create(&user).Error)
	now := common.GetTimestamp()
	for _, edge := range []struct{ from, to string }{{"default", "vip"}, {"vip", "pro"}} {
		plan := model.SubscriptionPlan{Title: edge.to, PriceAmount: 2, TotalAmount: -1, UpgradeGroup: edge.to}
		require.NoError(t, model.DB.Create(&plan).Error)
		require.NoError(t, model.DB.Create(&model.UserSubscription{UserId: user.Id, PlanId: plan.Id, StartTime: now - 60, EndTime: now + 3600, Status: "active", AmountTotal: -1, PriceAmount: common.GetPointer(2.0), UpgradeGroup: edge.to, PrevUserGroup: edge.from}).Error)
	}
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "special"}))
	assert.Contains(t, GetUserUsableGroupsForUser(user.Id, "pro"), "special")
}

func TestSubscriptionBindingRejectsPurchaseWithNeitherQuotaNorNewAccess(t *testing.T) {
	truncate(t)
	withOrdinaryGroupSettings(t, `{"default":"Default","special":"Special"}`, `{"default":1,"vip":1,"special":1}`, `[]`, nil)
	user := model.User{Username: "vip-without-new-benefit", AffCode: "no-new-benefit", Group: "vip", Quota: 10 * int(common.QuotaPerUnit)}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "No deliverable benefit", PriceAmount: 1, Enabled: true, TotalAmount: 100, UpgradeGroup: "vip", WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "special"}
	require.NoError(t, model.DB.Create(&plan).Error)
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "special"}))
	order := model.SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, TradeNo: "no-new-benefit", Money: 1, Status: common.TopUpStatusPending}
	require.ErrorContains(t, order.Insert(), "no longer has an available subscription group")
}

func TestSubscriptionRebindingReassessesRetainedQuota(t *testing.T) {
	truncate(t)
	user := model.User{Username: "legacy-cycle-rebind", AffCode: "legacy-rebind", Group: "vip", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Legacy", TotalAmount: 100, UpgradeGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100, PriceAmount: common.GetPointer(2.0), UpgradeGroup: "vip"}
	require.NoError(t, model.DB.Create(&sub).Error)
	first := model.SubscriptionPlan{Title: "First binding", SubscriptionGroup: "vip"}
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&first))
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	require.JSONEq(t, `{"access":["vip"],"quota":["vip"]}`, sub.LegacyCycleGroups)
	require.NoError(t, model.DeleteSubscriptionPlan(first.Id))
	require.NoError(t, model.DB.Model(&plan).Updates(map[string]any{"wallet_only_groups_enabled": true, "wallet_only_groups_mode": "blacklist", "wallet_only_groups": "vip"}).Error)
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "Second binding", SubscriptionGroup: "vip"}))
	require.NoError(t, model.DB.First(&sub, sub.Id).Error)
	require.JSONEq(t, `{"access":["vip"]}`, sub.LegacyCycleGroups)
}

func TestSubscriptionBindingRetainsWalletGroupOpenedByPaidUpgrade(t *testing.T) {
	truncate(t)
	withOrdinaryGroupSettings(t, `{"default":"Default"}`, `{"default":1,"vip":1,"special":1}`, `[]`, map[string]map[string]string{
		"vip": {"+:special": "Special"},
	})
	user := model.User{Username: "legacy-special-access", AffCode: "legacy-special", Group: "vip", Quota: 100, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Paid benefits", TotalAmount: -1, UpgradeGroup: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: -1, PriceAmount: common.GetPointer(2.0), UpgradeGroup: "vip", PrevUserGroup: "default"}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "New scoped plan", SubscriptionGroup: "special"}))
	assert.Contains(t, GetUserUsableGroupsForUser(user.Id, user.Group), "special")
	allow, err := model.UserActiveSubscriptionsAllowWalletOverflowForGroup(user.Id, "special")
	require.NoError(t, err)
	assert.True(t, allow)
}

func TestSubscriptionBindingRechecksTemporaryAccessRules(t *testing.T) {
	truncate(t)
	require.NoError(t, model.DB.AutoMigrate(&model.UserOAuthBinding{}, &model.CustomOAuthProvider{}))
	withOrdinaryGroupSettings(t, `{"default":"Default","vip":"VIP"}`, `{"default":1,"vip":1}`, `[{"group":"vip","conditions":[{"type":"min_quota","min_quota":100}]}]`, nil)
	user := model.User{Username: "temporary-access", AffCode: "temp-access", Group: "default", Quota: 10, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Legacy", Enabled: true, PriceAmount: 1, TotalAmount: 100, WalletOnlyGroupsEnabled: true, WalletOnlyGroupsMode: "whitelist", WalletOnlyGroups: "vip"}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100, PriceAmount: common.GetPointer(2.0)}
	require.NoError(t, model.DB.Create(&sub).Error)
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "vip"}))
	assert.NotContains(t, GetUserUsableGroupsForUser(user.Id, user.Group), "vip")
	_, err := model.PreConsumeUserSubscriptionForGroup("temporary-access-denied", user.Id, "model", 0, 10, "vip")
	require.Error(t, err)
	order := model.SubscriptionOrder{UserId: user.Id, PlanId: plan.Id, TradeNo: "temporarily-ineligible-checkout", Status: common.TopUpStatusPending}
	assert.ErrorContains(t, order.Insert(), "no longer has an available subscription group")
	require.NoError(t, model.DB.Model(&user).Update("quota", 100).Error)
	assert.Contains(t, GetUserUsableGroupsForUser(user.Id, user.Group), "vip")
	_, err = model.PreConsumeUserSubscriptionForGroup("temporary-access-restored", user.Id, "model", 0, 10, "vip")
	require.NoError(t, err)
}

func TestSubscriptionBillingUsesExactStoredGroup(t *testing.T) {
	truncate(t)
	const group = "vip " // Existing stored values are compared literally.
	user := model.User{Username: "exact-billing-group", AffCode: "exact-billing", Group: "default", Quota: 100, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: "Exact name", SubscriptionGroup: group, TotalAmount: 100, AllowWalletOverflow: common.GetPointer(false)}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100}
	require.NoError(t, model.DB.Create(&sub).Error)
	_, eligible, err := model.GetActiveSubscriptionAvailabilityForGroup(user.Id, group)
	require.NoError(t, err)
	require.True(t, eligible)
	result, err := model.PreConsumeUserSubscriptionForGroup("exact-stored-group", user.Id, "model", 0, 10, group)
	require.NoError(t, err)
	assert.Equal(t, sub.Id, result.UserSubscriptionId)
	require.NoError(t, model.DB.Model(&sub).Updates(map[string]any{"amount_used": 100, "allow_wallet_overflow": true}).Error)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{RequestId: "exact-stored-overflow", UserId: user.Id, UserGroup: user.Group, UsingGroup: group, IsPlayground: true, ForcePreConsume: true, UserSetting: dto.UserSetting{BillingPreference: "subscription_first"}}
	_, apiErr := NewBillingSession(ctx, info, 10)
	require.NotNil(t, apiErr, "the current plan forbids wallet overflow even if the period formerly allowed it")
	assert.Equal(t, types.ErrorCodeInsufficientUserQuota, apiErr.GetErrorCode())
	require.NoError(t, model.DB.First(&user, user.Id).Error)
	assert.Equal(t, 100, user.Quota)
}

func TestScopedSubscriptionRemainsEligibleWhenLegacyGroupIsNoLongerOrdinary(t *testing.T) {
	truncate(t)
	withOrdinaryGroupSettings(t, `{"default":"Default","vip":"VIP"}`, `{"default":1,"vip":1}`, `[]`, nil)
	user := model.User{Username: "mixed-scoped-access", AffCode: "mixed-scoped", Group: "default"}
	require.NoError(t, model.DB.Create(&user).Error)
	legacy := model.SubscriptionPlan{Title: "Legacy", TotalAmount: 100}
	require.NoError(t, model.DB.Create(&legacy).Error)
	model.InvalidateSubscriptionPlanCache(legacy.Id)
	now := common.GetTimestamp()
	start := model.UserSubscription{UserId: user.Id, PlanId: legacy.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, PriceAmount: common.GetPointer(1.0), AmountTotal: 100}
	require.NoError(t, model.DB.Create(&start).Error)
	scoped := model.SubscriptionPlan{Title: "Scoped", SubscriptionGroup: "vip", TotalAmount: 100}
	require.NoError(t, model.CreateSubscriptionPlanWithBinding(&scoped))
	model.InvalidateSubscriptionPlanCache(scoped.Id)
	require.NoError(t, model.DB.First(&start, start.Id).Error)
	require.Contains(t, start.LegacyCycleGroups, `"quota":["vip"]`)
	require.NoError(t, model.DB.Create(&model.UserSubscription{UserId: user.Id, PlanId: scoped.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100}).Error)
	require.NoError(t, setting.UpdateUserUsableGroupsByJSONString(`{"default":"Default"}`))
	_, eligible, err := model.GetActiveSubscriptionAvailabilityForGroup(user.Id, "vip")
	require.NoError(t, err)
	require.True(t, eligible, "the bound plan grants its own entitlement independently of the legacy cycle")
}
