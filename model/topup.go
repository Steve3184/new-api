package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type TopUp struct {
	PaymentCurrency  string  `json:"payment_currency" gorm:"type:varchar(3)"`
	ReferralSnapshot string  `json:"-" gorm:"type:text"`
	Id               int     `json:"id"`
	UserId           int     `json:"user_id" gorm:"index"`
	Amount           int64   `json:"amount"`
	Money            float64 `json:"money"`
	TradeNo          string  `json:"trade_no" gorm:"unique;type:varchar(255);index"`
	PaymentMethod    string  `json:"payment_method" gorm:"type:varchar(50)"`
	PaymentProvider  string  `json:"payment_provider" gorm:"type:varchar(50);default:''"`
	EpayGatewayID    string  `json:"epay_gateway_id" gorm:"type:varchar(100);index"`
	CreateTime       int64   `json:"create_time"`
	CompleteTime     int64   `json:"complete_time"`
	Status           string  `json:"status"`
	OrderType        string  `json:"order_type" gorm:"type:varchar(32);index"`
	RedemptionQuota  int     `json:"redemption_quota"`
	RedemptionCount  int     `json:"redemption_count"`
	RedemptionAmount int64   `json:"redemption_amount"`
	RedemptionName   string  `json:"redemption_name" gorm:"type:varchar(20)"`
	// PayURL is the hosted checkout page of a redirect gateway, kept so the
	// buyer can resume an unpaid order. It may embed a session token, so it is
	// only returned by the owner-scoped resume endpoint.
	PayURL          string `json:"-" gorm:"type:text"`
	PayURLExpiresAt int64  `json:"-"`
}

// TopUpPayURLLifetime bounds how long a stored checkout link is offered when
// the gateway does not report its own session expiry.
const TopUpPayURLLifetime int64 = 24 * 60 * 60

const (
	PaymentMethodStripe       = "stripe"
	PaymentMethodCreem        = "creem"
	PaymentMethodWaffo        = "waffo"
	PaymentMethodWaffoPancake = "waffo_pancake"
	PaymentMethodMonero       = "monero"
	PaymentMethodNowPayments  = "nowpayments"
	PaymentMethodBalance      = "balance"
)

const (
	PaymentProviderEpay         = "epay"
	PaymentProviderStripe       = "stripe"
	PaymentProviderCreem        = "creem"
	PaymentProviderWaffo        = "waffo"
	PaymentProviderWaffoPancake = "waffo_pancake"
	PaymentProviderMonero       = "monero"
	PaymentProviderNowPayments  = "nowpayments"
	PaymentProviderBalance      = "balance"
)

const (
	OrderTypeQuota             = "quota"
	OrderTypeRedemption        = "redemption"
	MaxRedemptionPurchaseCount = 100
)

var (
	ErrPaymentMethodMismatch = errors.New("payment method mismatch")
	ErrTopUpNotFound         = errors.New("topup not found")
	ErrTopUpStatusInvalid    = errors.New("topup status invalid")
	// The quota errors reach the web console, so they carry message keys.
	ErrInvalidTopUpQuota        error = common.NewMessage("Invalid top-up quota")
	ErrTopUpQuotaLimitExceeded  error = common.NewMessage("Top-up quota limit exceeded")
	ErrWalletQuotaLimitExceeded error = common.NewMessage("Wallet quota limit exceeded")
)

func (topUp *TopUp) Insert() error {
	if topUp.PaymentCurrency == "" {
		switch topUp.PaymentProvider {
		case PaymentProviderEpay:
			topUp.PaymentCurrency = "CNY"
		case PaymentProviderStripe:
			topUp.PaymentCurrency = "USD"
		}
	}
	if err := topUp.SnapshotReferral(topUp.PaymentCurrency); err != nil {
		return err
	}
	var err error
	err = DB.Create(topUp).Error
	return err
}

func (topUp *TopUp) IsRedemptionPurchase() bool {
	return topUp != nil && topUp.OrderType == OrderTypeRedemption
}

func topUpQuotaMaxCurrent(creditedQuota int) (int, error) {
	if creditedQuota <= 0 || creditedQuota > common.MaxWalletQuota {
		return 0, ErrInvalidTopUpQuota
	}
	return common.MaxWalletQuota - creditedQuota, nil
}

// ValidateTopUpQuotaCapacity performs the user-facing pre-payment check. The
// settlement path repeats the same invariant with an atomic conditional
// update, because the wallet balance can change after checkout creation.
func ValidateTopUpQuotaCapacity(userId int, creditedQuota int) error {
	maxCurrentQuota, err := topUpQuotaMaxCurrent(creditedQuota)
	if err != nil {
		return err
	}

	var user User
	if err := DB.Select("quota").Where("id = ?", userId).First(&user).Error; err != nil {
		return err
	}
	if user.Quota > maxCurrentQuota {
		return ErrTopUpQuotaLimitExceeded
	}
	return nil
}

// creditTopUpQuota atomically enforces the wallet ceiling while adding quota.
// Keeping the predicate and increment in one UPDATE prevents two
// concurrent callbacks from both passing a separate read/check.
func creditTopUpQuota(tx *gorm.DB, userId int, creditedQuota int, updates map[string]interface{}) error {
	maxCurrentQuota, err := topUpQuotaMaxCurrent(creditedQuota)
	if err != nil {
		return err
	}

	updateFields := make(map[string]interface{}, len(updates)+1)
	for key, value := range updates {
		updateFields[key] = value
	}
	updateFields["quota"] = gorm.Expr("quota + ?", creditedQuota)

	result := tx.Model(&User{}).
		Where("id = ? AND quota <= ?", userId, maxCurrentQuota).
		Updates(updateFields)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 1 {
		return nil
	}

	var count int64
	if err := tx.Model(&User{}).Where("id = ?", userId).Count(&count).Error; err != nil {
		return err
	}
	if count == 0 {
		return gorm.ErrRecordNotFound
	}
	return ErrTopUpQuotaLimitExceeded
}

// settleTopUp applies the post-payment fulfillment for both wallet top-ups
// and redemption-code purchases. Redemption purchases never touch wallet
// quota; they mint owned codes inside the same transaction as the paid order.
func settleTopUp(tx *gorm.DB, topUp *TopUp, quota int, updates map[string]interface{}) (int, error) {
	if topUp == nil {
		return 0, ErrInvalidTopUpQuota
	}
	if !topUp.IsRedemptionPurchase() {
		if err := creditTopUpQuota(tx, topUp.UserId, quota, updates); err != nil {
			return 0, err
		}
		// Persist the verified payment basis (or manual-completion exclusion)
		// alongside the wallet credit and referral ledger in the same transaction.
		if err := tx.Model(topUp).Update("referral_snapshot", topUp.ReferralSnapshot).Error; err != nil {
			return 0, err
		}
		if err := createReferralReward(tx, topUp); err != nil {
			return 0, err
		}
		return quota, nil
	}
	if topUp.RedemptionCount <= 0 || topUp.RedemptionCount > MaxRedemptionPurchaseCount || topUp.RedemptionQuota <= 0 || topUp.RedemptionAmount <= 0 {
		return 0, ErrInvalidTopUpQuota
	}
	if len(updates) > 0 {
		if err := tx.Model(&User{}).Where("id = ?", topUp.UserId).Updates(updates).Error; err != nil {
			return 0, err
		}
	}
	if _, err := CreatePurchasedRedemptionsTx(
		tx,
		topUp.UserId,
		topUp.RedemptionName,
		topUp.RedemptionQuota,
		topUp.RedemptionAmount,
		topUp.RedemptionCount,
		topUp.TradeNo,
	); err != nil {
		return 0, err
	}
	return 0, nil
}

func recordRedemptionPurchaseTopUpLog(topUp *TopUp, callerIp string, callbackPaymentMethod string) {
	if topUp == nil || !topUp.IsRedemptionPurchase() {
		return
	}
	RecordTopupLog(
		topUp.UserId,
		common.NewMessage("Redemption codes purchased, face value: {{value}}, quantity: {{count}}, payment amount: {{amount}}", map[string]any{"value": topUp.RedemptionAmount, "count": topUp.RedemptionCount, "amount": fmt.Sprintf("%.2f", topUp.Money)}),
		callerIp,
		topUp.PaymentMethod,
		callbackPaymentMethod,
	)
}

func (topUp *TopUp) Update() error {
	var err error
	err = DB.Save(topUp).Error
	return err
}

func GetTopUpById(id int) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("id = ?", id).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

func GetTopUpByTradeNo(tradeNo string) *TopUp {
	var topUp *TopUp
	var err error
	err = DB.Where("trade_no = ?", tradeNo).First(&topUp).Error
	if err != nil {
		return nil
	}
	return topUp
}

// SaveTopUpPayURL records the checkout link of a pending order. expiresAt of
// zero applies TopUpPayURLLifetime.
func SaveTopUpPayURL(tradeNo string, payURL string, expiresAt int64) error {
	if tradeNo == "" || payURL == "" {
		return nil
	}
	if expiresAt <= 0 {
		expiresAt = common.GetTimestamp() + TopUpPayURLLifetime
	}
	return DB.Model(&TopUp{}).
		Where("trade_no = ? AND status = ?", tradeNo, common.TopUpStatusPending).
		Updates(map[string]any{"pay_url": payURL, "pay_url_expires_at": expiresAt}).Error
}

func GetUserTopUpByTradeNo(userId int, tradeNo string) (*TopUp, error) {
	topUp := &TopUp{}
	if err := DB.Where("trade_no = ? AND user_id = ?", tradeNo, userId).First(topUp).Error; err != nil {
		return nil, err
	}
	return topUp, nil
}

// TopUpCryptoPayment exposes the quoted and received crypto amounts of a
// NOWPayments or Monero order next to its top-up record.
type TopUpCryptoPayment struct {
	PayCurrency   string `json:"pay_currency"`
	PayAmount     string `json:"pay_amount"`
	ActuallyPaid  string `json:"actually_paid"`
	GatewayStatus string `json:"gateway_status,omitempty"`
	ExpiresAt     int64  `json:"expires_at"`
}

type TopUpRecord struct {
	*TopUp
	CanContinuePayment bool                `json:"can_continue_payment"`
	CryptoPayment      *TopUpCryptoPayment `json:"crypto_payment,omitempty"`
}

// BuildTopUpRecords attaches resume availability and crypto payment details
// to a page of top-up rows with one query per crypto provider.
func BuildTopUpRecords(topUps []*TopUp) ([]*TopUpRecord, error) {
	records := make([]*TopUpRecord, 0, len(topUps))
	var nowPaymentsIDs, moneroIDs []int
	for _, topUp := range topUps {
		switch topUp.PaymentProvider {
		case PaymentProviderNowPayments:
			nowPaymentsIDs = append(nowPaymentsIDs, topUp.Id)
		case PaymentProviderMonero:
			moneroIDs = append(moneroIDs, topUp.Id)
		}
	}
	nowPayments := make(map[int]NowPaymentsPayment, len(nowPaymentsIDs))
	if len(nowPaymentsIDs) > 0 {
		var payments []NowPaymentsPayment
		if err := DB.Where("top_up_id IN ?", nowPaymentsIDs).Find(&payments).Error; err != nil {
			return nil, err
		}
		for _, payment := range payments {
			nowPayments[*payment.TopUpID] = payment
		}
	}
	moneroPayments := make(map[int]MoneroPayment, len(moneroIDs))
	if len(moneroIDs) > 0 {
		var payments []MoneroPayment
		if err := DB.Where("top_up_id IN ?", moneroIDs).Find(&payments).Error; err != nil {
			return nil, err
		}
		for _, payment := range payments {
			moneroPayments[payment.TopUpID] = payment
		}
	}

	now := common.GetTimestamp()
	for _, topUp := range topUps {
		record := &TopUpRecord{TopUp: topUp}
		pending := topUp.Status == common.TopUpStatusPending
		switch topUp.PaymentProvider {
		case PaymentProviderNowPayments:
			if payment, ok := nowPayments[topUp.Id]; ok {
				record.CryptoPayment = &TopUpCryptoPayment{
					PayCurrency:   payment.PayCurrency,
					PayAmount:     payment.PayAmount,
					ActuallyPaid:  payment.ActuallyPaidAmount,
					GatewayStatus: payment.GatewayStatus,
					ExpiresAt:     payment.ExpiresAt,
				}
				record.CanContinuePayment = pending && payment.Status == NowPaymentsPaymentStatusPending && payment.ExpiresAt > now
			}
		case PaymentProviderMonero:
			if payment, ok := moneroPayments[topUp.Id]; ok {
				record.CryptoPayment = &TopUpCryptoPayment{
					PayCurrency:  "xmr",
					PayAmount:    moneroAtomicToXMR(payment.ExpectedAtomic),
					ActuallyPaid: moneroAtomicToXMR(payment.ReceivedAtomic),
					ExpiresAt:    payment.ExpiresAt,
				}
				record.CanContinuePayment = pending && payment.Status == MoneroPaymentStatusPending && payment.ExpiresAt > now
			}
		case PaymentProviderEpay:
			// Epay links are re-signed on demand, so only the order age matters.
			record.CanContinuePayment = pending && topUp.CreateTime+TopUpPayURLLifetime > now
		default:
			record.CanContinuePayment = pending && topUp.PayURL != "" && topUp.PayURLExpiresAt > now
		}
		records = append(records, record)
	}
	return records, nil
}

func UpdatePendingTopUpStatus(tradeNo string, expectedPaymentProvider string, targetStatus string) error {
	if tradeNo == "" {
		return errors.New("payment order number not provided")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	return DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if expectedPaymentProvider != "" && topUp.PaymentProvider != expectedPaymentProvider {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}

		topUp.Status = targetStatus
		return tx.Save(topUp).Error
	})
}

// RechargeEpay 原子完成易支付订单：订单行锁、状态校验、成功更新与用户额度增加
// 在同一个事务内完成，因此同一订单的并发/重复回调（包括多实例部署下）最多充值一次。
// alreadyDone=true 表示订单此前已完成，本次为幂等重复回调。
// 进程内的 LockOrder 只是优化，正确性由本函数的数据库行锁保证。
func RechargeEpay(tradeNo string, actualPaymentMethod string, callerIp string, paid ...PaidFiat) (alreadyDone bool, err error) {
	if tradeNo == "" {
		return false, errors.New("payment order number not provided")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	var quotaToAdd int
	topUp := &TopUp{}
	err = DB.Transaction(func(tx *gorm.DB) error {
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return ErrTopUpNotFound
		}
		if topUp.PaymentProvider != PaymentProviderEpay {
			return ErrPaymentMethodMismatch
		}
		if topUp.Status == common.TopUpStatusSuccess {
			alreadyDone = true
			return nil
		}
		if topUp.Status != common.TopUpStatusPending {
			return ErrTopUpStatusInvalid
		}
		if actualPaymentMethod != "" && topUp.PaymentMethod != actualPaymentMethod {
			topUp.PaymentMethod = actualPaymentMethod
		}
		if topUp.IsRedemptionPurchase() {
			quotaToAdd = 1
		} else {
			var quotaErr error
			quotaToAdd, quotaErr = common.QuotaFromDecimalStrict(
				decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
			)
			if quotaErr != nil || quotaToAdd <= 0 {
				return ErrInvalidTopUpQuota
			}
		}
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}
		if err := applyReferralPaid(topUp, paid); err != nil {
			return err
		}
		var settleErr error
		quotaToAdd, settleErr = settleTopUp(tx, topUp, quotaToAdd, nil)
		return settleErr
	})
	if err != nil {
		if !errors.Is(err, ErrTopUpNotFound) && !errors.Is(err, ErrPaymentMethodMismatch) && !errors.Is(err, ErrTopUpStatusInvalid) {
			common.SysError(common.LogText("epay topup failed: %s", err.Error()))
		}
		return false, err
	}
	if alreadyDone {
		return true, nil
	}
	if topUp.IsRedemptionPurchase() {
		recordRedemptionPurchaseTopUpLog(topUp, callerIp, PaymentProviderEpay)
		return false, nil
	}
	syncCreditUserQuotaCache(topUp.UserId, quotaToAdd, common.LogText("epay topup"))

	common.SysLog(common.LogText("Epay top-up succeeded trade_no=%s user_id=%d quota_to_add=%d money=%.2f", topUp.TradeNo, topUp.UserId, quotaToAdd, topUp.Money))
	RecordTopupLog(topUp.UserId, common.NewMessage("Online top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"quota": logger.FormatQuota(quotaToAdd), "amount": fmt.Sprintf("%f", topUp.Money)}), callerIp, topUp.PaymentMethod, PaymentProviderEpay)
	return false, nil
}

func Recharge(referenceId string, customerId string, callerIp string, paid ...PaidFiat) (err error) {
	if referenceId == "" {
		return errors.New("payment order number not provided")
	}

	var quota int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("top-up order not found")
		}

		if topUp.PaymentProvider != PaymentProviderStripe {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("invalid top-up order status")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		if topUp.IsRedemptionPurchase() {
			quota = 1
		} else {
			quota, err = common.QuotaFromDecimalStrict(
				decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
			)
			if err != nil || quota <= 0 {
				return ErrInvalidTopUpQuota
			}
		}
		if err := applyReferralPaid(topUp, paid); err != nil {
			return err
		}
		quota, err = settleTopUp(tx, topUp, quota, map[string]interface{}{
			"stripe_customer": customerId,
		})
		return err
	})

	if err != nil {
		common.SysError(common.LogText("topup failed: %s", err.Error()))
		return errors.New("top-up failed")
	}
	if topUp.IsRedemptionPurchase() {
		recordRedemptionPurchaseTopUpLog(topUp, callerIp, PaymentProviderStripe)
		return nil
	}
	syncCreditUserQuotaCache(topUp.UserId, quota, common.LogText("stripe topup"))

	RecordTopupLog(topUp.UserId, common.NewMessage("Online top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"quota": logger.FormatQuota(quota), "amount": topUp.Amount}), callerIp, topUp.PaymentMethod, PaymentMethodStripe)

	return nil
}

// topUpQueryWindowSeconds 限制充值记录查询的时间窗口（秒）。
const topUpQueryWindowSeconds int64 = 30 * 24 * 60 * 60

// topUpQueryCutoff 返回允许查询的最早 create_time（秒级 Unix 时间戳）。
func topUpQueryCutoff() int64 {
	return common.GetTimestamp() - topUpQueryWindowSeconds
}

func GetUserTopUps(userId int, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	// Start transaction
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	cutoff := topUpQueryCutoff()

	// Get total count within transaction
	err = tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, cutoff).Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated topups within same transaction
	err = tx.Where("user_id = ? AND create_time >= ?", userId, cutoff).Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Commit transaction
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// GetAllTopUps 获取全平台的充值记录（管理员使用，不限制时间窗口）
func GetAllTopUps(pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err = tx.Model(&TopUp{}).Count(&total).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return topups, total, nil
}

// searchTopUpCountHardLimit 搜索充值记录时 COUNT 的安全上限，
// 防止对超大表执行无界 COUNT 触发 DoS。
const searchTopUpCountHardLimit = 10000

// SearchUserTopUps 按订单号搜索某用户的充值记录
func SearchUserTopUps(userId int, keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{}).Where("user_id = ? AND create_time >= ?", userId, topUpQueryCutoff())
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError(common.LogText("failed to count search topups: %s", err.Error()))
		return nil, 0, common.NewMessage("Failed to search top-up records")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError(common.LogText("failed to search topups: %s", err.Error()))
		return nil, 0, common.NewMessage("Failed to search top-up records")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// SearchAllTopUps 按订单号搜索全平台充值记录（管理员使用，不限制时间窗口）
func SearchAllTopUps(keyword string, pageInfo *common.PageInfo) (topups []*TopUp, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&TopUp{})
	if keyword != "" {
		pattern, perr := sanitizeLikePattern(keyword)
		if perr != nil {
			tx.Rollback()
			return nil, 0, perr
		}
		query = query.Where("trade_no LIKE ? ESCAPE '!'", pattern)
	}

	if err = query.Limit(searchTopUpCountHardLimit).Count(&total).Error; err != nil {
		tx.Rollback()
		common.SysError(common.LogText("failed to count search topups: %s", err.Error()))
		return nil, 0, common.NewMessage("Failed to search top-up records")
	}

	if err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&topups).Error; err != nil {
		tx.Rollback()
		common.SysError(common.LogText("failed to search topups: %s", err.Error()))
		return nil, 0, common.NewMessage("Failed to search top-up records")
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}
	return topups, total, nil
}

// ManualCompleteTopUp 管理员手动完成订单并给用户充值
func ManualCompleteTopUp(tradeNo string, callerIp string) error {
	if tradeNo == "" {
		return common.NewMessage("Order number not provided")
	}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	var userId int
	var quotaToAdd int
	var payMoney float64
	var paymentMethod string
	var redemptionPurchase bool
	var redemptionTopUp TopUp

	err := DB.Transaction(func(tx *gorm.DB) error {
		topUp := &TopUp{}
		// 行级锁，避免并发补单
		if err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error; err != nil {
			return common.NewMessage("Top-up order does not exist")
		}

		// 幂等处理：已成功直接返回
		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return common.NewMessage("Order is not pending payment and cannot be completed")
		}
		if topUp.PaymentProvider == PaymentProviderMonero || topUp.PaymentProvider == PaymentProviderNowPayments {
			return common.NewMessage("Crypto orders must be completed through gateway reconciliation")
		}

		// 计算应充值额度：
		// - Stripe 订单：Money 代表经分组倍率换算后的美元数量，直接 * QuotaPerUnit
		// - Creem 订单：Amount 即商品配置的额度
		// - 其他订单（如易支付）：Amount 为美元数量，* QuotaPerUnit
		if topUp.IsRedemptionPurchase() {
			quotaToAdd = 1
		} else {
			var quotaErr error
			switch topUp.PaymentProvider {
			case PaymentProviderStripe:
				quotaToAdd, quotaErr = common.QuotaFromDecimalStrict(
					decimal.NewFromFloat(topUp.Money).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
				)
			case PaymentProviderCreem:
				// Creem orders store the product quota itself, matching RechargeCreem.
				quotaToAdd, quotaErr = common.QuotaFromDecimalStrict(decimal.NewFromInt(topUp.Amount))
			default:
				quotaToAdd, quotaErr = common.QuotaFromDecimalStrict(
					decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
				)
			}
			if quotaErr != nil || quotaToAdd <= 0 {
				return ErrInvalidTopUpQuota
			}
		}

		// 标记完成
		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		// 增加用户额度（立即写库，保持一致性）
		// Administrator completion is not a verified fiat payment callback.
		topUp.ReferralSnapshot = ""
		var settleErr error
		quotaToAdd, settleErr = settleTopUp(tx, topUp, quotaToAdd, nil)
		if settleErr != nil {
			return settleErr
		}

		userId = topUp.UserId
		payMoney = topUp.Money
		paymentMethod = topUp.PaymentMethod
		redemptionPurchase = topUp.IsRedemptionPurchase()
		if redemptionPurchase {
			redemptionTopUp = *topUp
		}
		return nil
	})

	if err != nil {
		return err
	}

	// 事务外记录日志，避免阻塞
	if redemptionPurchase {
		recordRedemptionPurchaseTopUpLog(&redemptionTopUp, callerIp, "admin")
		return nil
	}
	syncCreditUserQuotaCache(userId, quotaToAdd, common.LogText("manual topup"))
	RecordTopupLog(userId, common.NewMessage("Administrator completed the order, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"quota": logger.FormatQuota(quotaToAdd), "amount": fmt.Sprintf("%f", payMoney)}), callerIp, paymentMethod, "admin")
	return nil
}
func RechargeCreem(referenceId string, customerEmail string, customerName string, callerIp string, paid ...PaidFiat) (err error) {
	if referenceId == "" {
		return errors.New("payment order number not provided")
	}

	var quota int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", referenceId).First(topUp).Error
		if err != nil {
			return errors.New("top-up order not found")
		}

		if topUp.PaymentProvider != PaymentProviderCreem {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("invalid top-up order status")
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		err = tx.Save(topUp).Error
		if err != nil {
			return err
		}

		// Creem 直接使用 Amount 作为充值额度（整数）
		if topUp.IsRedemptionPurchase() {
			quota = 1
		} else {
			quota, err = common.QuotaFromDecimalStrict(decimal.NewFromInt(topUp.Amount))
			if err != nil || quota <= 0 {
				return ErrInvalidTopUpQuota
			}
		}

		// 构建更新字段，优先使用邮箱，如果邮箱为空则使用用户名
		updateFields := map[string]interface{}{}

		// 如果有客户邮箱，尝试更新用户邮箱（仅当用户邮箱为空时）
		if customerEmail != "" {
			// 先检查用户当前邮箱是否为空
			var user User
			err = tx.Where("id = ?", topUp.UserId).First(&user).Error
			if err != nil {
				return err
			}

			// 如果用户邮箱为空，则更新为支付时使用的邮箱
			if user.Email == "" {
				updateFields["email"] = customerEmail
			}
		}

		if err := applyReferralPaid(topUp, paid); err != nil {
			return err
		}
		quota, err = settleTopUp(tx, topUp, quota, updateFields)
		return err
	})

	if err != nil {
		common.SysError(common.LogText("creem topup failed: %s", err.Error()))
		return errors.New("top-up failed")
	}
	if topUp.IsRedemptionPurchase() {
		recordRedemptionPurchaseTopUpLog(topUp, callerIp, PaymentProviderCreem)
		return nil
	}
	syncCreditUserQuotaCache(topUp.UserId, quota, common.LogText("creem topup"))

	RecordTopupLog(topUp.UserId, common.NewMessage("{{provider}} top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"provider": "Creem", "quota": quota, "amount": fmt.Sprintf("%.2f", topUp.Money)}), callerIp, topUp.PaymentMethod, PaymentMethodCreem)

	return nil
}

func RechargeWaffo(tradeNo string, callerIp string, paid ...PaidFiat) (err error) {
	if tradeNo == "" {
		return errors.New("payment order number not provided")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("top-up order not found")
		}

		if topUp.PaymentProvider != PaymentProviderWaffo {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil // 幂等：已成功直接返回
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("invalid top-up order status")
		}

		if topUp.IsRedemptionPurchase() {
			quotaToAdd = 1
		} else {
			quotaToAdd, err = common.QuotaFromDecimalStrict(
				decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
			)
			if err != nil || quotaToAdd <= 0 {
				return ErrInvalidTopUpQuota
			}
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := applyReferralPaid(topUp, paid); err != nil {
			return err
		}
		var settleErr error
		quotaToAdd, settleErr = settleTopUp(tx, topUp, quotaToAdd, nil)
		return settleErr
	})

	if err != nil {
		common.SysError(common.LogText("waffo topup failed: %s", err.Error()))
		return errors.New("top-up failed")
	}
	if topUp.IsRedemptionPurchase() {
		recordRedemptionPurchaseTopUpLog(topUp, callerIp, PaymentProviderWaffo)
		return nil
	}
	syncCreditUserQuotaCache(topUp.UserId, quotaToAdd, common.LogText("waffo topup"))

	if quotaToAdd > 0 {
		RecordTopupLog(topUp.UserId, common.NewMessage("{{provider}} top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"provider": "Waffo", "quota": logger.FormatQuota(quotaToAdd), "amount": fmt.Sprintf("%.2f", topUp.Money)}), callerIp, topUp.PaymentMethod, PaymentMethodWaffo)
	}

	return nil
}

func RechargeWaffoPancake(tradeNo string, paid ...PaidFiat) (err error) {
	if tradeNo == "" {
		return errors.New("payment order number not provided")
	}

	var quotaToAdd int
	topUp := &TopUp{}

	refCol := "`trade_no`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		refCol = `"trade_no"`
	}

	err = DB.Transaction(func(tx *gorm.DB) error {
		err := lockForUpdate(tx).Where(refCol+" = ?", tradeNo).First(topUp).Error
		if err != nil {
			return errors.New("top-up order not found")
		}

		if topUp.PaymentProvider != PaymentProviderWaffoPancake {
			return ErrPaymentMethodMismatch
		}

		if topUp.Status == common.TopUpStatusSuccess {
			return nil
		}

		if topUp.Status != common.TopUpStatusPending {
			return errors.New("invalid top-up order status")
		}

		if topUp.IsRedemptionPurchase() {
			quotaToAdd = 1
		} else {
			quotaToAdd, err = common.QuotaFromDecimalStrict(
				decimal.NewFromInt(topUp.Amount).Mul(decimal.NewFromFloat(common.QuotaPerUnit)),
			)
			if err != nil || quotaToAdd <= 0 {
				return ErrInvalidTopUpQuota
			}
		}

		topUp.CompleteTime = common.GetTimestamp()
		topUp.Status = common.TopUpStatusSuccess
		if err := tx.Save(topUp).Error; err != nil {
			return err
		}

		if err := applyReferralPaid(topUp, paid); err != nil {
			return err
		}
		var settleErr error
		quotaToAdd, settleErr = settleTopUp(tx, topUp, quotaToAdd, nil)
		return settleErr
	})

	if err != nil {
		common.SysError(common.LogText("waffo pancake topup failed: %s", err.Error()))
		return errors.New("top-up failed")
	}
	if topUp.IsRedemptionPurchase() {
		recordRedemptionPurchaseTopUpLog(topUp, "", PaymentProviderWaffoPancake)
		return nil
	}
	syncCreditUserQuotaCache(topUp.UserId, quotaToAdd, common.LogText("waffo pancake topup"))

	if quotaToAdd > 0 {
		RecordLog(topUp.UserId, LogTypeTopup, common.NewMessage("{{provider}} top-up succeeded, amount added: {{quota}}, payment amount: {{amount}}", map[string]any{"provider": "Waffo Pancake", "quota": logger.FormatQuota(quotaToAdd), "amount": common.NewMessage(fmt.Sprintf("%.2f", topUp.Money))}))
	}

	return nil
}
