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
import { describe, expect, test } from 'vitest'

import { NotificationPopover } from '../../notification-popover'

describe('manual notification dialog layout', () => {
  test('keeps the fixed dialog viewport clipped while the announcement content owns scrolling', () => {
    render(
      <NotificationPopover
        open
        onOpenChange={() => undefined}
        unreadCount={0}
        activeTab='notice'
        onTabChange={() => undefined}
        notice={'A long announcement\n'.repeat(80)}
        announcements={[]}
        loading={false}
        displayMode='dialog'
      />
    )

    expect(
      screen.getByRole('dialog', { name: 'System Announcements' })
    ).toBeInTheDocument()

    const bodyViewport = document.querySelector(
      '[data-slot="dialog-body-viewport"]'
    )
    const scrollArea = document.querySelector('[data-slot="scroll-area"]')
    expect(bodyViewport).toHaveClass('overflow-hidden')
    expect(bodyViewport).not.toHaveClass('overflow-y-auto')
    expect(scrollArea).toHaveClass('flex-1', 'min-h-0')
  })
})
