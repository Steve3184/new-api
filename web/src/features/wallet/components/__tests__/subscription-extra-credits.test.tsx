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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import {
  savePendingExtraCreditsPurchase,
  resolvePendingExtraCreditsPurchase,
} from '@/features/subscriptions/lib/extra-credits-request'
import { subscriptionPlanSchema } from '@/features/subscriptions/types'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { MySubscriptionItem } from '../my-subscription-item'

const initialCurrency = useSystemConfigStore.getState().config.currency

const plan = subscriptionPlanSchema.parse({
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
  allow_extra_credits: true,
  extra_credits_rate: 2,
})

const now = () => Math.floor(Date.now() / 1000)

function renderItem(
  overrides: Partial<React.ComponentProps<typeof MySubscriptionItem>> = {}
) {
  const onBuyExtraCredits = vi.fn()
  render(
    <MySubscriptionItem
      subscription={{
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'active',
        start_time: 0,
        end_time: now() + 86400,
        amount_total: 1000,
        amount_used: 0,
        extra_credits_total: 0,
        extra_credits_used: 0,
      }}
      plan={plan}
      canUpgrade={false}
      onUpgrade={vi.fn()}
      onChanged={vi.fn()}
      onBuyExtraCredits={onBuyExtraCredits}
      {...overrides}
    />
  )
  return { onBuyExtraCredits }
}

afterEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...initialCurrency } },
  }))
})

describe('buy extra credits button placement', () => {
  test('updates the recovery entry when another tab saves and resolves a purchase', async () => {
    window.localStorage.clear()
    useAuthStore.setState((state) => ({
      auth: { ...state.auth, user: { id: 1, username: 'user1', role: 1 } },
    }))
    Object.defineProperty(navigator, 'locks', {
      configurable: true,
      value: {
        request: async (
          _name: string,
          _options: unknown,
          callback: () => Promise<void>
        ) => callback(),
      },
    })
    renderItem({ plan: { ...plan, allow_extra_credits: false } })
    expect(
      screen.queryByRole('button', { name: 'Buy Extra Credits' })
    ).not.toBeInTheDocument()
    const body = {
      request_id: 'another-tab-recovery-entry',
      wallet_quota: 1_000_000,
      expected_rate: 2,
    }
    act(() => {
      savePendingExtraCreditsPurchase(1, 9, body)
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'new-api:subscription-extra-credits-pending',
        })
      )
    })
    expect(
      await screen.findByRole('button', { name: 'Buy Extra Credits' })
    ).toBeVisible()
    act(() => {
      resolvePendingExtraCreditsPurchase(1, 9, body)
      window.dispatchEvent(
        new StorageEvent('storage', {
          key: 'new-api:subscription-extra-credits-pending',
        })
      )
    })
    await waitFor(() =>
      expect(
        screen.queryByRole('button', { name: 'Buy Extra Credits' })
      ).not.toBeInTheDocument()
    )
  })

  test('sits beside Upgrade for an active subscription on an enabled plan', () => {
    const { onBuyExtraCredits } = renderItem({ canUpgrade: true })
    const buy = screen.getByRole('button', { name: 'Buy Extra Credits' })
    const upgrade = screen.getByRole('button', { name: 'Upgrade' })
    // Adjacent in the same footer row.
    expect(buy.parentElement).toBe(upgrade.parentElement)
    fireEvent.click(buy)
    expect(onBuyExtraCredits).toHaveBeenCalledTimes(1)
  })

  test('is not offered as a first purchase on an expired subscription', () => {
    renderItem({
      subscription: {
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'expired',
        start_time: 0,
        end_time: now() - 86400,
        amount_total: 1000,
        amount_used: 100,
        extra_credits_total: 0,
        extra_credits_used: 0,
      },
    })
    expect(
      screen.queryByRole('button', { name: 'Buy Extra Credits' })
    ).not.toBeInTheDocument()
  })

  test('is not offered to top up an expired subscription that still holds extras', () => {
    renderItem({
      canUpgrade: false,
      subscription: {
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'expired',
        start_time: 0,
        end_time: now() - 86400,
        amount_total: 1000,
        amount_used: 100,
        extra_credits_total: 1_000_000,
        extra_credits_used: 1_000_000,
      },
    })
    expect(
      screen.queryByRole('button', { name: 'Buy Extra Credits' })
    ).not.toBeInTheDocument()
  })

  test('is never offered for a cancelled subscription', () => {
    renderItem({
      subscription: {
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'cancelled',
        start_time: 0,
        end_time: now() - 86400,
        amount_total: 1000,
        amount_used: 0,
        extra_credits_total: 0,
        extra_credits_used: 0,
      },
    })
    expect(
      screen.queryByRole('button', { name: 'Buy Extra Credits' })
    ).not.toBeInTheDocument()
  })

  test('is hidden when the plan does not sell extra credits', () => {
    renderItem({ plan: { ...plan, allow_extra_credits: false } })
    expect(
      screen.queryByRole('button', { name: 'Buy Extra Credits' })
    ).not.toBeInTheDocument()
  })
})

describe('remaining extra credits on a subscription row', () => {
  test('shows the unspent extra credits on an active subscription', () => {
    renderItem({
      subscription: {
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'active',
        start_time: 0,
        end_time: now() + 86400,
        amount_total: 1000,
        amount_used: 0,
        extra_credits_total: 2_000_000,
        extra_credits_used: 500_000,
      },
    })
    // 1,500,000 quota units at 500000/unit is $3.
    expect(screen.getByText('Extra credits: $3')).toBeInTheDocument()
  })

  test('keeps showing the paid extras after the subscription expires', () => {
    renderItem({
      subscription: {
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'expired',
        start_time: 0,
        end_time: now() - 86400,
        amount_total: 1000,
        amount_used: 0,
        extra_credits_total: 2_000_000,
        extra_credits_used: 0,
      },
    })
    expect(screen.getByText('Extra credits: $4')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Unused extra credits are refunded to your wallet after expiry'
      )
    ).toBeVisible()
  })

  test('still shows the paid extras on an upgraded subscription without a buy button', () => {
    renderItem({
      subscription: {
        id: 9,
        user_id: 1,
        plan_id: 1,
        status: 'upgraded',
        start_time: 0,
        end_time: now() - 86400,
        amount_total: 1000,
        amount_used: 0,
        extra_credits_total: 1_000_000,
        extra_credits_used: 250_000,
      },
    })
    expect(screen.getByText('Extra credits: $1.5')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Buy Extra Credits' })
    ).not.toBeInTheDocument()
  })

  test('omits the extra credits line entirely when none were ever bought', () => {
    renderItem()
    expect(screen.queryByText(/Extra credits:/)).not.toBeInTheDocument()
  })
})
