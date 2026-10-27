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
import { render, screen } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SubscriptionSection } from '../subscription-section'

const plan = {
  id: 1,
  title: 'Pro',
  price_amount: 10,
  duration_unit: 'month',
  duration_value: 1,
  quota_reset_period: 'never',
  enabled: true,
  sort_order: 0,
  max_purchase_per_user: 0,
  total_amount: 1000,
}

function mockSubscriptionApi(plans: unknown[]) {
  vi.spyOn(api, 'get').mockImplementation(async (url) => ({
    data: {
      success: true,
      data:
        url === '/api/subscription/plans'
          ? plans
          : {
              billing_preference: 'subscription_first',
              subscriptions: [],
              all_subscriptions: [],
            },
    },
  }))
}

function renderSection() {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <SubscriptionSection
        topupInfo={null}
        recharge={<section aria-label='Recharge'>recharge</section>}
      />
    </QueryClientProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

describe('wallet subscription section layout', () => {
  test('puts recharge beside my subscriptions and the plans in their own row', async () => {
    mockSubscriptionApi([{ plan }])
    renderSection()
    const plansHeading = await screen.findByText('Subscription Plans')
    const recharge = screen.getByRole('region', { name: 'Recharge' })
    const topRow = recharge.parentElement
    expect(topRow).toContainElement(screen.getByText('My Subscriptions'))
    expect(topRow).not.toContainElement(plansHeading)
    expect(screen.getByText('Pro')).toBeInTheDocument()
  })

  test('keeps the recharge card mounted and alone when no plan is sold and the user has no subscription', async () => {
    mockSubscriptionApi([])
    renderSection()
    const recharge = screen.getByRole('region', { name: 'Recharge' })
    await vi.waitFor(() =>
      expect(api.get).toHaveBeenCalledWith('/api/subscription/self')
    )
    await vi.waitFor(() =>
      expect(recharge.parentElement?.children).toHaveLength(1)
    )
    // The card loaded while subscriptions were fetched stays mounted.
    expect(recharge).toBeInTheDocument()
    expect(screen.queryByText('My Subscriptions')).not.toBeInTheDocument()
    expect(screen.queryByText('Subscription Plans')).not.toBeInTheDocument()
  })
})
