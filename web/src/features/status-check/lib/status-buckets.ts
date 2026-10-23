/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import dayjs from 'dayjs'
import type { TFunction } from 'i18next'

import {
  getSuccessRateLevel,
  getSuccessRateLevelDotClass,
} from '@/features/performance-metrics/lib/format'
import type { StatusBeat } from '@/features/performance-metrics/types'

const HOUR_SECONDS = 3600

/**
 * Color of one status beat. A bucket with at least one successful final
 * request is shown as degraded at worst, never as down.
 */
export function getStatusBeatColorClass(
  beat: Pick<StatusBeat, 'availability' | 'success_count'>
): string {
  const level = getSuccessRateLevel(beat.availability)
  if (level === 'critical' && beat.success_count > 0) {
    return getSuccessRateLevelDotClass('warning')
  }
  return getSuccessRateLevelDotClass(level)
}

/** Time range covered by the bucket starting at `ts`, e.g. `10-23 10:05–10:10`. */
export function formatStatusBucketRange(
  ts: number,
  bucketSeconds: number
): string {
  const start = dayjs.unix(ts)
  const end = dayjs.unix(ts + bucketSeconds)
  return `${start.format('YYYY-MM-DD HH:mm')}–${end.format('HH:mm')}`
}

/** Bucket length as a short duration, e.g. `5 min` or `1 h`. */
export function formatStatusBucketLength(
  bucketSeconds: number,
  t: TFunction
): string {
  if (bucketSeconds % HOUR_SECONDS === 0) {
    return t('{{count}} h', { count: bucketSeconds / HOUR_SECONDS })
  }
  return t('{{count}} min', { count: Math.max(1, bucketSeconds / 60) })
}
