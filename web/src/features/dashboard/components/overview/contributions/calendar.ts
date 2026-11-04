/* Copyright (C) 2023-2026 QuantumNous */
import type { QuotaDataItem } from '@/features/dashboard/types'

export interface ContributionDay {
  date: string
  timestamp: number
  tokens: number
  requests: number
  week: number
  weekday: number
  level: number
  available: boolean
}

export function localDateKey(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`
}

export function contributionRange(now: Date, maxDays = 365) {
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const start = new Date(today)
  start.setDate(start.getDate() - 364)
  const end = new Date(today)
  end.setDate(end.getDate() + 1)
  const earliest = new Date(now.getTime() - Math.max(1, maxDays) * 86400000)
  const coverageStart = new Date(Math.max(start.getTime(), earliest.getTime()))
  // Query only complete local days when the site's range is shorter than a year.
  if (
    coverageStart.getHours() ||
    coverageStart.getMinutes() ||
    coverageStart.getSeconds() ||
    coverageStart.getMilliseconds()
  ) {
    coverageStart.setHours(0, 0, 0, 0)
    coverageStart.setDate(coverageStart.getDate() + 1)
  }
  return {
    start,
    today,
    coverageStart,
    start_timestamp: Math.floor(coverageStart.getTime() / 1000),
    end_timestamp: Math.floor(end.getTime() / 1000) - 1,
  }
}

export function buildContributionCalendar(
  rows: QuotaDataItem[],
  range: ReturnType<typeof contributionRange>
): ContributionDay[] {
  const totals = new Map<string, { tokens: number; requests: number }>()
  for (const row of rows) {
    const timestamp = Number(row.created_at)
    if (
      !Number.isFinite(timestamp) ||
      timestamp < range.start_timestamp ||
      timestamp > range.end_timestamp
    ) {
      continue
    }
    const key = localDateKey(new Date(timestamp * 1000))
    const total = totals.get(key) ?? { tokens: 0, requests: 0 }
    // Match dashboard token_used. Cache counters overlap differently by provider;
    // adding them to this aggregate would double-count some providers' inputs.
    const tokens = Number(row.token_used)
    const requests = Number(row.count)
    total.tokens += Number.isFinite(tokens) ? Math.max(0, tokens) : 0
    total.requests += Number.isFinite(requests) ? Math.max(0, requests) : 0
    totals.set(key, total)
  }
  // Rank each active day against the others so all four intensity steps are
  // actually used. A ratio against the single busiest day would push almost
  // every day into the same step whenever one outlier exists.
  const activeTotals = Array.from(totals.values(), (total) => total.tokens)
    .filter((tokens) => tokens > 0)
    .sort((a, b) => a - b)
  const levelFor = (tokens: number) => {
    if (tokens <= 0) return 0
    if (activeTotals.length <= 1) return 4
    let low = 0
    let high = activeTotals.length - 1
    while (low < high) {
      const middle = (low + high) >> 1
      if (activeTotals[middle] < tokens) low = middle + 1
      else high = middle
    }
    return Math.min(4, 1 + Math.floor((low / (activeTotals.length - 1)) * 4))
  }
  const cursor = new Date(range.start)
  cursor.setDate(cursor.getDate() - cursor.getDay())
  const gridEnd = new Date(range.today)
  gridEnd.setDate(gridEnd.getDate() + (6 - gridEnd.getDay()))
  const days: ContributionDay[] = []
  while (cursor <= gridEnd) {
    const date = localDateKey(cursor)
    const total = totals.get(date) ?? { tokens: 0, requests: 0 }
    days.push({
      date,
      timestamp: cursor.getTime(),
      ...total,
      week: Math.floor(days.length / 7),
      weekday: cursor.getDay(),
      level: levelFor(total.tokens),
      available: cursor >= range.coverageStart && cursor <= range.today,
    })
    cursor.setDate(cursor.getDate() + 1)
  }
  return days
}

/**
 * Opacity of the primary colour per intensity step. Tinting with real alpha over
 * the card keeps the full lightness range of the theme: mixing with the muted
 * grey instead made neighbouring steps nearly identical.
 */
export const CONTRIBUTION_LEVEL_ALPHA = [0, 0.3, 0.5, 0.75, 1]

export const contributionColors = [
  'color-mix(in oklch, var(--muted) 60%, transparent)',
  'color-mix(in oklch, var(--primary) 30%, transparent)',
  'color-mix(in oklch, var(--primary) 50%, transparent)',
  'color-mix(in oklch, var(--primary) 75%, transparent)',
  'var(--primary)',
]
