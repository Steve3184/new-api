/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, it } from 'vitest'

import {
  buildContributionCalendar,
  contributionRange,
  localDateKey,
} from '../calendar'

const NOW = new Date(2026, 4, 20, 23, 59, 59)
const NOON = (year: number, month: number, day: number) =>
  new Date(year, month, day, 12, 0, 0).getTime() / 1000

function rangeOf(maxDays: number) {
  return contributionRange(NOW, maxDays)
}

describe('contributionRange', () => {
  it('covers the last 365 local days ending today', () => {
    const range = contributionRange(NOW, 365)

    expect(localDateKey(range.start)).toBe('2025-05-21')
    expect(localDateKey(range.today)).toBe('2026-05-20')
    expect(localDateKey(range.coverageStart)).toBe('2025-05-21')
    expect(range.start_timestamp).toBe(
      Math.floor(new Date(2025, 4, 21, 0, 0, 0).getTime() / 1000)
    )
  })

  it('starts coverage at a local midnight when the site range is shorter', () => {
    const range = contributionRange(NOW, 30)

    expect(localDateKey(range.start)).toBe('2025-05-21')
    expect(localDateKey(range.coverageStart)).toBe('2026-04-21')
    expect(range.coverageStart.getHours()).toBe(0)
    expect(range.coverageStart.getMinutes()).toBe(0)
  })

  it('never reaches before the rolling year when the limit exceeds it', () => {
    const range = contributionRange(NOW, 5000)

    expect(localDateKey(range.coverageStart)).toBe('2025-05-21')
  })
})

describe('buildContributionCalendar', () => {
  it('starts every grid on Sunday and ends on Saturday', () => {
    const days = buildContributionCalendar([], rangeOf(365))

    expect(days[0].weekday).toBe(0)
    expect(days.at(-1)?.weekday).toBe(6)
    expect(days.length % 7).toBe(0)
    expect(days.slice(0, 7).map((day) => day.week)).toEqual([
      0, 0, 0, 0, 0, 0, 0,
    ])
    expect(days.slice(7, 14).map((day) => day.week)).toEqual([
      1, 1, 1, 1, 1, 1, 1,
    ])
  })

  it('sums tokens and requests of one local day', () => {
    const days = buildContributionCalendar(
      [
        { created_at: NOON(2026, 4, 20), token_used: 100, count: 1 },
        { created_at: NOON(2026, 4, 20) + 3600, token_used: 50, count: 2 },
      ],
      rangeOf(365)
    )

    const today = days.find((day) => day.date === '2026-05-20')
    expect(today?.tokens).toBe(150)
    expect(today?.requests).toBe(3)
  })

  it('counts cache tokens once by ignoring them, matching the usage totals', () => {
    const days = buildContributionCalendar(
      [
        {
          created_at: NOON(2026, 4, 19),
          token_used: 40,
          count: 1,
          cache_read_tokens: 900,
          cache_creation_tokens: 300,
        },
      ],
      rangeOf(365)
    )

    const day = days.find((entry) => entry.date === '2026-05-19')
    expect(day?.tokens).toBe(40)
  })

  it('ignores rows outside the query window', () => {
    const days = buildContributionCalendar(
      [
        { created_at: NOON(2024, 0, 1), token_used: 10_000 },
        { created_at: NOON(2026, 4, 18), token_used: 7 },
      ],
      rangeOf(30)
    )

    const total = days.reduce((sum, day) => sum + day.tokens, 0)
    expect(total).toBe(7)
  })

  it('treats unusable usage counters as zero instead of producing NaN', () => {
    const days = buildContributionCalendar(
      [
        { created_at: NOON(2026, 4, 20), token_used: Number.NaN, count: -3 },
        {
          created_at: NOON(2026, 4, 20) + 60,
          token_used: Number.POSITIVE_INFINITY,
        },
      ],
      rangeOf(365)
    )

    const today = days.find((day) => day.date === '2026-05-20')
    expect(today?.tokens).toBe(0)
    expect(today?.requests).toBe(0)
    expect(today?.level).toBe(0)
  })

  it('scales the busiest day to the darkest level', () => {
    const days = buildContributionCalendar(
      [
        { created_at: NOON(2026, 4, 20), token_used: 5_000_000 },
        { created_at: NOON(2026, 4, 19), token_used: 20 },
      ],
      rangeOf(365)
    )

    expect(days.find((day) => day.date === '2026-05-20')?.level).toBe(4)
    expect(days.find((day) => day.date === '2026-05-19')?.level).toBe(1)
    expect(days.find((day) => day.date === '2026-05-18')?.level).toBe(0)
  })

  it('marks days outside the site range and future days as unavailable', () => {
    const days = buildContributionCalendar([], rangeOf(30))
    const available = days.filter((day) => day.available)

    expect(available[0].date).toBe('2026-04-21')
    expect(available.at(-1)?.date).toBe('2026-05-20')
    expect(available).toHaveLength(30)
    expect(days.at(-1)?.weekday).toBe(6)
    expect(days.at(-1)?.available).toBe(false)
  })
})
