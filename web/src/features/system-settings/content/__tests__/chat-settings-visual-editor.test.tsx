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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { ChatSettingsVisualEditor } from '../chat-settings-visual-editor'

let client: QueryClient
let latestSettings: Record<string, unknown> = {}

beforeEach(() => {
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  const freshAt = Date.now() + 60_000
  client.setQueryData(['status'], {}, { updatedAt: freshAt })
  client.setQueryData(
    ['user-models'],
    { success: true, data: [] },
    { updatedAt: freshAt }
  )
  client.setQueryData(
    ['user-groups'],
    { success: true, data: { default: { desc: 'Standard', ratio: 1 } } },
    { updatedAt: freshAt }
  )
  client.setQueryData(
    ['token-auto-groups'],
    { success: true, data: { groups: [], max_count: 5 } },
    { updatedAt: freshAt }
  )
  latestSettings = {}
})

afterEach(() => {
  cleanup()
  client.clear()
})

function Editor(props: { chats: string; settings: string }) {
  const [chats, setChats] = useState(props.chats)
  const [settings, setSettings] = useState(props.settings)
  return (
    <ChatSettingsVisualEditor
      value={chats}
      onChange={setChats}
      presetSettings={settings}
      onPresetSettingsChange={(value) => {
        latestSettings = JSON.parse(value)
        setSettings(value)
      }}
    />
  )
}

function renderEditor(chats: object[], settings: object) {
  render(
    <QueryClientProvider client={client}>
      <Editor
        chats={JSON.stringify(chats)}
        settings={JSON.stringify(settings)}
      />
    </QueryClientProvider>
  )
}

describe('chat preset settings editor', () => {
  it('keeps the settings of a renamed preset and drops those of a deleted one', async () => {
    const user = userEvent.setup()
    renderEditor([{ Lobe: 'https://lobe.example' }], {
      Lobe: { standalone_menu: true },
    })

    await user.click(screen.getByRole('button', { name: 'Edit' }))
    const nameInput = await screen.findByPlaceholderText(
      'Please enter chat client name'
    )
    await user.clear(nameInput)
    await user.type(nameInput, 'Lobe Chat')
    await user.click(screen.getByRole('button', { name: 'Update' }))

    await waitFor(() =>
      expect(latestSettings).toEqual({
        'Lobe Chat': { standalone_menu: true },
      })
    )
    expect(
      screen.getByRole('switch', { name: 'Show Lobe Chat as a standalone menu' })
    ).toBeChecked()

    await user.click(screen.getByRole('button', { name: 'Open menu' }))
    await user.click(await screen.findByRole('menuitem', { name: /Delete/ }))

    await waitFor(() => expect(latestSettings).toEqual({}))
  })

  it('asks for the key settings when the auto key is first turned on and keeps the default name implicit', async () => {
    const user = userEvent.setup()
    renderEditor([{ Lobe: 'https://lobe.example/?key={key}' }], {})
    const autoKeySwitch = screen.getByRole('switch', {
      name: 'Create an API key for Lobe',
    })

    await user.click(autoKeySwitch)

    expect(await screen.findByText('API key for Lobe')).toBeInTheDocument()
    expect(screen.getByDisplayValue('app-Lobe')).toBeInTheDocument()
    expect(autoKeySwitch).not.toBeChecked()

    const saveButton = screen.getByRole('button', { name: 'Save changes' })
    await waitFor(() => expect(saveButton).toBeEnabled())
    await user.click(saveButton)

    await waitFor(() =>
      expect(latestSettings).toMatchObject({
        Lobe: { auto_key: { enabled: true, name: '' } },
      })
    )
    expect(autoKeySwitch).toBeChecked()
  })
})
