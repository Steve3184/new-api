/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { cleanup, render } from '@testing-library/react'
import dayjs from 'dayjs'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { StatusHistoryPoint } from '@/features/performance-metrics/types'

import { formatStatusBucketRange } from '../lib/status-buckets'
import { StatusHistoryLineChart } from '../status-history-chart'

type TooltipPattern = {
  title: { value: (datum: unknown) => string }
  content: Array<{ value: (datum: unknown) => string }>
}

const renderedSpecs: Array<{
  data: Array<{ values: unknown[] }>
  tooltip: { mark: TooltipPattern; dimension: TooltipPattern }
}> = []

vi.mock('@visactor/react-vchart', () => ({
  VChart: (props: { spec: (typeof renderedSpecs)[number] }) => {
    renderedSpecs.push(props.spec)
    return null
  },
}))

vi.mock('@/lib/use-chart-theme', () => ({
  useChartTheme: () => ({ resolvedTheme: 'dark', themeReady: true }),
}))

afterEach(() => {
  cleanup()
  renderedSpecs.length = 0
})

const BUCKET = 300
const TS = dayjs('2026-10-24T18:30:00').unix()

const point: StatusHistoryPoint = {
  ts: TS,
  request_count: 4,
  availability: 75,
  probe_filled: false,
  avg_latency_ms: 1200.4,
  avg_ttft_ms: 800,
  ttft_sample_count: 4,
  cache_hit_rate: 72.28,
  cache_sample_count: 4,
  cache_input_tokens: 1000,
}

describe('StatusHistoryLineChart tooltips', () => {
  it.each(['mark', 'dimension'] as const)(
    'shows the bucket time range and formatted value in the %s tooltip',
    (tooltipType) => {
      render(
        <StatusHistoryLineChart
          points={[point]}
          bucketSeconds={BUCKET}
          metric='cache'
        />
      )

      const spec = renderedSpecs.at(-1)
      expect(spec).toBeDefined()
      if (!spec) return
      const datum = spec.data[0].values[0]
      const pattern = spec.tooltip[tooltipType]
      expect(pattern.title.value(datum)).toBe(
        formatStatusBucketRange(TS, BUCKET)
      )
      expect(pattern.title.value(datum)).not.toContain(String(TS))
      expect(pattern.content[0].value(datum)).toBe('72.28%')
    }
  )
})
