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
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { SignUpForm } from '../components/sign-up-form'

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: {} }),
}))

afterEach(cleanup)

async function renderSignUp() {
  const root = createRootRoute({ component: () => <SignUpForm /> })
  const router = createRouter({
    routeTree: root,
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

describe('sign-up password checklists', () => {
  it('stay folded until the password field has focus', async () => {
    await renderSignUp()

    expect(
      await screen.findByRole('meter', { name: 'Password strength' })
    ).toBeInTheDocument()
    expect(screen.queryByText('Password requirement')).not.toBeInTheDocument()
    expect(
      screen.queryByText('For a stronger password')
    ).not.toBeInTheDocument()
  })

  it('open while the password or its visibility toggle has focus and fold when focus leaves the field', async () => {
    const user = userEvent.setup()
    await renderSignUp()

    await user.click(
      await screen.findByPlaceholderText('Enter password (8–128 characters)')
    )
    expect(screen.getByText('Password requirement')).toBeVisible()
    expect(screen.getByText('For a stronger password')).toBeVisible()

    await user.tab()
    expect(screen.getByText('Password requirement')).toBeVisible()

    await user.click(screen.getByPlaceholderText('Enter your username'))
    // The lists animate shut before they leave the page.
    await waitFor(() =>
      expect(screen.queryByText('Password requirement')).not.toBeInTheDocument()
    )
  })
})
