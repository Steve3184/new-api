package model

import (
	"sort"

	"github.com/QuantumNous/new-api/common"
)

type TokenAutoRouteGroupStatus struct {
	Group           string `json:"group"`
	TotalChannels   int    `json:"total_channels"`
	EnabledChannels int    `json:"enabled_channels"`
	AutoDisabled    int    `json:"auto_disabled"`
	ManualDisabled  int    `json:"manual_disabled"`
	State           string `json:"state"`
}

type TokenAutoRouteModelStatus struct {
	Model           string                      `json:"model"`
	TotalChannels   int                         `json:"total_channels"`
	EnabledChannels int                         `json:"enabled_channels"`
	AutoDisabled    int                         `json:"auto_disabled"`
	ManualDisabled  int                         `json:"manual_disabled"`
	State           string                      `json:"state"`
	LastReason      string                      `json:"last_reason,omitempty"`
	LastChangedAt   int64                       `json:"last_changed_at,omitempty"`
	Groups          []TokenAutoRouteGroupStatus `json:"groups"`
}

type tokenAutoRouteAbility struct {
	Group     string
	Model     string
	ChannelId int
	Enabled   bool
}

func tokenAutoRouteState(total, enabled, autoDisabled, manualDisabled int) string {
	if total == 0 {
		return "unavailable"
	}
	if enabled == total {
		return "available"
	}
	if enabled > 0 {
		return "degraded"
	}
	if autoDisabled > 0 {
		return "auto_disabled"
	}
	if manualDisabled > 0 {
		return "disabled"
	}
	return "unavailable"
}

// GetTokenAutoRouteModelStatuses returns a live snapshot from the ability and
// channel tables. It deliberately reports only counts and the persisted
// disable reason, never channel credentials.
func GetTokenAutoRouteModelStatuses(groups []string, models []string) ([]TokenAutoRouteModelStatus, error) {
	if len(groups) == 0 || len(models) == 0 {
		return []TokenAutoRouteModelStatus{}, nil
	}
	var abilities []tokenAutoRouteAbility
	if err := DB.Model(&Ability{}).
		Select(commonGroupCol+", model, channel_id, enabled").
		Where(commonGroupCol+" IN ? AND model IN ?", groups, models).
		Find(&abilities).Error; err != nil {
		return nil, err
	}
	channelIDs := make([]int, 0, len(abilities))
	seenChannelIDs := make(map[int]struct{}, len(abilities))
	for _, ability := range abilities {
		if _, ok := seenChannelIDs[ability.ChannelId]; ok {
			continue
		}
		seenChannelIDs[ability.ChannelId] = struct{}{}
		channelIDs = append(channelIDs, ability.ChannelId)
	}
	channels, err := GetChannelsByIds(channelIDs)
	if err != nil {
		return nil, err
	}
	channelByID := make(map[int]*Channel, len(channels))
	for _, channel := range channels {
		channelByID[channel.Id] = channel
	}

	modelStatus := make(map[string]*TokenAutoRouteModelStatus, len(models))
	for _, modelName := range models {
		modelStatus[modelName] = &TokenAutoRouteModelStatus{
			Model:  modelName,
			Groups: make([]TokenAutoRouteGroupStatus, 0, len(groups)),
		}
	}
	for _, ability := range abilities {
		status := modelStatus[ability.Model]
		if status == nil {
			continue
		}
		groupStatusIndex := -1
		for index := range status.Groups {
			if status.Groups[index].Group == ability.Group {
				groupStatusIndex = index
				break
			}
		}
		if groupStatusIndex < 0 {
			status.Groups = append(status.Groups, TokenAutoRouteGroupStatus{Group: ability.Group})
			groupStatusIndex = len(status.Groups) - 1
		}
		groupStatus := &status.Groups[groupStatusIndex]
		groupStatus.TotalChannels++
		status.TotalChannels++
		channel := channelByID[ability.ChannelId]
		if ability.Enabled && channel != nil && channel.Status == common.ChannelStatusEnabled {
			groupStatus.EnabledChannels++
			status.EnabledChannels++
		}
		if channel == nil {
			continue
		}
		switch channel.Status {
		case common.ChannelStatusAutoDisabled:
			groupStatus.AutoDisabled++
			status.AutoDisabled++
		case common.ChannelStatusManuallyDisabled:
			groupStatus.ManualDisabled++
			status.ManualDisabled++
		}
		otherInfo := channel.GetOtherInfo()
		if reason, ok := otherInfo["status_reason"].(string); ok && reason != "" {
			if changedAt, ok := otherInfo["status_time"].(float64); ok && int64(changedAt) >= status.LastChangedAt {
				status.LastReason = reason
				status.LastChangedAt = int64(changedAt)
			} else if status.LastReason == "" {
				status.LastReason = reason
			}
		}
	}

	result := make([]TokenAutoRouteModelStatus, 0, len(models))
	for _, modelName := range models {
		status := modelStatus[modelName]
		if status == nil {
			continue
		}
		status.State = tokenAutoRouteState(status.TotalChannels, status.EnabledChannels, status.AutoDisabled, status.ManualDisabled)
		for index := range status.Groups {
			group := &status.Groups[index]
			group.State = tokenAutoRouteState(group.TotalChannels, group.EnabledChannels, group.AutoDisabled, group.ManualDisabled)
		}
		sort.SliceStable(status.Groups, func(i, j int) bool { return status.Groups[i].Group < status.Groups[j].Group })
		result = append(result, *status)
	}
	return result, nil
}
