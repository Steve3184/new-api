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

import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import { subscriptionPlanSchema } from '../../types'
import {
  formValuesToPlanPayload,
  PLAN_FORM_DEFAULTS,
  planToFormValues,
} from '../plan-form'

describe('subscription plan usage limit form mapping', () => {
  test('converts independent usage limits to quota units for the plan payload', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      five_hour_limit: 1.25,
      weekly_limit: 2.5,
      monthly_limit: 5,
    })

    expect(payload.plan).toMatchObject({
      five_hour_limit: parseQuotaFromDollars(1.25),
      weekly_limit: parseQuotaFromDollars(2.5),
      monthly_limit: parseQuotaFromDollars(5),
    })
  })

  test('removes usage limits from benefits-only plans', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      benefits_only: true,
      total_amount: 10,
      five_hour_limit: 1,
      weekly_limit: 2,
      monthly_limit: 3,
    })

    expect(payload.plan).toMatchObject({
      total_amount: -1,
      five_hour_limit: 0,
      weekly_limit: 0,
      monthly_limit: 0,
    })
  })

  test('restores persisted usage limits to editable display amounts', () => {
    const plan = subscriptionPlanSchema.parse({
      id: 1,
      title: 'Usage limits',
      price_amount: 0,
      duration_unit: 'month',
      duration_value: 1,
      quota_reset_period: 'never',
      enabled: true,
      sort_order: 0,
      max_purchase_per_user: 0,
      total_amount: 0,
      five_hour_limit: 125000,
      weekly_limit: 250000,
      monthly_limit: 500000,
    })

    const values = planToFormValues(plan)

    expect(values.five_hour_limit).toBe(quotaUnitsToDollars(125000))
    expect(values.weekly_limit).toBe(quotaUnitsToDollars(250000))
    expect(values.monthly_limit).toBe(quotaUnitsToDollars(500000))
  })
})
