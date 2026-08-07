package model

import (
	"fmt"
	"sync"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

type RankingQuotaTotal struct {
	ModelName   string `json:"model_name"`
	TotalTokens int64  `json:"total_tokens"`
}

type RankingQuotaBucket struct {
	ModelName string `json:"model_name"`
	Bucket    int64  `json:"bucket"`
	Tokens    int64  `json:"tokens"`
}

// RankingUserTotal is the aggregate usage for one user in a ranking period.
// Both measures are selected in the same query so the service can render the
// quota and token leaderboards without issuing one query per user.
type RankingUserTotal struct {
	UserID      int    `gorm:"column:user_id"`
	Username    string `gorm:"column:username"`
	DisplayName string `gorm:"column:display_name"`
	TotalQuota  int64  `gorm:"column:total_quota"`
	TotalTokens int64  `gorm:"column:total_tokens"`
}

// RankingUserGroupTotal contains one user's usage for a single configured
// group. The service uses these rows to determine the most-used group for
// each metric.
type RankingUserGroupTotal struct {
	UserID      int    `gorm:"column:user_id"`
	UseGroup    string `gorm:"column:use_group"`
	TotalQuota  int64  `gorm:"column:total_quota"`
	TotalTokens int64  `gorm:"column:total_tokens"`
}

// RankingUserUsage contains the bounded result set needed to build both user
// leaderboards. The first two queries are limited in SQL; the group query is
// restricted to the union of those user IDs, keeping response latency stable
// as the number of users grows.
type RankingUserUsage struct {
	ByQuota  []RankingUserTotal
	ByTokens []RankingUserTotal
	Groups   []RankingUserGroupTotal
}

// GetRankingUserUsage aggregates quota_data for the requested period. The
// table is already written as hourly rollups, so this avoids scanning the raw
// consume log table while retaining the same accounting dimensions as the
// existing model rankings.
func GetRankingUserUsage(startTime int64, endTime int64, limit int) (RankingUserUsage, error) {
	if limit <= 0 {
		limit = 10
	}

	var byQuota []RankingUserTotal
	var byTokens []RankingUserTotal
	var quotaErr error
	var tokensErr error
	var totalsWaitGroup sync.WaitGroup
	totalsWaitGroup.Add(2)
	go func() {
		defer totalsWaitGroup.Done()
		byQuota, quotaErr = getRankingUserTotals(startTime, endTime, "quota", limit)
	}()
	go func() {
		defer totalsWaitGroup.Done()
		byTokens, tokensErr = getRankingUserTotals(startTime, endTime, "tokens", limit)
	}()
	totalsWaitGroup.Wait()
	if quotaErr != nil {
		return RankingUserUsage{}, quotaErr
	}
	if tokensErr != nil {
		return RankingUserUsage{}, tokensErr
	}

	userIDs := make([]int, 0, len(byQuota)+len(byTokens))
	seenUserIDs := make(map[int]struct{}, len(byQuota)+len(byTokens))
	for _, totals := range [][]RankingUserTotal{byQuota, byTokens} {
		for _, row := range totals {
			if row.UserID <= 0 {
				continue
			}
			if _, ok := seenUserIDs[row.UserID]; ok {
				continue
			}
			seenUserIDs[row.UserID] = struct{}{}
			userIDs = append(userIDs, row.UserID)
		}
	}

	groups, err := getRankingUserGroupTotals(startTime, endTime, userIDs)
	if err != nil {
		return RankingUserUsage{}, err
	}

	return RankingUserUsage{
		ByQuota:  byQuota,
		ByTokens: byTokens,
		Groups:   groups,
	}, nil
}

func getRankingUserTotals(startTime int64, endTime int64, metric string, limit int) ([]RankingUserTotal, error) {
	var orderColumn string
	var havingColumn string
	switch metric {
	case "quota":
		orderColumn = "total_quota"
		havingColumn = "quota_data.quota"
	case "tokens":
		orderColumn = "total_tokens"
		havingColumn = "quota_data.token_used"
	default:
		return nil, fmt.Errorf("invalid ranking user metric: %s", metric)
	}

	rows := make([]RankingUserTotal, 0, limit)
	query := DB.Table("quota_data").
		Joins("JOIN users ON users.id = quota_data.user_id AND users.deleted_at IS NULL AND users.status = ?", common.UserStatusEnabled).
		Select("quota_data.user_id, max(users.username) as username, max(users.display_name) as display_name, sum(quota_data.quota) as total_quota, sum(quota_data.token_used) as total_tokens").
		Where("quota_data.user_id > 0").
		Group("quota_data.user_id").
		Having(fmt.Sprintf("sum(%s) > 0", havingColumn)).
		Order(orderColumn + " DESC, user_id ASC").
		Limit(limit)
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func getRankingUserGroupTotals(startTime int64, endTime int64, userIDs []int) ([]RankingUserGroupTotal, error) {
	if len(userIDs) == 0 {
		return []RankingUserGroupTotal{}, nil
	}

	rows := make([]RankingUserGroupTotal, 0)
	query := DB.Table("quota_data").
		Select("user_id, use_group, sum(quota) as total_quota, sum(token_used) as total_tokens").
		Where("user_id IN ?", userIDs).
		Where("use_group <> ''").
		Group("user_id, use_group")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

func GetRankingQuotaTotals(startTime int64, endTime int64) ([]RankingQuotaTotal, error) {
	var rows []RankingQuotaTotal
	query := DB.Table("quota_data").
		Select("model_name, sum(token_used) as total_tokens").
		Where("model_name <> ''").
		Group("model_name").
		Having("sum(token_used) > 0").
		Order("total_tokens DESC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

func GetRankingQuotaBuckets(startTime int64, endTime int64, bucketSize int64) ([]RankingQuotaBucket, error) {
	if bucketSize <= 0 {
		bucketSize = 3600
	}
	bucketExpr := rankingBucketExpr(bucketSize)
	var rows []RankingQuotaBucket
	query := DB.Table("quota_data").
		Select(fmt.Sprintf("model_name, %s as bucket, sum(token_used) as tokens", bucketExpr)).
		Where("model_name <> ''").
		Group(fmt.Sprintf("model_name, %s", bucketExpr)).
		Having("sum(token_used) > 0").
		Order("bucket ASC")
	query = applyRankingQuotaTimeRange(query, startTime, endTime)
	err := query.Find(&rows).Error
	return rows, err
}

func rankingBucketExpr(bucketSize int64) string {
	if common.UsingMainDatabase(common.DatabaseTypeMySQL) {
		return fmt.Sprintf("FLOOR(created_at / %d) * %d", bucketSize, bucketSize)
	}
	return fmt.Sprintf("(created_at / %d) * %d", bucketSize, bucketSize)
}

func applyRankingQuotaTimeRange(query *gorm.DB, startTime int64, endTime int64) *gorm.DB {
	if startTime > 0 {
		query = query.Where("quota_data.created_at >= ?", startTime)
	}
	if endTime > 0 {
		query = query.Where("quota_data.created_at <= ?", endTime)
	}
	return query
}
