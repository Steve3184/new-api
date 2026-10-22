package perfmetrics

import (
	"math"
	"slices"
	"time"

	"github.com/QuantumNous/new-api/model"
)

// QueryStatus aggregates configured-group relay metrics and, when enabled,
// low-frequency flexible active probes. Probe samples have no cache fields.
// Degradation check samples only fill hours without any other request, so
// they never dilute real traffic. History always covers the last 24 hours.
func QueryStatus(hours int, groups []string, cacheExcludedModels []string) (StatusResult, error) {
	if hours <= 0 {
		hours = 24
	}
	if hours > 24*30 {
		hours = 24 * 30
	}
	endTs := time.Now().Unix()
	summaryStartTs := endTs - int64(hours)*3600
	// The beat strip reaches further back than the summary window so wide
	// screens can show more hours; summaries only use summaryStartTs onward.
	startTs := min(summaryStartTs, endTs-int64(StatusBeatHours)*3600)
	rows, err := model.GetPerfGroupSummaryBucketsAll(startTs, endTs, groups, "")
	if err != nil {
		return StatusResult{}, err
	}
	degradationRows, err := model.GetPerfGroupSummaryBucketsAll(startTs, endTs, groups, StatusCheckDegradationProbeModel)
	if err != nil {
		return StatusResult{}, err
	}
	var cacheRows []model.PerfGroupCacheTokenSummaryBucket
	if len(cacheExcludedModels) > 0 {
		cacheRows, err = model.GetPerfGroupCacheTokenSummaryBuckets(startTs, endTs, groups, cacheExcludedModels)
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
			cachedTokens:     row.CachedTokens,
			inputTokens:      row.InputTokens,
		}
		if len(excludedModels) > 0 {
			value.cacheHitCount = 0
			value.cacheSampleCount = 0
			value.cachedTokens = 0
			value.inputTokens = 0
		}
		buckets[row.Group][row.BucketTs] = value
	}
	for _, row := range cacheRows {
		if _, ok := buckets[row.Group]; !ok {
			buckets[row.Group] = make(map[int64]counters)
		}
		value := buckets[row.Group][row.BucketTs]
		value.cachedTokens = row.CachedTokens
		value.inputTokens = row.InputTokens
		buckets[row.Group][row.BucketTs] = value
	}

	degradationBuckets := make(map[string]map[int64]counters)
	for _, row := range degradationRows {
		if _, ok := degradationBuckets[row.Group]; !ok {
			degradationBuckets[row.Group] = make(map[int64]counters)
		}
		degradationBuckets[row.Group][row.BucketTs] = counters{
			requestCount:   row.RequestCount,
			successCount:   row.SuccessCount,
			totalLatencyMs: row.TotalLatencyMs,
			ttftSumMs:      row.TtftSumMs,
			ttftCount:      row.TtftCount,
		}
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
			hot.cachedTokens = 0
			hot.inputTokens = 0
		}
		mergeStatusCounters(&current, hot)
		buckets[bucket.group][bucket.bucketTs] = current
		if bucket.model == StatusCheckDegradationProbeModel {
			if _, ok := degradationBuckets[bucket.group]; !ok {
				degradationBuckets[bucket.group] = make(map[int64]counters)
			}
			degradation := degradationBuckets[bucket.group][bucket.bucketTs]
			mergeStatusCounters(&degradation, hot)
			degradationBuckets[bucket.group][bucket.bucketTs] = degradation
		}
		return true
	})

	currentHour := endTs - endTs%3600
	result := make([]StatusGroup, 0, len(groups))
	for _, group := range groups {
		realHourly := make(map[int64]counters)
		degradationHourly := make(map[int64]counters)
		for ts, value := range buckets[group] {
			hourTs := ts - ts%3600
			hour := realHourly[hourTs]
			mergeStatusCounters(&hour, subtractStatusCounters(value, degradationBuckets[group][ts]))
			realHourly[hourTs] = hour
		}
		for ts, value := range degradationBuckets[group] {
			hourTs := ts - ts%3600
			hour := degradationHourly[hourTs]
			mergeStatusCounters(&hour, value)
			degradationHourly[hourTs] = hour
		}

		effectiveHourly := make(map[int64]counters, len(realHourly))
		filledHours := make(map[int64]bool)
		for hourTs, value := range realHourly {
			if value.requestCount > 0 {
				effectiveHourly[hourTs] = value
			}
		}
		for hourTs, value := range degradationHourly {
			if _, hasRealRequests := effectiveHourly[hourTs]; !hasRealRequests && value.requestCount > 0 {
				effectiveHourly[hourTs] = value
				filledHours[hourTs] = true
			}
		}

		summaryStartHour := summaryStartTs - summaryStartTs%3600
		timestamps := make([]int64, 0, len(effectiveHourly))
		total := counters{}
		for hourTs, value := range effectiveHourly {
			if hourTs < summaryStartHour {
				continue
			}
			timestamps = append(timestamps, hourTs)
			mergeStatusCounters(&total, value)
		}
		slices.Sort(timestamps)
		rates := make([]float64, 0, len(timestamps))
		for _, hourTs := range timestamps {
			rates = append(rates, math.Round(successRate(effectiveHourly[hourTs])*100)/100)
		}
		if len(rates) > 24 {
			rates = rates[len(rates)-24:]
		}
		beats := make([]StatusBeat, 0, StatusBeatHours)
		for hourTs := currentHour - int64(StatusBeatHours-1)*3600; hourTs <= currentHour; hourTs += 3600 {
			value := effectiveHourly[hourTs]
			beats = append(beats, StatusBeat{
				Ts:           hourTs,
				RequestCount: value.requestCount,
				Availability: math.Round(successRate(value)*100) / 100,
				AvgTtftMs:    avg(value.ttftSumMs, value.ttftCount),
				ProbeFilled:  filledHours[hourTs],
			})
		}
		history := make([]StatusHistoryPoint, 0, 24)
		for hourTs := currentHour - 23*3600; hourTs <= currentHour; hourTs += 3600 {
			value := effectiveHourly[hourTs]
			history = append(history, StatusHistoryPoint{
				Ts:               hourTs,
				RequestCount:     value.requestCount,
				Availability:     math.Round(successRate(value)*100) / 100,
				ProbeFilled:      filledHours[hourTs],
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
	return StatusResult{Groups: result}, nil
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
