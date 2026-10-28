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
import {
  createMemoryHistory,
  createRootRoute,
  createRouter,
  RouterProvider,
} from '@tanstack/react-router'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MessageSquare } from 'lucide-react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { SidebarMenu, SidebarProvider } from '@/components/ui/sidebar'
import { api } from '@/lib/api'
import { useAuthStore } from '@/stores/auth-store'

import { ChatPresetsItem } from '../chat-presets-item'

let client: QueryClient

beforeEach(() => {
  window.localStorage.clear()
  useAuthStore.getState().auth.setUser({ id: 1, username: 'alice', role: 1 })
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
})

afterEach(() => {
  cleanup()
  client.clear()
  vi.restoreAllMocks()
  useAuthStore.setState(useAuthStore.getInitialState(), true)
  window.localStorage.clear()
})

async function renderChatPresets(
  chats: Array<Record<string, string>>,
  settings: Record<string, { standalone_menu?: boolean; auto_key?: boolean }>
) {
  client.setQueryData(
    ['status'],
    { chats, chat_preset_settings: settings },
    { updatedAt: Date.now() + 60_000 }
  )
  const router = createRouter({
    routeTree: createRootRoute({
      component: () => (
        <SidebarProvider>
          <SidebarMenu>
            <ChatPresetsItem
              item={{ title: 'Chat', icon: MessageSquare, type: 'chat-presets' }}
            />
          </SidebarMenu>
        </SidebarProvider>
      ),
    }),
    history: createMemoryHistory({ initialEntries: ['/'] }),
  })
  await router.load()
  render(
    <QueryClientProvider client={client}>
      <RouterProvider router={router} />
    </QueryClientProvider>
  )
}

describe('chat presets sidebar entries', () => {
  it('shows standalone presets as top-level entries and keeps the others in the Chat submenu', async () => {
    await renderChatPresets(
      [
        { Lobe: 'https://lobe.example/?key={key}' },
        { Cherry: 'cherrystudio://providers?data={cherryConfig}' },
        { AMA: 'ama://set-api-key?key={key}' },
      ],
      { Lobe: { standalone_menu: true }, AMA: { standalone_menu: true } }
    )

    expect(await screen.findByRole('link', { name: 'Lobe' })).toHaveAttribute(
      'href',
      '/chat/0'
    )
    expect(screen.getByRole('button', { name: 'AMA' })).toBeInTheDocument()
    expect(screen.queryByText('Cherry')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Chat' }))

    expect(await screen.findByText('Cherry')).toBeInTheDocument()
    expect(screen.getAllByText('Lobe')).toHaveLength(1)
    expect(screen.getAllByText('AMA')).toHaveLength(1)
  })

  it('drops the Chat submenu when every preset is standalone', async () => {
    await renderChatPresets([{ Lobe: 'https://lobe.example' }], {
      Lobe: { standalone_menu: true },
    })

    expect(await screen.findByRole('link', { name: 'Lobe' })).toBeVisible()
    expect(
      screen.queryByRole('button', { name: 'Chat' })
    ).not.toBeInTheDocument()
  })

  it('opens an external standalone preset with the key created for that preset', async () => {
    const post = vi.spyOn(api, 'post').mockResolvedValue({
      data: { success: true, data: { key: 'preset-key' } },
    })
    const open = vi.spyOn(window, 'open').mockReturnValue(null)
    await renderChatPresets([{ AMA: 'ama://set-api-key?key={key}' }], {
      AMA: { standalone_menu: true, auto_key: true },
    })

    await userEvent.click(await screen.findByRole('button', { name: 'AMA' }))

    await waitFor(() =>
      expect(open).toHaveBeenCalledWith(
        'ama://set-api-key?key=sk-preset-key',
        '_blank',
        'noopener'
      )
    )
    expect(post).toHaveBeenCalledWith('/api/token/chat-preset-key', {
      preset: 'AMA',
    })
  })
})
