package controller

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
)

type statusCheckProbeTaskSummary struct {
	Initialized      int `json:"initialized"`
	SkippedRecent    int `json:"skipped_recent"`
	SkippedBackoff   int `json:"skipped_backoff"`
	SkippedNoChannel int `json:"skipped_no_channel"`
	Tested           int `json:"tested"`
	Succeeded        int `json:"succeeded"`
	Failed           int `json:"failed"`
}

var runStatusCheckProbeChannelTest = testChannel

// runStatusCheckProbeTask performs at most one no-charge channel test for each
// explicitly configured group. A real relay request resets that group's probe
// streak; otherwise MaxConsecutiveProbes bounds the recurring spend.
func runStatusCheckProbeTask(ctx context.Context) (statusCheckProbeTaskSummary, error) {
	summary := statusCheckProbeTaskSummary{}
	groups := statusCheckFlexibleProbeGroups()
	if len(groups) == 0 {
		return summary, nil
	}

	groupNames := make([]string, 0, len(groups))
	for _, probeGroup := range groups {
		groupNames = append(groupNames, probeGroup.Group)
	}
	states, err := model.GetStatusCheckProbeStates(groupNames)
	if err != nil {
		return summary, err
	}
	now := time.Now().Unix()

	for _, probeGroup := range groups {
		group := probeGroup.Group
		if ctx != nil && ctx.Err() != nil {
			return summary, ctx.Err()
		}
		state, exists := states[group]
		if !exists {
			if err := model.InitializeStatusCheckProbeState(group, now); err != nil {
				return summary, err
			}
			summary.Initialized++
			continue
		}

		lastPassiveAt := perfmetrics.LatestStatusCheckPassiveActivity(group, state.LastPassiveAt)
		if lastPassiveAt > state.LastPassiveAt {
			if err := model.TouchStatusCheckProbePassiveActivity(group, lastPassiveAt); err != nil {
				return summary, err
			}
			state.LastPassiveAt = lastPassiveAt
		}
		lastActivityAt := state.LastProbeAt
		if lastPassiveAt > lastActivityAt {
			lastActivityAt = lastPassiveAt
		}
		if now-lastActivityAt < int64(probeGroup.Config.IdleMinutes)*60 {
			summary.SkippedRecent++
			continue
		}

		consecutiveProbes := state.ConsecutiveProbeCount
		if lastPassiveAt > state.LastProbeAt {
			consecutiveProbes = 0
		}
		if consecutiveProbes >= probeGroup.Config.MaxConsecutiveProbes {
			summary.SkippedBackoff++
			continue
		}

		channel, err := model.GetEnabledChannelForStatusCheckProbe(group)
		if err != nil {
			return summary, err
		}
		if channel == nil {
			summary.SkippedNoChannel++
			continue
		}

		testUserID, err := resolveChannelTestUserID(nil)
		if err != nil {
			return summary, err
		}
		startedAt := time.Now()
		result := runStatusCheckProbeChannelTest(ctx, channel, testUserID, "", "", shouldUseStreamForAutomaticChannelTest(channel))
		latencyMs := time.Since(startedAt).Milliseconds()
		success := result.newAPIError == nil
		perfmetrics.RecordStatusCheckProbe(group, latencyMs, success)
		if err := model.RecordStatusCheckProbeResult(group, time.Now().Unix()); err != nil {
			return summary, err
		}
		summary.Tested++
		if success {
			summary.Succeeded++
		} else {
			summary.Failed++
		}
	}
	return summary, nil
}
