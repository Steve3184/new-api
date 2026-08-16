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

import { getMoneroPaymentStatusMessage } from './monero-payment-status'

describe('Monero payment status messages', () => {
  test('shows a waiting message before a transaction is detected', () => {
    expect(getMoneroPaymentStatusMessage(undefined)).toEqual({
      key: 'Waiting for transaction...',
    })
  })

  test('shows the observed confirmation count after detection', () => {
    expect(
      getMoneroPaymentStatusMessage({
        status: 'pending',
        transaction_detected: true,
        confirmations: 0,
        required_confirmations: 1,
      })
    ).toEqual({
      key: 'Transaction detected, waiting for confirmations... (Confirmations: {{count}})',
      values: { count: 0 },
    })
  })
})
