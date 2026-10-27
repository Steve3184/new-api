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
import { describe, expect, test } from 'vitest'

import { subscriptionPlanSchema } from '../../types'
import {
  applyUsageTier,
  getUpgradeOptions,
  parseUsageTiers,
} from '../usage-tiers'

const basePlan = {
  title: 'Pro',
  price_amount: 10,
  duration_unit: 'month',
  duration_value: 1,
  quota_reset_period: 'never',
  enabled: true,
  sort_order: 0,
  max_purchase_per_user: 0,
  total_amount: 1000,
  five_hour_limit: 0,
  weekly_limit: 300,
}

const pro = subscriptionPlanSchema.parse({
  ...basePlan,
  id: 1,
  allow_tier_switch: true,
  usage_tiers:
    '[{"multiplier":5,"price_amount":40,"stripe_price_id":"price_5"},{"multiplier":2,"price_amount":18}]',
  upgrade_plan_ids: '2,3,1',
})
const max = subscriptionPlanSchema.parse({
  ...basePlan,
  id: 2,
  title: 'Max',
  price_amount: 30,
})
const walletOnly = subscriptionPlanSchema.parse({
  ...basePlan,
  id: 3,
  allow_balance_pay: false,
})

const subscription = {
  id: 7,
  user_id: 1,
  plan_id: 1,
  status: 'active',
  start_time: 0,
  end_time: 4102444800,
  amount_total: 2000,
  amount_used: 0,
  multiplier: 2,
}

describe('subscription usage tiers', () => {
  test('sorts stored tiers and ignores malformed data', () => {
    expect(
      parseUsageTiers(pro.usage_tiers).map((tier) => tier.multiplier)
    ).toEqual([2, 5])
    expect(parseUsageTiers('not json')).toEqual([])
    expect(parseUsageTiers('[{"multiplier":1,"price_amount":5}]')).toEqual([])
  })

  test('scales every positive limit and uses the tier price and product id', () => {
    const view = applyUsageTier(pro, 5)
    expect(view).toMatchObject({
      price_amount: 40,
      stripe_price_id: 'price_5',
      total_amount: 5000,
      weekly_limit: 1500,
      five_hour_limit: 0,
    })
  })

  test('keeps unlimited and benefits-only quotas and falls back to 1x for unknown tiers', () => {
    expect(applyUsageTier({ ...pro, total_amount: 0 }, 5).total_amount).toBe(0)
    expect(applyUsageTier({ ...pro, total_amount: -1 }, 5).total_amount).toBe(
      -1
    )
    expect(applyUsageTier(pro, 3)).toMatchObject({
      price_amount: 10,
      total_amount: 1000,
    })
  })

  test('offers higher tiers of the own plan and every tier of balance-payable upgrade plans', () => {
    const options = getUpgradeOptions(subscription, [pro, max, walletOnly])
    expect(
      options.map((option) => `${option.plan.id}:${option.multiplier}`)
    ).toEqual(['1:5', '2:1'])
  })

  test('offers no tier switch when the plan does not allow it', () => {
    const options = getUpgradeOptions(subscription, [
      { ...pro, allow_tier_switch: false },
      max,
    ])
    expect(options.map((option) => option.plan.id)).toEqual([2])
  })
})
