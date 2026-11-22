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
import { createElement, Fragment, type ReactNode } from 'react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { PresetTwo } from '../preset-two'

const state = vi.hoisted(() => ({
  user: null as { id: number } | null,
  slaEnabled: false,
  inView: false,
  headerNavModules: undefined as string | undefined,
}))

// The network is the boundary: what the pricing and platform traffic
// endpoints answer is set per test.
const api = vi.hoisted(() => ({
  getPricing: vi.fn(),
  getPlatformTraffic: vi.fn(),
}))

vi.mock('@/features/pricing/api', () => ({ getPricing: api.getPricing }))

vi.mock('@/features/home/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/home/api')>()),
  getPlatformTraffic: api.getPlatformTraffic,
}))

vi.mock('@/components/layout', () => ({
  PublicLayout: (props: {
    children?: ReactNode
    headerProps?: { floating?: boolean }
  }) => (
    <div
      data-testid='public-layout'
      data-floating={String(props.headerProps?.floating)}
    >
      {props.children}
    </div>
  ),
}))

vi.mock('@/components/layout/components/footer', () => ({
  Footer: () => null,
}))

vi.mock('@/components/ui/button', () => ({
  Button: (props: { children?: ReactNode; 'aria-label'?: string }) => (
    <button type='button' aria-label={props['aria-label']}>
      {props.children}
    </button>
  ),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: (props: { children?: ReactNode }) => <a href='/'>{props.children}</a>,
}))

vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: () => null,
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: {
      docs_link: 'https://docs.example.com',
      server_address: 'https://api.example.com/',
      HeaderNavModules: state.headerNavModules,
    },
  }),
}))

vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    systemName: 'Acme Gateway',
    homepage: {
      presetTitleMode: 'i18n',
      presetSlaEnabled: state.slaEnabled,
      presetSlaText: '99.9% uptime commitment',
    },
  }),
}))

vi.mock('@/stores/auth-store', () => ({
  useAuthStore: (selector: (store: unknown) => unknown) =>
    selector({ auth: { user: state.user } }),
}))

vi.mock('motion/react', () => {
  const motionProps = new Set([
    'variants',
    'initial',
    'animate',
    'exit',
    'whileInView',
    'viewport',
    'transition',
    'layout',
    'style',
  ])
  return {
    motion: new Proxy(
      {},
      {
        get: (_target, tag) => (props: Record<string, unknown>) =>
          createElement(
            String(tag),
            Object.fromEntries(
              Object.entries(props).filter(([key]) => !motionProps.has(key))
            )
          ),
      }
    ),
    AnimatePresence: (props: { children?: ReactNode }) =>
      createElement(Fragment, null, props.children),
    animate: () => ({ stop: () => undefined }),
    useInView: () => state.inView,
    useReducedMotion: () => true,
    useScroll: () => ({ scrollYProgress: 0 }),
    useSpring: (value: unknown) => value,
  }
})

// jsdom has no canvas backend, so the 2D context behind the hero fibers is
// faked. Each cleared frame records the blend mode it was painted with.
const frames: string[] = []

function renderHome() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <PresetTwo />
    </QueryClientProvider>
  )
}

/** The figure shown above a stat caption. */
function figureAbove(caption: string) {
  return screen.getByText(caption).previousElementSibling
}

beforeEach(() => {
  frames.length = 0
  api.getPricing.mockReset().mockResolvedValue({ success: true, data: [] })
  api.getPlatformTraffic
    .mockReset()
    .mockResolvedValue({ success: true, data: { rpm: 0, tpm: 0 } })
  const context = {
    globalCompositeOperation: 'source-over',
    setTransform: () => undefined,
    clearRect: () => frames.push('blank'),
    beginPath: () => undefined,
    moveTo: () => undefined,
    lineTo: () => undefined,
    arc: () => undefined,
    fill: () => undefined,
    stroke: () => {
      frames[frames.length - 1] = context.globalCompositeOperation
    },
  }
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    context as unknown as CanvasRenderingContext2D
  )
  vi.spyOn(
    HTMLCanvasElement.prototype,
    'getBoundingClientRect'
  ).mockReturnValue(new DOMRect(0, 0, 1440, 1350))
})

afterEach(() => {
  state.user = null
  state.slaEnabled = false
  state.inView = false
  state.headerNavModules = undefined
  vi.restoreAllMocks()
})

describe('homepage preset two', () => {
  it('offers sign-up entry points to anonymous visitors', () => {
    renderHome()

    expect(screen.getAllByText('Get Started')).toHaveLength(2)
    expect(screen.queryByText('Go to Dashboard')).not.toBeInTheDocument()
  })

  it('offers the dashboard to signed-in visitors', () => {
    state.user = { id: 1 }
    renderHome()

    expect(screen.getAllByText('Go to Dashboard')).toHaveLength(2)
    expect(screen.queryByText('Get Started')).not.toBeInTheDocument()
  })

  it('builds the integration snippet from the configured server address', () => {
    renderHome()

    const snippet = screen.getByText(
      (_content, element) => element?.tagName === 'CODE'
    )
    expect(snippet).toHaveTextContent(
      'curl https://api.example.com/v1/chat/completions'
    )
  })

  it('shows the SLA commitment only when it is enabled', () => {
    const { unmount } = renderHome()
    expect(
      screen.queryByText('99.9% uptime commitment')
    ).not.toBeInTheDocument()
    unmount()

    state.slaEnabled = true
    renderHome()
    expect(screen.getByText('99.9% uptime commitment')).toBeInTheDocument()
  })

  it('holds the hero fibers as one additive still frame under reduced motion', () => {
    const requestFrame = vi.spyOn(window, 'requestAnimationFrame')

    renderHome()

    expect(frames).toEqual(['lighter'])
    expect(requestFrame).not.toHaveBeenCalled()
  })

  it('shows the catalog size and live traffic of the platform', async () => {
    state.inView = true
    api.getPricing.mockResolvedValue({
      success: true,
      data: [
        { model_name: 'alpha', vendor_id: 1 },
        { model_name: 'beta', vendor_id: 2 },
        { model_name: 'gamma', vendor_id: 2 },
        // No vendor configured: the first is attributed by its name, the
        // second cannot be told and counts toward no vendor.
        { model_name: 'claude-opus-4-8' },
        { model_name: 'mystery-model' },
      ],
      vendors: [
        { id: 1, name: 'Acme' },
        { id: 2, name: 'Globex' },
      ],
    })
    api.getPlatformTraffic.mockResolvedValue({
      success: true,
      data: { rpm: 1234, tpm: 2_500_000 },
    })

    renderHome()

    expect(await screen.findByText('models available')).toBeInTheDocument()
    expect(figureAbove('models available')).toHaveTextContent('5')
    expect(figureAbove('providers connected')).toHaveTextContent('3')
    expect(figureAbove('requests per minute, live')).toHaveTextContent('1.2K')
    expect(figureAbove('tokens per minute, live')).toHaveTextContent('2.5M')
    expect(
      screen.queryByText('upstream services integrated')
    ).not.toBeInTheDocument()
  })

  it('keeps the capability figure in every slot whose live figure is missing', async () => {
    state.inView = true
    api.getPricing.mockResolvedValue({
      success: true,
      data: [{ model_name: 'alpha' }, { model_name: 'beta' }],
    })
    api.getPlatformTraffic.mockRejectedValue(new Error('traffic unavailable'))

    renderHome()

    expect(await screen.findByText('models available')).toBeInTheDocument()
    expect(figureAbove('models available')).toHaveTextContent('2')
    expect(figureAbove('upstream services integrated')).toHaveTextContent('50+')
    expect(figureAbove('compatible API routes')).toHaveTextContent('50+')
    expect(figureAbove('scheduling controls')).toHaveTextContent('10+')
    expect(
      screen.queryByText('requests per minute, live')
    ).not.toBeInTheDocument()
  })

  it('does not request figures from modules a signed-out visitor cannot open', async () => {
    state.inView = true
    state.headerNavModules = JSON.stringify({
      pricing: { enabled: true, requireAuth: true },
      rankings: { enabled: false, requireAuth: false },
    })

    renderHome()

    expect(
      await screen.findByText('upstream services integrated')
    ).toBeInTheDocument()
    expect(api.getPricing).not.toHaveBeenCalled()
    expect(api.getPlatformTraffic).not.toHaveBeenCalled()
  })

  it('waits to load platform figures until the band is near the viewport', () => {
    renderHome()

    expect(api.getPricing).not.toHaveBeenCalled()
    expect(api.getPlatformTraffic).not.toHaveBeenCalled()
  })
})
