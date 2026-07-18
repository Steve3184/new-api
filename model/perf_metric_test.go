package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPerfGroupCacheSummaryBucketsExcludesConfiguredModels(t *testing.T) {
	truncateTables(t)
	const bucketTs int64 = 1_720_000_000
	require.NoError(t, DB.Create([]PerfMetric{
		{
			ModelName:        "cache-capable",
			Group:            "default",
			BucketTs:         bucketTs,
			RequestCount:     10,
			CacheHitCount:    8,
			CacheSampleCount: 10,
		},
		{
			ModelName:        "no-cache-model",
			Group:            "default",
			BucketTs:         bucketTs,
			RequestCount:     10,
			CacheSampleCount: 10,
		},
	}).Error)

	allRows, err := GetPerfGroupCacheSummaryBuckets(bucketTs-1, bucketTs+1, []string{"default"}, nil)
	require.NoError(t, err)
	require.Len(t, allRows, 1)
	assert.Equal(t, int64(8), allRows[0].CacheHitCount)
	assert.Equal(t, int64(20), allRows[0].CacheSampleCount)

	filteredRows, err := GetPerfGroupCacheSummaryBuckets(bucketTs-1, bucketTs+1, []string{"default"}, []string{"no-cache-model"})
	require.NoError(t, err)
	require.Len(t, filteredRows, 1)
	assert.Equal(t, int64(8), filteredRows[0].CacheHitCount)
	assert.Equal(t, int64(10), filteredRows[0].CacheSampleCount)
}
