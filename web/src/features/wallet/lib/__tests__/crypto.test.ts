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
*/
import { describe, expect, test } from 'vitest'

import {
  getCryptoCurrencyInfo,
  getCryptoCurrencyLabel,
  getCryptoPaymentQrValue,
} from '../crypto'

describe('NOWPayments currency presentation', () => {
  test('renders usdtbsc as USDT on the BSC network', () => {
    const info = getCryptoCurrencyInfo('usdtbsc')

    expect(info.symbol).toBe('USDT')
    expect(info.network).toBe('BSC')
    expect(getCryptoCurrencyLabel('usdtbsc')).toBe('USDT (BSC)')
  })

  test('creates native BTC and ETH wallet QR payloads', () => {
    expect(getCryptoPaymentQrValue('btc', 'bc1qexample', '0.001')).toBe(
      'bitcoin:bc1qexample?amount=0.001'
    )
    expect(getCryptoPaymentQrValue('eth', '0xexample', '0.25')).toBe(
      'ethereum:0xexample?value=0.25'
    )
  })

  test('uses the deposit address for token network IDs', () => {
    expect(getCryptoPaymentQrValue('usdtbsc', '0xdeposit', '12')).toBe(
      '0xdeposit'
    )
  })
})
