package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPerfGroupCacheTokenSummaryBucketsExcludesConfiguredModels(t *testing.T) {
	truncateTables(t)
	const bucketTs int64 = 1_720_000_000
	require.NoError(t, DB.Create([]PerfMetric{
		{
			ModelName:    "cache-capable",
			Group:        "default",
			BucketTs:     bucketTs,
			RequestCount: 10,
			CachedTokens: 800,
			InputTokens:  1000,
		},
		{
			ModelName:    "no-cache-model",
			Group:        "default",
			BucketTs:     bucketTs,
			RequestCount: 10,
			InputTokens:  1000,
		},
	}).Error)

	allRows, err := GetPerfGroupCacheTokenSummaryBuckets(bucketTs-1, bucketTs+1, []string{"default"}, nil)
	require.NoError(t, err)
	require.Len(t, allRows, 1)
	assert.Equal(t, int64(800), allRows[0].CachedTokens)
	assert.Equal(t, int64(2000), allRows[0].InputTokens)

	filteredRows, err := GetPerfGroupCacheTokenSummaryBuckets(bucketTs-1, bucketTs+1, []string{"default"}, []string{"no-cache-model"})
	require.NoError(t, err)
	require.Len(t, filteredRows, 1)
	assert.Equal(t, int64(800), filteredRows[0].CachedTokens)
	assert.Equal(t, int64(1000), filteredRows[0].InputTokens)
}
