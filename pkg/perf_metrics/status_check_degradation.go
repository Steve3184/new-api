package perfmetrics

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	// StatusCheckDegradationProbeModel stores degradation check samples so the
	// status page can use them for hours without real requests.
	StatusCheckDegradationProbeModel             = "__status_check_degradation__"
	defaultStatusCheckDegradationIntervalMinutes = 60
	minStatusCheckDegradationIntervalMinutes     = 5
	maxStatusCheckDegradationIntervalMinutes     = 7 * 24 * 60
	maxStatusCheckDegradationModels              = 20
	statusCheckDegradationHistoryLimit           = StatusBeatCount
	statusCheckDegradationHistoryWindow          = 10 * 24 * time.Hour
	StatusCheckDegradationRetention              = 11 * 24 * time.Hour
)

// StatusCheckDegradationEfforts lists the accepted reasoning effort values.
// An empty effort leaves the upstream model default unchanged.
var StatusCheckDegradationEfforts = []string{"", "none", "minimal", "low", "medium", "high", "xhigh", "max"}

type StatusCheckDegradationModel struct {
	Model  string `json:"model"`
	Effort string `json:"effort"`
}

type StatusCheckDegradationGroupConfig struct {
	Enabled         bool                          `json:"enabled"`
	IntervalMinutes int                           `json:"interval_minutes"`
	Models          []StatusCheckDegradationModel `json:"models"`
}

// StatusCheckDegradationConfig keeps per-group model degradation checks.
// Groups absent from this map never receive degradation checks.
type StatusCheckDegradationConfig struct {
	Groups map[string]StatusCheckDegradationGroupConfig `json:"groups"`
}

func ParseStatusCheckDegradationConfig(value string) (StatusCheckDegradationConfig, error) {
	config := StatusCheckDegradationConfig{Groups: map[string]StatusCheckDegradationGroupConfig{}}
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) == "null" {
		return config, nil
	}
	if err := common.UnmarshalJsonStr(value, &config); err != nil {
		return StatusCheckDegradationConfig{}, err
	}
	if len(config.Groups) > 100 {
		return StatusCheckDegradationConfig{}, fmt.Errorf("groups cannot exceed 100 entries")
	}
	normalizedGroups := make(map[string]StatusCheckDegradationGroupConfig, len(config.Groups))
	for group, groupConfig := range config.Groups {
		group = strings.TrimSpace(group)
		if group == "" || len(group) > 64 {
			return StatusCheckDegradationConfig{}, fmt.Errorf("group names must be non-empty and at most 64 characters")
		}
		if group == "auto" {
			return StatusCheckDegradationConfig{}, fmt.Errorf("automatic group selection cannot be checked")
		}
		if groupConfig.IntervalMinutes == 0 {
			groupConfig.IntervalMinutes = defaultStatusCheckDegradationIntervalMinutes
		}
		if groupConfig.IntervalMinutes < minStatusCheckDegradationIntervalMinutes || groupConfig.IntervalMinutes > maxStatusCheckDegradationIntervalMinutes {
			return StatusCheckDegradationConfig{}, fmt.Errorf("interval_minutes for group %q must be between %d and %d", group, minStatusCheckDegradationIntervalMinutes, maxStatusCheckDegradationIntervalMinutes)
		}
		if len(groupConfig.Models) > maxStatusCheckDegradationModels {
			return StatusCheckDegradationConfig{}, fmt.Errorf("models for group %q cannot exceed %d entries", group, maxStatusCheckDegradationModels)
		}
		models := make([]StatusCheckDegradationModel, 0, len(groupConfig.Models))
		for _, entry := range groupConfig.Models {
			entry.Model = strings.TrimSpace(entry.Model)
			entry.Effort = strings.TrimSpace(entry.Effort)
			if entry.Model == "" || len(entry.Model) > 255 {
				return StatusCheckDegradationConfig{}, fmt.Errorf("model names for group %q must be non-empty and at most 255 characters", group)
			}
			if !slices.Contains(StatusCheckDegradationEfforts, entry.Effort) {
				return StatusCheckDegradationConfig{}, fmt.Errorf("effort %q for model %q is not supported", entry.Effort, entry.Model)
			}
			if slices.Contains(models, entry) {
				continue
			}
			models = append(models, entry)
		}
		groupConfig.Models = models
		normalizedGroups[group] = groupConfig
	}
	config.Groups = normalizedGroups
	return config, nil
}

func GetStatusCheckDegradationConfig() StatusCheckDegradationConfig {
	config, err := ParseStatusCheckDegradationConfig(common.StatusCheckDegradationMode)
	if err != nil {
		return StatusCheckDegradationConfig{Groups: map[string]StatusCheckDegradationGroupConfig{}}
	}
	return config
}

// EnabledGroup returns the group's configuration when checks are enabled and
// at least one model is configured.
func (config StatusCheckDegradationConfig) EnabledGroup(group string) (StatusCheckDegradationGroupConfig, bool) {
	groupConfig, ok := config.Groups[strings.TrimSpace(group)]
	return groupConfig, ok && groupConfig.Enabled && len(groupConfig.Models) > 0
}

// isStatusCheckProbeModel reports whether samples of modelName come from
// status-page active checks rather than real model traffic.
func isStatusCheckProbeModel(modelName string) bool {
	return modelName == StatusCheckProbeModel || modelName == StatusCheckDegradationProbeModel
}

// RecordStatusCheckDegradationProbe records one degradation check request.
// A wrong answer is still a successful request; only request failures count
// against availability.
func RecordStatusCheckDegradationProbe(group string, latencyMs int64, ttftMs int64, success bool) {
	Record(Sample{
		Model:     StatusCheckDegradationProbeModel,
		Group:     group,
		LatencyMs: latencyMs,
		TtftMs:    ttftMs,
		HasTtft:   ttftMs > 0,
		Success:   success,
	})
}

// AttachStatusCheckDegradation adds the configured degradation rows and their
// recent checks to each visible status group.
func AttachStatusCheckDegradation(result *StatusResult, hours int) error {
	config := GetStatusCheckDegradationConfig()
	groupNames := make([]string, 0, len(result.Groups))
	for _, group := range result.Groups {
		if _, enabled := config.EnabledGroup(group.Group); enabled {
			groupNames = append(groupNames, group.Group)
		}
	}
	if len(groupNames) == 0 {
		return nil
	}
	if hours <= 0 {
		hours = 24
	}
	hours = min(hours, 24*30)
	// Checks reach back further than the summary window so wide rows fill up;
	// the pass rate covers only the requested summary window.
	summarySince := time.Now().Add(-time.Duration(hours) * time.Hour).Unix()
	since := min(summarySince, time.Now().Add(-statusCheckDegradationHistoryWindow).Unix())
	rows, err := model.GetStatusCheckDegradationResultsSince(groupNames, since)
	if err != nil {
		return err
	}
	checksByKey := make(map[model.StatusCheckDegradationKey][]StatusDegradationCheck)
	for _, row := range rows {
		key := model.StatusCheckDegradationKey{Group: row.Group, Model: row.Model, Effort: row.Effort}
		checksByKey[key] = append(checksByKey[key], StatusDegradationCheck{
			Ts:        row.CheckedAt,
			Status:    row.Status,
			Answer:    row.Answer,
			LatencyMs: row.LatencyMs,
		})
	}
	for i := range result.Groups {
		groupConfig, enabled := config.EnabledGroup(result.Groups[i].Group)
		if !enabled {
			continue
		}
		items := make([]StatusDegradationModel, 0, len(groupConfig.Models))
		for _, entry := range groupConfig.Models {
			checks := checksByKey[model.StatusCheckDegradationKey{Group: result.Groups[i].Group, Model: entry.Model, Effort: entry.Effort}]
			item := StatusDegradationModel{
				Model:  entry.Model,
				Effort: entry.Effort,
				Checks: checks[max(0, len(checks)-statusCheckDegradationHistoryLimit):],
			}
			judged := 0
			passed := 0
			for _, check := range checks {
				if check.Ts < summarySince {
					continue
				}
				switch check.Status {
				case model.StatusCheckDegradationPassed:
					judged++
					passed++
				case model.StatusCheckDegradationDegraded:
					judged++
				}
			}
			if judged > 0 {
				item.PassRate = float64(passed) * 100 / float64(judged)
			}
			item.JudgedCount = judged
			items = append(items, item)
		}
		result.Groups[i].Degradation = items
	}
	return nil
}
