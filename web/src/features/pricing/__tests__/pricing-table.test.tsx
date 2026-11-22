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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor, within } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { PricingTable } from '../components/pricing-table'
import type { PricingModel } from '../types'

function pricingModel(overrides: Partial<PricingModel> = {}): PricingModel {
  return {
    id: 1,
    model_name: 'example-model',
    quota_type: 0,
    model_ratio: 1,
    completion_ratio: 3,
    enable_groups: ['default'],
    group_ratio: { default: 1 },
    ...overrides,
  }
}

let queryClient: QueryClient
beforeEach(() => {
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  })
})
afterEach(() => {
  queryClient.clear()
  vi.restoreAllMocks()
})

describe('pricing table', () => {
  it('renders performance metrics that arrive after the first paint', async () => {
    let resolveMetrics!: (value: unknown) => void
    vi.spyOn(api, 'get').mockImplementation((url: string) =>
      url === '/api/perf-metrics/summary'
        ? new Promise((resolve) => {
            resolveMetrics = resolve
          })
        : Promise.resolve({ data: { success: true } })
    )

    render(
      <QueryClientProvider client={queryClient}>
        <PricingTable models={[pricingModel()]} />
      </QueryClientProvider>
    )

    const perfBadge = () =>
      screen.getByLabelText('Performance metrics for the last 24 hours')
    expect(
      within(
        await screen.findByLabelText(
          'Performance metrics for the last 24 hours'
        )
      ).queryByText('99.50%')
    ).toBeNull()

    resolveMetrics({
      data: {
        success: true,
        data: {
          models: [
            {
              model_name: 'example-model',
              avg_latency_ms: 1200,
              avg_tps: 42,
              success_rate: 99.5,
            },
          ],
          recent_success_series: {},
          window_start: 0,
          window_end: 0,
        },
      },
    })

    await waitFor(() =>
      expect(within(perfBadge()).getByText('99.50%')).toBeVisible()
    )
    expect(within(perfBadge()).getByText('1.20s')).toBeVisible()
  })
})
