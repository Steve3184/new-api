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
  within,
  fireEvent,
  waitFor,
} from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { Profile } from '../../index'

afterEach(() => {
  vi.restoreAllMocks()
  useAuthStore.getState().auth.reset()
})

test('preferences occupy the sidebar when check-in and sidebar customization are disabled, and save with notification edits', async () => {
  const profile = {
    id: 1,
    username: 'admin',
    display_name: 'Admin',
    role: 100,
    group: 'default',
    quota: 100,
    used_quota: 0,
    request_count: 0,
    status: 1,
    aff_count: 0,
    setting: '{}',
    permissions: { sidebar_settings: false },
  }
  useAuthStore.setState((state) => ({
    auth: { ...state.auth, user: profile as typeof state.auth.user },
  }))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  client.setQueryData(['status'], { checkin_enabled: false })
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: profile },
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  render(
    <QueryClientProvider client={client}>
      <Profile />
    </QueryClientProvider>
  )
  const sidebar = screen.getByRole('complementary', { name: 'Preferences' })
  const toggle = await within(sidebar).findByRole('switch', {
    name: 'Accept Unpriced Models',
  })
  expect(toggle).toBeVisible()
  expect(screen.queryByText('Check-in Calendar')).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Notification Email'), {
    target: { value: 'alerts@example.com' },
  })
  fireEvent.click(toggle)
  fireEvent.click(
    within(sidebar).getByRole('button', { name: 'Save Settings' })
  )
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith(
      '/api/user/setting',
      expect.objectContaining({
        accept_unset_model_ratio_model: true,
        notification_email: 'alerts@example.com',
      })
    )
  )
  client.clear()
})
