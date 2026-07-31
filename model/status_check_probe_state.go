package model

import (
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// StatusCheckProbeState stores the idle and backoff state for one explicitly
// configured status-check group. It deliberately stores timestamps rather than
// request details, so automatic probes cannot expose request content.
type StatusCheckProbeState struct {
	Group                 string `json:"group" gorm:"column:group;size:64;primaryKey"`
	LastPassiveAt         int64  `json:"last_passive_at" gorm:"bigint;default:0"`
	LastProbeAt           int64  `json:"last_probe_at" gorm:"bigint;default:0"`
	ConsecutiveProbeCount int    `json:"consecutive_probe_count" gorm:"default:0"`
}

func (StatusCheckProbeState) TableName() string {
	return "status_check_probe_states"
}

func GetStatusCheckProbeStates(groups []string) (map[string]StatusCheckProbeState, error) {
	states := make(map[string]StatusCheckProbeState, len(groups))
	if len(groups) == 0 {
		return states, nil
	}

	var rows []StatusCheckProbeState
	if err := DB.Where(commonGroupCol+" IN ?", groups).Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		states[row.Group] = row
	}
	return states, nil
}

// InitializeStatusCheckProbeState establishes the idle window when flexible
// probing is first enabled for a group. The first probe therefore waits for
// the configured idle interval instead of charging immediately after enable.
func InitializeStatusCheckProbeState(group string, now int64) error {
	group = strings.TrimSpace(group)
	if group == "" {
		return nil
	}
	return DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&StatusCheckProbeState{
		Group:       group,
		LastProbeAt: now,
	}).Error
}

// TouchStatusCheckProbePassiveActivity advances the last real-request time.
// The conditional update preserves newer activity written by another node.
func TouchStatusCheckProbePassiveActivity(group string, timestamp int64) error {
	group = strings.TrimSpace(group)
	if group == "" || timestamp <= 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_passive_at": gorm.Expr("CASE WHEN last_passive_at < ? THEN ? ELSE last_passive_at END", timestamp, timestamp),
		}),
	}).Create(&StatusCheckProbeState{
		Group:         group,
		LastPassiveAt: timestamp,
	}).Error
}

// RecordStatusCheckProbeResult increments the automatic-probe backoff count.
// A real request after the previous probe resets the count to this probe.
func RecordStatusCheckProbeResult(group string, timestamp int64) error {
	group = strings.TrimSpace(group)
	if group == "" || timestamp <= 0 {
		return nil
	}
	return DB.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "group"}},
		DoUpdates: clause.Assignments(map[string]any{
			"last_probe_at": timestamp,
			"consecutive_probe_count": gorm.Expr(
				"CASE WHEN last_passive_at > last_probe_at THEN 1 ELSE consecutive_probe_count + 1 END",
			),
		}),
	}).Create(&StatusCheckProbeState{
		Group:                 group,
		LastProbeAt:           timestamp,
		ConsecutiveProbeCount: 1,
	}).Error
}
