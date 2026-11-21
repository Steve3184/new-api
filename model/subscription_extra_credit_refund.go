package model

import (
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

// RefundExpiredExtraCredits returns the unspent extra credits of ended
// subscriptions to their owners' wallets. The credits were bought at one or
// more rates, so they are converted back at the average rate of the
// subscription's receipts, rounded down. A subscription that still has funds
// reserved by an in-flight request is left for a later pass.
func RefundExpiredExtraCredits(limit int) (int, error) {
	if limit <= 0 {
		limit = 200
	}
	var candidates []UserSubscription
	// Rows without a receipt carry no price to refund at, so they are not candidates.
	err := DB.Select("id", "user_id").
		Where("status IN ? AND extra_credits_total > extra_credits_used AND funding_reserved = 0", []string{"expired", SubscriptionStatusUpgraded}).
		Where("EXISTS (SELECT 1 FROM subscription_extra_credit_purchases WHERE subscription_extra_credit_purchases.subscription_id = user_subscriptions.id)").
		Order("id asc").Limit(limit).Find(&candidates).Error
	if err != nil {
		return 0, err
	}
	refunded := 0
	var failures []error
	for _, candidate := range candidates {
		done, err := refundSubscriptionExtraCredits(candidate.Id, candidate.UserId)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if done {
			refunded++
		}
	}
	return refunded, errors.Join(failures...)
}

func refundSubscriptionExtraCredits(subscriptionID, userID int) (bool, error) {
	var credits, walletQuota int64
	err := DB.Transaction(func(tx *gorm.DB) error {
		// Same lock order as purchase and settlement: the user row, then the subscription.
		var user User
		if err := lockForUpdate(tx).Select("id", "username", "quota").Where("id = ?", userID).Take(&user).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil // The account is gone, so there is no wallet to credit.
			}
			return err
		}
		var sub UserSubscription
		if err := lockForUpdate(tx).Where("id = ? AND user_id = ?", subscriptionID, userID).Take(&sub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		unspent := sub.ExtraCreditsTotal - sub.ExtraCreditsUsed
		if unspent <= 0 || sub.FundingReserved != 0 || (sub.Status != "expired" && sub.Status != SubscriptionStatusUpgraded) {
			return nil
		}
		var bought struct {
			Wallet  int64
			Credits int64
		}
		if err := tx.Model(&SubscriptionExtraCreditPurchase{}).Where("subscription_id = ?", sub.Id).
			Select("COALESCE(SUM(wallet_quota), 0) AS wallet, COALESCE(SUM(credits), 0) AS credits").Scan(&bought).Error; err != nil {
			return err
		}
		// Never refund more than was paid: inconsistent receipts are left alone.
		if bought.Wallet <= 0 || bought.Credits < unspent {
			return nil
		}
		quotient, _ := decimal.NewFromInt(unspent).Mul(decimal.NewFromInt(bought.Wallet)).QuoRem(decimal.NewFromInt(bought.Credits), 0)
		refund, err := common.WalletQuotaFromDecimalStrict(quotient)
		if err != nil {
			return err
		}
		if refund > 0 && user.Quota > common.MaxWalletQuota-refund {
			return ErrWalletQuotaLimitExceeded
		}
		// Zeroing the unspent balance in the same transaction makes a repeated pass a no-op.
		result := tx.Model(&UserSubscription{}).
			Where("id = ? AND extra_credits_total > extra_credits_used AND funding_reserved = 0", sub.Id).
			UpdateColumn("extra_credits_total", gorm.Expr("extra_credits_used"))
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if refund > 0 {
			if err := tx.Model(&User{}).Where("id = ?", userID).Update("quota", gorm.Expr("quota + ?", refund)).Error; err != nil {
				return err
			}
		}
		credits, walletQuota = unspent, int64(refund)
		return nil
	})
	if err != nil {
		return false, err
	}
	if credits == 0 {
		return false, nil
	}
	if walletQuota > 0 {
		if err := cacheIncrUserQuota(userID, walletQuota); err != nil {
			common.SysLog(common.LogText("failed to sync extra credit refund to user quota cache: %s", err.Error()))
		}
		RecordLog(userID, LogTypeRefund, common.NewMessage("Unused extra credits refunded: {{credits}} extra credits returned as {{quota}} wallet quota; subscription: {{subscription}}", map[string]any{"credits": credits, "quota": walletQuota, "subscription": subscriptionID}))
	}
	return true, nil
}
