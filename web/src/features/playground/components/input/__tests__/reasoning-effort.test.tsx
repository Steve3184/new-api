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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { expect, test, vi } from 'vitest'

import type { PlaygroundConfig } from '../../../types'
import { PlaygroundEffortSelector } from '../playground-effort-selector'

function EffortControl() {
  const [value, setValue] =
    useState<PlaygroundConfig['reasoning_effort']>('default')
  return <PlaygroundEffortSelector value={value} onChange={setValue} />
}

test('shows all six levels and updates the visible and accessible selection', async () => {
  const user = userEvent.setup()
  render(<EffortControl />)
  const trigger = screen.getByRole('button', { name: 'Reasoning effort' })
  await user.click(trigger)
  expect(trigger).toHaveAttribute('aria-expanded', 'true')
  for (const effort of ['none', 'low', 'medium', 'high', 'xhigh', 'max']) {
    expect(
      screen.getByRole('menuitemradio', { name: effort })
    ).toBeInTheDocument()
  }
  await user.click(
    screen.getByRole('menuitemradio', { name: 'none' })
  )
  expect(trigger).toHaveTextContent('none')
  await user.click(trigger)
  expect(
    screen.getByRole('menuitemradio', { name: 'none' })
  ).toHaveAttribute('aria-checked', 'true')
})

test('supports opening, selecting and returning focus with the keyboard', async () => {
  const user = userEvent.setup()
  render(<EffortControl />)
  const trigger = screen.getByRole('button', { name: 'Reasoning effort' })
  trigger.focus()
  await user.keyboard('{ArrowDown}')
  await waitFor(() =>
    expect(screen.getByRole('menuitemradio', { name: 'Default' })).toHaveFocus()
  )
  await user.keyboard('{End}{Enter}')
  expect(trigger).toHaveTextContent('max')
  await waitFor(() => expect(trigger).toHaveFocus())
  expect(trigger).toHaveAttribute('aria-expanded', 'false')
})

test('disabled control cannot open or change the selection', async () => {
  const user = userEvent.setup()
  const onChange = vi.fn()
  render(<PlaygroundEffortSelector value='high' disabled onChange={onChange} />)
  const trigger = screen.getByRole('button', { name: 'Reasoning effort' })
  expect(trigger).toBeDisabled()
  await user.click(trigger)
  expect(screen.queryByRole('menu')).not.toBeInTheDocument()
  expect(onChange).not.toHaveBeenCalled()
})
