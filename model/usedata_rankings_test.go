package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetRankingUserUsageAggregatesBothMetricsAndGroups(t *testing.T) {
	truncateTables(t)

	rows := []QuotaData{
		{UserID: 1, Username: "alice", CreatedAt: 1000, UseGroup: "vip", Quota: 90, TokenUsed: 5},
		{UserID: 1, Username: "alice", CreatedAt: 1000, UseGroup: "default", Quota: 10, TokenUsed: 100},
		{UserID: 2, Username: "bob", CreatedAt: 1000, UseGroup: "default", Quota: 70, TokenUsed: 70},
		{UserID: 2, Username: "bob", CreatedAt: 1000, UseGroup: "vip", Quota: 30, TokenUsed: 10},
		// This row is outside the requested range and must not affect either
		// total or the most-used group.
		{UserID: 1, Username: "alice", CreatedAt: 3000, UseGroup: "vip", Quota: 1000, TokenUsed: 1000},
		// Empty groups are retained in user totals but are not candidates for
		// the most-used group label.
		{UserID: 3, Username: "carol", CreatedAt: 1000, UseGroup: "", Quota: 40, TokenUsed: 40},
		{UserID: 0, Username: "invalid", CreatedAt: 1000, UseGroup: "vip", Quota: 999, TokenUsed: 999},
	}
	require.NoError(t, DB.Create(&rows).Error)
	require.NoError(t, DB.Create(&[]User{
		{Id: 1, Username: "alice-current", DisplayName: "Alice", Status: common.UserStatusEnabled, AffCode: "rank-aff-alice"},
		{Id: 2, Username: "bob", Status: common.UserStatusEnabled, AffCode: "rank-aff-bob"},
		{Id: 3, Username: "carol", Status: common.UserStatusEnabled, AffCode: "rank-aff-carol"},
	}).Error)

	usage, err := GetRankingUserUsage(1000, 2000, 10)
	require.NoError(t, err)
	require.Len(t, usage.ByQuota, 3)
	require.Len(t, usage.ByTokens, 3)

	require.Equal(t, RankingUserTotal{UserID: 1, Username: "alice-current", DisplayName: "Alice", TotalQuota: 100, TotalTokens: 105}, usage.ByQuota[0])
	require.Equal(t, RankingUserTotal{UserID: 1, Username: "alice-current", DisplayName: "Alice", TotalQuota: 100, TotalTokens: 105}, usage.ByTokens[0])

	groupsByUser := make(map[int]map[string]RankingUserGroupTotal)
	for _, group := range usage.Groups {
		if groupsByUser[group.UserID] == nil {
			groupsByUser[group.UserID] = make(map[string]RankingUserGroupTotal)
		}
		groupsByUser[group.UserID][group.UseGroup] = group
	}
	require.Equal(t, int64(90), groupsByUser[1]["vip"].TotalQuota)
	require.Equal(t, int64(100), groupsByUser[1]["default"].TotalTokens)
	_, hasEmptyGroup := groupsByUser[3][""]
	require.False(t, hasEmptyGroup)
}

func TestGetRankingUserUsageLimitsEachMetricInSQL(t *testing.T) {
	truncateTables(t)
	for userID := 1; userID <= 12; userID++ {
		require.NoError(t, DB.Create(&User{
			Id:       userID,
			Username: "user-" + string(rune('a'+userID-1)),
			Status:   common.UserStatusEnabled,
			AffCode:  "rank-aff-" + string(rune('a'+userID-1)),
		}).Error)
		require.NoError(t, DB.Create(&QuotaData{
			UserID:    userID,
			Username:  "user",
			CreatedAt: 1000,
			UseGroup:  "default",
			Quota:     userID,
			TokenUsed: 13 - userID,
		}).Error)
	}

	usage, err := GetRankingUserUsage(1000, 1000, 10)
	require.NoError(t, err)
	require.Len(t, usage.ByQuota, 10)
	require.Len(t, usage.ByTokens, 10)
	require.Equal(t, 12, usage.ByQuota[0].UserID)
	require.Equal(t, 1, usage.ByTokens[0].UserID)
}

func TestGetRankingUserUsageExcludesDeletedAndDisabledUsers(t *testing.T) {
	truncateTables(t)
	active := User{Id: 1, Username: "active", Status: common.UserStatusEnabled, AffCode: "rank-aff-active"}
	deleted := User{Id: 2, Username: "deleted", Status: common.UserStatusEnabled, AffCode: "rank-aff-deleted"}
	disabled := User{Id: 3, Username: "disabled", Status: common.UserStatusDisabled, AffCode: "rank-aff-disabled"}
	require.NoError(t, DB.Create(&[]User{active, deleted, disabled}).Error)
	require.NoError(t, DB.Delete(&deleted).Error)
	require.NoError(t, DB.Create(&[]QuotaData{
		{UserID: 1, Username: "active", CreatedAt: 1000, UseGroup: "default", Quota: 10, TokenUsed: 10},
		{UserID: 2, Username: "deleted", CreatedAt: 1000, UseGroup: "default", Quota: 100, TokenUsed: 100},
		{UserID: 3, Username: "disabled", CreatedAt: 1000, UseGroup: "default", Quota: 200, TokenUsed: 200},
	}).Error)

	usage, err := GetRankingUserUsage(1000, 1000, 10)
	require.NoError(t, err)
	require.Len(t, usage.ByQuota, 1)
	require.Len(t, usage.ByTokens, 1)
	require.Equal(t, 1, usage.ByQuota[0].UserID)
	require.Equal(t, 1, usage.ByTokens[0].UserID)
}
