package model

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/logger"

	"gorm.io/gorm"
)

type Redemption struct {
	Id                 int            `json:"id"`
	UserId             int            `json:"user_id"`
	Key                string         `json:"key" gorm:"type:char(32);uniqueIndex"`
	Status             int            `json:"status" gorm:"default:1"`
	Name               string         `json:"name" gorm:"index"`
	Quota              int            `json:"quota" gorm:"default:100"`
	SubscriptionPlanId int            `json:"subscription_plan_id" gorm:"index;default:0"`
	CreatedTime        int64          `json:"created_time" gorm:"bigint"`
	RedeemedTime       int64          `json:"redeemed_time" gorm:"bigint"`
	Count              int            `json:"count" gorm:"-:all"` // only for api request
	UsedUserId         int            `json:"used_user_id"`
	CreatorType        string         `json:"creator_type" gorm:"type:varchar(16);index"`
	OwnerId            int            `json:"owner_id" gorm:"index"`
	PurchaseTradeNo    string         `json:"purchase_trade_no" gorm:"type:varchar(255);index"`
	PurchaseAmount     int64          `json:"purchase_amount"`
	RefundedTime       int64          `json:"refunded_time" gorm:"bigint"`
	DeletedAt          gorm.DeletedAt `gorm:"index"`
	ExpiredTime        int64          `json:"expired_time" gorm:"bigint"` // 过期时间，0 表示不过期
}

const (
	RedemptionCreatorAdmin = "admin"
	RedemptionCreatorUser  = "user"
)

var (
	ErrRedemptionRefundNotAllowed = errors.New("兑换码不可退款")
	ErrRedemptionNotOwned         = errors.New("兑换码不属于当前用户")
)

type RedemptionResult struct {
	Quota                 int
	SubscriptionPlanId    int
	SubscriptionPlanTitle string
}

func normalizeSubscriptionRedemptionQuotas() error {
	return DB.Model(&Redemption{}).
		Where("subscription_plan_id > ? AND quota <> ?", 0, 0).
		Update("quota", 0).Error
}

func GetAllRedemptions(startIdx int, num int, creatorType ...string) (redemptions []*Redemption, total int64, err error) {
	// 开始事务
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// 获取总数
	query := applyRedemptionCreatorFilter(tx.Model(&Redemption{}), creatorType)
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 获取分页数据
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// 提交事务
	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func SearchRedemptions(keyword string, status string, startIdx int, num int, creatorType ...string) (redemptions []*Redemption, total int64, err error) {
	tx := DB.Begin()
	if tx.Error != nil {
		return nil, 0, tx.Error
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	query := tx.Model(&Redemption{})
	query = applyRedemptionCreatorFilter(query, creatorType)

	if keyword != "" {
		if id, err := strconv.Atoi(keyword); err == nil {
			query = query.Where("id = ? OR name LIKE ?", id, keyword+"%")
		} else {
			query = query.Where("name LIKE ?", keyword+"%")
		}
	}

	if status != "" {
		now := common.GetTimestamp()
		switch status {
		case "expired":
			query = query.Where(
				"status = ? AND expired_time != 0 AND expired_time < ?",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusEnabled):
			query = query.Where(
				"status = ? AND (expired_time = 0 OR expired_time >= ?)",
				common.RedemptionCodeStatusEnabled,
				now,
			)
		case strconv.Itoa(common.RedemptionCodeStatusDisabled):
			query = query.Where("status = ?", common.RedemptionCodeStatusDisabled)
		case strconv.Itoa(common.RedemptionCodeStatusUsed):
			query = query.Where("status = ?", common.RedemptionCodeStatusUsed)
		}
	}

	// Get total count
	err = query.Count(&total).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	// Get paginated data
	err = query.Order("id desc").Limit(num).Offset(startIdx).Find(&redemptions).Error
	if err != nil {
		tx.Rollback()
		return nil, 0, err
	}

	if err = tx.Commit().Error; err != nil {
		return nil, 0, err
	}

	return redemptions, total, nil
}

func applyRedemptionCreatorFilter(query *gorm.DB, creatorType []string) *gorm.DB {
	if len(creatorType) == 0 || strings.TrimSpace(creatorType[0]) == "" || creatorType[0] == "all" {
		return query
	}
	switch strings.TrimSpace(creatorType[0]) {
	case RedemptionCreatorUser:
		return query.Where("creator_type = ?", RedemptionCreatorUser)
	case RedemptionCreatorAdmin:
		// Legacy rows predate creator_type and were all administrator-created.
		return query.Where("(creator_type = ? OR creator_type = '')", RedemptionCreatorAdmin)
	default:
		return query
	}
}

func GetRedemptionById(id int) (*Redemption, error) {
	if id == 0 {
		return nil, errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	var err error = nil
	err = DB.First(&redemption, "id = ?", id).Error
	return &redemption, err
}

func Redeem(key string, userId int) (quota int, err error) {
	result, err := RedeemWithResult(key, userId)
	if err != nil {
		return 0, err
	}
	return result.Quota, nil
}

func RedeemWithResult(key string, userId int) (result *RedemptionResult, err error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("未提供兑换码")
	}
	if userId == 0 {
		return nil, errors.New("无效的 user id")
	}
	keyCol := "`key`"
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		keyCol = `"key"`
	}
	const maxAttempts = 3
	for attempt := 0; attempt < maxAttempts; attempt++ {
		redemption := &Redemption{}
		result = &RedemptionResult{}
		upgradeGroup := ""
		err = DB.Transaction(func(tx *gorm.DB) error {
			lookupErr := lockForUpdate(tx).Where(keyCol+" = ?", key).First(redemption).Error
			if lookupErr != nil {
				if isRetryableRedemptionError(lookupErr) {
					return lookupErr
				}
				return errors.New("无效的兑换码")
			}
			if redemption.Status != common.RedemptionCodeStatusEnabled {
				return errors.New("该兑换码已被使用")
			}
			if redemption.ExpiredTime != 0 && redemption.ExpiredTime < common.GetTimestamp() {
				return errors.New("该兑换码已过期")
			}
			var plan *SubscriptionPlan
			if redemption.SubscriptionPlanId > 0 {
				var planErr error
				plan, planErr = getSubscriptionPlanByIdTx(tx, redemption.SubscriptionPlanId)
				if planErr != nil {
					if isRetryableRedemptionError(planErr) {
						return planErr
					}
					return errors.New("兑换码关联的订阅套餐不存在")
				}
				if !plan.Enabled {
					return errors.New("兑换码关联的订阅套餐已禁用")
				}
			}
			// Compare-and-swap on status: only the transaction that flips
			// enabled -> used may credit quota, so a concurrent redeem of the
			// same code loses here even without a row lock (e.g. on SQLite).
			updates := map[string]interface{}{
				"redeemed_time": common.GetTimestamp(),
				"status":        common.RedemptionCodeStatusUsed,
				"used_user_id":  userId,
			}
			if plan != nil {
				updates["quota"] = 0
			}
			updateResult := tx.Model(&Redemption{}).
				Where("id = ? AND status = ?", redemption.Id, common.RedemptionCodeStatusEnabled).
				Updates(updates)
			if updateResult.Error != nil {
				return updateResult.Error
			}
			if updateResult.RowsAffected == 0 {
				return errors.New("该兑换码已被使用")
			}
			if plan != nil {
				if _, err := CreateUserSubscriptionFromPlanTx(tx, userId, plan, "redemption"); err != nil {
					return err
				}
				result.SubscriptionPlanId = plan.Id
				result.SubscriptionPlanTitle = plan.Title
				upgradeGroup = strings.TrimSpace(plan.UpgradeGroup)
				return nil
			}
			result.Quota = redemption.Quota
			quotaUpdate := tx.Model(&User{}).
				Where("id = ?", userId).
				Update("quota", gorm.Expr("quota + ?", redemption.Quota))
			if quotaUpdate.Error != nil {
				return quotaUpdate.Error
			}
			if quotaUpdate.RowsAffected == 0 {
				return errors.New("用户不存在")
			}
			return nil
		})
		if err == nil {
			if result.SubscriptionPlanId > 0 {
				InvalidateUserSubscriptionRateLimitCache(userId)
				if upgradeGroup != "" {
					refreshSubscriptionUserGroupCache(userId, "redemption completion")
				}
				RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码兑换订阅套餐 %s，兑换码ID %d", result.SubscriptionPlanTitle, redemption.Id))
			} else {
				RecordLog(userId, LogTypeTopup, fmt.Sprintf("通过兑换码充值 %s，兑换码ID %d", logger.LogQuota(result.Quota), redemption.Id))
			}
			return result, nil
		}
		if !isRetryableRedemptionError(err) || attempt == maxAttempts-1 {
			break
		}
		time.Sleep(time.Duration(attempt+1) * 10 * time.Millisecond)
	}
	if err != nil {
		common.SysError("redemption failed: " + err.Error())
		return nil, ErrRedeemFailed
	}
	return nil, ErrRedeemFailed
}

func isRetryableRedemptionError(err error) bool {
	if err == nil {
		return false
	}
	errText := strings.ToLower(err.Error())
	return strings.Contains(errText, "database is locked") ||
		strings.Contains(errText, "database table is locked")
}

func (redemption *Redemption) Insert() error {
	if redemption.SubscriptionPlanId <= 0 {
		return DB.Create(redemption).Error
	}

	redemption.Quota = 0
	status := redemption.Status
	if status == 0 {
		status = common.RedemptionCodeStatusEnabled
	}
	return DB.Model(&Redemption{}).Create(map[string]interface{}{
		"user_id":              redemption.UserId,
		"key":                  redemption.Key,
		"status":               status,
		"name":                 redemption.Name,
		"quota":                0,
		"subscription_plan_id": redemption.SubscriptionPlanId,
		"created_time":         redemption.CreatedTime,
		"redeemed_time":        redemption.RedeemedTime,
		"used_user_id":         redemption.UsedUserId,
		"creator_type":         redemption.CreatorType,
		"owner_id":             redemption.OwnerId,
		"purchase_trade_no":    redemption.PurchaseTradeNo,
		"purchase_amount":      redemption.PurchaseAmount,
		"refunded_time":        redemption.RefundedTime,
		"expired_time":         redemption.ExpiredTime,
	}).Error
}

func (redemption *Redemption) SelectUpdate() error {
	// This can update zero values
	return DB.Model(redemption).Select("redeemed_time", "status").Updates(redemption).Error
}

// Update writes every editable field explicitly, including zero values.
func (redemption *Redemption) Update() error {
	var err error
	err = DB.Model(redemption).Select("name", "status", "quota", "subscription_plan_id", "redeemed_time", "expired_time").Updates(redemption).Error
	return err
}

func CreatePurchasedRedemptionsTx(tx *gorm.DB, userId int, name string, quota int, purchaseAmount int64, count int, tradeNo string) ([]*Redemption, error) {
	if userId <= 0 || quota <= 0 || purchaseAmount <= 0 || count <= 0 || count > MaxRedemptionPurchaseCount || strings.TrimSpace(tradeNo) == "" {
		return nil, errors.New("兑换码购买参数无效")
	}
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) == 0 {
		name = "Purchased code"
	}
	if utf8.RuneCountInString(name) > 20 {
		name = string([]rune(name)[:20])
	}
	redemptions := make([]*Redemption, 0, count)
	for i := 0; i < count; i++ {
		redemptions = append(redemptions, &Redemption{
			UserId:          userId,
			OwnerId:         userId,
			CreatorType:     RedemptionCreatorUser,
			Name:            name,
			Key:             common.GetUUID(),
			Status:          common.RedemptionCodeStatusEnabled,
			Quota:           quota,
			CreatedTime:     common.GetTimestamp(),
			PurchaseTradeNo: tradeNo,
			PurchaseAmount:  purchaseAmount,
		})
	}
	if err := tx.Create(&redemptions).Error; err != nil {
		return nil, err
	}
	return redemptions, nil
}

func GetUserRedemptions(userId int, pageInfo *common.PageInfo) (redemptions []*Redemption, total int64, err error) {
	query := DB.Model(&Redemption{}).Where("owner_id = ? AND creator_type = ?", userId, RedemptionCreatorUser)
	if err = query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err = query.Order("id desc").Limit(pageInfo.GetPageSize()).Offset(pageInfo.GetStartIdx()).Find(&redemptions).Error
	return redemptions, total, err
}

func RefundPurchasedRedemption(id int, userId int) (int, error) {
	if id <= 0 || userId <= 0 {
		return 0, ErrRedemptionRefundNotAllowed
	}
	var quota int
	err := DB.Transaction(func(tx *gorm.DB) error {
		redemption := &Redemption{}
		if err := lockForUpdate(tx).Where("id = ?", id).First(redemption).Error; err != nil {
			return ErrRedemptionRefundNotAllowed
		}
		if redemption.OwnerId != userId || redemption.CreatorType != RedemptionCreatorUser {
			return ErrRedemptionNotOwned
		}
		if redemption.PurchaseTradeNo == "" || redemption.Status != common.RedemptionCodeStatusEnabled || redemption.RefundedTime != 0 || redemption.Quota <= 0 {
			return ErrRedemptionRefundNotAllowed
		}
		quota = redemption.Quota
		if err := creditTopUpQuota(tx, userId, quota, nil); err != nil {
			return err
		}
		result := tx.Model(&Redemption{}).
			Where("id = ? AND owner_id = ? AND status = ? AND refunded_time = 0", id, userId, common.RedemptionCodeStatusEnabled).
			Updates(map[string]interface{}{
				"status":        common.RedemptionCodeStatusDisabled,
				"refunded_time": common.GetTimestamp(),
			})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrRedemptionRefundNotAllowed
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	syncCreditUserQuotaCache(userId, quota, "redemption refund")
	RecordLog(userId, LogTypeRefund, fmt.Sprintf("兑换码退款到账 %s，兑换码ID %d", logger.LogQuota(quota), id))
	return quota, nil
}

func (redemption *Redemption) Delete() error {
	var err error
	err = DB.Delete(redemption).Error
	return err
}

func DeleteRedemptionById(id int) (err error) {
	if id == 0 {
		return errors.New("id 为空！")
	}
	redemption := Redemption{Id: id}
	err = DB.Where(redemption).First(&redemption).Error
	if err != nil {
		return err
	}
	return redemption.Delete()
}

func BatchDeleteRedemptions(ids []int) (int64, error) {
	if len(ids) == 0 {
		return 0, errors.New("ids 为空！")
	}
	result := DB.Where("id IN ?", ids).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}

func DeleteInvalidRedemptions() (int64, error) {
	now := common.GetTimestamp()
	result := DB.Where("status IN ? OR (status = ? AND expired_time != 0 AND expired_time < ?)", []int{common.RedemptionCodeStatusUsed, common.RedemptionCodeStatusDisabled}, common.RedemptionCodeStatusEnabled, now).Delete(&Redemption{})
	return result.RowsAffected, result.Error
}
