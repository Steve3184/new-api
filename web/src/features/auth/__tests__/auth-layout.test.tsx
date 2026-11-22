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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type { SiteAppearanceConfig } from '@/stores/system-config-store'

import { AuthLayout } from '../auth-layout'

let appearance: Partial<SiteAppearanceConfig>

// Router links are marked so a test can tell them from plain anchors, which
// would reload the whole app instead of switching routes in place.
vi.mock('@tanstack/react-router', () => ({
  Link: (props: { to: string; children: React.ReactNode }) => (
    <a href={props.to} data-router-link=''>
      {props.children}
    </a>
  ),
}))

vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    systemName: 'Example API',
    logo: '/logo.png',
    loading: false,
    appearance,
  }),
}))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: { user_agreement_enabled: true, privacy_policy_enabled: true },
  }),
}))

function renderLayout() {
  return render(
    <AuthLayout>
      <p>Sign in form</p>
    </AuthLayout>
  )
}

beforeEach(() => {
  appearance = {
    authPageStyle: 'centered',
    authBackgroundEnabled: true,
    backgroundImage: '/background.webp',
    backgroundBlurOpacity: 40,
    authSplitImage: '',
  }
})

describe('auth layout', () => {
  test('keeps the centered card, its blurred background and the brand link', () => {
    const { container } = renderLayout()

    expect(screen.getByText('Sign in form')).toBeVisible()
    expect(
      container.querySelector('[data-auth-layout=centered]')
    ).not.toBeNull()
    expect(
      container.querySelector('[data-console-background=image]')
    ).not.toBeNull()
    expect(screen.getByRole('heading', { name: 'Example API' })).toBeVisible()
  })

  test('hides the background image when the administrator turns it off', () => {
    appearance = { ...appearance, authBackgroundEnabled: false }
    const { container } = renderLayout()

    expect(
      container.querySelector('[data-console-background=image]')
    ).toBeNull()
  })

  test('renders the split layout with a branded panel and legal links', () => {
    // Without a configured illustration or background, the panel falls back to
    // the animated particle field.
    appearance = {
      ...appearance,
      authPageStyle: 'split',
      authBackgroundEnabled: false,
    }
    const { container } = renderLayout()

    expect(container.querySelector('[data-auth-layout=split]')).not.toBeNull()
    expect(screen.getByText('Sign in form')).toBeVisible()
    // One copy for the desktop panel and one for narrow screens; CSS decides
    // which one is visible.
    expect(
      screen.getAllByRole('link', { name: 'User Agreement' })
    ).toHaveLength(2)
    expect(
      screen.getAllByRole('link', { name: 'Privacy Policy' })
    ).toHaveLength(2)
    // The legal pages are app routes, so the links switch routes in place.
    for (const link of screen.getAllByRole('link', {
      name: /User Agreement|Privacy Policy/,
    })) {
      expect(link).toHaveAttribute('data-router-link')
    }
    // The particle field stands in for the illustration panel.
    expect(container.querySelector('canvas')).not.toBeNull()
  })

  test('uses the configured illustration instead of the particle field', () => {
    appearance = {
      ...appearance,
      authPageStyle: 'split',
      authSplitImage: '/login-illustration.webp',
    }
    const { container } = renderLayout()

    const illustration = container.querySelector(
      'img[src="/login-illustration.webp"]'
    )
    expect(illustration).not.toBeNull()
    expect(illustration).toHaveAttribute('aria-hidden', 'true')
    expect(container.querySelector('canvas')).toBeNull()
  })
})
