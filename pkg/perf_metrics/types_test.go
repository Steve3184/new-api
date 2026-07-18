package perfmetrics

import (
	"testing"

	"github.com/QuantumNous/new-api/dto"
	"github.com/stretchr/testify/assert"
)

func TestAtomicBucketIgnoresTTFTForSynchronousSamples(t *testing.T) {
	bucket := &atomicBucket{}

	bucket.add(Sample{TtftMs: 450, HasTtft: false})
	snapshot := bucket.snapshot()

	assert.Equal(t, int64(0), snapshot.ttftCount)
	assert.Equal(t, int64(0), snapshot.ttftSumMs)
}

func TestAtomicBucketTracksCacheHitRateSamples(t *testing.T) {
	bucket := &atomicBucket{}

	bucket.add(Sample{CacheSample: true, CacheHit: true})
	bucket.add(Sample{CacheSample: true})
	bucket.add(Sample{})
	snapshot := bucket.snapshot()

	assert.Equal(t, int64(3), snapshot.requestCount)
	assert.Equal(t, int64(2), snapshot.cacheSampleCount)
	assert.Equal(t, int64(1), snapshot.cacheHitCount)
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
