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
import { render, screen, within } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { PaymentConfirmDialog } from '../payment-confirm-dialog'

describe('payment confirmation fee breakdown', () => {
  test('shows a three percent fee and the final payer total', () => {
    render(
      <PaymentConfirmDialog
        open
        onOpenChange={vi.fn()}
        onConfirm={vi.fn()}
        topupAmount={1}
        paymentAmount={1.03}
        paymentMethod={{
          name: 'Alipay',
          type: 'alipay',
          fee_rate: 3,
        }}
        calculating={false}
        processing={false}
      />
    )

    const dialog = screen.getByRole('alertdialog')
    expect(within(dialog).getByText('1.03')).toBeInTheDocument()
    expect(within(dialog).getByText('Payment fee')).toBeInTheDocument()
    expect(within(dialog).getByText('0.03')).toBeInTheDocument()
    expect(
      within(dialog).getByText(
        'The payment fee is included in the total shown above.'
      )
    ).toBeInTheDocument()
  })
})
