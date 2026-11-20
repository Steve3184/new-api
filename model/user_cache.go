package model

import (
	"errors"
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/relaykit/dto"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const userCacheSchemaVersion = 2

type UserBase struct {
	Id             int    `json:"id"`
	Group          string `json:"group"`
	Email          string `json:"email"`
	Quota          int    `json:"quota"`
	Status         int    `json:"status"`
	Role           int    `json:"role"`
	Username       string `json:"username"`
	Setting        string `json:"setting"`
	AuthVersion    int64  `json:"-"`
	CacheSchema    int    `json:"-"`
	CacheExpiresAt int64  `json:"-"`
	CacheMAC       string `json:"-"`
}

func (user UserBase) cacheMAC() (string, error) {
	user.Quota = 0
	return authCacheMAC(getUserCacheKey(user.Id), struct {
		User           UserBase
		AuthVersion    int64
		CacheSchema    int
		CacheExpiresAt int64
	}{user, user.AuthVersion, user.CacheSchema, user.CacheExpiresAt})
}

func (user *UserBase) WriteContext(c *gin.Context) {
	common.SetContextKey(c, constant.ContextKeyUserGroup, user.Group)
	common.SetContextKey(c, constant.ContextKeyUserQuota, user.Quota)
	common.SetContextKey(c, constant.ContextKeyUserStatus, user.Status)
	common.SetContextKey(c, constant.ContextKeyUserEmail, user.Email)
	common.SetContextKey(c, constant.ContextKeyUserName, user.Username)
	common.SetContextKey(c, constant.ContextKeyUserSetting, user.GetSetting())
}

func (user *UserBase) GetSetting() dto.UserSetting {
	setting := dto.UserSetting{}
	if user.Setting != "" {
		err := common.Unmarshal([]byte(user.Setting), &setting)
		if err != nil {
			common.SysLog(common.LogText("failed to unmarshal setting: %s", err.Error()))
		}
	}
	return setting
}

// getUserCacheKey returns the key for user cache
func getUserCacheKey(userId int) string {
	return fmt.Sprintf("user:%d", userId)
}

func userCacheTTLSeconds() int {
	ttl := common.RedisKeyCacheSeconds()
	if ttl <= 0 {
		return 60
	}
	return ttl
}

// invalidateUserCache clears user cache
func invalidateUserCache(userId int) error {
	if !common.RedisEnabled {
		return nil
	}
	return common.RedisDelKey(getUserCacheKey(userId))
}

func populateUserCache(user User) error {
	if !common.RedisEnabled {
		return nil
	}
	return writeUserCache(user.ToBaseUser(), true)
}

// updateUserCache refreshes non-quota user cache fields.
// Quota is maintained by atomic quota delta paths and must not be overwritten
// by stale user snapshots from profile/settings updates.
func updateUserCache(user User) error {
	if !common.RedisEnabled {
		return nil
	}
	return writeUserCache(user.ToBaseUser(), false)
}

// GetUserCache gets complete user cache from hash
func GetUserCache(userId int) (*UserBase, error) {
	// Try getting from Redis first
	userCache, err := cacheGetUserBase(userId)
	if err == nil {
		return userCache, nil
	}

	// Redis misses and read failures both fall back to the shared database. A
	// version fence newer than the database is the one exception: allowing that
	// snapshot would re-authorize a user while a restrictive update is pending.
	var user *User
	if common.RedisEnabled {
		err = DB.Transaction(func(tx *gorm.DB) error {
			var locked User
			if err := lockForUpdate(tx).First(&locked, userId).Error; err != nil {
				return err
			}
			floor, floorErr := getUserAuthVersionFloor(userId)
			if floorErr == nil && floor > locked.AuthVersion {
				return ErrUserAuthCachePending
			}
			if err := populateUserCache(locked); err != nil {
				if errors.Is(err, ErrUserAuthCachePending) {
					return err
				}
				common.SysError(common.LogText("failed to synchronously populate user cache: %s", err))
			}
			user = &locked
			return nil
		})
	} else {
		user, err = GetUserById(userId, false)
	}
	if err != nil {
		return nil, err
	}
	return user.ToBaseUser(), nil
}

func cacheGetUserBase(userId int) (*UserBase, error) {
	if !common.RedisEnabled {
		return nil, fmt.Errorf("redis is not enabled")
	}
	var userCache UserBase
	// Try getting from Redis first
	err := common.RedisHGetObj(getUserCacheKey(userId), &userCache)
	if err != nil {
		return nil, err
	}
	if userCache.Id != userId || userCache.CacheSchema != userCacheSchemaVersion || userCache.AuthVersion <= 0 {
		return nil, fmt.Errorf("user cache schema is stale")
	}
	mac, err := userCache.cacheMAC()
	if err != nil || !validAuthCacheMAC(mac, userCache.CacheMAC, userCache.CacheExpiresAt) {
		return nil, fmt.Errorf("user cache integrity check failed")
	}
	floor, err := getUserAuthVersionFloor(userId)
	if err != nil {
		return nil, err
	}
	if floor > userCache.AuthVersion {
		return nil, ErrUserAuthCachePending
	}
	return &userCache, nil
}

// A delayed notification may arrive after a refill that already includes the
// SQL change. Invalidate under the same row lock as hydration and wallet holds;
// neither replay the change nor delete a hold while its purchase is in flight.
func invalidateCommittedWalletCache(userId int) error {
	if !common.RedisEnabled {
		return nil
	}
	return DB.Transaction(func(tx *gorm.DB) error {
		var user User
		if err := lockForUpdate(tx.Unscoped()).Select("id").Where("id = ?", userId).Find(&user).Error; err != nil {
			return err
		}
		return invalidateUserCache(userId)
	})
}

// Legacy post-commit notification signatures. Reservation and compensation
// still use the Redis operations in quota_reserve.go directly.
func cacheIncrUserQuota(userId int, _ int64) error {
	return invalidateCommittedWalletCache(userId)
}

func cacheDecrUserQuota(userId int, _ int64) error {
	return invalidateCommittedWalletCache(userId)
}

// A subsequent read hydrates the full user hash from the committed wallet.
func syncCreditUserQuotaCache(userId int, quota int, operation string) {
	if quota <= 0 {
		return
	}
	if err := invalidateCommittedWalletCache(userId); err != nil {
		common.SysLog(common.LogText("failed to sync %s credit to user quota cache: %s", operation, err.Error()))
	}
}

// Helper functions to get individual fields if needed
func getUserGroupCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Group, nil
}

func getUserQuotaCache(userId int) (int, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return 0, err
	}
	return cache.Quota, nil
}

func getUserNameCache(userId int) (string, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return "", err
	}
	return cache.Username, nil
}

func getUserSettingCache(userId int) (dto.UserSetting, error) {
	cache, err := GetUserCache(userId)
	if err != nil {
		return dto.UserSetting{}, err
	}
	return cache.GetSetting(), nil
}

// RefreshUserGroupCache publishes current user metadata after a group change.
func RefreshUserGroupCache(userId int) error {
	return PublishUserAuthCache(userId)
}

// GetUserLanguage returns the user's language preference from cache
// Uses the existing GetUserCache mechanism for efficiency
func GetUserLanguage(userId int) string {
	userCache, err := GetUserCache(userId)
	if err != nil {
		return ""
	}
	return userCache.GetSetting().Language
}
