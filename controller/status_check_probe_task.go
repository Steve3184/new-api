package controller

import (
	"context"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	perfmetrics "github.com/QuantumNous/new-api/pkg/perf_metrics"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
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

		testUserID, err := resolveChannelTestUserID(nil)
		if err != nil {
			return summary, err
		}
		startedAt := time.Now()
		result, _, tested, err := runStatusCheckProbeChain(group, probeGroup.Config.Model, func(channel *model.Channel) testResult {
			return runStatusCheckProbeChannelTest(ctx, channel, testUserID, probeGroup.Config.Model, "", shouldUseStreamForAutomaticChannelTest(channel))
		})
		if err != nil {
			return summary, err
		}
		if !tested {
			summary.SkippedNoChannel++
			continue
		}
		latencyMs := time.Since(startedAt).Milliseconds()
		success := result.localErr == nil && result.newAPIError == nil
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

// runStatusCheckProbeChain tests channels the way a relay request retries
// inside one group: one channel per priority tier, highest priority first, up
// to the group's retry limit, stopping at the first success. Only the last
// attempt decides the outcome, so failures a lower-priority channel recovers
// from never count against availability. With a model, channels are picked
// like relay routing for that model; without one, the first channel of each
// tier runs its own test model. tested is false when no channel can serve the
// probe. lastStartedAt is when the deciding attempt began.
func runStatusCheckProbeChain(group string, modelName string, test func(*model.Channel) testResult) (result testResult, lastStartedAt time.Time, tested bool, err error) {
	attempts := ratio_setting.GetGroupRetryTimes(group, common.RetryTimes) + 1
	var tiers []*model.Channel
	if modelName == "" {
		tiers, err = model.GetEnabledChannelsForStatusCheckProbe(group)
		if err != nil {
			return testResult{}, time.Time{}, false, err
		}
	}
	for retry := range max(attempts, 1) {
		var channel *model.Channel
		if modelName == "" {
			if retry >= len(tiers) {
				break
			}
			channel = tiers[retry]
		} else {
			channel, err = model.GetRandomSatisfiedChannel(group, modelName, retry, nil)
			if err != nil || channel == nil {
				// A missing channel after an attempt keeps that attempt's
				// outcome; before any attempt the group cannot be probed.
				return result, lastStartedAt, tested, nil
			}
		}
		lastStartedAt = time.Now()
		result = test(channel)
		tested = true
		if result.localErr == nil && result.newAPIError == nil {
			break
		}
	}
	return result, lastStartedAt, tested, nil
}
