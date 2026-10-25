package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setOAuthBindRewardsForTest(t *testing.T, rewards string) {
	t.Helper()
	cfg := config.GlobalConfig.Get("oauth_bind_reward")
	require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{"rewards": rewards}))
	t.Cleanup(func() {
		require.NoError(t, config.UpdateConfigFromMap(cfg, map[string]string{"rewards": "{}"}))
	})
}

func createOAuthBindRewardUser(t *testing.T, username string, quota int) User {
	t.Helper()
	user := User{Username: username, Role: common.RoleCommonUser, Status: common.UserStatusEnabled, Group: "default", AuthVersion: 1, AffCode: username, Quota: quota}
	require.NoError(t, DB.Create(&user).Error)
	return user
}

func claimOAuthBindRewardForTest(t *testing.T, provider, subject string, userId int) int {
	t.Helper()
	granted := 0
	require.NoError(t, DB.Transaction(func(tx *gorm.DB) error {
		var err error
		granted, err = ClaimOAuthBindRewardWithTx(tx, provider, subject, userId)
		return err
	}))
	return granted
}

func userQuotaForTest(t *testing.T, userId int) int {
	t.Helper()
	var user User
	require.NoError(t, DB.Select("quota").First(&user, userId).Error)
	return user.Quota
}

func TestOAuthBindRewardPaysOncePerAccountAndPerUser(t *testing.T) {
	truncateTables(t)
	setOAuthBindRewardsForTest(t, `{"linuxdo":500,"custom:7":300}`)
	alice := createOAuthBindRewardUser(t, "bind-reward-alice", 100)
	bob := createOAuthBindRewardUser(t, "bind-reward-bob", 0)

	assert.Equal(t, 500, claimOAuthBindRewardForTest(t, "linuxdo", "ld-1", alice.Id))
	FinishOAuthBindReward(alice.Id, 500, "Linux DO")
	assert.Equal(t, 600, userQuotaForTest(t, alice.Id))
	var logs int64
	require.NoError(t, LOG_DB.Model(&Log{}).Where("user_id = ? AND type = ?", alice.Id, LogTypeSystem).Count(&logs).Error)
	assert.EqualValues(t, 1, logs)

	cases := []struct {
		name     string
		provider string
		subject  string
		userId   int
		want     int
	}{
		{name: "same account rebound by the same user", provider: "linuxdo", subject: "ld-1", userId: alice.Id},
		{name: "same account moved to another user", provider: "linuxdo", subject: "ld-1", userId: bob.Id},
		{name: "another account on the same user", provider: "linuxdo", subject: "ld-2", userId: alice.Id},
		{name: "another provider pays independently", provider: "custom:7", subject: "ld-1", userId: alice.Id, want: 300},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, claimOAuthBindRewardForTest(t, tc.provider, tc.subject, tc.userId))
		})
	}
	assert.Equal(t, 900, userQuotaForTest(t, alice.Id))
	assert.Equal(t, 0, userQuotaForTest(t, bob.Id))
}

func TestOAuthBindRewardRecordsUnpaidClaims(t *testing.T) {
	truncateTables(t)
	setOAuthBindRewardsForTest(t, `{"github":500}`)
	user := createOAuthBindRewardUser(t, "bind-reward-unpaid", 0)
	full := createOAuthBindRewardUser(t, "bind-reward-full", common.MaxWalletQuota-10)

	// A provider without a reward still records the first link, so enabling
	// a reward later cannot pay an account linked before.
	assert.Equal(t, 0, claimOAuthBindRewardForTest(t, "discord", "dc-1", user.Id))
	setOAuthBindRewardsForTest(t, `{"github":500,"discord":500}`)
	assert.Equal(t, 0, claimOAuthBindRewardForTest(t, "discord", "dc-1", user.Id))

	// A wallet at its limit keeps the claim but is paid nothing.
	assert.Equal(t, 0, claimOAuthBindRewardForTest(t, "github", "gh-1", full.Id))
	assert.Equal(t, common.MaxWalletQuota-10, userQuotaForTest(t, full.Id))
	var claim OAuthBindReward
	require.NoError(t, DB.Where("provider = ? AND subject = ?", "github", "gh-1").First(&claim).Error)
	assert.Equal(t, 0, claim.Quota)
	assert.Equal(t, 0, claimOAuthBindRewardForTest(t, "github", "gh-1", user.Id))
}

func TestOAuthBindRewardBackfillMarksExistingBindings(t *testing.T) {
	truncateTables(t)
	setOAuthBindRewardsForTest(t, `{"linuxdo":500,"wechat":500,"custom:3":500}`)
	linked := createOAuthBindRewardUser(t, "bind-reward-linked", 0)
	require.NoError(t, DB.Model(&linked).Updates(map[string]any{"linux_do_id": "ld-9", "wechat_id": "wx-9"}).Error)
	require.NoError(t, DB.Create(&UserOAuthBinding{UserId: linked.Id, ProviderId: 3, ProviderUserId: "c-9"}).Error)
	deleted := createOAuthBindRewardUser(t, "bind-reward-deleted", 0)
	require.NoError(t, DB.Model(&deleted).Update("linux_do_id", "ld-10").Error)
	require.NoError(t, DB.Delete(&deleted).Error)

	require.NoError(t, backfillOAuthBindRewards(DB))
	require.NoError(t, backfillOAuthBindRewards(DB))

	claimed, err := GetClaimedOAuthBindRewards(linked.Id)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{"linuxdo", "wechat", "custom:3"}, claimed)
	fresh := createOAuthBindRewardUser(t, "bind-reward-fresh", 0)
	assert.Equal(t, 0, claimOAuthBindRewardForTest(t, "linuxdo", "ld-10", fresh.Id))
	assert.Equal(t, 0, claimOAuthBindRewardForTest(t, "custom:3", "c-9", fresh.Id))
	assert.Equal(t, 500, claimOAuthBindRewardForTest(t, "linuxdo", "ld-11", fresh.Id))
}

func TestOAuthBindRewardOptionValidation(t *testing.T) {
	cases := []struct {
		name  string
		value string
		valid bool
	}{
		{name: "built-in and custom providers", value: `{"linuxdo":500,"wechat":0,"custom:12":1}`, valid: true},
		{name: "empty map", value: `{}`, valid: true},
		{name: "unknown provider", value: `{"gitlab":500}`},
		{name: "custom provider without id", value: `{"custom:":500}`},
		{name: "custom provider with padded id", value: `{"custom:012":500}`},
		{name: "negative reward", value: `{"linuxdo":-1}`},
		{name: "reward above the wallet limit", value: `{"linuxdo":9007199254740992}`},
		{name: "null", value: `null`},
		{name: "not a map", value: `[500]`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOptionValue(operation_setting.OAuthBindRewardsOptionKey, tc.value)
			if tc.valid {
				assert.NoError(t, err)
				return
			}
			assert.Error(t, err)
		})
	}
}
