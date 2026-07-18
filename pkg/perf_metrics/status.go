package perfmetrics

import (
	"math"
	"sort"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// QueryStatus aggregates the passive relay metrics by configured group. It
// never probes an upstream model, so callers can safely use it for a status UI.
func QueryStatus(hours int, groups []string, cacheExcludedModels []string) (StatusResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	startTs := endTs - int64(hours)*3600
	rows, err := model.GetPerfGroupSummaryBucketsAll(startTs, endTs, groups)
	if err != nil {
		return StatusResult{}, err
	}
	var cacheRows []model.PerfGroupCacheSummaryBucket
	if len(cacheExcludedModels) > 0 {
		cacheRows, err = model.GetPerfGroupCacheSummaryBuckets(startTs, endTs, groups, cacheExcludedModels)
		if err != nil {
			return StatusResult{}, err
		}
	}

	allowed := allowedGroupSet(groups)
	excludedModels := make(map[string]struct{}, len(cacheExcludedModels))
	for _, modelName := range cacheExcludedModels {
		excludedModels[modelName] = struct{}{}
	}
	buckets := make(map[string]map[int64]counters)
	for _, row := range rows {
		if allowed != nil {
			if _, ok := allowed[row.Group]; !ok {
				continue
			}
		}
		if _, ok := buckets[row.Group]; !ok {
			buckets[row.Group] = make(map[int64]counters)
		}
		value := counters{
			requestCount:     row.RequestCount,
			successCount:     row.SuccessCount,
			totalLatencyMs:   row.TotalLatencyMs,
			ttftSumMs:        row.TtftSumMs,
			ttftCount:        row.TtftCount,
			cacheHitCount:    row.CacheHitCount,
			cacheSampleCount: row.CacheSampleCount,
		}
		if len(excludedModels) > 0 {
			value.cacheHitCount = 0
			value.cacheSampleCount = 0
		}
		buckets[row.Group][row.BucketTs] = value
	}
	for _, row := range cacheRows {
		if _, ok := buckets[row.Group]; !ok {
			buckets[row.Group] = make(map[int64]counters)
		}
		value := buckets[row.Group][row.BucketTs]
		value.cacheHitCount = row.CacheHitCount
		value.cacheSampleCount = row.CacheSampleCount
		buckets[row.Group][row.BucketTs] = value
	}

	hotBuckets.Range(func(key, value any) bool {
		bucket := key.(bucketKey)
		if bucket.bucketTs < startTs || bucket.bucketTs > endTs {
			return true
		}
		if allowed != nil {
			if _, ok := allowed[bucket.group]; !ok {
				return true
			}
		}
		if _, ok := buckets[bucket.group]; !ok {
			buckets[bucket.group] = make(map[int64]counters)
		}
		current := buckets[bucket.group][bucket.bucketTs]
		hot := value.(*atomicBucket).snapshot()
		if _, excluded := excludedModels[bucket.model]; excluded {
			hot.cacheHitCount = 0
			hot.cacheSampleCount = 0
		}
		mergeStatusCounters(&current, hot)
		buckets[bucket.group][bucket.bucketTs] = current
		return true
	})

	result := make([]StatusGroup, 0, len(groups))
	for _, group := range groups {
		groupBuckets := buckets[group]
		timestamps := make([]int64, 0, len(groupBuckets))
		total := counters{}
		for ts, value := range groupBuckets {
			timestamps = append(timestamps, ts)
			mergeStatusCounters(&total, value)
		}
		sort.Slice(timestamps, func(i, j int) bool { return timestamps[i] < timestamps[j] })
		rates := make([]float64, 0, len(timestamps))
		history := make([]StatusHistoryPoint, 0, len(timestamps))
		for _, ts := range timestamps {
			value := groupBuckets[ts]
			rates = append(rates, math.Round(successRate(value)*100)/100)
			cacheRate := 0.0
			if value.cacheSampleCount > 0 {
				cacheRate = float64(value.cacheHitCount) / float64(value.cacheSampleCount) * 100
			}
			history = append(history, StatusHistoryPoint{
				Ts:               ts,
				AvgTtftMs:        avg(value.ttftSumMs, value.ttftCount),
				TtftSampleCount:  value.ttftCount,
				CacheHitRate:     math.Round(cacheRate*100) / 100,
				CacheSampleCount: value.cacheSampleCount,
			})
		}
		if len(rates) > 24 {
			rates = rates[len(rates)-24:]
		}
		if len(history) > 24 {
			history = history[len(history)-24:]
		}
		cacheRate := 0.0
		if total.cacheSampleCount > 0 {
			cacheRate = float64(total.cacheHitCount) / float64(total.cacheSampleCount) * 100
		}
		result = append(result, StatusGroup{
			Group:            group,
			Availability:     math.Round(successRate(total)*100) / 100,
			AvgTtftMs:        avg(total.ttftSumMs, total.ttftCount),
			CacheHitRate:     math.Round(cacheRate*100) / 100,
			CacheSampleCount: total.cacheSampleCount,
			RequestCount:     total.requestCount,
			Availability24:   rates,
			History24:        history,
		})
	}
	return StatusResult{Groups: result}, nil
}

func mergeStatusCounters(target *counters, value counters) {
	target.requestCount += value.requestCount
	target.successCount += value.successCount
	target.totalLatencyMs += value.totalLatencyMs
	target.ttftSumMs += value.ttftSumMs
	target.ttftCount += value.ttftCount
	target.cacheHitCount += value.cacheHitCount
	target.cacheSampleCount += value.cacheSampleCount
}
