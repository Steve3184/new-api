/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import type { TFunction } from 'i18next'
import { describe, expect, it } from 'vitest'

import { getSuccessRateLevelDotClass } from '@/features/performance-metrics/lib/format'

import {
  formatStatusBucketLength,
  getStatusBeatColorClass,
} from '../lib/status-buckets'

const t = ((key: string, options?: { count?: number }) =>
  key.replace('{{count}}', String(options?.count))) as unknown as TFunction

describe('getStatusBeatColorClass', () => {
  it('caps a low-availability bucket with a successful request at warning', () => {
    expect(
      getStatusBeatColorClass({ availability: 10, success_count: 1 })
    ).toBe(getSuccessRateLevelDotClass('warning'))
  })

  it('shows a bucket without any successful request as critical', () => {
    expect(getStatusBeatColorClass({ availability: 0, success_count: 0 })).toBe(
      getSuccessRateLevelDotClass('critical')
    )
  })

  it('keeps healthy buckets at their own level', () => {
    expect(
      getStatusBeatColorClass({ availability: 100, success_count: 3 })
    ).toBe(getSuccessRateLevelDotClass('excellent'))
  })
})

describe('formatStatusBucketLength', () => {
  it.each([
    [60, '1 min'],
    [300, '5 min'],
    [3600, '1 h'],
  ])('formats %i seconds as %s', (seconds, expected) => {
    expect(formatStatusBucketLength(seconds, t)).toBe(expected)
  })
})
