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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { updateSubscriptionRenewal } from '@/features/subscriptions/api'
import {
  subscriptionPlanSchema,
  type UserSubscription,
} from '@/features/subscriptions/types'

import { MySubscriptionItem } from '../my-subscription-item'
import { SubscriptionPlanTile } from '../subscription-plan-tile'

vi.mock('@/features/subscriptions/api', () => ({
  updateSubscriptionRenewal: vi.fn(async () => ({ success: true })),
}))

const pro = subscriptionPlanSchema.parse({
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
  usage_tiers: '[{"multiplier":5,"price_amount":40}]',
  allow_tier_switch: true,
  allow_auto_renew: true,
})

const activeSubscription: UserSubscription = {
  id: 9,
  user_id: 1,
  plan_id: 1,
  status: 'active',
  start_time: 0,
  end_time: Math.floor(Date.now() / 1000) + 86400 * 10,
  amount_total: 1000,
  amount_used: 250,
  multiplier: 1,
  auto_renew: false,
}

function renderTile(
  overrides: Partial<React.ComponentProps<typeof SubscriptionPlanTile>> = {}
) {
  const onSubscribe = vi.fn()
  const onUpgrade = vi.fn()
  render(
    <SubscriptionPlanTile
      plan={pro}
      purchaseCount={0}
      activeCount={0}
      onSubscribe={onSubscribe}
      onUpgrade={onUpgrade}
      {...overrides}
    />
  )
  return { onSubscribe, onUpgrade }
}

beforeEach(() => {
  vi.mocked(updateSubscriptionRenewal).mockClear()
})

describe('subscription plan tile', () => {
  test('switching the usage tier subscribes with the tier price and scaled quota', () => {
    const { onSubscribe } = renderTile()
    const tier5 = screen.getByRole('button', { name: '5x' })
    fireEvent.click(tier5)
    expect(tier5).toHaveAttribute('aria-pressed', 'true')
    fireEvent.click(screen.getByRole('button', { name: 'Subscribe Now' }))
    expect(onSubscribe).toHaveBeenCalledWith(
      expect.objectContaining({ id: 1, price_amount: 40, total_amount: 5000 }),
      5
    )
  })

  test('marks the subscribed tier as current and offers an upgrade to a higher tier', () => {
    const { onUpgrade } = renderTile({
      currentSubscription: activeSubscription,
    })
    expect(
      screen.getByRole('button', { name: 'Your current plan' })
    ).toBeDisabled()
    fireEvent.click(screen.getByRole('button', { name: '5x' }))
    fireEvent.click(screen.getByRole('button', { name: 'Upgrade to 5x' }))
    expect(onUpgrade).toHaveBeenCalledWith(activeSubscription, pro, 5)
  })

  test('offers a cross-plan upgrade from another active subscription', () => {
    const source = { ...activeSubscription, plan_id: 2 }
    const { onUpgrade } = renderTile({ upgradeSource: source })
    fireEvent.click(screen.getByRole('button', { name: 'Upgrade to Pro' }))
    expect(onUpgrade).toHaveBeenCalledWith(source, pro, 1)
  })
})

describe('my subscription item', () => {
  test('turns on auto-renewal for the current tier', async () => {
    const onChanged = vi.fn()
    render(
      <MySubscriptionItem
        subscription={activeSubscription}
        plan={pro}
        canUpgrade
        onUpgrade={vi.fn()}
        onChanged={onChanged}
      />
    )
    fireEvent.click(
      screen.getByRole('switch', { name: 'Auto-renew from balance' })
    )
    await waitFor(() =>
      expect(updateSubscriptionRenewal).toHaveBeenCalledWith(9, {
        auto_renew: true,
        renew_multiplier: 1,
      })
    )
    await waitFor(() => expect(onChanged).toHaveBeenCalled())
  })

  test('hides renewal controls when the plan does not allow auto-renewal', () => {
    render(
      <MySubscriptionItem
        subscription={activeSubscription}
        plan={{ ...pro, allow_auto_renew: false }}
        canUpgrade={false}
        onUpgrade={vi.fn()}
        onChanged={vi.fn()}
      />
    )
    expect(screen.queryByRole('switch')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Upgrade' })
    ).not.toBeInTheDocument()
  })
})
