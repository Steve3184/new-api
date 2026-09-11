/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, it } from 'vitest'

import { formatPerfBucketLabel } from '../lib/bucket-label'

// Built from local components, so the clock the runner uses does not matter.
const SAMPLE_MS = new Date(2026, 8, 11, 14, 30, 0).getTime()
const FIVE_MINUTES_MS = 5 * 60 * 1000

describe('perf chart bucket labels', () => {
  it('separates samples inside one hour for sub-hour buckets', () => {
    for (const bucket of ['minute', '5min'] as const) {
      const first = formatPerfBucketLabel(SAMPLE_MS, bucket)
      const second = formatPerfBucketLabel(SAMPLE_MS + FIVE_MINUTES_MS, bucket)

      expect(first).not.toBe(second)
    }
  })

  it('collapses samples inside one hour for the hourly bucket', () => {
    const first = formatPerfBucketLabel(SAMPLE_MS, 'hour')
    const second = formatPerfBucketLabel(SAMPLE_MS + FIVE_MINUTES_MS, 'hour')

    expect(first).toBe(second)
  })

  it('distinguishes the same time of day on different days for the hourly bucket', () => {
    const today = formatPerfBucketLabel(SAMPLE_MS, 'hour')
    const tomorrow = formatPerfBucketLabel(SAMPLE_MS + 24 * 60 * 60 * 1000, 'hour')

    expect(today).not.toBe(tomorrow)
  })
})
