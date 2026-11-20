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
import { afterEach, describe, expect, test } from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { subscriptionPlanSchema, type UserSubscription } from '../../types'
import {
  canBuyExtraCredits,
  convertExtraCreditsAmount,
  hasExtraCredits,
  isValidExtraCreditsRateText,
  MAX_EXTRA_CREDITS_RATE,
  MAX_EXTRA_CREDITS_WALLET_QUOTA,
  MIN_EXTRA_CREDITS_RATE,
  parseExtraCreditsRateText,
  planAllowsExtraCredits,
  remainingExtraCredits,
} from '../extra-credits'

const initialCurrency = useSystemConfigStore.getState().config.currency

// quotaPerUnit=500000 means $1 of display amount is 500000 raw quota units.
function setUsdDisplay() {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...state.config.currency,
        quotaDisplayType: 'USD',
        quotaPerUnit: 500000,
        usdExchangeRate: 1,
      },
    },
  }))
}

function makePlan(
  overrides: Partial<ReturnType<typeof subscriptionPlanSchema.parse>>
) {
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
    total_amount: 0,
    ...overrides,
  })
}

function makeSub(overrides: Partial<UserSubscription>): UserSubscription {
  return {
    id: 1,
    user_id: 1,
    plan_id: 1,
    status: 'active',
    start_time: 0,
    end_time: Math.floor(Date.now() / 1000) + 86400,
    amount_total: 0,
    amount_used: 0,
    ...overrides,
  }
}

afterEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...initialCurrency } },
  }))
})

describe('plan extra-credits defaults and eligibility', () => {
  test('a plan parsed without the fields defaults extra credits off at a zero rate', () => {
    const plan = makePlan({})
    expect(plan.allow_extra_credits).toBe(false)
    expect(plan.extra_credits_rate).toBe(0)
    expect(planAllowsExtraCredits(plan)).toBe(false)
  })

  test('a plan needs both the toggle on and a positive finite rate to sell extra credits', () => {
    expect(
      planAllowsExtraCredits(
        makePlan({ allow_extra_credits: true, extra_credits_rate: 2 })
      )
    ).toBe(true)
    expect(
      planAllowsExtraCredits(
        makePlan({ allow_extra_credits: true, extra_credits_rate: 0 })
      )
    ).toBe(false)
    expect(
      planAllowsExtraCredits(
        makePlan({ allow_extra_credits: false, extra_credits_rate: 2 })
      )
    ).toBe(false)
  })
})

describe('buying extra credits is allowed only for open subscriptions', () => {
  const now = Math.floor(Date.now() / 1000)
  const plan = makePlan({ allow_extra_credits: true, extra_credits_rate: 2 })

  test('allows an active subscription on an enabled plan', () => {
    expect(
      canBuyExtraCredits(
        makeSub({ status: 'active', end_time: now + 1000 }),
        plan,
        now
      )
    ).toBe(true)
  })

  test('refuses a first purchase on an expired subscription with no prior extras', () => {
    expect(
      canBuyExtraCredits(
        makeSub({ status: 'expired', end_time: now - 1000 }),
        plan,
        now
      )
    ).toBe(false)
    // An active row whose end_time has passed is expired too.
    expect(
      canBuyExtraCredits(
        makeSub({ status: 'active', end_time: now - 1000 }),
        plan,
        now
      )
    ).toBe(false)
    expect(
      canBuyExtraCredits(
        makeSub({ status: 'expired', end_time: now + 1000 }),
        plan,
        now
      )
    ).toBe(false)
  })

  test('allows topping up an expired subscription whose extras were already purchased', () => {
    expect(
      canBuyExtraCredits(
        makeSub({
          status: 'expired',
          end_time: now - 1000,
          extra_credits_total: 1000,
          extra_credits_used: 1000,
        }),
        plan,
        now
      )
    ).toBe(true)
    expect(
      canBuyExtraCredits(
        makeSub({
          status: 'active',
          end_time: now - 1000,
          extra_credits_total: 1000,
        }),
        plan,
        now
      )
    ).toBe(true)
  })

  test('refuses cancelled and upgraded subscriptions', () => {
    expect(
      canBuyExtraCredits(
        makeSub({ status: 'cancelled', extra_credits_total: 1000 }),
        plan,
        now
      )
    ).toBe(false)
    expect(
      canBuyExtraCredits(
        makeSub({
          status: 'upgraded',
          end_time: now - 1000,
          extra_credits_total: 1000,
        }),
        plan,
        now
      )
    ).toBe(false)
  })

  test('refuses any subscription when the plan is missing or does not sell extra credits', () => {
    expect(canBuyExtraCredits(makeSub({}), undefined, now)).toBe(false)
    expect(
      canBuyExtraCredits(
        makeSub({}),
        makePlan({ allow_extra_credits: false }),
        now
      )
    ).toBe(false)
  })
})

describe('remaining extra credits', () => {
  test('is total minus used and never negative', () => {
    expect(
      remainingExtraCredits({
        extra_credits_total: 1000,
        extra_credits_used: 250,
      })
    ).toBe(750)
    expect(
      remainingExtraCredits({
        extra_credits_total: 1000,
        extra_credits_used: 4000,
      })
    ).toBe(0)
  })

  test('hasExtraCredits is true once any were ever purchased, even if fully spent', () => {
    expect(
      hasExtraCredits({ extra_credits_total: 1000, extra_credits_used: 1000 })
    ).toBe(true)
    expect(
      hasExtraCredits({ extra_credits_total: 0, extra_credits_used: 0 })
    ).toBe(false)
  })
})

describe('validating an exchange rate from its exact decimal text', () => {
  test('rejects a pasted rate whose excess decimals only vanish after float parsing', () => {
    // Number.parseFloat maps this to exactly 1000000, so a validator that only
    // sees the parsed number cannot tell it apart from the valid ceiling.
    expect(Number.parseFloat('1000000.00000000001')).toBe(
      MAX_EXTRA_CREDITS_RATE
    )
    expect(parseExtraCreditsRateText('1000000.00000000001')).toBeNull()
    expect(isValidExtraCreditsRateText('1000000.00000000001')).toBe(false)
    expect(parseExtraCreditsRateText('1000000.000001')).toBeNull()
  })

  test('rejects the raw rate length and exponent outside the backend limits', () => {
    expect(parseExtraCreditsRateText(`1${'0'.repeat(59)}e-65`)).toBeNull()
    expect(parseExtraCreditsRateText(`1.${'0'.repeat(63)}`)).toBeNull()
  })

  test('accepts the backend bounds and rejects anything finer than six decimals', () => {
    expect(parseExtraCreditsRateText('0.000001')).toBe(MIN_EXTRA_CREDITS_RATE)
    expect(parseExtraCreditsRateText('1000000')).toBe(MAX_EXTRA_CREDITS_RATE)
    expect(parseExtraCreditsRateText('1.123456')).toBe(1.123456)
    // Match decimal.Exponent(): explicit 65 minus 59 fraction digits is 6.
    expect(parseExtraCreditsRateText(`0.${'0'.repeat(58)}1e65`)).toBe(1_000_000)
    expect(parseExtraCreditsRateText('2.5')).toBe(2.5)
    // Below the floor, above the ceiling, or finer than the decimal(18,6) column.
    expect(parseExtraCreditsRateText('0.0000009')).toBeNull()
    expect(parseExtraCreditsRateText('1000000.5')).toBeNull()
    expect(parseExtraCreditsRateText('1.1234567')).toBeNull()
    expect(parseExtraCreditsRateText('-2')).toBeNull()
    expect(parseExtraCreditsRateText('0')).toBeNull()
  })

  test('rejects text that is not a plain finite decimal rate', () => {
    expect(parseExtraCreditsRateText('')).toBeNull()
    expect(parseExtraCreditsRateText('   ')).toBeNull()
    expect(parseExtraCreditsRateText('abc')).toBeNull()
    expect(parseExtraCreditsRateText('2,5')).toBeNull()
    expect(parseExtraCreditsRateText('Infinity')).toBeNull()
    expect(parseExtraCreditsRateText('NaN')).toBeNull()
    expect(parseExtraCreditsRateText('0x10')).toBeNull()
    expect(parseExtraCreditsRateText(undefined)).toBeNull()
    expect(parseExtraCreditsRateText(2.5)).toBeNull()
  })
})

describe('converting an entered display amount to a safe purchase', () => {
  test('rejects fractional raw quota instead of rounding it to another charge', () => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: { ...state.config.currency, quotaDisplayType: 'TOKENS' },
      },
    }))
    expect(convertExtraCreditsAmount('1.5', 1)).toBeNull()
    expect(convertExtraCreditsAmount('2', 1)?.walletQuota).toBe(2)
  })
  test('keeps large decimal inputs distinct at raw quota precision', () => {
    setUsdDisplay()
    expect(
      convertExtraCreditsAmount('18000000000.000002', 1)?.walletQuota
    ).toBe(9000000000000001)
    expect(
      convertExtraCreditsAmount('18000000000.000004', 1)?.walletQuota
    ).toBe(9000000000000002)
  })
  test('converts dollars to raw wallet quota and floors granted credits by the rate', () => {
    setUsdDisplay()
    // $2 -> 1,000,000 wallet quota; rate 2.5 -> 2,500,000 credits.
    expect(convertExtraCreditsAmount(2, 2.5)).toEqual({
      walletQuota: 1_000_000,
      credits: 2_500_000,
    })
  })

  test('floors granted credits rather than rounding up', () => {
    setUsdDisplay()
    // 1 wallet quota * 0.3 = 0.3 -> floored to 0 -> rejected as too small.
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: { ...state.config.currency, quotaPerUnit: 3 },
      },
    }))
    // $1 -> 3 wallet quota; rate 1.5 -> 4.5 -> floored to 4 credits.
    expect(convertExtraCreditsAmount(1, 1.5)).toEqual({
      walletQuota: 3,
      credits: 4,
    })
  })

  test('rejects invalid, non-positive, or absurd amounts and rates', () => {
    setUsdDisplay()
    expect(convertExtraCreditsAmount(0, 2)).toBeNull()
    expect(convertExtraCreditsAmount(-5, 2)).toBeNull()
    expect(convertExtraCreditsAmount(Number.NaN, 2)).toBeNull()
    expect(convertExtraCreditsAmount(5, 0)).toBeNull()
    expect(convertExtraCreditsAmount(5, Number.POSITIVE_INFINITY)).toBeNull()
  })

  test('rejects an amount so tiny it rounds to zero wallet quota', () => {
    setUsdDisplay()
    // $0.0000005 * 500000 = 0.25 -> rounds to 0 wallet quota -> rejected.
    expect(convertExtraCreditsAmount(0.0000005, 2)).toBeNull()
  })

  test('rejects a wallet quota above the backend ceiling', () => {
    setUsdDisplay()
    const overMax = (MAX_EXTRA_CREDITS_WALLET_QUOTA + 1_000_000) / 500000
    expect(convertExtraCreditsAmount(overMax, 1)).toBeNull()
  })
})
