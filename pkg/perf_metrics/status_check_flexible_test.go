package perfmetrics

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStatusCheckFlexibleConfigValidatesBounds(t *testing.T) {
	config, err := ParseStatusCheckFlexibleConfig(`{"groups":{"default":{"enabled":true,"idle_minutes":20,"max_consecutive_probes":40}}}`)
	require.NoError(t, err)
	groupConfig, enabled := config.EnabledGroup("default")
	assert.True(t, enabled)
	assert.Equal(t, 20, groupConfig.IdleMinutes)
	assert.Equal(t, 40, groupConfig.MaxConsecutiveProbes)

	_, err = ParseStatusCheckFlexibleConfig(`{"groups":{"default":{"enabled":true,"idle_minutes":0,"max_consecutive_probes":40}}}`)
	require.ErrorContains(t, err, "idle_minutes")

	_, err = ParseStatusCheckFlexibleConfig(`{"groups":{"default":{"enabled":true,"idle_minutes":1441,"max_consecutive_probes":40}}}`)
	require.ErrorContains(t, err, "idle_minutes")

	_, err = ParseStatusCheckFlexibleConfig(`{"groups":{"default":{"enabled":true,"idle_minutes":15,"max_consecutive_probes":1001}}}`)
	require.ErrorContains(t, err, "max_consecutive_probes")

	_, err = ParseStatusCheckFlexibleConfig(`{"groups":{"auto":{"enabled":true,"idle_minutes":15,"max_consecutive_probes":40}}}`)
	require.ErrorContains(t, err, "automatic group")
}

func TestRecordStatusCheckProbeDoesNotAddCacheMetrics(t *testing.T) {
	hotBuckets = sync.Map{}
	t.Cleanup(func() { hotBuckets = sync.Map{} })

	RecordStatusCheckProbe("default", 125, true)

	var snapshot counters
	found := false
	hotBuckets.Range(func(key, value any) bool {
		bucket := key.(bucketKey)
		if bucket.model != StatusCheckProbeModel || bucket.group != "default" {
			return true
		}
		found = true
		snapshot = value.(*atomicBucket).snapshot()
		return false
	})
	require.True(t, found)
	assert.Equal(t, int64(1), snapshot.requestCount)
	assert.Equal(t, int64(1), snapshot.successCount)
	assert.Equal(t, int64(125), snapshot.totalLatencyMs)
	assert.Zero(t, snapshot.cacheHitCount)
	assert.Zero(t, snapshot.cacheSampleCount)
	assert.Zero(t, snapshot.cachedTokens)
	assert.Zero(t, snapshot.inputTokens)
}
