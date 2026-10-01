/* Copyright (C) 2023-2026 QuantumNous */
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { expect, test, vi } from 'vitest'

import { PlaygroundEmptyState } from '../playground-empty-state'

const state = vi.hoisted(() => ({ presets: null as unknown }))
vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({
    status: { playground: { chat_presets: state.presets } },
  }),
}))

test('legacy settings show built-in presets and empty custom lists hide them', () => {
  state.presets = null
  const view = render(<PlaygroundEmptyState onSelectPrompt={vi.fn()} />)
  expect(screen.getAllByRole('button')).toHaveLength(4)
  state.presets = []
  view.rerender(<PlaygroundEmptyState onSelectPrompt={vi.fn()} />)
  expect(screen.queryAllByRole('button')).toHaveLength(0)
})

test('custom title remains literal and clicking sends the exact configured content', async () => {
  state.presets = [
    {
      title: '<Custom title>',
      icon: 'InvalidName',
      content: '  exact\nprompt  ',
    },
  ]
  const onSelect = vi.fn()
  render(<PlaygroundEmptyState onSelectPrompt={onSelect} />)
  await userEvent.click(screen.getByRole('button', { name: '<Custom title>' }))
  expect(onSelect).toHaveBeenCalledWith('  exact\nprompt  ')
})
