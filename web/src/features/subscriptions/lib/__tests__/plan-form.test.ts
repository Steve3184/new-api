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

import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import { subscriptionPlanSchema } from '../../types'
import {
  formValuesToPlanPayload,
  PLAN_FORM_DEFAULTS,
  getPlanFormSchema,
  planToFormValues,
} from '../plan-form'

describe('subscription plan usage limit form mapping', () => {
  test('preserves independent lifetime and simultaneous limits when creating and editing', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      max_purchase_per_user: 10,
      max_active_per_user: 1,
    })
    expect(payload.plan).toMatchObject({
      max_purchase_per_user: 10,
      max_active_per_user: 1,
    })
    const plan = subscriptionPlanSchema.parse({ ...payload.plan, id: 1 })
    expect(planToFormValues(plan).max_active_per_user).toBe(1)
    expect(
      formValuesToPlanPayload({
        ...planToFormValues(plan),
        max_active_per_user: 0,
      }).plan.max_active_per_user
    ).toBe(0)
  })

  test('rejects negative and fractional simultaneous limits', () => {
    const schema = getPlanFormSchema(i18next.t)
    expect(
      schema.safeParse({
        ...PLAN_FORM_DEFAULTS,
        title: 'Plan',
        max_active_per_user: -1,
      }).success
    ).toBe(false)
    expect(
      schema.safeParse({
        ...PLAN_FORM_DEFAULTS,
        title: 'Plan',
        max_active_per_user: 1.5,
      }).success
    ).toBe(false)
  })
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

describe('subscription plan billing mode form mapping', () => {
  test('keeps a legacy plan unbound and preserves its original group rules', () => {
    const legacy = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 1,
      upgrade_group: 'vip',
      downgrade_group: 'default',
      wallet_only_groups_enabled: true,
      wallet_only_groups: 'team',
    })
    expect(
      formValuesToPlanPayload(planToFormValues(legacy)).plan
    ).toMatchObject({
      subscription_group: '',
      upgrade_group: 'vip',
      downgrade_group: 'default',
      wallet_only_groups_enabled: true,
      wallet_only_groups: 'team',
    })
  })

  test('preserves the explicit binding when editing a scoped plan', () => {
    const scoped = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 2,
      subscription_group: 'subscriber',
    })
    const values = planToFormValues(scoped)
    expect(values.subscription_group).toBe('subscriber')
    expect(formValuesToPlanPayload(values).plan.subscription_group).toBe(
      'subscriber'
    )
  })
})

describe('subscription plan automatic API key form mapping', () => {
  test('defaults a new plan to not creating an API key', () => {
    expect(PLAN_FORM_DEFAULTS.auto_create_key).toBe(false)
    expect(
      formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan.auto_create_key
    ).toBe(false)
  })

  test('treats a persisted plan without the field as not creating an API key', () => {
    const plan = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 3,
      subscription_group: 'subscriber',
      auto_create_key: undefined,
    })
    expect(planToFormValues(plan).auto_create_key).toBe(false)
  })

  test('round-trips automatic API key creation for a bound plan', () => {
    const plan = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 4,
      subscription_group: 'subscriber',
      auto_create_key: true,
    })
    const values = planToFormValues(plan)
    expect(values.auto_create_key).toBe(true)
    expect(formValuesToPlanPayload(values).plan).toMatchObject({
      subscription_group: 'subscriber',
      auto_create_key: true,
    })
  })

  test('sends false when a plan is unbound even if the switch was on', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      subscription_group: '',
      auto_create_key: true,
    })
    expect(payload.plan).toMatchObject({
      subscription_group: '',
      auto_create_key: false,
    })
  })
})

describe('subscription plan tiers form mapping', () => {
  test('round-trips usage tiers, upgrade targets and renewal settings', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      usage_tiers: [
        { multiplier: 5, price_amount: 40, stripe_price_id: ' price_5 ' },
        { multiplier: 2, price_amount: 18, stripe_price_id: '' },
      ],
      allow_tier_switch: true,
      upgrade_plan_ids: ['3', '4'],
      allow_auto_renew: true,
    })
    expect(JSON.parse(payload.plan.usage_tiers || '[]')).toEqual([
      { multiplier: 2, price_amount: 18 },
      { multiplier: 5, price_amount: 40, stripe_price_id: 'price_5' },
    ])
    expect(payload.plan).toMatchObject({
      allow_tier_switch: true,
      upgrade_plan_ids: '3,4',
      allow_auto_renew: true,
    })
    const values = planToFormValues(
      subscriptionPlanSchema.parse({ ...payload.plan, id: 1 })
    )
    expect(values.usage_tiers.map((tier) => tier.multiplier)).toEqual([2, 5])
    expect(values.upgrade_plan_ids).toEqual(['3', '4'])
  })

  test('drops auto-renewal when balance payment is disabled', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      allow_balance_pay: false,
      allow_auto_renew: true,
    })
    expect(payload.plan.allow_auto_renew).toBe(false)
  })

  test('rejects duplicate and out-of-range multipliers', () => {
    const schema = getPlanFormSchema(i18next.t.bind(i18next))
    const base = { ...PLAN_FORM_DEFAULTS, title: 'Plan' }
    expect(
      schema.safeParse({
        ...base,
        usage_tiers: [
          { multiplier: 2, price_amount: 1 },
          { multiplier: 2, price_amount: 2 },
        ],
      }).success
    ).toBe(false)
    expect(
      schema.safeParse({
        ...base,
        usage_tiers: [{ multiplier: 1, price_amount: 1 }],
      }).success
    ).toBe(false)
    expect(
      schema.safeParse({
        ...base,
        usage_tiers: [{ multiplier: 5, price_amount: 1 }],
      }).success
    ).toBe(true)
  })
})

describe('subscription plan extra credits form mapping', () => {
  test('defaults extra credits to disabled with no rate text', () => {
    expect(PLAN_FORM_DEFAULTS.allow_extra_credits).toBe(false)
    expect(PLAN_FORM_DEFAULTS.extra_credits_rate).toBe('')
    expect(formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan).toMatchObject({
      allow_extra_credits: false,
      extra_credits_rate: 0,
    })
  })

  test('keeps a legacy plan without the fields disabled when it is edited', () => {
    const legacy = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 3,
      title: 'Legacy',
    })
    expect(legacy.allow_extra_credits).toBe(false)
    expect(legacy.extra_credits_rate).toBe(0)
    const values = planToFormValues(legacy)
    expect(values.allow_extra_credits).toBe(false)
    expect(formValuesToPlanPayload(values).plan).toMatchObject({
      allow_extra_credits: false,
      extra_credits_rate: 0,
    })
  })

  test('round-trips an enabled plan with its rate', () => {
    const plan = subscriptionPlanSchema.parse({
      ...formValuesToPlanPayload(PLAN_FORM_DEFAULTS).plan,
      id: 4,
      title: 'Pro',
      allow_extra_credits: true,
      extra_credits_rate: 2.5,
    })
    const values = planToFormValues(plan)
    expect(values.allow_extra_credits).toBe(true)
    expect(values.extra_credits_rate).toBe('2.5')
    expect(formValuesToPlanPayload(values).plan).toMatchObject({
      allow_extra_credits: true,
      extra_credits_rate: 2.5,
    })
  })

  test('never submits a rate for a disabled plan', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      allow_extra_credits: false,
      extra_credits_rate: '5',
    })
    expect(payload.plan).toMatchObject({
      allow_extra_credits: false,
      extra_credits_rate: 0,
    })
  })

  test('submits the exact typed rate instead of a float-parsed approximation', () => {
    const payload = formValuesToPlanPayload({
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      allow_extra_credits: true,
      extra_credits_rate: '0.000001',
    })
    expect(payload.plan.extra_credits_rate).toBe(0.000001)
  })

  test('requires a rate within the backend range only when enabled', () => {
    const schema = getPlanFormSchema(i18next.t)
    const base = { ...PLAN_FORM_DEFAULTS, title: 'Plan' }
    const accepts = (rate: string) =>
      schema.safeParse({
        ...base,
        allow_extra_credits: true,
        extra_credits_rate: rate,
      }).success
    // A disabled plan needs no rate at all.
    expect(
      schema.safeParse({
        ...base,
        allow_extra_credits: false,
        extra_credits_rate: '',
      }).success
    ).toBe(true)
    expect(accepts('')).toBe(false)
    expect(accepts('0')).toBe(false)
    expect(accepts('1000001')).toBe(false)
    expect(accepts('2.5')).toBe(true)
  })

  test('accepts the backend rate bounds and rejects more than six decimals', () => {
    const schema = getPlanFormSchema(i18next.t)
    const base = {
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      allow_extra_credits: true,
    }
    const accepts = (rate: string) =>
      schema.safeParse({ ...base, extra_credits_rate: rate }).success
    expect(accepts('0.000001')).toBe(true)
    expect(accepts('1000000')).toBe(true)
    expect(accepts('1.123456')).toBe(true)
    expect(accepts('0.0000009')).toBe(false)
    expect(accepts('1.1234567')).toBe(false)
  })

  test('rejects pasted text whose extra decimals only disappear when parsed as a float', () => {
    const schema = getPlanFormSchema(i18next.t)
    const parsed = schema.safeParse({
      ...PLAN_FORM_DEFAULTS,
      title: 'Plan',
      allow_extra_credits: true,
      // Number.parseFloat maps this to exactly 1000000, inside the bounds.
      extra_credits_rate: '1000000.00000000001',
    })
    expect(parsed.success).toBe(false)
    expect(
      parsed.error?.issues.map((issue) => [issue.path.join('.'), issue.message])
    ).toEqual([['extra_credits_rate', 'Invalid extra credits exchange rate']])
  })
})
