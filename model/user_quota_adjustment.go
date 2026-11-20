package model

import (
	"context"
	"errors"

	"github.com/QuantumNous/new-api/common"
	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

var (
	ErrInvalidUserQuotaAdjustment = errors.New("invalid user quota adjustment")
	ErrUserQuotaPermission        = errors.New("cannot adjust quota for this user role")
)

// UserQuotaAdjustment is the immutable database snapshot of a committed manual
// adjustment. Pending relay deductions in the quota cache are not part of it.
type UserQuotaAdjustment struct {
	UserID   int
	Username string
	Before   int
	After    int
}

func AdjustUserQuota(userID, operatorRole int, mode string, value int) (*UserQuotaAdjustment, error) {
	if userID <= 0 || (mode != "add" && mode != "subtract" && mode != "override") {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if mode != "override" && value <= 0 {
		return nil, ErrInvalidUserQuotaAdjustment
	}
	if value > common.MaxWalletQuota || value < -common.MaxWalletQuota {
		return nil, ErrWalletQuotaLimitExceeded
	}

	var adjustment UserQuotaAdjustment
	var cacheTicket string
	var cacheErr error
	err := DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx).First(&user, userID).Error; err != nil {
			return err
		}
		if operatorRole != common.RoleRootUser && operatorRole <= user.Role {
			return ErrUserQuotaPermission
		}
		if user.Quota > common.MaxWalletQuota || user.Quota < -common.MaxWalletQuota {
			return ErrWalletQuotaLimitExceeded
		}
		quota := decimal.NewFromInt(int64(value))
		switch mode {
		case "add":
			quota = decimal.NewFromInt(int64(user.Quota)).Add(quota)
		case "subtract":
			quota = decimal.NewFromInt(int64(user.Quota)).Sub(quota)
		}
		after, err := common.WalletQuotaFromDecimalStrict(quota)
		if err != nil {
			return ErrWalletQuotaLimitExceeded
		}
		// An unchanged override is a successful operation, including on MySQL
		// configurations that count only changed rows in RowsAffected.
		if after != user.Quota {
			result := tx.Model(&User{}).Where("id = ?", userID).Update("quota", after)
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return gorm.ErrRecordNotFound
			}
			if common.RedisEnabled {
				// Mark only the live hash while hydration is excluded. A refill
				// includes the committed adjustment and won't carry its ticket.
				cacheTicket = "quota-adjust:" + common.GetUUID()
				const tag = `if redis.call('HEXISTS', KEYS[1], 'Quota') == 1 then
  redis.call('HSET', KEYS[1], ARGV[1], '1')
end
return 1`
				cacheErr = common.RDB.Eval(context.Background(), tag, []string{getUserCacheKey(userID)}, cacheTicket).Err()
			}
		}
		adjustment = UserQuotaAdjustment{UserID: user.Id, Username: user.Username, Before: user.Quota, After: after}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Preserve in-flight reservations on the original hash, even for overrides.
	if cacheTicket != "" && cacheErr == nil {
		delta := int64(adjustment.After) - int64(adjustment.Before)
		const publish = `if redis.call('HEXISTS', KEYS[1], ARGV[1]) == 1 then
  redis.call('HINCRBY', KEYS[1], 'Quota', ARGV[2])
  redis.call('HDEL', KEYS[1], ARGV[1])
end
return 1`
		cacheErr = common.RDB.Eval(context.Background(), publish, []string{getUserCacheKey(userID)}, cacheTicket, delta).Err()
	}
	if cacheErr != nil {
		// SQL is already committed. An ambiguous Redis reply is safe to
		// recover with invalidation, never with another unguarded delta.
		if err := invalidateCommittedWalletCache(userID); err != nil {
			common.SysError(common.LogText("failed to sync manual quota adjustment for user %d: %s", userID, errors.Join(cacheErr, err)))
		}
	}
	return &adjustment, nil
}
