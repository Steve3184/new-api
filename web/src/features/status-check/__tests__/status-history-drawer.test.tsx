/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import type { StatusGroup } from '@/features/performance-metrics/types'

import { StatusHistoryDrawer } from '../status-history-drawer'

vi.mock('../status-history-chart', () => ({
  StatusHistoryLineChart: (props: { points: unknown[]; metric: string }) => (
    <div data-testid={`chart-${props.metric}`}>{props.points.length}</div>
  ),
}))

afterEach(cleanup)

const BUCKET = 300

function buildGroup(): StatusGroup {
  return {
    group: 'default',
    availability: 100,
    avg_latency_ms: 0,
    avg_ttft_ms: 0,
    cache_hit_rate: 0,
    cache_sample_count: 0,
    cache_input_tokens: 0,
    request_count: 0,
    availability_24h: [],
    history_24h: Array.from({ length: (24 * 3600) / BUCKET }, (_, index) => ({
      ts: index * BUCKET,
      request_count: 0,
      availability: 0,
      probe_filled: false,
      avg_latency_ms: 0,
      avg_ttft_ms: 0,
      ttft_sample_count: 0,
      cache_hit_rate: 0,
      cache_sample_count: 0,
      cache_input_tokens: 0,
    })),
    beats: [],
  }
}

describe('StatusHistoryDrawer', () => {
  it('charts every 5-minute bucket of the last 24 hours when buckets are 5 minutes', async () => {
    render(
      <StatusHistoryDrawer
        group={buildGroup()}
        bucketSeconds={BUCKET}
        open
        onOpenChange={vi.fn()}
      />
    )

    expect(
      screen.getByText('Status data for the last 24 hours, one point per 5 min')
    ).toBeVisible()
    expect(await screen.findByTestId('chart-availability')).toHaveTextContent(
      '288'
    )
  })
})
