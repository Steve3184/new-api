package perfmetrics

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/relaykit/dto"
	"github.com/QuantumNous/new-api/relaykit/types"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAtomicBucketIgnoresTTFTForSynchronousSamples(t *testing.T) {
	bucket := &atomicBucket{}

	bucket.add(Sample{TtftMs: 450, HasTtft: false})
	snapshot := bucket.snapshot()

	assert.Equal(t, int64(0), snapshot.ttftCount)
	assert.Equal(t, int64(0), snapshot.ttftSumMs)
}

func TestAtomicBucketTracksCacheTokenTotals(t *testing.T) {
	bucket := &atomicBucket{}

	bucket.add(Sample{CacheSample: true, CacheHit: true, CachedTokens: 80, InputTokens: 100})
	bucket.add(Sample{CacheSample: true, CachedTokens: 150, InputTokens: 100})
	bucket.add(Sample{})
	snapshot := bucket.snapshot()

	assert.Equal(t, int64(3), snapshot.requestCount)
	assert.Equal(t, int64(2), snapshot.cacheSampleCount)
	assert.Equal(t, int64(1), snapshot.cacheHitCount)
	assert.Equal(t, int64(230), snapshot.cachedTokens)
	assert.Equal(t, int64(350), snapshot.inputTokens)
}

func TestHasCacheHitSupportsChatAndResponsesUsage(t *testing.T) {
	assert.True(t, hasCacheHit(&dto.Usage{
		PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 10},
	}))
	assert.True(t, hasCacheHit(&dto.Usage{
		InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 10},
	}))
	assert.False(t, hasCacheHit(&dto.Usage{}))
	assert.False(t, hasCacheHit(nil))
}

func TestCacheTokenUsageUsesCanonicalInputTokens(t *testing.T) {
	tests := []struct {
		name       string
		usage      *dto.Usage
		wantCached int64
		wantInput  int64
	}{
		{
			name: "chat usage",
			usage: &dto.Usage{
				PromptTokens:        100,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 80},
			},
			wantCached: 80,
			wantInput:  100,
		},
		{
			name: "responses usage",
			usage: &dto.Usage{
				InputTokens:        200,
				InputTokensDetails: &dto.InputTokenDetails{CachedTokens: 50},
			},
			wantCached: 50,
			wantInput:  200,
		},
		{
			name: "legacy cache hit field",
			usage: &dto.Usage{
				PromptTokens:         120,
				PromptCacheHitTokens: 30,
			},
			wantCached: 30,
			wantInput:  120,
		},
		{
			name: "cached tokens adjust the denominator",
			usage: &dto.Usage{
				PromptTokens:        100,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 150},
			},
			wantCached: 150,
			wantInput:  250,
		},
		{
			name:       "missing input tokens",
			usage:      &dto.Usage{PromptTokensDetails: dto.InputTokenDetails{CachedTokens: 10}},
			wantCached: 0,
			wantInput:  0,
		},
		{
			name: "negative cached tokens",
			usage: &dto.Usage{
				PromptTokens:        100,
				PromptTokensDetails: dto.InputTokenDetails{CachedTokens: -10},
			},
			wantCached: 0,
			wantInput:  100,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cachedTokens, inputTokens := cacheTokenUsage(tt.usage)
			assert.Equal(t, tt.wantCached, cachedTokens)
			assert.Equal(t, tt.wantInput, inputTokens)
		})
	}
}

func TestCacheTokenRateClampsInvalidAggregates(t *testing.T) {
	assert.Equal(t, 0.0, cacheTokenRate(counters{cachedTokens: -10, inputTokens: 100}))
	assert.Equal(t, 0.0, cacheTokenRate(counters{cachedTokens: 10, inputTokens: -100}))
	assert.Equal(t, 60.0, cacheTokenRate(counters{cachedTokens: 150, inputTokens: 100}))
	assert.Equal(t, 25.0, cacheTokenRate(counters{cachedTokens: 25, inputTokens: 100}))
}

func TestAdjustedCacheInputTokensSaturatesOnOverflow(t *testing.T) {
	assert.Equal(t, maxPerfMetricInt64, adjustedCacheInputTokens(maxPerfMetricInt64-10, 20))
}

func TestRecordRelayResultCanExcludeUpstreamBadRequests(t *testing.T) {
	previous := perf_metrics_setting.GetSetting()
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
			"perf_metrics_setting.enabled":                     strconv.FormatBool(previous.Enabled),
			"perf_metrics_setting.exclude_upstream_400_errors": strconv.FormatBool(previous.ExcludeUpstream400Errors),
		}))
		hotBuckets = sync.Map{}
	})

	tests := []struct {
		name             string
		exclude400       bool
		upstreamStatus   int
		responseStatus   int
		wantRequestCount int64
	}{
		{
			name:             "records upstream 400 by default",
			responseStatus:   http.StatusBadRequest,
			upstreamStatus:   http.StatusBadRequest,
			wantRequestCount: 1,
		},
		{
			name:             "excludes mapped upstream 400 when enabled",
			exclude400:       true,
			responseStatus:   http.StatusServiceUnavailable,
			upstreamStatus:   http.StatusBadRequest,
			wantRequestCount: 0,
		},
		{
			name:             "records other upstream failures",
			exclude400:       true,
			responseStatus:   http.StatusTooManyRequests,
			upstreamStatus:   http.StatusTooManyRequests,
			wantRequestCount: 1,
		},
		{
			name:             "records local 400 failures",
			exclude400:       true,
			responseStatus:   http.StatusBadRequest,
			wantRequestCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hotBuckets = sync.Map{}
			require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
				"perf_metrics_setting.enabled":                     "true",
				"perf_metrics_setting.exclude_upstream_400_errors": strconv.FormatBool(tt.exclude400),
			}))

			relayErr := types.NewErrorWithStatusCode(errors.New("request failed"), types.ErrorCodeBadResponseStatusCode, tt.responseStatus)
			if tt.upstreamStatus != 0 {
				relayErr.SetUpstreamStatusCode(tt.upstreamStatus)
			}
			RecordRelayResult(context.Background(), &common.RelayInfo{
				OriginModelName: "gpt-test",
				UsingGroup:      "default",
				StartTime:       time.Now(),
			}, relayErr)

			var requestCount int64
			hotBuckets.Range(func(_, value any) bool {
				requestCount += value.(*atomicBucket).snapshot().requestCount
				return true
			})
			assert.Equal(t, tt.wantRequestCount, requestCount)
		})
	}
}
