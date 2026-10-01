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
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { useState } from 'react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { ReferralSettingsForm } from '../index'

function Fixture() {
  const [container, setContainer] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setContainer} />
      <SettingsPageProvider actionsContainer={container}>
        <ReferralSettingsForm
          settings={{
            'referral_setting.enabled': true,
            'referral_setting.percent': 5,
            'referral_setting.min_paid_cny': 20,
            'referral_setting.max_orders': 10,
            'referral_setting.delay_hours': 24,
            'referral_setting.currency_rates': '{"CNY":1,"USD":7.3}',
          }}
        />
      </SettingsPageProvider>
    </>
  )
}

async function renderSettings() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const router = createRouter({
    routeTree: createRootRoute({ component: Fixture }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  const view = render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  await screen.findByRole('spinbutton', { name: 'Referral percentage' })
  return () => {
    view.unmount()
    client.clear()
  }
}
afterEach(() => vi.restoreAllMocks())

test('saves changed rules and the feature switch without resetting unrelated parameters', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const cleanup = await renderSettings()
  fireEvent.change(
    screen.getByRole('spinbutton', { name: 'Referral percentage' }),
    { target: { value: '7.5' } }
  )
  fireEvent.click(
    screen.getByRole('switch', { name: 'Enable fiat referral rewards' })
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  await waitFor(() => expect(put).toHaveBeenCalledTimes(2))
  expect(put).toHaveBeenCalledWith('/api/option/', {
    key: 'referral_setting.percent',
    value: '7.5',
  })
  expect(put).toHaveBeenCalledWith('/api/option/', {
    key: 'referral_setting.enabled',
    value: 'false',
  })
  cleanup()
})

test('invalid currency rates prevent saving and show the validation message', async () => {
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const cleanup = await renderSettings()
  fireEvent.change(
    screen.getByRole('textbox', { name: 'Currency conversion rates to CNY' }),
    { target: { value: '{"CNY":2}' } }
  )
  fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }))
  expect(
    await screen.findByText(
      'Provide positive currency rates with CNY equal to 1 and a USD rate.'
    )
  ).toBeVisible()
  expect(put).not.toHaveBeenCalled()
  cleanup()
})
