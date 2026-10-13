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
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { toast } from 'sonner'
import { expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { STATUS_QUERY_KEY } from '@/lib/status-query'

import { UserAuthForm } from '../user-auth-form'

it('warns on password login without legal consent and submits after consent', async () => {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  })
  client.setQueryData(STATUS_QUERY_KEY, {
    user_agreement_enabled: true,
    password_login_enabled: true,
    password_login_encryption_enabled: false,
  })
  const post = vi
    .spyOn(api, 'post')
    .mockResolvedValue({ data: { success: false, message: 'Login failed' } })
  const error = vi.spyOn(toast, 'error')
  const root = createRootRoute()
  const signIn = createRoute({
    getParentRoute: () => root,
    path: '/sign-in',
    component: UserAuthForm,
  })
  const router = createRouter({
    routeTree: root.addChildren([signIn]),
    history: createMemoryHistory({ initialEntries: ['/sign-in'] }),
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  const user = userEvent.setup()
  await user.type(await screen.findByLabelText('Username or Email'), 'alice')
  await user.type(screen.getByLabelText('Password'), 'test-password')
  const submit = screen.getByRole('button', { name: 'Sign in' })
  expect(submit).toBeEnabled()
  await user.click(submit)
  await waitFor(() =>
    expect(error).toHaveBeenCalledWith('Please agree to the legal terms first')
  )
  expect(post).not.toHaveBeenCalled()
  await user.click(screen.getByRole('checkbox'))
  await user.click(submit)
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      expect.stringContaining('/api/user/login'),
      expect.anything(),
      expect.anything()
    )
  )
})
