/* Copyright (C) 2023-2026 QuantumNous */
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'
import { useContributionsStore } from '@/stores/contributions-store'

import { ContributionsCard } from '../contributions-card'

const DAY_MS = 24 * 60 * 60 * 1000
let client: QueryClient
let usageHandler: () => Promise<unknown>

function selfResponse(rows: unknown[]) {
  return { data: { success: true, data: rows } }
}

function renderCard() {
  return render(
    <QueryClientProvider client={client}>
      <ContributionsCard />
    </QueryClientProvider>
  )
}

/** A timestamp inside the current local day, so the grid shows real usage. */
function hoursAgo(hours: number) {
  return Math.floor((Date.now() - hours * 60 * 60 * 1000) / 1000)
}

beforeEach(() => {
  window.localStorage.clear()
  useContributionsStore.setState({ view: '2d' })
  useAuthStore
    .getState()
    .auth.setUser({ id: 7, username: 'calendar-user', role: 1 })
  client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  usageHandler = async () => selfResponse([])
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/status') {
      return { data: { success: true, data: { log_max_range_days: 365 } } }
    }
    if (url === '/api/data/self') return usageHandler()
    throw new Error(`Unexpected contributions request: ${url}`)
  })
})

afterEach(() => {
  cleanup()
  client.clear()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  useContributionsStore.setState({ view: '2d' })
  window.localStorage.clear()
})

describe('contributions card', () => {
  it('shows a labelled calendar from the current user usage', async () => {
    usageHandler = async () =>
      selfResponse([{ created_at: hoursAgo(2), token_used: 1234, count: 2 }])

    renderCard()

    expect(
      await screen.findByRole('heading', { name: 'Usage calendar' })
    ).toBeVisible()
    // Every day carries its own accessible description, so the usage is
    // reachable without relying on hover.
    expect(
      await screen.findByRole('button', { name: /: 1,234 tokens$/ })
    ).toBeInTheDocument()
    const cells = screen.getAllByRole('button', {
      name: /tokens$|No statistics available$/,
    })
    expect(cells.length).toBeGreaterThan(360)
  })

  it('still loads when the status request fails', async () => {
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/status') throw new Error('status unavailable')
      if (url === '/api/data/self') return selfResponse([])
      throw new Error(`Unexpected contributions request: ${url}`)
    })

    renderCard()

    // The card must not stay on the loading skeleton: the site default range is
    // used and unavailable days simply render dimmed.
    expect(
      await screen.findByText('No usage recorded in this period.')
    ).toBeVisible()
    expect(screen.queryByLabelText('Loading usage data')).toBeNull()
  })

  it('offers a retry when the usage request fails', async () => {
    const user = userEvent.setup()
    usageHandler = async () => {
      throw new Error('usage unavailable')
    }

    renderCard()

    expect(await screen.findByText('Failed to load usage data')).toBeVisible()

    usageHandler = async () =>
      selfResponse([{ created_at: hoursAgo(1), token_used: 8, count: 1 }])
    await user.click(screen.getByRole('button', { name: 'Retry' }))

    await waitFor(() =>
      expect(screen.queryByText('Failed to load usage data')).toBeNull()
    )
    expect(
      await screen.findByRole('button', { name: /: 8 tokens$/ })
    ).toBeInTheDocument()
  })

  it('reports an empty period without claiming the calendar is broken', async () => {
    renderCard()

    expect(
      await screen.findByText('No usage recorded in this period.')
    ).toBeVisible()
    expect(screen.queryByText('Failed to load usage data')).toBeNull()
  })

  it('switches to 3D, remembers the choice and keeps the calendar reachable', async () => {
    const user = userEvent.setup()
    usageHandler = async () =>
      selfResponse([{ created_at: hoursAgo(3), token_used: 10, count: 1 }])

    renderCard()
    await screen.findByRole('button', { name: /: 10 tokens$/ })
    // Toggle items are pressed buttons rather than radios.
    await user.click(screen.getByRole('button', { name: '3D' }))

    await waitFor(() =>
      expect(useContributionsStore.getState().view).toBe('3d')
    )
    expect(
      JSON.parse(window.localStorage.getItem('contributions-view') ?? '{}')
    ).toEqual({ state: { view: '3d' }, version: 0 })
    // The 2D grid stays mounted as the keyboard-accessible view in 3D mode.
    expect(
      screen.getByRole('group', { name: 'Usage by day' })
    ).toBeInTheDocument()
  })
})

describe('contributions request range', () => {
  it('asks for a rolling year ending today', async () => {
    renderCard()
    await screen.findByText('No usage recorded in this period.')

    const call = vi
      .mocked(api.get)
      .mock.calls.find(([url]) => url === '/api/data/self')
    const params = call?.[1]?.params as {
      start_timestamp: number
      end_timestamp: number
    }
    expect(params.end_timestamp * 1000).toBeGreaterThanOrEqual(
      Date.now() - DAY_MS
    )
    expect(params.end_timestamp - params.start_timestamp).toBeLessThanOrEqual(
      365 * 24 * 60 * 60
    )
  })
})
