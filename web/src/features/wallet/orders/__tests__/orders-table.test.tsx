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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { createInstance } from 'i18next'
import { I18nextProvider, initReactI18next } from 'react-i18next'
import { afterEach, beforeAll, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { OrdersTable } from '../orders-table'

vi.mock('@tanstack/react-router', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@tanstack/react-router')>()
  return {
    ...actual,
    getRouteApi: () => ({
      useSearch: () => ({}),
      useNavigate: () => vi.fn(),
    }),
  }
})

const i18n = createInstance()

beforeAll(async () => {
  await i18n.use(initReactI18next).init({
    lng: 'en',
    fallbackLng: false,
    resources: { en: { translation: {} } },
    interpolation: { escapeValue: false },
  })
})

afterEach(() => {
  vi.restoreAllMocks()
})

it('renders the orders table with its search toolbar and loaded rows', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: {
      success: true,
      data: {
        total: 1,
        items: [
          {
            id: 1,
            user_id: 1,
            amount: 10,
            money: 10,
            trade_no: 'order-render-1',
            payment_method: 'stripe',
            payment_provider: 'stripe',
            create_time: 1,
            status: 'pending',
          },
        ],
      },
    },
  })

  render(
    <I18nextProvider i18n={i18n}>
      <QueryClientProvider client={new QueryClient()}>
        <OrdersTable />
      </QueryClientProvider>
    </I18nextProvider>
  )

  expect(
    (await screen.findAllByText('order-render-1')).length
  ).toBeGreaterThan(0)
  expect(
    screen.getAllByPlaceholderText('Search by order number...').length
  ).toBeGreaterThan(0)
})
