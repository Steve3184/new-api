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
import { describe, expect, test } from 'vitest'

import { UserLeaderboard } from '../user-leaderboard'

describe('user leaderboard layout', () => {
  test('keeps the wide-layout divider on the midpoint without a horizontal grid gap', () => {
    const { container } = render(
      <UserLeaderboard byQuota={[]} byTokens={[]} period='week' />
    )
    const columns = container.querySelector('section > div.grid')

    expect(columns).not.toBeNull()
    if (!columns) return

    expect(columns.classList.contains('md:grid-cols-2')).toBe(true)
    expect(
      [...columns.classList].some((className) => className.startsWith('gap-x-'))
    ).toBe(false)

    const [firstColumn] = [...columns.children]
    expect(firstColumn).not.toBeNull()
    expect(firstColumn.classList.contains('md:first:border-r')).toBe(true)
  })
})
