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

import { Response } from '../response'

describe('streaming response alerts', () => {
  test.each([
    ['NOTE', 'Note'],
    ['CAUTION', 'Caution'],
  ])('renders %s with a visible icon and removes its marker', (kind, label) => {
    const { container } = render(
      <Response
        parserId={`alert-${kind}`}
      >{`> [!${kind}]\n> Details`}</Response>
    )

    expect(screen.getByText(label)).toBeInTheDocument()
    expect(screen.getByText('Details')).toBeInTheDocument()
    expect(container.querySelector('[data-alert-icon]')).toBeInTheDocument()
    expect(screen.queryByText(`[!${kind}]`)).not.toBeInTheDocument()
  })
})
