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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import type { TopupInfo } from '../../types'
import { RedemptionPurchaseCard } from '../redemption-purchase-card'

vi.mock('../../api', () => ({
  calculateRedemptionPurchaseAmount: vi.fn(async () => ({
    success: true,
    data: '50',
  })),
  getUserRedemptions: vi.fn(async () => ({
    success: true,
    data: { items: [], total: 0, page: 1, page_size: 20 },
  })),
  isApiSuccess: (response: { success?: boolean }) => response.success === true,
  refundUserRedemption: vi.fn(),
  requestRedemptionPurchase: vi.fn(),
}))

const topupInfo: TopupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: false,
  pay_methods: [{ name: 'Alipay', type: 'alipay', min_topup: 50 }],
  min_topup: 1,
  stripe_min_topup: 1,
  amount_options: [10, 50],
  discount: {},
  enable_redemption_purchase: true,
  redemption_purchase_methods: ['alipay'],
}

function renderCard() {
  return render(
    <RedemptionPurchaseCard
      topupInfo={topupInfo}
      presetAmounts={[{ value: 10 }, { value: 50 }]}
      onMoneroInvoice={() => undefined}
      onRefreshUser={() => undefined}
    />
  )
}

describe('redemption purchase layout', () => {
  test('renders purchase controls and owned codes as two responsive columns', async () => {
    const { container } = renderCard()

    await waitFor(() =>
      expect(screen.getByText('Your Redemption Codes')).toBeInTheDocument()
    )

    const columns = [...container.querySelectorAll('div')].find((element) =>
      element.className.includes(
        'lg:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]'
      )
    )

    expect(columns).not.toBeUndefined()
    expect(columns?.children).toHaveLength(2)
    expect(screen.getByRole('button', { name: 'Alipay' })).toBeInTheDocument()
    expect(screen.getByLabelText('Code denomination')).toHaveValue(50)
  })

  test('disables a payment method below its configured minimum and opens confirmation after it is met', async () => {
    renderCard()
    const amountInput = screen.getByLabelText('Code denomination')

    const getPaymentButton = () => {
      const label = screen.getByText('Alipay', { selector: 'span' })
      return label.closest('button') as HTMLButtonElement
    }

    fireEvent.change(amountInput, { target: { value: '10' } })
    await waitFor(() => expect(getPaymentButton()).toBeDisabled())

    fireEvent.change(amountInput, { target: { value: '50' } })
    await waitFor(() => expect(getPaymentButton()).not.toBeDisabled())
    const paymentButton = getPaymentButton()
    fireEvent.click(paymentButton)

    expect(
      await screen.findByText('Review your payment details')
    ).toBeInTheDocument()
  })
})
