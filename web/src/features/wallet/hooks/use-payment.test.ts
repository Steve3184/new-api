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
import { requestPaymentAmount } from './use-payment'
import { normalizeTopupPaymentMethods } from './use-topup-info'

describe('payment amount routing', () => {
  test('falls back to the upstream method type when no custom name is set', () => {
    expect(
      normalizeTopupPaymentMethods(
        [
          { type: 'alipay', gateway: 'primary' },
          { name: 'Backup Alipay', type: 'alipay', gateway: 'backup' },
        ],
        1
      )
    ).toMatchObject([
      { name: 'alipay', type: 'alipay', gateway: 'primary' },
      { name: 'Backup Alipay', type: 'alipay', gateway: 'backup' },
    ])
  })

  test('preserves Epay gateway and fee metadata from top-up info', () => {
    expect(
      normalizeTopupPaymentMethods(
        [
          {
            name: 'Backup Alipay',
            type: 'alipay',
            gateway: 'backup',
            fee: '0.30',
            fee_rate: '1.5',
          },
        ],
        1
      )
    ).toEqual([
      {
        name: 'Backup Alipay',
        type: 'alipay',
        color: undefined,
        icon: undefined,
        gateway: 'backup',
        fee: 0.3,
        fee_rate: 1.5,
        min_topup: 0,
      },
    ])
  })

  test('uses the dedicated Waffo amount calculator', async () => {
    const calls: string[] = []
    const amount = await requestPaymentAmount(120, PAYMENT_TYPES.WAFFO, {
      regular: async () => {
        calls.push('regular')
        return { success: true, data: '1' }
      },
      stripe: async () => {
        calls.push('stripe')
        return { success: true, data: '2' }
      },
      waffo: async (request) => {
        calls.push(`waffo:${request.amount}`)
        return { success: true, data: '18.75' }
      },
      waffoPancake: async () => {
        calls.push('pancake')
        return { success: true, data: '4' }
      },
    })

    expect(amount).toBe(18.75)
    expect(calls).toEqual(['waffo:120'])
  })

  test('sends the selected Epay gateway to the amount calculator', async () => {
    let request: unknown
    const amount = await requestPaymentAmount(
      120,
      'alipay',
      {
        regular: async (value) => {
          request = value
          return { success: true, data: '19.20' }
        },
        stripe: async () => ({ success: false }),
        waffo: async () => ({ success: false }),
        waffoPancake: async () => ({ success: false }),
      },
      'backup'
    )

    expect(amount).toBe(19.2)
    expect(request).toEqual({
      amount: 120,
      payment_method: 'alipay',
      epay_gateway: 'backup',
    })
  })
})
