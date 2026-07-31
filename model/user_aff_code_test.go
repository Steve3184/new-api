package model

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAllocateUniqueAffCodeSkipsSoftDeletedCodeBeforeInsert(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}))
	suffix := time.Now().UnixNano()
	takenCode := fmt.Sprintf("taken-%d", suffix)
	freshCode := fmt.Sprintf("fresh-%d", suffix)
	deletedUser := &User{
		Username: fmt.Sprintf("aff-del-%d", suffix%1000000000),
		Password: "password",
		Status:   common.UserStatusEnabled,
		AffCode:  takenCode,
	}
	require.NoError(t, DB.Create(deletedUser).Error)
	require.NoError(t, DB.Delete(deletedUser).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Unscoped().Where("id IN ?", []int{deletedUser.Id}).Delete(&User{}).Error)
	})

	user := &User{
		Username: fmt.Sprintf("aff-new-%d", suffix%1000000000),
		Password: "password",
		Status:   common.UserStatusEnabled,
	}
	t.Cleanup(func() {
		if user.Id != 0 {
			require.NoError(t, DB.Unscoped().Where("id = ?", user.Id).Delete(&User{}).Error)
		}
	})

	candidates := []string{takenCode, freshCode}
	candidateIndex := 0
	persistCalls := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		return allocateUniqueAffCode(tx, user, func() string {
			candidate := candidates[candidateIndex]
			candidateIndex++
			return candidate
		}, func(tx *gorm.DB) error {
			persistCalls++
			return tx.Create(user).Error
		})
	})

	require.NoError(t, err)
	assert.Equal(t, freshCode, user.AffCode)
	assert.Equal(t, 2, candidateIndex)
	assert.Equal(t, 1, persistCalls)
}

func TestEnsureUserAffCodeBackfillsLegacyUser(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}))
	suffix := time.Now().UnixNano()
	user := &User{
		Username: fmt.Sprintf("aff-old-%d", suffix%1000000000),
		Password: "password",
		Status:   common.UserStatusEnabled,
	}
	require.NoError(t, DB.Create(user).Error)
	t.Cleanup(func() {
		require.NoError(t, DB.Unscoped().Where("id = ?", user.Id).Delete(&User{}).Error)
	})

	affCode, err := EnsureUserAffCode(user.Id)
	require.NoError(t, err)
	assert.Len(t, affCode, affCodeLength)

	var persisted User
	require.NoError(t, DB.Select("aff_code").First(&persisted, user.Id).Error)
	assert.Equal(t, affCode, persisted.AffCode)
}

func TestAllocateUniqueAffCodeRetriesAfterUniqueConstraintConflict(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&User{}))
	suffix := time.Now().UnixNano()
	user := &User{
		Username: fmt.Sprintf("aff-rty-%d", suffix%1000000000),
		Password: "password",
		Status:   common.UserStatusEnabled,
	}
	t.Cleanup(func() {
		if user.Id != 0 {
			require.NoError(t, DB.Unscoped().Where("id = ?", user.Id).Delete(&User{}).Error)
		}
	})

	firstCode := fmt.Sprintf("first-%d", suffix)
	secondCode := fmt.Sprintf("second-%d", suffix)
	candidates := []string{firstCode, secondCode}
	candidateIndex := 0
	persistCalls := 0
	err := DB.Transaction(func(tx *gorm.DB) error {
		return allocateUniqueAffCode(tx, user, func() string {
			candidate := candidates[candidateIndex]
			candidateIndex++
			return candidate
		}, func(tx *gorm.DB) error {
			persistCalls++
			if persistCalls == 1 {
				return errors.New("UNIQUE constraint failed: users.aff_code")
			}
			return tx.Create(user).Error
		})
	})

	require.NoError(t, err)
	assert.Equal(t, secondCode, user.AffCode)
	assert.Equal(t, 2, persistCalls)
}
