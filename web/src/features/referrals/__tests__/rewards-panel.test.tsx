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
import {
  render,
  screen,
  fireEvent,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import type { ReferralData } from '../api'
import { ReferralRewardsPanel } from '../rewards-panel'

const data: ReferralData = {
  invited_users: 5,
  rules: {
    enabled: true,
    percent: 5,
    min_paid_cny: 20,
    max_orders: 10,
    delay_hours: 24,
    currency_rates: '{"CNY":1,"USD":7.3}',
  },
  total: 11,
  summary: [{ status: 'pending', quota: 500000, count: 1 }],
  items: [
    {
      id: 1,
      inviter_id: 2,
      invitee_id: 3,
      trade_no: 'paid-order',
      paid_cny: 100,
      percent: 5,
      quota: 500000,
      status: 'pending',
      created_at: 100,
      available_at: 86500,
      credited_at: 0,
      reason: '',
    },
  ],
}
afterEach(() => vi.restoreAllMocks())

function renderPanel(admin = false) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const view = render(
    <QueryClientProvider client={client}>
      <ReferralRewardsPanel admin={admin} />
    </QueryClientProvider>
  )
  return { ...view, client }
}

test('users see the exact threshold and delay, their own records and paginated results without admin actions', async () => {
  const get = vi
    .spyOn(api, 'get')
    .mockResolvedValue({ data: { success: true, data } })
  const view = renderPanel()
  expect(await screen.findByText(/strictly above CNY 20/)).toHaveTextContent(
    'after 24 hours'
  )
  expect(
    screen.queryByRole('button', { name: 'Reverse reward' })
  ).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Go to next page' }))
  await waitFor(() =>
    expect(get).toHaveBeenCalledWith(
      '/api/user/self/referral-rewards',
      expect.objectContaining({ params: expect.objectContaining({ p: 2 }) })
    )
  )
  view.unmount()
  view.client.clear()
})

test('admin reversal requires a reason and submits only after confirmation', async () => {
  vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data } })
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: true } })
  const view = renderPanel(true)
  fireEvent.click(await screen.findByRole('button', { name: 'Reverse reward' }))
  const dialog = screen.getByRole('alertdialog')
  const confirm = within(dialog).getByRole('button', { name: 'Reverse reward' })
  expect(confirm).toBeDisabled()
  fireEvent.change(within(dialog).getByLabelText('Reason'), {
    target: { value: 'Payment refunded' },
  })
  fireEvent.click(confirm)
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith('/api/user/referral-rewards/1/reverse', {
      reason: 'Payment refunded',
    })
  )
  view.unmount()
  view.client.clear()
})

test('empty rewards show an empty state and API failure exposes retry', async () => {
  vi.spyOn(api, 'get')
    .mockResolvedValueOnce({
      data: { success: true, data: { ...data, items: [], total: 0 } },
    })
    .mockRejectedValue(new Error('unavailable'))
  const view = renderPanel()
  expect(await screen.findByText('No referral rewards yet')).toBeVisible()
  await view.client.invalidateQueries({ queryKey: ['referral-rewards'] })
  expect(await screen.findByRole('button', { name: /retry/i })).toBeVisible()
  view.unmount()
  view.client.clear()
})
