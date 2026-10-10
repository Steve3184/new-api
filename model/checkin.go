package model

import (
	"database/sql"
	"errors"
	"fmt"
	"github.com/QuantumNous/new-api/common"
	"math/rand"
	"time"

	"github.com/QuantumNous/new-api/setting/operation_setting"
	"gorm.io/gorm"
)

// Checkin 签到记录
type Checkin struct {
	Id           int    `json:"id" gorm:"primaryKey;autoIncrement"`
	UserId       int    `json:"user_id" gorm:"not null;uniqueIndex:idx_user_checkin_date"`
	CheckinDate  string `json:"checkin_date" gorm:"type:varchar(10);not null;uniqueIndex:idx_user_checkin_date"` // 格式: YYYY-MM-DD
	QuotaAwarded int    `json:"quota_awarded" gorm:"not null"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint"`
}

// CheckinRecord 用于API返回的签到记录（不包含敏感字段）
type CheckinRecord struct {
	CheckinDate  string `json:"checkin_date"`
	QuotaAwarded int    `json:"quota_awarded"`
}

type CheckinEligibility struct {
	Eligible            bool  `json:"eligible"`
	AlreadyCheckedIn    bool  `json:"already_checked_in"`
	BalanceMet          bool  `json:"balance_met"`
	TotalSpendMet       bool  `json:"total_spend_met"`
	DailyUserLimitMet   bool  `json:"daily_user_limit_met"`
	DailyQuotaLimitMet  bool  `json:"daily_quota_limit_met"`
	CurrentQuota        int   `json:"current_quota"`
	CurrentUsedQuota    int   `json:"current_used_quota"`
	TodayUserCount      int64 `json:"today_user_count"`
	TodayQuotaAwarded   int64 `json:"today_quota_awarded"`
	DailyQuotaRemaining int64 `json:"daily_quota_remaining"`
}

func (Checkin) TableName() string {
	return "checkins"
}

// GetUserCheckinRecords 获取用户在指定日期范围内的签到记录
func GetUserCheckinRecords(userId int, startDate, endDate string) ([]Checkin, error) {
	var records []Checkin
	err := DB.Where("user_id = ? AND checkin_date >= ? AND checkin_date <= ?",
		userId, startDate, endDate).
		Order("checkin_date DESC").
		Find(&records).Error
	return records, err
}

// HasCheckedInToday 检查用户今天是否已签到
func HasCheckedInToday(userId int) (bool, error) {
	today := time.Now().Format("2006-01-02")
	var count int64
	err := DB.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&count).Error
	return count > 0, err
}

func getCheckinEligibility(tx *gorm.DB, userId int, setting *operation_setting.CheckinSetting, lockUser bool) (CheckinEligibility, error) {
	var user User
	query := tx.Select("id", "quota", "used_quota").Where("id = ?", userId)
	if lockUser {
		query = lockForUpdate(query)
	}
	if err := query.First(&user).Error; err != nil {
		return CheckinEligibility{}, err
	}

	today := time.Now().Format("2006-01-02")
	var daily struct {
		UserCount    int64 `gorm:"column:user_count"`
		QuotaAwarded int64 `gorm:"column:quota_awarded"`
	}
	if err := tx.Model(&Checkin{}).
		Select("COUNT(*) AS user_count, COALESCE(SUM(quota_awarded), 0) AS quota_awarded").
		Where("checkin_date = ?", today).
		Scan(&daily).Error; err != nil {
		return CheckinEligibility{}, err
	}
	var checkedCount int64
	if err := tx.Model(&Checkin{}).
		Where("user_id = ? AND checkin_date = ?", userId, today).
		Count(&checkedCount).Error; err != nil {
		return CheckinEligibility{}, err
	}

	remaining := int64(-1)
	if setting.DailyQuotaLimit > 0 {
		remaining = max(int64(setting.DailyQuotaLimit)-daily.QuotaAwarded, 0)
	}
	eligibility := CheckinEligibility{
		AlreadyCheckedIn:    checkedCount > 0,
		BalanceMet:          setting.MinUserQuota <= 0 || user.Quota > setting.MinUserQuota,
		TotalSpendMet:       setting.MinUsedQuota <= 0 || user.UsedQuota >= setting.MinUsedQuota,
		DailyUserLimitMet:   setting.DailyUserLimit <= 0 || daily.UserCount < int64(setting.DailyUserLimit),
		DailyQuotaLimitMet:  setting.DailyQuotaLimit <= 0 || remaining >= int64(setting.MaxQuota),
		CurrentQuota:        user.Quota,
		CurrentUsedQuota:    user.UsedQuota,
		TodayUserCount:      daily.UserCount,
		TodayQuotaAwarded:   daily.QuotaAwarded,
		DailyQuotaRemaining: remaining,
	}
	eligibility.Eligible = !eligibility.AlreadyCheckedIn && eligibility.BalanceMet && eligibility.TotalSpendMet && eligibility.DailyUserLimitMet && eligibility.DailyQuotaLimitMet
	return eligibility, nil
}

func GetCheckinEligibility(userId int) (CheckinEligibility, error) {
	return getCheckinEligibility(DB, userId, operation_setting.GetCheckinSetting(), false)
}

func checkinEligibilityError(eligibility CheckinEligibility, setting *operation_setting.CheckinSetting) error {
	switch {
	case eligibility.AlreadyCheckedIn:
		return errors.New("今日已签到")
	case !eligibility.BalanceMet:
		return fmt.Errorf("用户余额必须大于 %d 才能签到", setting.MinUserQuota)
	case !eligibility.TotalSpendMet:
		return fmt.Errorf("用户累计消费必须达到 %d 才能签到", setting.MinUsedQuota)
	case !eligibility.DailyUserLimitMet:
		return errors.New("今日签到人数已达上限")
	case !eligibility.DailyQuotaLimitMet:
		return errors.New("今日签到赠金额度不足")
	default:
		return errors.New("当前不满足签到条件")
	}
}

// UserCheckin executes eligibility checks and the award in one serializable transaction.
func UserCheckin(userId int) (*Checkin, error) {
	setting := operation_setting.GetCheckinSetting()
	if !setting.Enabled {
		return nil, common.NewMessage("Check-in feature is not enabled")
	}
	if setting.MinQuota < 0 || setting.MaxQuota < setting.MinQuota {
		return nil, errors.New("签到奖励范围配置无效")
	}

	var checkin *Checkin
	err := DB.Transaction(func(tx *gorm.DB) error {
		eligibility, err := getCheckinEligibility(tx, userId, setting, true)
		if err != nil {
			return err
		}
		if !eligibility.Eligible {
			return checkinEligibilityError(eligibility, setting)
		}
		quotaAwarded := setting.MinQuota
		if setting.MaxQuota > setting.MinQuota {
			quotaAwarded += rand.Intn(setting.MaxQuota - setting.MinQuota + 1)
		}
		checkin = &Checkin{
			UserId:       userId,
			CheckinDate:  time.Now().Format("2006-01-02"),
			QuotaAwarded: quotaAwarded,
			CreatedAt:    time.Now().Unix(),
		}
		if err := tx.Create(checkin).Error; err != nil {
			return common.NewMessage("Check-in failed, please try again later")
		}
		column := "quota"
		if setting.DeductibleGroups != "" {
			column = "checkin_quota"
		}
		if err := tx.Model(&User{}).Where("id = ?", userId).
			Update(column, gorm.Expr(column+" + ?", quotaAwarded)).Error; err != nil {
			return errors.New("签到失败：更新额度出错")
		}
		return nil
	}, &sql.TxOptions{Isolation: sql.LevelSerializable})

	if err != nil {
		return nil, err
	}
	if setting.DeductibleGroups == "" {
		go func() { _ = cacheIncrUserQuota(userId, int64(checkin.QuotaAwarded)) }()
	}
	return checkin, nil
}

// GetUserCheckinStats 获取用户签到统计信息
func GetUserCheckinStats(userId int, month string) (map[string]any, error) {
	// 获取指定月份的所有签到记录
	startDate := month + "-01"
	endDate := month + "-31"

	records, err := GetUserCheckinRecords(userId, startDate, endDate)
	if err != nil {
		return nil, err
	}

	// 转换为不包含敏感字段的记录
	checkinRecords := make([]CheckinRecord, len(records))
	for i, r := range records {
		checkinRecords[i] = CheckinRecord{
			CheckinDate:  r.CheckinDate,
			QuotaAwarded: r.QuotaAwarded,
		}
	}

	// 检查今天是否已签到
	hasCheckedToday, _ := HasCheckedInToday(userId)

	// 获取用户所有时间的签到统计
	var totalCheckins int64
	var totalQuota int64
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Count(&totalCheckins)
	DB.Model(&Checkin{}).Where("user_id = ?", userId).Select("COALESCE(SUM(quota_awarded), 0)").Scan(&totalQuota)

	return map[string]any{
		"total_quota":      totalQuota,      // 所有时间累计获得的额度
		"total_checkins":   totalCheckins,   // 所有时间累计签到次数
		"checkin_count":    len(records),    // 本月签到次数
		"checked_in_today": hasCheckedToday, // 今天是否已签到
		"records":          checkinRecords,  // 本月签到记录详情（不含id和user_id）
	}, nil
}
