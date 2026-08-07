package service

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestBuildRankedUsersUsesMetricSpecificMostUsedGroup(t *testing.T) {
	totals := []model.RankingUserTotal{{
		UserID:      7,
		Username:    "alice-current",
		DisplayName: "Alice",
		TotalQuota:  100,
		TotalTokens: 105,
	}}
	groups := map[int][]model.RankingUserGroupTotal{
		7: {
			{UserID: 7, UseGroup: "vip", TotalQuota: 90, TotalTokens: 5},
			{UserID: 7, UseGroup: "default", TotalQuota: 10, TotalTokens: 100},
		},
	}
	quotaRows := buildRankedUsers(totals, groups, rankingUserMetricQuota)
	tokenRows := buildRankedUsers(totals, groups, rankingUserMetricTokens)
	require.Len(t, quotaRows, 1)
	require.Len(t, tokenRows, 1)
	require.Equal(t, "alice-current", quotaRows[0].Username)
	require.Equal(t, "Alice", quotaRows[0].DisplayName)
	require.Equal(t, "vip", quotaRows[0].TopGroup)
	require.Equal(t, "default", tokenRows[0].TopGroup)
}

func TestRankingUserTopGroupBreaksTiesDeterministically(t *testing.T) {
	groups := []model.RankingUserGroupTotal{
		{UseGroup: "vip", TotalQuota: 20, TotalTokens: 20},
		{UseGroup: "default", TotalQuota: 20, TotalTokens: 20},
	}
	require.Equal(t, "default", rankingUserTopGroup(groups, rankingUserMetricQuota))
	require.Equal(t, "default", rankingUserTopGroup(groups, rankingUserMetricTokens))
}
