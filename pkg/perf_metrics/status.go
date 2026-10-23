package perfmetrics

import (
	"math"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
)

// QueryStatus aggregates configured-group relay metrics and, when enabled,
// low-frequency flexible active probes. Beats and history use the configured
// metrics bucket length. Degradation check samples only fill buckets without
// any other request, so they never dilute real traffic. Cache counters come
// from real model traffic only; probes and excluded models never contribute.
// History always covers the last 24 hours.
func QueryStatus(hours int, groups []string, cacheExcludedModels []string) (StatusResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	bucketSeconds := perf_metrics_setting.GetBucketSeconds()
	endTs := time.Now().Unix()
	currentBucket := endTs - endTs%bucketSeconds
	summaryStartTs := endTs - int64(hours)*3600
	summaryStartBucket := summaryStartTs - summaryStartTs%bucketSeconds
	historyStartBucket := currentBucket - (24*3600/bucketSeconds-1)*bucketSeconds
	beatStartBucket := currentBucket - int64(StatusBeatCount-1)*bucketSeconds
	// The beat strip may reach further back than the summary window so wide
	// screens can show more beats; summaries only use summaryStartTs onward.
	startTs := min(summaryStartBucket, historyStartBucket, beatStartBucket)
	rows, err := model.GetPerfGroupSummaryBucketsAll(startTs, endTs, groups, "")
	if err != nil {
		return StatusResult{}, err
	}
	degradationRows, err := model.GetPerfGroupSummaryBucketsAll(startTs, endTs, groups, StatusCheckDegradationProbeModel)
	if err != nil {
		return StatusResult{}, err
	}
	cacheExcluded := append(slices.Clone(cacheExcludedModels), StatusCheckProbeModel, StatusCheckDegradationProbeModel)
	cacheRows, err := model.GetPerfGroupCacheTokenSummaryBuckets(startTs, endTs, groups, cacheExcluded)
	if err != nil {
		return StatusResult{}, err
	}

	allowed := allowedGroupSet(groups)
	// Stored rows may use an older bucket length; every timestamp is folded
	// into the current bucket length.
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
		ts := row.BucketTs - row.BucketTs%bucketSeconds
		value := buckets[row.Group][ts]
		mergeStatusCounters(&value, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
		})
		buckets[row.Group][ts] = value
	}
	for _, row := range cacheRows {
		if _, ok := buckets[row.Group]; !ok {
			buckets[row.Group] = make(map[int64]counters)
		}
		ts := row.BucketTs - row.BucketTs%bucketSeconds
		value := buckets[row.Group][ts]
		value.cacheHitCount += row.CacheHitCount
		value.cacheSampleCount += row.CacheSampleCount
		value.cachedTokens += row.CachedTokens
		value.inputTokens += row.InputTokens
		buckets[row.Group][ts] = value
	}

	degradationBuckets := make(map[string]map[int64]counters)
	for _, row := range degradationRows {
		if _, ok := degradationBuckets[row.Group]; !ok {
			degradationBuckets[row.Group] = make(map[int64]counters)
		}
		ts := row.BucketTs - row.BucketTs%bucketSeconds
		value := degradationBuckets[row.Group][ts]
		mergeStatusCounters(&value, counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
		})
		degradationBuckets[row.Group][ts] = value
	}

	excludedModels := make(map[string]struct{}, len(cacheExcluded))
	for _, modelName := range cacheExcluded {
		excludedModels[modelName] = struct{}{}
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
		ts := bucket.bucketTs - bucket.bucketTs%bucketSeconds
		current := buckets[bucket.group][ts]
		hot := value.(*atomicBucket).snapshot()
		if _, excluded := excludedModels[bucket.model]; excluded {
			hot.cacheHitCount = 0
			hot.cacheSampleCount = 0
			hot.cachedTokens = 0
			hot.inputTokens = 0
		}
		mergeStatusCounters(&current, hot)
		buckets[bucket.group][ts] = current
		if bucket.model == StatusCheckDegradationProbeModel {
			if _, ok := degradationBuckets[bucket.group]; !ok {
				degradationBuckets[bucket.group] = make(map[int64]counters)
			}
			degradation := degradationBuckets[bucket.group][ts]
			mergeStatusCounters(&degradation, hot)
			degradationBuckets[bucket.group][ts] = degradation
		}
		return true
	})

	result := make([]StatusGroup, 0, len(groups))
	for _, group := range groups {
		effective := make(map[int64]counters, len(buckets[group]))
		filled := make(map[int64]bool)
		for ts, value := range buckets[group] {
			realValue := subtractStatusCounters(value, degradationBuckets[group][ts])
			if realValue.requestCount > 0 {
				effective[ts] = realValue
			}
		}
		for ts, value := range degradationBuckets[group] {
			if _, hasRealRequests := effective[ts]; !hasRealRequests && value.requestCount > 0 {
				effective[ts] = value
				filled[ts] = true
			}
		}

		total := counters{}
		hourly := make(map[int64]counters)
		for ts, value := range effective {
			if ts < summaryStartBucket {
				continue
			}
			mergeStatusCounters(&total, value)
			hour := hourly[ts-ts%3600]
			mergeStatusCounters(&hour, value)
			hourly[ts-ts%3600] = hour
		}
		hourTimestamps := make([]int64, 0, len(hourly))
		for hourTs := range hourly {
			hourTimestamps = append(hourTimestamps, hourTs)
		}
		slices.Sort(hourTimestamps)
		rates := make([]float64, 0, len(hourTimestamps))
		for _, hourTs := range hourTimestamps {
			rates = append(rates, math.Round(successRate(hourly[hourTs])*100)/100)
		}
		if len(rates) > 24 {
			rates = rates[len(rates)-24:]
		}

		beats := make([]StatusBeat, 0, StatusBeatCount)
		for ts := beatStartBucket; ts <= currentBucket; ts += bucketSeconds {
			value := effective[ts]
			beats = append(beats, StatusBeat{
				Ts:           ts,
				RequestCount: value.requestCount,
				SuccessCount: value.successCount,
				Availability: math.Round(successRate(value)*100) / 100,
				AvgTtftMs:    avg(value.ttftSumMs, value.ttftCount),
				ProbeFilled:  filled[ts],
			})
		}
		history := make([]StatusHistoryPoint, 0, (currentBucket-historyStartBucket)/bucketSeconds+1)
		for ts := historyStartBucket; ts <= currentBucket; ts += bucketSeconds {
			value := effective[ts]
			history = append(history, StatusHistoryPoint{
				Ts:               ts,
				RequestCount:     value.requestCount,
				Availability:     math.Round(successRate(value)*100) / 100,
				ProbeFilled:      filled[ts],
				AvgLatencyMs:     avg(value.totalLatencyMs, value.requestCount),
				AvgTtftMs:        avg(value.ttftSumMs, value.ttftCount),
				TtftSampleCount:  value.ttftCount,
				CacheHitRate:     math.Round(cacheTokenRate(value)*100) / 100,
				CacheSampleCount: value.cacheSampleCount,
				CacheInputTokens: value.inputTokens,
			})
		}
		cacheRate := cacheTokenRate(total)
		result = append(result, StatusGroup{
			Group:            group,
			Availability:     math.Round(successRate(total)*100) / 100,
			AvgLatencyMs:     avg(total.totalLatencyMs, total.requestCount),
			AvgTtftMs:        avg(total.ttftSumMs, total.ttftCount),
			CacheHitRate:     math.Round(cacheRate*100) / 100,
			CacheSampleCount: total.cacheSampleCount,
			CacheInputTokens: total.inputTokens,
			RequestCount:     total.requestCount,
			Availability24:   rates,
			History24:        history,
			Beats:            beats,
		})
	}
	return StatusResult{BucketSeconds: bucketSeconds, Groups: result}, nil
}

// subtractStatusCounters removes the request-level counters of part from
// value. Degradation samples carry no cache fields, so those stay untouched.
func subtractStatusCounters(value counters, part counters) counters {
	value.requestCount -= part.requestCount
	value.successCount -= part.successCount
	value.totalLatencyMs -= part.totalLatencyMs
	value.ttftSumMs -= part.ttftSumMs
	value.ttftCount -= part.ttftCount
	return value
}

func mergeStatusCounters(target *counters, value counters) {
	target.requestCount += value.requestCount
	target.successCount += value.successCount
	target.totalLatencyMs += value.totalLatencyMs
	target.ttftSumMs += value.ttftSumMs
	target.ttftCount += value.ttftCount
	target.cacheHitCount += value.cacheHitCount
	target.cacheSampleCount += value.cacheSampleCount
	target.cachedTokens += value.cachedTokens
	target.inputTokens += value.inputTokens
}

func cacheTokenRate(value counters) float64 {
	if value.inputTokens <= 0 || value.cachedTokens <= 0 {
		return 0
	}
	inputTokens := value.inputTokens
	if value.cachedTokens > inputTokens {
		inputTokens = adjustedCacheInputTokens(inputTokens, value.cachedTokens)
	}
	return float64(value.cachedTokens) / float64(inputTokens) * 100
}
