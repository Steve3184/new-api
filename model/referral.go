package model

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/shopspring/decimal"
	"golang.org/x/text/currency"
	"gorm.io/gorm"
)

// ReferralReward is the durable ledger. A reversed order still occupies its
// qualifying slot, so refund/recharge cycles cannot renew the first-N offer.
type ReferralReward struct {
	ID          int     `json:"id"`
	TopUpID     int     `json:"top_up_id" gorm:"uniqueIndex"`
	InviterID   int     `json:"inviter_id" gorm:"index"`
	InviteeID   int     `json:"invitee_id" gorm:"index"`
	TradeNo     string  `json:"trade_no" gorm:"type:varchar(255);index"`
	PaidCNY     float64 `json:"paid_cny"`
	Percent     float64 `json:"percent"`
	Quota       int     `json:"quota"`
	Status      string  `json:"status" gorm:"type:varchar(20);index:idx_referral_due,priority:1"`
	CreatedAt   int64   `json:"created_at"`
	AvailableAt int64   `json:"available_at" gorm:"index:idx_referral_due,priority:2"`
	CreditedAt  int64   `json:"credited_at"`
	ReversedAt  int64   `json:"reversed_at"`
	Reason      string  `json:"reason" gorm:"type:varchar(500)"`
	OperatorID  int     `json:"operator_id"`
}

type referralSnapshot struct {
	Rules        operation_setting.ReferralSetting `json:"rules"`
	Currency     string                            `json:"currency"`
	Paid         float64                           `json:"paid"`
	CNYRate      float64                           `json:"cny_rate"`
	USDRate      float64                           `json:"usd_rate"`
	QuotaPerUnit float64                           `json:"quota_per_unit"`
}

func (topUp *TopUp) isFiatReferralPayment() bool {
	if topUp.IsRedemptionPurchase() {
		return false
	}
	switch topUp.PaymentProvider {
	case PaymentProviderEpay:
		// Epay gateways may also expose non-fiat methods; only known fiat rails qualify.
		switch topUp.PaymentMethod {
		case "alipay", "wxpay", "qqpay", "bank", "bankpay", "unionpay":
			return true
		default:
			return false
		}
	case PaymentProviderStripe, PaymentProviderCreem, PaymentProviderWaffo, PaymentProviderWaffoPancake:
		return true
	default:
		return false
	}
}

// SnapshotReferral records checkout rules and currency conversions. Old orders
// have no snapshot and are deliberately not rewarded retroactively.
func (topUp *TopUp) SnapshotReferral(currency string) error {
	rules := operation_setting.GetReferralSetting()
	if !rules.Enabled || rules.Percent <= 0 || !topUp.isFiatReferralPayment() || !operation_setting.IsPaymentComplianceConfirmed() {
		return nil
	}
	var rates map[string]float64
	if err := common.UnmarshalJsonStr(rules.CurrencyRates, &rates); err != nil {
		return err
	}
	currency = strings.ToUpper(strings.TrimSpace(currency))
	if rates[currency] <= 0 || rates["USD"] <= 0 {
		return nil
	}
	snapshot := referralSnapshot{Rules: rules, Currency: currency, Paid: topUp.Money, CNYRate: rates[currency], USDRate: rates["USD"], QuotaPerUnit: common.QuotaPerUnit}
	encoded, err := common.Marshal(snapshot)
	if err != nil {
		return err
	}
	topUp.ReferralSnapshot = string(encoded)
	return nil
}

// PaidFiat is supplied from verified payment callbacks when a provider permits
// discounts. It changes only the reward basis, never the existing wallet credit.
type PaidFiat struct {
	Amount   float64
	Currency string
}

// PaidFiatFromMinorUnits converts ISO currency minor units without assuming
// every payment currency has two decimal places. Unknown currencies fail closed.
func PaidFiatFromMinorUnits(amount int64, code string) PaidFiat {
	code = strings.ToUpper(strings.TrimSpace(code))
	paid := PaidFiat{Currency: code}
	unit, err := currency.ParseISO(code)
	if err != nil || amount < 0 {
		return paid
	}
	scale, _ := currency.Standard.Rounding(unit)
	paid.Amount = decimal.New(amount, -int32(scale)).InexactFloat64()
	return paid
}

func applyReferralPaid(topUp *TopUp, paid []PaidFiat) error {
	if topUp.ReferralSnapshot == "" || len(paid) == 0 {
		return nil
	}
	var snapshot referralSnapshot
	if err := common.UnmarshalJsonStr(topUp.ReferralSnapshot, &snapshot); err != nil {
		return err
	}
	p := paid[0]
	if math.IsNaN(p.Amount) || math.IsInf(p.Amount, 0) || p.Amount < 0 || !strings.EqualFold(p.Currency, snapshot.Currency) {
		// Unsupported currency changes must not result in a guessed reward.
		topUp.ReferralSnapshot = ""
		return nil
	}
	snapshot.Paid = p.Amount
	encoded, err := common.Marshal(snapshot)
	if err != nil {
		return err
	}
	topUp.ReferralSnapshot = string(encoded)
	return nil
}

func createReferralReward(tx *gorm.DB, topUp *TopUp) error {
	if topUp.ReferralSnapshot == "" || !topUp.isFiatReferralPayment() {
		return nil
	}
	var snapshot referralSnapshot
	if err := common.UnmarshalJsonStr(topUp.ReferralSnapshot, &snapshot); err != nil {
		return err
	}
	if snapshot.Paid <= 0 || math.IsNaN(snapshot.Paid) || math.IsInf(snapshot.Paid, 0) || snapshot.CNYRate <= 0 || snapshot.USDRate <= 0 {
		return nil
	}
	paidCNY := decimal.NewFromFloat(snapshot.Paid).Mul(decimal.NewFromFloat(snapshot.CNYRate))
	if paidCNY.LessThanOrEqual(decimal.NewFromFloat(snapshot.Rules.MinPaidCNY)) {
		return nil
	}
	// settleTopUp has already updated (and therefore locked) this user. All
	// qualifying orders for the same invitee serialize before counting slots.
	var user User
	if err := tx.Select("id", "inviter_id").First(&user, topUp.UserId).Error; err != nil {
		return err
	}
	if user.InviterId == 0 || user.InviterId == user.Id {
		return nil
	}
	var inviterCount int64
	if err := tx.Model(&User{}).Where("id = ? AND status = ?", user.InviterId, common.UserStatusEnabled).Count(&inviterCount).Error; err != nil {
		return err
	}
	if inviterCount == 0 {
		return nil
	}
	var count int64
	if err := tx.Model(&ReferralReward{}).Where("invitee_id = ?", user.Id).Count(&count).Error; err != nil {
		return err
	}
	if count >= int64(snapshot.Rules.MaxOrders) {
		return nil
	}
	quota, err := common.WalletQuotaFromDecimalStrict(paidCNY.Div(decimal.NewFromFloat(snapshot.USDRate)).Mul(decimal.NewFromFloat(snapshot.Rules.Percent)).Div(decimal.NewFromInt(100)).Mul(decimal.NewFromFloat(snapshot.QuotaPerUnit)))
	if err != nil {
		return err
	}
	if quota <= 0 {
		return nil
	}
	return tx.Create(&ReferralReward{
		TopUpID: topUp.Id, InviterID: user.InviterId, InviteeID: user.Id, TradeNo: topUp.TradeNo,
		PaidCNY: paidCNY.InexactFloat64(), Percent: snapshot.Rules.Percent, Quota: quota,
		Status: "pending", CreatedAt: topUp.CompleteTime, AvailableAt: topUp.CompleteTime + int64(snapshot.Rules.DelayHours)*3600,
	}).Error
}

func CreditReferralReward(id int, now int64) error {
	var reward ReferralReward
	credited := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&reward, id).Error; err != nil {
			return err
		}
		if reward.Status != "pending" || reward.AvailableAt > now {
			return nil
		}
		var order TopUp
		if err := tx.First(&order, reward.TopUpID).Error; err != nil {
			return err
		}
		if order.Status != common.TopUpStatusSuccess {
			return tx.Model(&reward).Updates(map[string]any{"status": "reversed", "reversed_at": now, "reason": "payment no longer successful"}).Error
		}
		// The status predicate also protects SQLite, where FOR UPDATE is unavailable.
		result := tx.Model(&reward).Where("status = ?", "pending").Updates(map[string]any{"status": "credited", "credited_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if err := creditTopUpQuota(tx, reward.InviterID, reward.Quota, nil); err != nil {
			return err
		}
		credited = true
		return nil
	})
	if err == nil && credited {
		syncCreditUserQuotaCache(reward.InviterID, reward.Quota, "referral reward")
		RecordLog(reward.InviterID, LogTypeSystem, fmt.Sprintf("邀请充值奖励到账，奖励ID %d，额度 %d", reward.ID, reward.Quota))
	}
	return err
}

func ReverseReferralReward(id, operator int, reason string) error {
	reason = strings.TrimSpace(reason)
	if reason == "" || len(reason) > 500 {
		return errors.New("a reversal reason of at most 500 bytes is required")
	}
	var reward ReferralReward
	debited := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).First(&reward, id).Error; err != nil {
			return err
		}
		if reward.Status == "reversed" {
			return nil
		}
		wasCredited := reward.Status == "credited"
		result := tx.Model(&reward).Where("status = ?", reward.Status).Updates(map[string]any{"status": "reversed", "reversed_at": common.GetTimestamp(), "reason": reason, "operator_id": operator})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		if wasCredited {
			// A spent reward becomes wallet debt; it cannot escape reversal by spending first.
			result = tx.Model(&User{}).Where("id = ? AND quota >= ?", reward.InviterID, -common.MaxWalletQuota+reward.Quota).Update("quota", gorm.Expr("quota - ?", reward.Quota))
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrWalletQuotaLimitExceeded
			}
			debited = true
		}
		return nil
	})
	if err == nil && debited {
		if cacheErr := cacheIncrUserQuota(reward.InviterID, -int64(reward.Quota)); cacheErr != nil {
			common.SysError("referral reversal cache: " + cacheErr.Error())
		}
	}
	return err
}
