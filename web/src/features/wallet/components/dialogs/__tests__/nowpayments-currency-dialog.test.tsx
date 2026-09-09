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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { NowPaymentsCurrencyDialog } from '../nowpayments-currency-dialog'

describe('NOWPayments currency dialog', () => {
  test('lets users choose USDT on BSC before starting payment', async () => {
    const user = userEvent.setup()
    const onSelectedCurrencyChange = vi.fn()
    const onConfirm = vi.fn()

    render(
      <NowPaymentsCurrencyDialog
        open
        onOpenChange={vi.fn()}
        currencies={['btc', 'usdtbsc']}
        selectedCurrency='btc'
        onSelectedCurrencyChange={onSelectedCurrencyChange}
        onConfirm={onConfirm}
      />
    )

    const usdtOption = screen.getByRole('button', {
      name: /USDT \(BSC\)/i,
    })
    expect(usdtOption).toHaveAttribute('aria-pressed', 'false')

    await user.click(usdtOption)
    expect(onSelectedCurrencyChange).toHaveBeenCalledWith('usdtbsc')

    await user.click(screen.getByRole('button', { name: 'Pay' }))
    expect(onConfirm).toHaveBeenCalledOnce()
    expect(screen.queryByRole('combobox')).not.toBeInTheDocument()
  })
})
