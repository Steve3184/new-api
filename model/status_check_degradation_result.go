package model

import (
	"strings"
)

const (
	StatusCheckDegradationPassed   = "passed"
	StatusCheckDegradationDegraded = "degraded"
	StatusCheckDegradationError    = "error"
)

// StatusCheckDegradationResult stores one model degradation check. Only the
// short parsed answer is kept, never the prompt or the full upstream response.
type StatusCheckDegradationResult struct {
	ID        int64  `json:"id" gorm:"primaryKey"`
	Group     string `json:"group" gorm:"column:group;size:64;index:idx_scdr_lookup,priority:1"`
	Model     string `json:"model" gorm:"size:255;index:idx_scdr_lookup,priority:2"`
	Effort    string `json:"effort" gorm:"size:16;index:idx_scdr_lookup,priority:3"`
	CheckedAt int64  `json:"checked_at" gorm:"bigint;index:idx_scdr_lookup,priority:4;index"`
	Status    string `json:"status" gorm:"size:16"`
	Answer    string `json:"answer" gorm:"size:64"`
	LatencyMs int64  `json:"latency_ms" gorm:"bigint;default:0"`
}

func (StatusCheckDegradationResult) TableName() string {
	return "status_check_degradation_results"
}

// StatusCheckDegradationKey identifies one configured group/model/effort row.
type StatusCheckDegradationKey struct {
	Group  string
	Model  string
	Effort string
}

func CreateStatusCheckDegradationResult(result *StatusCheckDegradationResult) error {
	result.Group = strings.TrimSpace(result.Group)
	if len(result.Answer) > 64 {
		result.Answer = result.Answer[:64]
	}
	return DB.Create(result).Error
}

// GetStatusCheckDegradationLastCheckedAt returns the newest check time for
// each group/model/effort row of the given groups.
func GetStatusCheckDegradationLastCheckedAt(groups []string) (map[StatusCheckDegradationKey]int64, error) {
	lastCheckedAt := make(map[StatusCheckDegradationKey]int64)
	if len(groups) == 0 {
		return lastCheckedAt, nil
	}
	var rows []struct {
		Group     string
		Model     string
		Effort    string
		CheckedAt int64
	}
	err := DB.Model(&StatusCheckDegradationResult{}).
		Select(commonGroupCol+" AS "+commonGroupCol+", model, effort, MAX(checked_at) AS checked_at").
		Where(commonGroupCol+" IN ?", groups).
		Group(commonGroupCol + ", model, effort").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		lastCheckedAt[StatusCheckDegradationKey{Group: row.Group, Model: row.Model, Effort: row.Effort}] = row.CheckedAt
	}
	return lastCheckedAt, nil
}

// GetStatusCheckDegradationResultsSince returns checks of the given groups
// newer than since, oldest first.
func GetStatusCheckDegradationResultsSince(groups []string, since int64) ([]StatusCheckDegradationResult, error) {
	var results []StatusCheckDegradationResult
	if len(groups) == 0 {
		return results, nil
	}
	err := DB.Where(commonGroupCol+" IN ? AND checked_at >= ?", groups, since).
		Order("checked_at asc").
		Order("id asc").
		Find(&results).Error
	return results, err
}

func DeleteStatusCheckDegradationResultsBefore(before int64) error {
	return DB.Where("checked_at < ?", before).Delete(&StatusCheckDegradationResult{}).Error
}
