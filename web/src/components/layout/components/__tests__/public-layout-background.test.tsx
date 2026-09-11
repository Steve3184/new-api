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
import { render } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { PublicLayout } from '../public-layout'

const appearance = {
  backgroundImage: '',
  backgroundBlurOpacity: 40,
}

vi.mock('@/hooks/use-system-config', () => ({
  useSystemConfig: () => ({
    systemName: 'New API',
    logo: '',
    loading: false,
    logoLoaded: true,
    appearance,
  }),
}))

vi.mock('@tanstack/react-router', () => ({
  Link: (props: React.AnchorHTMLAttributes<HTMLAnchorElement>) => (
    <a {...props} />
  ),
  useNavigate: () => vi.fn(),
  useRouterState: () => ({ location: { pathname: '/' } }),
}))

vi.mock('@/components/language-switcher', () => ({
  LanguageSwitcher: () => null,
}))
vi.mock('@/components/notification-popover', () => ({
  NotificationPopover: () => null,
}))
vi.mock('@/components/profile-dropdown', () => ({
  ProfileDropdown: () => null,
}))
vi.mock('@/components/theme-switch', () => ({ ThemeSwitch: () => null }))
vi.mock('@/hooks/use-notifications', () => ({
  useNotifications: () => ({
    popoverOpen: false,
    setPopoverOpen: vi.fn(),
    unreadCount: 0,
    activeTab: 'notice',
    setActiveTab: vi.fn(),
    notice: '',
    announcements: [],
    loading: false,
    noticeButtonMode: 'popover',
  }),
}))
vi.mock('@/hooks/use-top-nav-links', () => ({ useTopNavLinks: () => [] }))
vi.mock('@/stores/auth-store', () => ({
  useAuthStore: () => ({ auth: { user: null } }),
}))

function renderLayout(backgroundMode?: 'none' | 'hero') {
  const { container } = render(
    <PublicLayout
      backgroundMode={backgroundMode}
      showMainContainer={false}
      showAuthButtons={false}
    >
      <div>content</div>
    </PublicLayout>
  )
  const root = container.firstElementChild
  if (!(root instanceof HTMLElement)) {
    throw new Error('Expected the public layout root element')
  }
  return root
}

describe('public layout background surfaces', () => {
  beforeEach(() => {
    appearance.backgroundImage = ''
    appearance.backgroundBlurOpacity = 40
  })

  test('tints cards, lists and sidebars when a hero background image is set', () => {
    appearance.backgroundImage = '/background.webp'

    const root = renderLayout('hero')

    expect(root).toHaveAttribute('data-console-background', 'image')
    expect(root.style.getPropertyValue('--console-surface-opacity')).toBe('40%')
    expect(root.style.getPropertyValue('--console-card-opacity')).toBe('55%')
    expect(root.style.getPropertyValue('--console-sidebar-opacity')).toBe('50%')
    expect(root.style.getPropertyValue('--console-list-opacity')).toBe('60%')
  })

  test('follows the configured blur opacity and clamps the derived steps', () => {
    appearance.backgroundImage = '/background.webp'
    appearance.backgroundBlurOpacity = 100

    const root = renderLayout('hero')

    expect(root.style.getPropertyValue('--console-surface-opacity')).toBe('100%')
    expect(root.style.getPropertyValue('--console-card-opacity')).toBe('85%')
    expect(root.style.getPropertyValue('--console-sidebar-opacity')).toBe('80%')
    expect(root.style.getPropertyValue('--console-list-opacity')).toBe('75%')
  })

  test('leaves surfaces solid without a hero background image', () => {
    const root = renderLayout('hero')

    expect(root).not.toHaveAttribute('data-console-background')
    expect(root.style.getPropertyValue('--console-card-opacity')).toBe('')
  })

  test('leaves surfaces solid when the page does not request the hero backdrop', () => {
    appearance.backgroundImage = '/background.webp'

    const root = renderLayout()

    expect(root).not.toHaveAttribute('data-console-background')
    expect(root.style.getPropertyValue('--console-card-opacity')).toBe('')
  })
})
