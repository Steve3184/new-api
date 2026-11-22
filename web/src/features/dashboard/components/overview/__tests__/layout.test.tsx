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
import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { OverviewDashboard } from '../overview-dashboard'

const getAnimationsDescriptor = Object.getOwnPropertyDescriptor(
  Element.prototype,
  'getAnimations'
)
let client: QueryClient
let panels: Record<string, boolean>

beforeEach(() => {
  // The scroll areas ask for running animations, which jsdom lacks.
  Object.defineProperty(Element.prototype, 'getAnimations', {
    configurable: true,
    value: () => [],
  })
  window.localStorage.clear()
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  useAuthStore.getState().auth.setUser({
    id: 1,
    username: 'dashboard-user',
    role: 1,
    quota: 1000000,
    used_quota: 1000,
    request_count: 1,
  })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  panels = {
    api_info_enabled: true,
    announcements_enabled: true,
    faq_enabled: true,
    uptime_kuma_enabled: true,
  }
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    switch (url) {
      case '/api/status':
        return { data: { data: panels } }
      case '/api/token/?p=1&size=10':
        return {
          data: {
            success: true,
            data: { items: [{ id: 1, name: 'App key', status: 1 }] },
          },
        }
      case '/api/user/models':
        return { data: { success: true, data: ['gpt-4o-mini'] } }
      case '/api/data/self':
      case '/api/uptime/status':
        return { data: { success: true, data: [] } }
      default:
        throw new Error(`Unexpected dashboard request: ${url}`)
    }
  })
})

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  client.clear()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  useSystemConfigStore.setState(useSystemConfigStore.getInitialState(), true)
  window.localStorage.clear()
  if (getAnimationsDescriptor) {
    Object.defineProperty(
      Element.prototype,
      'getAnimations',
      getAnimationsDescriptor
    )
  } else {
    Reflect.deleteProperty(Element.prototype, 'getAnimations')
  }
})

async function renderOverview() {
  const router = createRouter({
    routeTree: createRootRoute({ component: OverviewDashboard }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

/**
 * The item of the overview content grid that holds `element`. jsdom does not
 * lay out CSS grids, so the tests read each item's wide-screen placement.
 */
function gridItemOf(element: Element) {
  const grid = screen.getByLabelText('Usage calendar').parentElement
    ?.parentElement as Element
  return [...grid.children].find((item) => item.contains(element))
}

describe('overview content grid', () => {
  it('pairs the usage calendar with API routes and Uptime Kuma with the panels below', async () => {
    await renderOverview()

    const uptime = gridItemOf(await screen.findByText('Uptime'))
    const apiInfo = gridItemOf(screen.getByText('API Info'))
    const calendar = gridItemOf(screen.getByLabelText('Usage calendar'))
    const below = gridItemOf(screen.getByText('Announcements'))

    expect(calendar).toHaveClass('xl:col-start-1', 'xl:row-start-1')
    expect(apiInfo).toHaveClass('xl:col-start-2', 'xl:row-start-1')
    // API routes take the calendar's row height instead of setting it, so the
    // calendar card never gains empty space below its text.
    expect(apiInfo).toHaveClass('xl:h-0', 'xl:min-h-full')
    expect(below).toHaveClass('xl:col-start-1', 'xl:row-start-2')
    expect(below).toContainElement(screen.getByText('FAQ'))
    // Uptime Kuma shares a row with the panels below the calendar only, so a
    // taller calendar cannot stretch it.
    expect(uptime).toHaveClass('xl:col-start-2', 'xl:row-start-2')
  })

  it('keeps a lone Uptime Kuma panel at its own height beside the calendar', async () => {
    panels = { ...panels, api_info_enabled: false }
    await renderOverview()

    const uptime = gridItemOf(await screen.findByText('Uptime'))

    expect(screen.queryByText('API Info')).not.toBeInTheDocument()
    expect(uptime).toHaveClass('xl:row-span-2', 'xl:self-start')
  })

  it('gives the usage calendar the full width when no side panel is shown', async () => {
    panels = { ...panels, api_info_enabled: false, uptime_kuma_enabled: false }
    await renderOverview()

    expect(await screen.findByText('Announcements')).toBeInTheDocument()
    const grid =
      screen.getByLabelText('Usage calendar').parentElement?.parentElement

    expect(grid).not.toHaveClass('xl:grid-cols-[minmax(0,1fr)_22rem]')
    expect(screen.queryByText('API Info')).not.toBeInTheDocument()
    expect(screen.queryByText('Uptime')).not.toBeInTheDocument()
  })
})
