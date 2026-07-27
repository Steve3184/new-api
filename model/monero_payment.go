package model

import (
	"errors"
	"fmt"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"
	"github.com/bytedance/gopkg/util/gopool"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

const (
	MoneroPaymentStatusPending = "pending"
	MoneroPaymentStatusSuccess = "success"
	MoneroPaymentStatusExpired = "expired"

	moneroAtomicUnits = int32(12)
)

// MoneroPayment stores the immutable invoice quote next to the generic top-up
// record. Decimal values are strings so SQLite, MySQL, and PostgreSQL retain
// identical precision for atomic XMR units and the quoted USD exchange rate.
type MoneroPayment struct {
	ID             int    `json:"id"`
	TopUpID        int    `json:"top_up_id" gorm:"uniqueIndex"`
	Address        string `json:"address" gorm:"type:varchar(255);uniqueIndex"`
	AccountIndex   int    `json:"account_index"`
	AddressIndex   int    `json:"address_index"`
	Network        string `json:"network" gorm:"type:varchar(16);index"`
	ExpectedAtomic string `json:"expected_atomic" gorm:"type:varchar(32)"`
	ReceivedAtomic string `json:"received_atomic" gorm:"type:varchar(32)"`
	QuoteUSD       string `json:"quote_usd" gorm:"type:varchar(32)"`
	USDPerXMR      string `json:"usd_per_xmr" gorm:"type:varchar(32)"`
	// QuotaPerUSD is the invoice-frozen internal quota credited for one USD.
	// It includes the display-currency rate and any invoice pricing modifiers.
	QuotaPerUSD    string `json:"quota_per_usd" gorm:"type:varchar(32)"`
	TransactionIDs string `json:"transaction_ids" gorm:"type:text"`
	Status         string `json:"status" gorm:"type:varchar(16);index"`
	ExpiresAt      int64  `json:"expires_at" gorm:"index"`
	SettledAt      int64  `json:"settled_at"`
	CreateTime     int64  `json:"create_time" gorm:"index"`
}

type MoneroPaymentInvoice struct {
	TopUp   *TopUp
	Payment *MoneroPayment
}

func CreateMoneroPaymentInvoice(topUp *TopUp, payment *MoneroPayment) error {
	if topUp == nil || payment == nil {
		return errors.New("monero invoice is required")
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(topUp).Error; err != nil {
			return err
		}
		payment.TopUpID = topUp.Id
		return tx.Create(payment).Error
	})
}

func ListPendingMoneroPayments(network string) ([]MoneroPayment, error) {
	var payments []MoneroPayment
	err := DB.Where("status = ? AND network = ?", MoneroPaymentStatusPending, network).Find(&payments).Error
	return payments, err
}

// ListTerminalMoneroPaymentAddressAuditCandidates returns only invoices whose
// Monero payment and associated top-up have both reached a matching terminal
// state. The address audit must never inspect a pending invoice as eligible.
func ListTerminalMoneroPaymentAddressAuditCandidates(network string) ([]MoneroPayment, error) {
	var payments []MoneroPayment
	err := DB.
		Joins("JOIN top_ups ON top_ups.id = monero_payments.top_up_id").
		Where("monero_payments.network = ?", network).
		Where(
			"(monero_payments.status = ? AND top_ups.status = ?) OR (monero_payments.status = ? AND top_ups.status = ?)",
			MoneroPaymentStatusSuccess,
			common.TopUpStatusSuccess,
			MoneroPaymentStatusExpired,
			common.TopUpStatusExpired,
		).
		Order("monero_payments.id asc").
		Find(&payments).Error
	return payments, err
}

func GetMoneroPaymentByAddressAndUser(address string, userID int) (*MoneroPayment, error) {
	payment := &MoneroPayment{}
	err := DB.
		Joins("JOIN top_ups ON top_ups.id = monero_payments.top_up_id").
		Where("monero_payments.address = ? AND top_ups.user_id = ?", address, userID).
		First(payment).Error
	if err != nil {
		return nil, err
	}
	return payment, nil
}

func ExpirePendingMoneroPayments(now int64) error {
	return DB.Transaction(func(tx *gorm.DB) error {
		var payments []MoneroPayment
		if err := lockForUpdate(tx).
			Where("status = ? AND expires_at > 0 AND expires_at <= ?", MoneroPaymentStatusPending, now).
			Find(&payments).Error; err != nil {
			return err
		}
		for _, payment := range payments {
			if err := tx.Model(&MoneroPayment{}).Where("id = ? AND status = ?", payment.ID, MoneroPaymentStatusPending).Update("status", MoneroPaymentStatusExpired).Error; err != nil {
				return err
			}
			if err := tx.Model(&TopUp{}).Where("id = ? AND status = ?", payment.TopUpID, common.TopUpStatusPending).Update("status", common.TopUpStatusExpired).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// SettleMoneroPayment is the only place an observed XMR transfer can credit a
// user. It locks both records, rechecks the payment provider/status, and uses
// the quote snapshot rather than a later exchange-rate value.
func SettleMoneroPayment(paymentID int, receivedAtomic string, transactionIDs []string) (*MoneroPaymentInvoice, int, error) {
	received, err := decimal.NewFromString(strings.TrimSpace(receivedAtomic))
	if err != nil || !received.IsPositive() || !received.Equal(received.Truncate(0)) {
		return nil, 0, errors.New("invalid monero received amount")
	}

	var settled *MoneroPaymentInvoice
	var quotaToAdd int
	err = DB.Transaction(func(tx *gorm.DB) error {
		payment := &MoneroPayment{}
		if err := lockForUpdate(tx).Where("id = ?", paymentID).First(payment).Error; err != nil {
			return err
		}
		if payment.Status == MoneroPaymentStatusSuccess {
			topUp := &TopUp{}
			if err := tx.First(topUp, payment.TopUpID).Error; err != nil {
				return err
			}
			settled = &MoneroPaymentInvoice{TopUp: topUp, Payment: payment}
			return nil
		}
		if payment.Status != MoneroPaymentStatusPending {
			return errors.New("monero payment is not pending")
		}

		expected, parseErr := decimal.NewFromString(payment.ExpectedAtomic)
		if parseErr != nil || !expected.IsPositive() || received.LessThan(expected) {
			return errors.New("monero payment is below the expected amount")
		}
		usdPerXMR, parseErr := decimal.NewFromString(payment.USDPerXMR)
		if parseErr != nil || !usdPerXMR.IsPositive() {
			return errors.New("invalid monero USD quote")
		}
		quotaPerUSD, parseErr := decimal.NewFromString(payment.QuotaPerUSD)
		if parseErr != nil || !quotaPerUSD.IsPositive() {
			return errors.New("invalid monero quota quote")
		}

		paidUSD := received.Shift(-moneroAtomicUnits).Mul(usdPerXMR)
		var clamp *common.QuotaClamp
		quotaToAdd, clamp = common.QuotaFromDecimalChecked(paidUSD.Mul(quotaPerUSD))
		if clamp != nil {
			return fmt.Errorf("monero quota conversion: %w", clamp)
		}
		if quotaToAdd <= 0 {
			return errors.New("monero payment has no creditable quota")
		}

		topUp := &TopUp{}
		if err := lockForUpdate(tx).Where("id = ?", payment.TopUpID).First(topUp).Error; err != nil {
			return err
		}
		if topUp.PaymentProvider != PaymentProviderMonero {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			settled = &MoneroPaymentInvoice{TopUp: topUp, Payment: payment}
			return nil
		}
		if topUp.Status != common.TopUpStatusPending {
			return errors.New("monero topup is not pending")
		}

		now := common.GetTimestamp()
		topUp.Money = paidUSD.InexactFloat64()
		topUp.CompleteTime = now
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		payment.ReceivedAtomic = received.StringFixed(0)
		payment.TransactionIDs = strings.Join(transactionIDs, ",")
		payment.Status = MoneroPaymentStatusSuccess
		payment.SettledAt = now
		if err := tx.Save(payment).Error; err != nil {
			return err
		}
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Update("quota", gorm.Expr("quota + ?", quotaToAdd)).Error; err != nil {
			return err
		}
		settled = &MoneroPaymentInvoice{TopUp: topUp, Payment: payment}
		return nil
	})
	if err != nil {
		return nil, 0, err
	}
	if settled == nil || quotaToAdd == 0 {
		return settled, quotaToAdd, nil
	}

	gopool.Go(func() {
		if cacheErr := cacheIncrUserQuota(settled.TopUp.UserId, int64(quotaToAdd)); cacheErr != nil {
			common.SysError("failed to synchronize monero topup quota cache: " + cacheErr.Error())
		}
	})
	RecordTopupLog(
		settled.TopUp.UserId,
		fmt.Sprintf("使用 Monero 充值成功，充值额度: %v，支付金额：%.8f USD", logger.FormatQuota(quotaToAdd), settled.TopUp.Money),
		"monero-rpc",
		PaymentMethodMonero,
		PaymentProviderMonero,
	)
	return settled, quotaToAdd, nil
}
