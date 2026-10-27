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
import i18next from 'i18next'
import { describe, expect, test } from 'vitest'

import { subscriptionPlanSchema } from '../../types'
import { describePlan } from '../plan-benefits'

const t = i18next.t.bind(i18next)

function plan(overrides: Record<string, unknown> = {}) {
  return subscriptionPlanSchema.parse({
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
    ...overrides,
  })
}

describe('plan description', () => {
  test('lists quota, caps, group, rate limits and renewal in buying order', () => {
    const description = describePlan(
      plan({
        quota_reset_period: 'weekly',
        five_hour_limit: 10,
        monthly_limit: 500,
        upgrade_group: 'vip',
        rate_limit_groups: '[{"group":"vip","rpm":60}]',
        allow_auto_renew: true,
      }),
      t
    )
    expect(description.features.map((feature) => feature.key)).toEqual([
      'quota',
      'five-hour',
      'monthly',
      'group',
      'rpm-vip',
      'overflow',
      'auto-renew',
    ])
    expect(description.features[0].label).toMatch(/refreshed every week$/)
    expect(description.features[3].label).toBe('Access to the vip group')
    expect(description.features[4].label).toBe('60 requests per minute in vip')
  })

  test('describes unlimited and benefits-only plans without quota caps or wallet fallback', () => {
    expect(
      describePlan(plan({ total_amount: 0 }), t).features.map((f) => f.label)
    ).toEqual(['Unlimited usage during the plan'])
    const benefitsOnly = describePlan(
      plan({ total_amount: -1, weekly_limit: 100 }),
      t
    )
    expect(benefitsOnly.features.map((f) => f.key)).toEqual(['quota'])
  })

  test('hides auto-renewal when balance payment is disabled', () => {
    const description = describePlan(
      plan({ allow_auto_renew: true, allow_balance_pay: false }),
      t
    )
    expect(description.features.some((f) => f.key === 'auto-renew')).toBe(false)
  })

  test('moves purchase rules and the expiry group into notes', () => {
    const description = describePlan(
      plan({
        max_purchase_per_user: 2,
        max_active_per_user: 1,
        downgrade_group: 'free',
      }),
      t
    )
    expect(description.notes).toEqual([
      'Limited to 2 purchases per account',
      'Up to 1 active at the same time',
      'Your group changes to free when the plan ends',
    ])
  })
})
