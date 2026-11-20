package service

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// subscriptionFundingFixture is one request reserved from one subscription.
type subscriptionFundingFixture struct {
	user    model.User
	sub     model.UserSubscription
	info    *relaycommon.RelayInfo
	session *BillingSession
	funding *SubscriptionFunding
}

func newSubscriptionFundingFixture(t *testing.T, requestID string, preConsume int, prepare ...func(*model.UserSubscription)) *subscriptionFundingFixture {
	t.Helper()
	truncate(t)
	user := model.User{Username: requestID, AffCode: requestID, Group: "default", Quota: 100, Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&user).Error)
	plan := model.SubscriptionPlan{Title: requestID, SubscriptionGroup: "vip", TotalAmount: 100}
	require.NoError(t, model.DB.Create(&plan).Error)
	model.InvalidateSubscriptionPlanCache(plan.Id)
	now := common.GetTimestamp()
	sub := model.UserSubscription{UserId: user.Id, PlanId: plan.Id, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 100}
	require.NoError(t, model.DB.Create(&sub).Error)
	if len(prepare) > 0 {
		prepare[0](&sub)
		require.NoError(t, model.DB.Save(&sub).Error)
	}
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	ctx.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	info := &relaycommon.RelayInfo{RequestId: requestID, UserId: user.Id, UserGroup: user.Group, UsingGroup: "vip", OriginModelName: "test-model", IsPlayground: true, UserSetting: dto.UserSetting{BillingPreference: "subscription_only"}}
	session, apiErr := NewBillingSession(ctx, info, preConsume)
	require.Nil(t, apiErr)
	return &subscriptionFundingFixture{user: user, sub: sub, info: info, session: session, funding: session.funding.(*SubscriptionFunding)}
}

// replaceReceipt models the retention cleanup that drops an unfinished receipt
// and lets another subscription claim the same request key.
func (f *subscriptionFundingFixture) replaceReceipt(t *testing.T) model.UserSubscription {
	t.Helper()
	require.NoError(t, model.DB.Where("request_id = ?", f.info.RequestId).Delete(&model.SubscriptionPreConsumeRecord{}).Error)
	other := model.User{Username: f.info.RequestId + "-reuser", AffCode: f.info.RequestId + "-reuser", Group: "default", Status: common.UserStatusEnabled}
	require.NoError(t, model.DB.Create(&other).Error)
	now := common.GetTimestamp()
	otherSub := model.UserSubscription{UserId: other.Id, PlanId: f.sub.PlanId, Status: "active", StartTime: now - 60, EndTime: now + 3600, AmountTotal: 5, AmountUsed: 5, FundingReserved: 5}
	require.NoError(t, model.DB.Create(&otherSub).Error)
	replacement := model.SubscriptionPreConsumeRecord{
		RequestId: f.info.RequestId, UserId: other.Id, UserSubscriptionId: otherSub.Id,
		PreConsumed: 5, ReservedTotal: 5, Allocations: "[]", RequestGroup: "vip",
		Status: "consumed", Durable: true,
	}
	require.NoError(t, model.DB.Create(&replacement).Error)
	return otherSub
}

func TestSubscriptionCompatibilityRefundDoesNotTouchReplacement(t *testing.T) {
	f := newSubscriptionFundingFixture(t, "compatibility-old-request", 10)
	require.NoError(t, model.DB.Where("request_id = ?", f.info.RequestId).Delete(&model.SubscriptionPreConsumeRecord{}).Error)
	replacement := model.SubscriptionPreConsumeRecord{RequestId: f.info.RequestId, UserId: f.user.Id, UserSubscriptionId: f.sub.Id, PreConsumed: 5, ReservedTotal: 5, Allocations: `[]`, Status: "consumed"}
	require.NoError(t, model.DB.Create(&replacement).Error)
	require.NoError(t, PostConsumeQuota(f.info, -3, 10, false))
	require.NoError(t, model.DB.First(&replacement, replacement.Id).Error)
	require.EqualValues(t, 5, replacement.ReservedTotal, "compatibility caller owns ordinary quota, not the current same-key receipt")
	var sub model.UserSubscription
	require.NoError(t, model.DB.First(&sub, f.sub.Id).Error)
	require.EqualValues(t, 7, sub.AmountUsed)
}

func TestSubscriptionFundingReceiptIdentity(t *testing.T) {
	t.Run("pre-consume captures its receipt and keeps accepting its own deltas", func(t *testing.T) {
		f := newSubscriptionFundingFixture(t, "funding-identity", 10)
		var stored model.SubscriptionPreConsumeRecord
		require.NoError(t, model.DB.Where("request_id = ?", f.info.RequestId).First(&stored).Error)
		require.Equal(t, stored.Id, f.funding.receipt.Id, "pre-consume must capture the receipt primary key")
		// Identity-only guard: this request keeps reserving and releasing through
		// the same receipt, so a pinned reserved total would reject the retries.
		require.NoError(t, f.funding.Settle(6))
		require.NoError(t, f.funding.Settle(-2))
		require.NoError(t, model.DB.Where("request_id = ?", f.info.RequestId).First(&stored).Error)
		require.EqualValues(t, 14, stored.ReservedTotal)
		require.NoError(t, f.session.Settle(14))
		require.NoError(t, model.DB.Where("request_id = ?", f.info.RequestId).First(&stored).Error)
		require.Equal(t, "settled", stored.Status)
		require.False(t, stored.Durable)
		var saved model.UserSubscription
		require.NoError(t, model.DB.First(&saved, f.sub.Id).Error)
		require.EqualValues(t, 14, saved.AmountUsed)
		require.Zero(t, saved.FundingReserved, "absolute settlement releases the whole reservation")
	})

	t.Run("a receipt reused by another subscription is never adjusted", func(t *testing.T) {
		f := newSubscriptionFundingFixture(t, "funding-identity-reuse", 10)
		otherSub := f.replaceReceipt(t)
		require.ErrorContains(t, f.funding.Settle(4), "receipt changed")
		require.Error(t, f.session.Reserve(24))
		require.ErrorContains(t, f.session.Settle(30), "receipt changed")
		require.ErrorContains(t, f.funding.Refund(), "receipt changed")
		var replacement model.SubscriptionPreConsumeRecord
		require.NoError(t, model.DB.Where("request_id = ?", f.info.RequestId).First(&replacement).Error)
		require.EqualValues(t, 5, replacement.ReservedTotal)
		require.Equal(t, "consumed", replacement.Status)
		require.True(t, replacement.Durable)
		var savedOther model.UserSubscription
		require.NoError(t, model.DB.First(&savedOther, otherSub.Id).Error)
		require.EqualValues(t, 5, savedOther.AmountUsed)
		require.EqualValues(t, 5, savedOther.FundingReserved)
	})

	t.Run("purchased-credit funding captures its receipt", func(t *testing.T) {
		f := newSubscriptionFundingFixture(t, "funding-identity-credits", 10, func(sub *model.UserSubscription) {
			sub.ExtraCreditsTotal = 20
			sub.ExtraCreditsGroups = `["vip"]`
			sub.ExtraCreditsScopedGroups = `["vip"]`
		})
		require.NotZero(t, f.funding.receipt.Id, "the credits path must return the receipt identity as well")
		require.Equal(t, f.sub.Id, f.funding.receipt.UserSubscriptionId)
		require.NoError(t, f.session.Settle(8))
		var record model.SubscriptionPreConsumeRecord
		require.NoError(t, model.DB.Where("request_id = ?", f.info.RequestId).First(&record).Error)
		require.Equal(t, "settled", record.Status)
		require.False(t, record.Durable)
	})
}
