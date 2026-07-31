package perfmetrics

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	StatusCheckProbeModel                  = "__status_check_probe__"
	defaultStatusCheckIdleMinutes          = 15
	defaultStatusCheckMaxConsecutiveProbes = 40
	maxStatusCheckIdleMinutes              = 24 * 60
	maxStatusCheckConsecutiveProbes        = 1000
	statusCheckPassiveActivityTTL          = 48 * time.Hour
	statusCheckPassivePersistInterval      = time.Minute
)

// StatusCheckFlexibleGroupConfig controls active checks for one explicitly
// configured status-page group. The limits cap accidental spend from malformed
// settings.
type StatusCheckFlexibleGroupConfig struct {
	Enabled              bool `json:"enabled"`
	IdleMinutes          int  `json:"idle_minutes"`
	MaxConsecutiveProbes int  `json:"max_consecutive_probes"`
}

// StatusCheckFlexibleConfig keeps independent probe settings for each status
// group. Groups absent from this map never receive automatic probes.
type StatusCheckFlexibleConfig struct {
	Groups map[string]StatusCheckFlexibleGroupConfig `json:"groups"`
}

type passiveActivity struct {
	timestamp   int64
	persistedAt int64
}

var (
	statusCheckPassiveActivityMu sync.RWMutex
	statusCheckPassiveActivities = map[string]passiveActivity{}
)

func DefaultStatusCheckFlexibleGroupConfig() StatusCheckFlexibleGroupConfig {
	return StatusCheckFlexibleGroupConfig{
		Enabled:              false,
		IdleMinutes:          defaultStatusCheckIdleMinutes,
		MaxConsecutiveProbes: defaultStatusCheckMaxConsecutiveProbes,
	}
}

func DefaultStatusCheckFlexibleConfig() StatusCheckFlexibleConfig {
	return StatusCheckFlexibleConfig{Groups: map[string]StatusCheckFlexibleGroupConfig{}}
}

func ParseStatusCheckFlexibleConfig(value string) (StatusCheckFlexibleConfig, error) {
	config := DefaultStatusCheckFlexibleConfig()
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) == "null" {
		return config, nil
	}
	if err := common.UnmarshalJsonStr(value, &config); err != nil {
		return StatusCheckFlexibleConfig{}, err
	}
	if len(config.Groups) > 100 {
		return StatusCheckFlexibleConfig{}, fmt.Errorf("groups cannot exceed 100 entries")
	}
	normalizedGroups := make(map[string]StatusCheckFlexibleGroupConfig, len(config.Groups))
	for group, groupConfig := range config.Groups {
		group = strings.TrimSpace(group)
		if group == "" || len(group) > 64 {
			return StatusCheckFlexibleConfig{}, fmt.Errorf("group names must be non-empty and at most 64 characters")
		}
		if group == "auto" {
			return StatusCheckFlexibleConfig{}, fmt.Errorf("automatic group selection cannot be probed")
		}
		if groupConfig.IdleMinutes < 1 || groupConfig.IdleMinutes > maxStatusCheckIdleMinutes {
			return StatusCheckFlexibleConfig{}, fmt.Errorf("idle_minutes for group %q must be between 1 and %d", group, maxStatusCheckIdleMinutes)
		}
		if groupConfig.MaxConsecutiveProbes < 1 || groupConfig.MaxConsecutiveProbes > maxStatusCheckConsecutiveProbes {
			return StatusCheckFlexibleConfig{}, fmt.Errorf("max_consecutive_probes for group %q must be between 1 and %d", group, maxStatusCheckConsecutiveProbes)
		}
		normalizedGroups[group] = groupConfig
	}
	config.Groups = normalizedGroups
	return config, nil
}

func GetStatusCheckFlexibleConfig() StatusCheckFlexibleConfig {
	config, err := ParseStatusCheckFlexibleConfig(common.StatusCheckFlexibleMode)
	if err != nil {
		return DefaultStatusCheckFlexibleConfig()
	}
	return config
}

func (config StatusCheckFlexibleConfig) EnabledGroup(group string) (StatusCheckFlexibleGroupConfig, bool) {
	groupConfig, ok := config.Groups[strings.TrimSpace(group)]
	return groupConfig, ok && groupConfig.Enabled
}

func statusCheckFlexibleGroupConfigured(group string) bool {
	group = strings.TrimSpace(group)
	if group == "" || group == "auto" {
		return false
	}
	if _, enabled := GetStatusCheckFlexibleConfig().EnabledGroup(group); !enabled {
		return false
	}
	var groups []string
	if err := common.UnmarshalJsonStr(common.StatusCheckGroups, &groups); err != nil {
		return false
	}
	for _, configuredGroup := range groups {
		if strings.TrimSpace(configuredGroup) == group {
			return true
		}
	}
	return false
}

// NoteStatusCheckPassiveActivity tracks a normal relay request separately from
// the metrics bucket. Redis is used across masters; without Redis, persistence
// is rate-limited to once per group per minute to avoid a write per request.
func NoteStatusCheckPassiveActivity(group string) {
	if !statusCheckFlexibleGroupConfigured(group) {
		return
	}

	group = strings.TrimSpace(group)
	now := time.Now().Unix()
	persist := false
	statusCheckPassiveActivityMu.Lock()
	activity := statusCheckPassiveActivities[group]
	activity.timestamp = now
	if !common.RedisEnabled || common.RDB == nil {
		if now-activity.persistedAt >= int64(statusCheckPassivePersistInterval.Seconds()) {
			activity.persistedAt = now
			persist = true
		}
	}
	statusCheckPassiveActivities[group] = activity
	statusCheckPassiveActivityMu.Unlock()

	if common.RedisEnabled && common.RDB != nil {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		err := common.RDB.Set(ctx, statusCheckPassiveRedisKey(group), now, statusCheckPassiveActivityTTL).Err()
		cancel()
		if err == nil {
			return
		}
		statusCheckPassiveActivityMu.Lock()
		activity = statusCheckPassiveActivities[group]
		if now-activity.persistedAt >= int64(statusCheckPassivePersistInterval.Seconds()) {
			activity.persistedAt = now
			persist = true
		}
		statusCheckPassiveActivities[group] = activity
		statusCheckPassiveActivityMu.Unlock()
	}

	if persist {
		if err := model.TouchStatusCheckProbePassiveActivity(group, now); err != nil {
			common.SysError(fmt.Sprintf("failed to record status check passive activity for group=%s: %v", group, err))
		}
	}
}

// LatestStatusCheckPassiveActivity combines the durable state with the local
// and Redis observations so a different master cannot probe an active group.
func LatestStatusCheckPassiveActivity(group string, stored int64) int64 {
	latest := stored
	statusCheckPassiveActivityMu.RLock()
	if activity, ok := statusCheckPassiveActivities[group]; ok && activity.timestamp > latest {
		latest = activity.timestamp
	}
	statusCheckPassiveActivityMu.RUnlock()

	if !common.RedisEnabled || common.RDB == nil {
		return latest
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	value, err := common.RDB.Get(ctx, statusCheckPassiveRedisKey(group)).Result()
	cancel()
	if err != nil {
		return latest
	}
	if timestamp, parseErr := strconv.ParseInt(value, 10, 64); parseErr == nil && timestamp > latest {
		return timestamp
	}
	return latest
}

// RecordStatusCheckProbe adds an active probe to availability and average
// latency only. Cache counters and token fields intentionally remain zero.
func RecordStatusCheckProbe(group string, latencyMs int64, success bool) {
	Record(Sample{
		Model:     StatusCheckProbeModel,
		Group:     group,
		LatencyMs: latencyMs,
		Success:   success,
	})
}

func statusCheckPassiveRedisKey(group string) string {
	digest := sha256.Sum256([]byte(group))
	return fmt.Sprintf("status-check:passive:%x", digest[:])
}
