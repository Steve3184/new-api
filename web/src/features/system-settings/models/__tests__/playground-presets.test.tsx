/* Copyright (C) 2023-2026 QuantumNous */
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useForm } from 'react-hook-form'
import { expect, test } from 'vitest'

import { Form } from '@/components/ui/form'

import { PlaygroundPresetsEditor } from '../playground-presets-editor'
import {
  DEFAULT_PLAYGROUND_SETTINGS,
  parsePlaygroundSettings,
  playgroundSettingsSchema,
} from '../playground-settings'

function Editor() {
  const form = useForm({ defaultValues: DEFAULT_PLAYGROUND_SETTINGS })
  return (
    <Form {...form}>
      <PlaygroundPresetsEditor />
    </Form>
  )
}

test('customize allows editing, reordering, deletion and restoring defaults', async () => {
  const user = userEvent.setup()
  render(<Editor />)
  await user.click(screen.getByRole('button', { name: 'Customize presets' }))
  const title = screen.getAllByRole('textbox', { name: 'Title' })[0]
  await user.clear(title)
  await user.type(title, 'Custom title')
  await user.click(screen.getAllByRole('button', { name: 'Move down' })[0])
  expect(screen.getAllByRole('textbox', { name: 'Title' })[1]).toHaveValue(
    'Custom title'
  )
  await user.click(screen.getAllByRole('button', { name: 'Delete' })[1])
  expect(screen.getAllByRole('textbox', { name: 'Title' })).toHaveLength(3)
  await user.click(screen.getByRole('button', { name: 'Add preset' }))
  expect(screen.getAllByRole('textbox', { name: 'Title' })).toHaveLength(4)
  await user.click(screen.getByRole('button', { name: 'Reset to default' }))
  expect(
    screen.getByRole('button', { name: 'Customize presets' })
  ).toBeInTheDocument()
})

test('old settings preserve model choices and preset validation rejects blank content', () => {
  const { chat_presets: _, ...legacy } = DEFAULT_PLAYGROUND_SETTINGS
  legacy.models = { ...legacy.models, chat: ['custom-model'] }
  expect(parsePlaygroundSettings(JSON.stringify(legacy))).toEqual({
    ...legacy,
    chat_presets: null,
  })
  expect(
    playgroundSettingsSchema.safeParse({
      ...legacy,
      chat_presets: [{ title: 'Title', icon: 'LuSparkles', content: ' ' }],
    }).success
  ).toBe(false)
  expect(
    playgroundSettingsSchema.parse({ ...legacy, chat_presets: [] }).chat_presets
  ).toEqual([])
})
