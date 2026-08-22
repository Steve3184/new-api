package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupRedemptionPurchaseModelTest(t *testing.T, quota int) *User {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Redemption{}))

	user := &User{
		Username: fmt.Sprintf("redemption-purchase-%d", time.Now().UnixNano()),
		AffCode:  fmt.Sprintf("redemption-aff-%d", time.Now().UnixNano()),
		Status:   common.UserStatusEnabled,
		Quota:    quota,
	}
	require.NoError(t, DB.Create(user).Error)

	t.Cleanup(func() {
		DB.Where("owner_id = ? OR user_id = ? OR used_user_id = ?", user.Id, user.Id, user.Id).
			Unscoped().Delete(&Redemption{})
		DB.Where("user_id = ?", user.Id).Delete(&TopUp{})
		DB.Where("user_id = ?", user.Id).Delete(&Log{})
		DB.Delete(&User{}, user.Id)
	})
	return user
}

func TestRechargeEpayRedemptionPurchaseMintsCodesWithoutWalletCredit(t *testing.T) {
	user := setupRedemptionPurchaseModelTest(t, 123)
	tradeNo := fmt.Sprintf("redemption-purchase-settlement-%d", time.Now().UnixNano())
	topUp := &TopUp{
		UserId:           user.Id,
		Amount:           100,
		Money:            100,
		TradeNo:          tradeNo,
		PaymentMethod:    "alipay",
		PaymentProvider:  PaymentProviderEpay,
		CreateTime:       common.GetTimestamp(),
		Status:           common.TopUpStatusPending,
		OrderType:        OrderTypeRedemption,
		RedemptionQuota:  500,
		RedemptionCount:  20,
		RedemptionAmount: 5,
		RedemptionName:   "Code 5",
	}
	require.NoError(t, topUp.Insert())
	t.Cleanup(func() {
		DB.Where("trade_no = ?", tradeNo).Delete(&TopUp{})
		DB.Where("purchase_trade_no = ?", tradeNo).Unscoped().Delete(&Redemption{})
	})

	alreadyDone, err := RechargeEpay(tradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.False(t, alreadyDone)
	assert.Equal(t, 123, getRedemptionPurchaseUserQuota(t, user.Id))

	var codes []Redemption
	require.NoError(t, DB.Where("purchase_trade_no = ?", tradeNo).Order("id asc").Find(&codes).Error)
	require.Len(t, codes, 20)
	for _, code := range codes {
		assert.Equal(t, user.Id, code.OwnerId)
		assert.Equal(t, user.Id, code.UserId)
		assert.Equal(t, RedemptionCreatorUser, code.CreatorType)
		assert.Equal(t, 500, code.Quota)
		assert.Equal(t, common.RedemptionCodeStatusEnabled, code.Status)
		assert.Equal(t, int64(5), code.PurchaseAmount)
	}

	alreadyDone, err = RechargeEpay(tradeNo, "alipay", "127.0.0.1")
	require.NoError(t, err)
	assert.True(t, alreadyDone)
	assert.Equal(t, 123, getRedemptionPurchaseUserQuota(t, user.Id))
	var codeCount int64
	require.NoError(t, DB.Model(&Redemption{}).Where("purchase_trade_no = ?", tradeNo).Count(&codeCount).Error)
	assert.Equal(t, int64(20), codeCount)

	settled := GetTopUpByTradeNo(tradeNo)
	require.NotNil(t, settled)
	assert.Equal(t, common.TopUpStatusSuccess, settled.Status)
}

func TestRefundPurchasedRedemptionEnforcesOwnershipAndUsage(t *testing.T) {
	user := setupRedemptionPurchaseModelTest(t, 100)
	otherUser := setupRedemptionPurchaseModelTest(t, 0)

	owned := createPurchasedRedemptionForTest(t, user.Id, "refund-owned", 500)
	used := createPurchasedRedemptionForTest(t, user.Id, "refund-used", 700)
	require.NoError(t, DB.Model(&Redemption{}).Where("id = ?", used.Id).Updates(map[string]interface{}{
		"status":        common.RedemptionCodeStatusUsed,
		"used_user_id":  otherUser.Id,
		"redeemed_time": common.GetTimestamp(),
	}).Error)
	unowned := createPurchasedRedemptionForTest(t, otherUser.Id, "refund-unowned", 900)
	admin := &Redemption{
		UserId:          user.Id,
		OwnerId:         user.Id,
		CreatorType:     RedemptionCreatorAdmin,
		Key:             common.GetUUID(),
		Status:          common.RedemptionCodeStatusEnabled,
		Quota:           1100,
		PurchaseTradeNo: "admin-created-code",
		PurchaseAmount:  11,
		CreatedTime:     common.GetTimestamp(),
	}
	require.NoError(t, DB.Create(admin).Error)

	refundedQuota, err := RefundPurchasedRedemption(owned.Id, user.Id)
	require.NoError(t, err)
	assert.Equal(t, 500, refundedQuota)
	assert.Equal(t, 600, getRedemptionPurchaseUserQuota(t, user.Id))

	var refunded Redemption
	require.NoError(t, DB.First(&refunded, owned.Id).Error)
	assert.Equal(t, common.RedemptionCodeStatusDisabled, refunded.Status)
	assert.NotZero(t, refunded.RefundedTime)

	_, err = RefundPurchasedRedemption(owned.Id, user.Id)
	assert.ErrorIs(t, err, ErrRedemptionRefundNotAllowed)
	assert.Equal(t, 600, getRedemptionPurchaseUserQuota(t, user.Id))

	_, err = RefundPurchasedRedemption(used.Id, user.Id)
	assert.ErrorIs(t, err, ErrRedemptionRefundNotAllowed)
	_, err = RefundPurchasedRedemption(admin.Id, user.Id)
	assert.ErrorIs(t, err, ErrRedemptionNotOwned)
	_, err = RefundPurchasedRedemption(unowned.Id, user.Id)
	assert.ErrorIs(t, err, ErrRedemptionNotOwned)
}

func TestRedemptionCreatorFilterTreatsLegacyRowsAsAdmin(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Redemption{}))
	rows := []*Redemption{
		{
			Name:        "filter-admin",
			Key:         common.GetUUID(),
			Status:      common.RedemptionCodeStatusEnabled,
			CreatorType: RedemptionCreatorAdmin,
			CreatedTime: common.GetTimestamp(),
		},
		{
			Name:        "filter-legacy",
			Key:         common.GetUUID(),
			Status:      common.RedemptionCodeStatusEnabled,
			CreatedTime: common.GetTimestamp(),
		},
		{
			Name:        "filter-user",
			Key:         common.GetUUID(),
			Status:      common.RedemptionCodeStatusEnabled,
			CreatorType: RedemptionCreatorUser,
			OwnerId:     99001,
			CreatedTime: common.GetTimestamp(),
		},
	}
	require.NoError(t, DB.Create(&rows).Error)
	ids := make([]int, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.Id)
	}
	t.Cleanup(func() {
		DB.Unscoped().Where("id IN ?", ids).Delete(&Redemption{})
	})

	userRows, userTotal, err := SearchRedemptions("filter-", "", 0, 20, RedemptionCreatorUser)
	require.NoError(t, err)
	assert.Equal(t, int64(1), userTotal)
	require.Len(t, userRows, 1)
	assert.Equal(t, RedemptionCreatorUser, userRows[0].CreatorType)

	adminRows, adminTotal, err := SearchRedemptions("filter-", "", 0, 20, RedemptionCreatorAdmin)
	require.NoError(t, err)
	assert.Equal(t, int64(2), adminTotal)
	assert.Len(t, adminRows, 2)
	for _, row := range adminRows {
		assert.NotEqual(t, RedemptionCreatorUser, row.CreatorType)
	}
}

func createPurchasedRedemptionForTest(t *testing.T, userID int, tradeNo string, quota int) *Redemption {
	t.Helper()
	var redemptions []*Redemption
	err := DB.Transaction(func(tx *gorm.DB) error {
		var err error
		redemptions, err = CreatePurchasedRedemptionsTx(tx, userID, "Purchased code", quota, int64(quota/100), 1, tradeNo)
		return err
	})
	require.NoError(t, err)
	require.Len(t, redemptions, 1)
	t.Cleanup(func() {
		DB.Unscoped().Where("purchase_trade_no = ?", tradeNo).Delete(&Redemption{})
	})
	return redemptions[0]
}

func getRedemptionPurchaseUserQuota(t *testing.T, userID int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("quota").First(&user, userID).Error)
	return user.Quota
}
