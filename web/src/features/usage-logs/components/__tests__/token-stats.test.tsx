import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import {
  act,
  cleanup,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import i18next from 'i18next'
import { afterEach, expect, it, vi } from 'vitest'

import { Button } from '@/components/ui/button'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { CommonLogsStats } from '../common-logs-stats'
import { UsageLogsProvider, useUsageLogsContext } from '../usage-logs-provider'

function StatsFixture() {
  const context = useUsageLogsContext()
  return (
    <>
      <CommonLogsStats />
      <Button
        onClick={() => context.setSensitiveVisible(!context.sensitiveVisible)}
      >
        Toggle privacy
      </Button>
    </>
  )
}

async function renderStats(role: number) {
  useAuthStore.getState().auth.setUser({ id: 11, username: 'alice', role })
  const root = createRootRoute()
  const auth = createRoute({ getParentRoute: () => root, id: '_authenticated' })
  const logs = createRoute({
    getParentRoute: () => auth,
    path: '/usage-logs/$section',
    component: () => (
      <UsageLogsProvider>
        <StatsFixture />
      </UsageLogsProvider>
    ),
    validateSearch: (search: Record<string, unknown>) => search,
  })
  const router = createRouter({
    routeTree: root.addChildren([auth.addChildren([logs])]),
    history: createMemoryHistory({
      initialEntries: ['/usage-logs/common?model=model-a&page=2'],
    }),
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
  return router
}

afterEach(async () => {
  cleanup()
  vi.restoreAllMocks()
  useAuthStore.getState().auth.setUser(null)
  await i18next.changeLanguage('en')
})

it.each([1, 10])(
  'shows filtered totals beside TPM and refreshes them after filtering for role %s',
  async (role) => {
    const request = vi.spyOn(api, 'get').mockImplementation(async (url) => ({
      data: {
        success: true,
        data: {
          quota: 0,
          rpm: 1,
          tpm: 12345,
          total_tokens: String(url).includes('model_name=model-a')
            ? 1234567
            : 80,
        },
      },
    }))
    const router = await renderStats(role)
    const total = await screen.findByTitle(
      'Total tokens in filtered logs: input + output + cache read + cache write.'
    )
    expect(within(total).getByText('1.23M')).toBeVisible()
    await act(() => i18next.changeLanguage('fr'))
    expect(within(total).getByText('1,23M')).toBeVisible()
    expect(screen.getByText('12,35K')).toBeVisible()
    await act(() => i18next.changeLanguage('en'))
    expect(within(total).getByText('Total Tokens')).toBeVisible()
    expect(screen.getByText('12.35K')).toBeVisible()
    expect(total.parentElement).toHaveClass('flex-wrap')
    expect(request).toHaveBeenCalledWith(
      expect.stringContaining(
        role === 1 ? '/api/log/self/stat?' : '/api/log/stat?'
      )
    )
    expect(request).toHaveBeenCalledWith(
      expect.stringContaining('model_name=model-a')
    )
    await router.history.push('/usage-logs/common?model=model-b&page=3')
    await waitFor(() => expect(within(total).getByText('80')).toBeVisible())
    await userEvent.click(
      screen.getByRole('button', { name: 'Toggle privacy' })
    )
    expect(within(total).getByText('••••')).toBeVisible()
    expect(within(total).queryByText('80')).not.toBeInTheDocument()
  }
)

it.each([null, { quota: 0, rpm: 0, tpm: 0 }])(
  'shows zero when statistics are empty or come from an older server (%s)',
  async (data) => {
    vi.spyOn(api, 'get').mockResolvedValue({ data: { success: true, data } })
    await renderStats(1)
    const total = await screen.findByTitle(
      'Total tokens in filtered logs: input + output + cache read + cache write.'
    )
    expect(within(total).getByText('0')).toBeVisible()
  }
)
