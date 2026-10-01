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
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import type { PlaygroundConfig } from '../../../types'
import { PlaygroundEffortSelector } from '../playground-effort-selector'

// Base UI measures edge-aligned thumbs; jsdom has no layout.
beforeEach(() => {
  vi.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(
    function (this: HTMLElement) {
      return new DOMRect(
        0,
        0,
        this.dataset.slot === 'slider-thumb' ? 28 : 256,
        28
      )
    }
  )
})
afterEach(() => vi.restoreAllMocks())

function EffortControl() {
  const [value, setValue] =
    useState<PlaygroundConfig['reasoning_effort']>('medium')
  return <PlaygroundEffortSelector value={value} onChange={setValue} />
}

test('keyboard steps through all six levels and reset restores the default', async () => {
  const user = userEvent.setup()
  render(<EffortControl />)
  const trigger = screen.getByRole('button', { name: 'Reasoning effort' })
  await user.click(trigger)
  expect(trigger).toHaveAttribute('aria-expanded', 'true')
  const slider = await screen.findByRole('slider', { name: 'Reasoning effort' })
  expect(trigger).toHaveTextContent('medium')
  slider.focus()
  await user.keyboard('{Home}')
  for (const effort of ['none', 'low', 'medium', 'high', 'xhigh', 'max']) {
    expect(trigger).toHaveTextContent(effort)
    expect(slider).toHaveAttribute('aria-valuetext', effort)
    await user.keyboard('{ArrowRight}')
  }
  await user.click(screen.getByRole('button', { name: 'Reset to medium' }))
  expect(trigger).toHaveTextContent('medium')
  expect(slider).toHaveValue('2')
  expect(screen.getByRole('button', { name: 'Reset to medium' })).toBeDisabled()
})

test('keyboard opens the popover and Escape restores trigger focus', async () => {
  const user = userEvent.setup()
  render(<EffortControl />)
  const trigger = screen.getByRole('button', { name: 'Reasoning effort' })
  trigger.focus()
  await user.keyboard('{Enter}')
  expect(
    await screen.findByRole('slider', { name: 'Reasoning effort' })
  ).toBeInTheDocument()
  await user.keyboard('{Escape}')
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
  expect(screen.queryByRole('slider')).not.toBeInTheDocument()
  expect(onChange).not.toHaveBeenCalled()
})
