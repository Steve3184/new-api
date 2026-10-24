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
import { render, screen } from '@testing-library/react'
import { createElement, Fragment, type ReactNode } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { PresetTwo } from '../preset-two'

const state = vi.hoisted(() => ({
  user: null as { id: number } | null,
  slaEnabled: false,
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
    useInView: () => false,
    useReducedMotion: () => true,
    useScroll: () => ({ scrollYProgress: 0 }),
    useSpring: (value: unknown) => value,
  }
})

afterEach(() => {
  state.user = null
  state.slaEnabled = false
})

describe('homepage preset two', () => {
  it('offers sign-up entry points to anonymous visitors', () => {
    render(<PresetTwo />)

    expect(screen.getAllByText('Get Started')).toHaveLength(2)
    expect(screen.queryByText('Go to Dashboard')).not.toBeInTheDocument()
  })

  it('offers the dashboard to signed-in visitors', () => {
    state.user = { id: 1 }
    render(<PresetTwo />)

    expect(screen.getAllByText('Go to Dashboard')).toHaveLength(2)
    expect(screen.queryByText('Get Started')).not.toBeInTheDocument()
  })

  it('builds the integration snippet from the configured server address', () => {
    render(<PresetTwo />)

    const snippet = screen.getByText(
      (_content, element) => element?.tagName === 'CODE'
    )
    expect(snippet).toHaveTextContent(
      'curl https://api.example.com/v1/chat/completions'
    )
  })

  it('shows the SLA commitment only when it is enabled', () => {
    const { unmount } = render(<PresetTwo />)
    expect(
      screen.queryByText('99.9% uptime commitment')
    ).not.toBeInTheDocument()
    unmount()

    state.slaEnabled = true
    render(<PresetTwo />)
    expect(screen.getByText('99.9% uptime commitment')).toBeInTheDocument()
  })
})
