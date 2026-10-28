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
import { fireEvent, render, renderHook, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import type { ReactNode } from 'react'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { beforeAll, describe, expect, it, vi } from 'vitest'

import type { TopupRecord } from '../../types'
import { useOrdersColumns } from '../orders-columns'

const i18n = createInstance()

beforeAll(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: false,
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

const baseRecord: TopupRecord = {
  id: 1,
  user_id: 1,
  amount: 10,
  money: 10,
  trade_no: 'order-1',
  payment_method: 'stripe',
  payment_provider: 'stripe',
  create_time: 1,
  status: 'pending',
}

function wrapper(props: { children: ReactNode }) {
  return <I18nextProvider i18n={i18n}>{props.children}</I18nextProvider>
}

function renderActions(
  record: TopupRecord,
  options: { isAdmin?: boolean; currentUserId?: number } = {}
) {
  const onContinue = vi.fn()
  const onComplete = vi.fn()
  const { result } = renderHook(
    () =>
      useOrdersColumns({
        isAdmin: options.isAdmin ?? false,
        currentUserId: options.currentUserId ?? 1,
        resumingTradeNo: null,
        onContinue,
        onComplete,
      }),
    { wrapper }
  )
  const column = result.current.find((item) => item.id === 'actions')
  const cell = column?.cell as (context: unknown) => ReactNode
  render(<I18nextProvider i18n={i18n}>{cell({ row: { original: record } })}</I18nextProvider>)
  return { onContinue, onComplete }
}

describe('order actions', () => {
  it('offers Continue Payment to the owner when the server allows resuming', () => {
    const { onContinue } = renderActions({
      ...baseRecord,
      can_continue_payment: true,
    })

    fireEvent.click(screen.getByRole('button', { name: 'Continue Payment' }))

    expect(onContinue).toHaveBeenCalledWith(
      expect.objectContaining({ trade_no: 'order-1' })
    )
  })

  it('hides Continue Payment when the server reports the payment cannot be resumed', () => {
    renderActions({ ...baseRecord, can_continue_payment: false })

    expect(
      screen.queryByRole('button', { name: 'Continue Payment' })
    ).not.toBeInTheDocument()
  })

  it('does not let an administrator resume another user order but still lets them complete it', () => {
    renderActions(
      { ...baseRecord, user_id: 2, can_continue_payment: true },
      { isAdmin: true, currentUserId: 1 }
    )

    expect(
      screen.queryByRole('button', { name: 'Continue Payment' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Complete Order' })
    ).toBeInTheDocument()
  })

  it('lets an administrator complete an expired crypto order', () => {
    renderActions(
      {
        ...baseRecord,
        status: 'expired',
        payment_method: 'nowpayments',
        payment_provider: 'nowpayments',
      },
      { isAdmin: true }
    )

    expect(
      screen.getByRole('button', { name: 'Complete Order' })
    ).toBeInTheDocument()
  })

  it('does not offer Complete Order for an expired card order', () => {
    renderActions({ ...baseRecord, status: 'expired' }, { isAdmin: true })

    expect(
      screen.queryByRole('button', { name: 'Complete Order' })
    ).not.toBeInTheDocument()
  })

  it('never offers Complete Order to a non-admin user', () => {
    renderActions({ ...baseRecord, can_continue_payment: true })

    expect(
      screen.queryByRole('button', { name: 'Complete Order' })
    ).not.toBeInTheDocument()
  })
})
