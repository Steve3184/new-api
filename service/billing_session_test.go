package service

import (
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
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

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
	oldOptions := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		require.NoError(t, model.UpdateOption("SubscriptionGroups", `{}`))
		common.OptionMap = oldOptions
	})
	user := &model.User{Username: "coexisting-subscriptions", Password: "password", Status: common.UserStatusEnabled, Group: "default", Quota: 100}
	require.NoError(t, model.DB.Create(user).Error)
	require.ErrorContains(t, model.UpdateOption("SubscriptionGroups", `{"vip":{"auto_create_key":false}}`), "Bind at least one subscription plan")
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
	assert.True(t, eligible, "legacy plans continue to cover an ordinary group before activation")
	require.NoError(t, model.DB.Model(&legacySub).Update("status", "cancelled").Error)
	_, eligible, err = model.GetActiveSubscriptionAvailabilityForGroup(user.Id, "vip")
	require.NoError(t, err)
	assert.False(t, eligible, "prebinding a scoped plan does not give subscription billing to an ordinary group")
	require.NoError(t, model.DB.Model(&legacySub).Update("status", "active").Error)
	legacyRatio := ratio_setting.GetGroupRatio("vip")
	require.NoError(t, model.UpdateOption("SubscriptionGroups", `{"vip":{"auto_create_key":false}}`))
	assert.Equal(t, 1.0, ratio_setting.GetGroupRatio("vip"))
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
	require.NoError(t, model.UpdateOption("SubscriptionGroups", `{}`))
	assert.Equal(t, legacyRatio, ratio_setting.GetGroupRatio("vip"))
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
