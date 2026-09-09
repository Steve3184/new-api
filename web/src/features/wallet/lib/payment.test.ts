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

import { PAYMENT_TYPES } from '../constants'
import {
  dispatchSelectedPayment,
  getMinTopupAmount,
  getTopupPaymentMethods,
  isNowPaymentsPayment,
  isStripePayment,
  isWaffoPayment,
  isWaffoPancakePayment,
} from './payment'

describe('payment type classification', () => {
  test('keeps Waffo and Waffo Pancake on their dedicated flows', () => {
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO)).toBe(true)
    expect(isWaffoPayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(false)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO_PANCAKE)).toBe(true)
    expect(isWaffoPancakePayment(PAYMENT_TYPES.WAFFO)).toBe(false)
    expect(isStripePayment(PAYMENT_TYPES.STRIPE)).toBe(true)
    expect(isNowPaymentsPayment(PAYMENT_TYPES.NOWPAYMENTS)).toBe(true)
    expect(isNowPaymentsPayment(PAYMENT_TYPES.MONERO)).toBe(false)
  })

  test('uses the configured NOWPayments minimum for wallet presets', () => {
    expect(
      getMinTopupAmount({
        enable_online_topup: false,
        enable_stripe_topup: false,
        pay_methods: [],
        min_topup: 1,
        stripe_min_topup: 1,
        amount_options: [],
        discount: {},
        enable_nowpayments_topup: true,
        nowpayments_min_topup: 25,
      })
    ).toBe(25)
  })

  test('adds NOWPayments to the available payment methods when currencies are configured', () => {
    const methods = getTopupPaymentMethods(
      {
        enable_online_topup: true,
        enable_stripe_topup: false,
        pay_methods: [{ name: 'Alipay', type: PAYMENT_TYPES.ALIPAY }],
        min_topup: 1,
        stripe_min_topup: 1,
        amount_options: [],
        discount: {},
      },
      true,
      ['btc', 'usdtbsc']
    )

    expect(methods.map((method) => method.type)).toEqual([
      PAYMENT_TYPES.ALIPAY,
      PAYMENT_TYPES.NOWPAYMENTS,
    ])
  })

  test('does not add NOWPayments without an enabled currency', () => {
    const methods = getTopupPaymentMethods(null, true, [])
    expect(methods).toEqual([])
  })

  test('does not duplicate an existing NOWPayments method', () => {
    const methods = getTopupPaymentMethods(
      {
        enable_online_topup: true,
        enable_stripe_topup: false,
        pay_methods: [{ name: 'NOWPayments', type: PAYMENT_TYPES.NOWPAYMENTS }],
        min_topup: 1,
        stripe_min_topup: 1,
        amount_options: [],
        discount: {},
      },
      true,
      ['btc']
    )

    expect(
      methods.filter((method) => method.type === PAYMENT_TYPES.NOWPAYMENTS)
    ).toHaveLength(1)
  })
})

describe('payment dispatch', () => {
  test('keeps the selected Waffo method index through confirmation', async () => {
    const calls: string[] = []
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      3,
      {
        regular: async () => {
          calls.push('regular')
          return false
        },
        waffo: async (amount, index) => {
          calls.push(`waffo:${amount}:${index}`)
          return true
        },
        waffoPancake: async () => {
          calls.push('pancake')
          return false
        },
      }
    )

    expect(success).toBe(true)
    expect(calls).toEqual(['waffo:120:3'])
  })

  test('does not create a Waffo order without a selected method index', async () => {
    let called = false
    const success = await dispatchSelectedPayment(
      { name: 'Waffo Card', type: PAYMENT_TYPES.WAFFO },
      120,
      null,
      {
        regular: async () => false,
        waffo: async () => {
          called = true
          return true
        },
        waffoPancake: async () => false,
      }
    )

    expect(success).toBe(false)
    expect(called).toBe(false)
  })
})
