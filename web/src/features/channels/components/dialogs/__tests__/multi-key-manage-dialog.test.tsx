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
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

const mocks = vi.hoisted(() => ({
  getMultiKeyStatus: vi.fn(),
  useChannels: vi.fn(),
}))

vi.mock('../../../api', () => ({
  deleteDisabledMultiKeys: vi.fn(),
  deleteMultiKey: vi.fn(),
  disableAllMultiKeys: vi.fn(),
  disableMultiKey: vi.fn(),
  enableAllMultiKeys: vi.fn(),
  enableMultiKey: vi.fn(),
  getMultiKeyStatus: mocks.getMultiKeyStatus,
}))

vi.mock('../../channels-provider', () => ({
  useChannels: mocks.useChannels,
}))

const { MultiKeyManageDialog } = await import('../multi-key-manage-dialog')

describe('multi-key management dialog', () => {
  beforeEach(() => {
    mocks.getMultiKeyStatus.mockResolvedValue({
      success: true,
      data: {
        keys: [],
        total: 0,
        page: 1,
        page_size: 10,
        total_pages: 0,
        enabled_count: 0,
        manual_disabled_count: 0,
        auto_disabled_count: 0,
      },
    })
    mocks.useChannels.mockReturnValue({
      currentRow: {
        id: 1,
        name: 'multi-key channel',
        channel_info: { multi_key_mode: 'random' },
      },
    })
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'admin',
      role: 100,
    })
  })

  test('uses the wide desktop layout so table actions remain visible', async () => {
    const queryClient = new QueryClient()

    render(
      <QueryClientProvider client={queryClient}>
        <MultiKeyManageDialog open onOpenChange={() => undefined} />
      </QueryClientProvider>
    )

    const dialog = await screen.findByRole('dialog')
    expect(dialog).toHaveClass('sm:max-w-5xl')
  })
})
