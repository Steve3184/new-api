package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	NowPaymentsOrderTypeTopUp        = "topup"
	NowPaymentsOrderTypeSubscription = "subscription"

	NowPaymentsPaymentStatusPending = "pending"
	NowPaymentsPaymentStatusSuccess = "success"
	NowPaymentsPaymentStatusExpired = "expired"
	NowPaymentsPaymentStatusFailed  = "failed"
)

var ErrNowPaymentsPaymentConflict = errors.New("NOWPayments payment already exists")

// NowPaymentsPayment stores the immutable quote and the last verified gateway
// status for either a wallet top-up or subscription order. Decimal values stay
// as strings so every supported database retains identical precision.
type NowPaymentsPayment struct {
	ID                  int    `json:"id"`
	TopUpID             *int   `json:"top_up_id" gorm:"uniqueIndex"`
	SubscriptionOrderID *int   `json:"subscription_order_id" gorm:"uniqueIndex"`
	PaymentID           string `json:"payment_id" gorm:"type:varchar(64);uniqueIndex"`
	OrderID             string `json:"order_id" gorm:"type:varchar(255);uniqueIndex"`
	OrderType           string `json:"order_type" gorm:"type:varchar(24);index"`
	// ParentPaymentID links a repeated deposit, which NOWPayments reports as a
	// new payment to the original address, back to the invoice it pays.
	ParentPaymentID    string `json:"parent_payment_id" gorm:"type:varchar(64);index"`
	PayCurrency        string `json:"pay_currency" gorm:"type:varchar(32);index"`
	PriceCurrency      string `json:"price_currency" gorm:"type:varchar(16)"`
	PriceAmount        string `json:"price_amount" gorm:"type:varchar(64)"`
	PayAmount          string `json:"pay_amount" gorm:"type:varchar(64)"`
	ActuallyPaidAmount string `json:"actually_paid_amount" gorm:"type:varchar(64)"`
	CreditedQuota      int    `json:"credited_quota"`
	SettledQuota       int    `json:"settled_quota"`
	PayAddress         string `json:"pay_address" gorm:"type:varchar(255)"`
	PayinExtraID       string `json:"payin_extra_id" gorm:"type:varchar(255)"`
	GatewayStatus      string `json:"gateway_status" gorm:"type:varchar(32);index"`
	Status             string `json:"status" gorm:"type:varchar(16);index"`
	IPNPayload         string `json:"ipn_payload" gorm:"column:ipn_payload;type:text"`
	ExpiresAt          int64  `json:"expires_at" gorm:"index"`
	SettledAt          int64  `json:"settled_at"`
	CreateTime         int64  `json:"create_time" gorm:"index"`
}

func CreateNowPaymentsTopUp(topUp *TopUp, payment *NowPaymentsPayment) error {
	if topUp == nil || payment == nil || payment.OrderID == "" || payment.PaymentID == "" {
		return errors.New("NOWPayments top-up invoice is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(topUp).Error; err != nil {
			return err
		}
		payment.TopUpID = &topUp.Id
		payment.OrderType = NowPaymentsOrderTypeTopUp
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(payment).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&NowPaymentsPayment{}).
			Where("top_up_id = ? AND payment_id = ? AND order_id = ?", topUp.Id, payment.PaymentID, payment.OrderID).
			Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrNowPaymentsPaymentConflict
		}
		return nil
	})
}

func CreateNowPaymentsSubscription(order *SubscriptionOrder, payment *NowPaymentsPayment) error {
	if order == nil || payment == nil || payment.OrderID == "" || payment.PaymentID == "" {
		return errors.New("NOWPayments subscription invoice is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).Select("id").Where("id = ?", order.UserId).First(&user).Error; err != nil {
			return err
		}
		plan, err := getSubscriptionPlanByIdTx(tx, order.PlanId)
		if err != nil {
			return err
		}
		if err := CheckSubscriptionPurchaseCapacity(tx, order.UserId, plan); err != nil {
			return err
		}
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		payment.SubscriptionOrderID = &order.Id
		payment.OrderType = NowPaymentsOrderTypeSubscription
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(payment).Error; err != nil {
			return err
		}
		var count int64
		if err := tx.Model(&NowPaymentsPayment{}).
			Where("subscription_order_id = ? AND payment_id = ? AND order_id = ?", order.Id, payment.PaymentID, payment.OrderID).
			Count(&count).Error; err != nil {
			return err
		}
		if count != 1 {
			return ErrNowPaymentsPaymentConflict
		}
		return nil
	})
}

func GetNowPaymentsPaymentByIDAndUser(paymentID string, userID int) (*NowPaymentsPayment, error) {
	paymentID = strings.TrimSpace(paymentID)
	if paymentID == "" {
		return nil, gorm.ErrRecordNotFound
	}
	payment := &NowPaymentsPayment{}
	if err := DB.Where("payment_id = ?", paymentID).First(payment).Error; err != nil {
		return nil, err
	}
	owned := false
	switch payment.OrderType {
	case NowPaymentsOrderTypeTopUp:
		if payment.TopUpID != nil {
			var topUp TopUp
			owned = DB.Select("id").Where("id = ? AND user_id = ?", *payment.TopUpID, userID).First(&topUp).Error == nil
		}
	case NowPaymentsOrderTypeSubscription:
		if payment.SubscriptionOrderID != nil {
			var order SubscriptionOrder
			owned = DB.Select("id").Where("id = ? AND user_id = ?", *payment.SubscriptionOrderID, userID).First(&order).Error == nil
		}
	}
	if !owned {
		return nil, gorm.ErrRecordNotFound
	}
	return payment, nil
}

func GetNowPaymentsPaymentByPaymentID(paymentID string) (*NowPaymentsPayment, error) {
	payment := &NowPaymentsPayment{}
	if err := DB.Where("payment_id = ?", strings.TrimSpace(paymentID)).First(payment).Error; err != nil {
		return nil, err
	}
	return payment, nil
}

func UpdateNowPaymentsPendingState(paymentID, gatewayStatus, actuallyPaid, payload string) error {
	updates := map[string]interface{}{
		"gateway_status":       gatewayStatus,
		"actually_paid_amount": actuallyPaid,
		"ipn_payload":          payload,
	}
	return DB.Model(&NowPaymentsPayment{}).
		Where("payment_id = ? AND status = ?", paymentID, NowPaymentsPaymentStatusPending).
		Updates(updates).Error
}

func FailNowPaymentsPayment(paymentID, gatewayStatus, actuallyPaid, payload, targetStatus string) error {
	if targetStatus != NowPaymentsPaymentStatusExpired && targetStatus != NowPaymentsPaymentStatusFailed {
		return errors.New("invalid NOWPayments terminal status")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		payment := &NowPaymentsPayment{}
		if err := lockForUpdate(tx).Where("payment_id = ?", paymentID).First(payment).Error; err != nil {
			return err
		}
		if payment.Status == NowPaymentsPaymentStatusSuccess || payment.Status == targetStatus {
			return nil
		}
		if payment.Status != NowPaymentsPaymentStatusPending {
			return errors.New("NOWPayments payment is not pending")
		}
		payment.GatewayStatus = gatewayStatus
		payment.ActuallyPaidAmount = actuallyPaid
		payment.IPNPayload = payload
		payment.Status = targetStatus
		if err := tx.Save(payment).Error; err != nil {
			return err
		}
		switch payment.OrderType {
		case NowPaymentsOrderTypeTopUp:
			if payment.TopUpID == nil {
				return errors.New("NOWPayments top-up reference is missing")
			}
			return tx.Model(&TopUp{}).
				Where("id = ? AND payment_provider = ? AND status = ?", *payment.TopUpID, PaymentProviderNowPayments, common.TopUpStatusPending).
				Update("status", map[string]string{
					NowPaymentsPaymentStatusExpired: common.TopUpStatusExpired,
					NowPaymentsPaymentStatusFailed:  common.TopUpStatusFailed,
				}[targetStatus]).Error
		case NowPaymentsOrderTypeSubscription:
			if payment.SubscriptionOrderID == nil {
				return errors.New("NOWPayments subscription reference is missing")
			}
			return tx.Model(&SubscriptionOrder{}).
				Where("id = ? AND payment_provider = ? AND status = ?", *payment.SubscriptionOrderID, PaymentProviderNowPayments, common.TopUpStatusPending).
				Update("status", map[string]string{
					NowPaymentsPaymentStatusExpired: common.TopUpStatusExpired,
					NowPaymentsPaymentStatusFailed:  common.TopUpStatusFailed,
				}[targetStatus]).Error
		default:
			return errors.New("invalid NOWPayments order type")
		}
	})
}

var ErrNowPaymentsRepeatedDepositUnsupported = errors.New("NOWPayments repeated deposit can only top up a wallet order")

// SettleNowPaymentsTopUp credits a verified gateway status in proportion to
// the received crypto amount.
func SettleNowPaymentsTopUp(paymentID, gatewayStatus, actuallyPaid, payload, callerIP string) (bool, error) {
	return settleNowPaymentsTopUp(paymentID, gatewayStatus, actuallyPaid, payload, callerIP, false)
}

// SettleNowPaymentsTopUpByAdmin credits an administrator-confirmed amount,
// including for an invoice the gateway already expired or failed.
func SettleNowPaymentsTopUpByAdmin(paymentID, actuallyPaid, callerIP string) (bool, error) {
	return settleNowPaymentsTopUp(paymentID, "admin", actuallyPaid, "", callerIP, true)
}

func settleNowPaymentsTopUp(paymentID, gatewayStatus, actuallyPaid, payload, callerIP string, byAdmin bool) (bool, error) {
	var topUp *TopUp
	quotaToAdd := 0
	alreadySettled := false
	err := DB.Transaction(func(tx *gorm.DB) error {
		payment := &NowPaymentsPayment{}
		if err := lockForUpdate(tx).Where("payment_id = ?", paymentID).First(payment).Error; err != nil {
			return err
		}
		if payment.OrderType != NowPaymentsOrderTypeTopUp || payment.TopUpID == nil || *payment.TopUpID <= 0 {
			return ErrPaymentMethodMismatch
		}
		topUp = &TopUp{}
		if err := lockForUpdate(tx).Where("id = ?", *payment.TopUpID).First(topUp).Error; err != nil {
			return err
		}
		if topUp.PaymentProvider != PaymentProviderNowPayments {
			return ErrPaymentMethodMismatch
		}
		if payment.Status == NowPaymentsPaymentStatusSuccess && topUp.Status == common.TopUpStatusSuccess && payment.SettledQuota == 0 {
			// Rows created before partial settlement tracking were already fully
			// credited when marked successful.
			alreadySettled = true
			return nil
		}
		validPending := payment.Status == NowPaymentsPaymentStatusPending && topUp.Status == common.TopUpStatusPending
		if byAdmin {
			validPending = payment.Status != NowPaymentsPaymentStatusSuccess && topUp.Status != common.TopUpStatusSuccess
		}
		validIncrement := payment.Status == NowPaymentsPaymentStatusSuccess && topUp.Status == common.TopUpStatusSuccess
		if !validPending && !validIncrement {
			return ErrTopUpStatusInvalid
		}
		if validIncrement && topUp.IsRedemptionPurchase() {
			// The codes were minted on the first settlement; extra funds on the
			// same invoice must not mint another batch.
			alreadySettled = true
			return nil
		}
		if payload == "" {
			payload = payment.IPNPayload
		}
		if payment.CreditedQuota <= 0 {
			return ErrInvalidTopUpQuota
		}
		paid, err := decimal.NewFromString(actuallyPaid)
		if err != nil || !paid.IsPositive() {
			return errors.New("invalid NOWPayments paid amount")
		}
		price, err := decimal.NewFromString(payment.PriceAmount)
		if err != nil || !price.IsPositive() {
			return errors.New("invalid NOWPayments price amount")
		}
		quotedCrypto, err := decimal.NewFromString(payment.PayAmount)
		if err != nil || !quotedCrypto.IsPositive() {
			return errors.New("invalid NOWPayments quoted crypto amount")
		}
		if topUp.IsRedemptionPurchase() && paid.LessThan(quotedCrypto) {
			return errors.New("NOWPayments redemption payment is below the quoted amount")
		}
		ratio := paid.Div(quotedCrypto)
		targetQuota := payment.CreditedQuota
		if !topUp.IsRedemptionPurchase() {
			targetQuota, err = common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(int64(payment.CreditedQuota)).Mul(ratio))
			if err != nil || targetQuota <= 0 {
				return ErrInvalidTopUpQuota
			}
		}
		quotaToAdd = targetQuota - payment.SettledQuota
		if quotaToAdd <= 0 {
			alreadySettled = true
			return nil
		}

		now := common.GetTimestamp()
		payment.GatewayStatus = gatewayStatus
		payment.ActuallyPaidAmount = paid.String()
		payment.IPNPayload = payload
		payment.Status = NowPaymentsPaymentStatusSuccess
		payment.SettledQuota = targetQuota
		payment.SettledAt = now
		if err := tx.Save(payment).Error; err != nil {
			return err
		}
		topUp.Money = price.Mul(ratio).InexactFloat64()
		topUp.CompleteTime = now
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		_, err = settleTopUp(tx, topUp, quotaToAdd, nil)
		return err
	})
	if err != nil {
		return false, err
	}
	if alreadySettled || topUp == nil {
		return true, nil
	}
	if topUp.IsRedemptionPurchase() {
		recordRedemptionPurchaseTopUpLog(topUp, callerIP, PaymentProviderNowPayments)
		return false, nil
	}
	syncCreditUserQuotaCache(topUp.UserId, quotaToAdd, "NOWPayments topup")
	RecordTopupLog(
		topUp.UserId,
		common.NewMessage("{{provider}} top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"provider": "NOWPayments", "quota": logger.FormatQuota(quotaToAdd), "amount": fmt.Sprintf("%.8f USD", topUp.Money)}),
		callerIP,
		PaymentMethodNowPayments,
		PaymentProviderNowPayments,
	)
	return false, nil
}

// SettleNowPaymentsRepeatedDeposit credits a deposit NOWPayments reported as
// a child payment of an existing wallet invoice. The child row makes repeated
// callbacks idempotent and lets a later status add only the difference.
func SettleNowPaymentsRepeatedDeposit(parentPaymentID, childPaymentID, gatewayStatus, actuallyPaid, payload, callerIP string) error {
	var topUp *TopUp
	quotaToAdd := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		parent := &NowPaymentsPayment{}
		if err := lockForUpdate(tx).Where("payment_id = ?", parentPaymentID).First(parent).Error; err != nil {
			return err
		}
		if parent.OrderType != NowPaymentsOrderTypeTopUp || parent.TopUpID == nil || parent.ParentPaymentID != "" {
			return ErrNowPaymentsRepeatedDepositUnsupported
		}
		topUp = &TopUp{}
		if err := lockForUpdate(tx).Where("id = ?", *parent.TopUpID).First(topUp).Error; err != nil {
			return err
		}
		if topUp.PaymentProvider != PaymentProviderNowPayments {
			return ErrPaymentMethodMismatch
		}
		if topUp.IsRedemptionPurchase() {
			return ErrNowPaymentsRepeatedDepositUnsupported
		}
		paid, err := decimal.NewFromString(actuallyPaid)
		if err != nil || !paid.IsPositive() {
			return errors.New("invalid NOWPayments paid amount")
		}
		price, err := decimal.NewFromString(parent.PriceAmount)
		if err != nil || !price.IsPositive() {
			return errors.New("invalid NOWPayments price amount")
		}
		quotedCrypto, err := decimal.NewFromString(parent.PayAmount)
		if err != nil || !quotedCrypto.IsPositive() || parent.CreditedQuota <= 0 {
			return errors.New("invalid NOWPayments quoted crypto amount")
		}
		ratio := paid.Div(quotedCrypto)
		targetQuota, err := common.WalletQuotaFromDecimalStrict(decimal.NewFromInt(int64(parent.CreditedQuota)).Mul(ratio))
		if err != nil || targetQuota <= 0 {
			return ErrInvalidTopUpQuota
		}

		now := common.GetTimestamp()
		child := &NowPaymentsPayment{}
		childErr := tx.Where("payment_id = ?", childPaymentID).First(child).Error
		switch {
		case errors.Is(childErr, gorm.ErrRecordNotFound):
			child = &NowPaymentsPayment{
				PaymentID:       childPaymentID,
				OrderID:         parent.OrderID + "#" + childPaymentID,
				OrderType:       NowPaymentsOrderTypeTopUp,
				ParentPaymentID: parent.PaymentID,
				PayCurrency:     parent.PayCurrency,
				PriceCurrency:   parent.PriceCurrency,
				PayAddress:      parent.PayAddress,
				PayinExtraID:    parent.PayinExtraID,
				CreditedQuota:   parent.CreditedQuota,
				CreateTime:      now,
			}
		case childErr != nil:
			return childErr
		case child.ParentPaymentID != parent.PaymentID:
			return ErrNowPaymentsPaymentConflict
		}
		quotaToAdd = targetQuota - child.SettledQuota
		if quotaToAdd <= 0 {
			quotaToAdd = 0
			return nil
		}
		previousPrice, _ := decimal.NewFromString(child.PriceAmount)
		childPrice := price.Mul(ratio).Round(8)
		child.PriceAmount = childPrice.String()
		child.PayAmount = paid.String()
		child.ActuallyPaidAmount = paid.String()
		child.SettledQuota = targetQuota
		child.GatewayStatus = gatewayStatus
		child.Status = NowPaymentsPaymentStatusSuccess
		child.IPNPayload = payload
		child.SettledAt = now
		if err := tx.Save(child).Error; err != nil {
			return err
		}
		topUp.Money = decimal.NewFromFloat(topUp.Money).Add(childPrice.Sub(previousPrice)).InexactFloat64()
		if topUp.Status != common.TopUpStatusSuccess {
			topUp.Status = common.TopUpStatusSuccess
			topUp.CompleteTime = now
		}
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		_, err = settleTopUp(tx, topUp, quotaToAdd, nil)
		return err
	})
	if err != nil || quotaToAdd == 0 {
		return err
	}
	syncCreditUserQuotaCache(topUp.UserId, quotaToAdd, "NOWPayments repeated deposit")
	RecordTopupLog(
		topUp.UserId,
		common.NewMessage("{{provider}} top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"provider": "NOWPayments", "quota": logger.FormatQuota(quotaToAdd), "amount": fmt.Sprintf("%s %s", actuallyPaid, strings.ToUpper(topUp.PaymentMethod))}),
		callerIP,
		PaymentMethodNowPayments,
		PaymentProviderNowPayments,
	)
	return nil
}

func GetNowPaymentsPaymentByTopUpID(topUpID int) (*NowPaymentsPayment, error) {
	payment := &NowPaymentsPayment{}
	if err := DB.Where("top_up_id = ?", topUpID).First(payment).Error; err != nil {
		return nil, err
	}
	return payment, nil
}

// CompleteNowPaymentsSubscription settles the gateway payment and activates
// the linked subscription in one transaction. This prevents a verified
// callback from leaving the subscription active while the payment row remains
// pending (or the inverse) when either write fails.
func CompleteNowPaymentsSubscription(paymentID, gatewayStatus, actuallyPaid, payload string) error {
	var completion subscriptionCompletionResult
	err := DB.Transaction(func(tx *gorm.DB) error {
		payment := &NowPaymentsPayment{}
		if err := lockForUpdate(tx).Where("payment_id = ?", paymentID).First(payment).Error; err != nil {
			return err
		}
		if payment.OrderType != NowPaymentsOrderTypeSubscription || payment.SubscriptionOrderID == nil || *payment.SubscriptionOrderID <= 0 {
			return ErrPaymentMethodMismatch
		}
		if payment.Status == NowPaymentsPaymentStatusSuccess {
			return nil
		}
		if payment.Status != NowPaymentsPaymentStatusPending {
			return ErrSubscriptionOrderStatusInvalid
		}
		var err error
		completion, err = completeSubscriptionOrderTx(
			tx,
			payment.OrderID,
			payload,
			PaymentProviderNowPayments,
			PaymentMethodNowPayments,
			nil,
		)
		if err != nil {
			return err
		}
		payment.GatewayStatus = gatewayStatus
		payment.ActuallyPaidAmount = actuallyPaid
		payment.IPNPayload = payload
		payment.Status = NowPaymentsPaymentStatusSuccess
		payment.SettledAt = common.GetTimestamp()
		return tx.Save(payment).Error
	})
	if err != nil {
		return err
	}
	finalizeSubscriptionCompletion(completion)
	return nil
}

// MarkNowPaymentsSubscriptionSettled is kept as a compatibility wrapper for
// callers that used the payment-only settlement API before subscription
// fulfillment was made atomic.
func MarkNowPaymentsSubscriptionSettled(paymentID, gatewayStatus, actuallyPaid, payload string) error {
	return CompleteNowPaymentsSubscription(paymentID, gatewayStatus, actuallyPaid, payload)
}
