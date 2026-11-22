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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useMemo, useState } from 'react'
import { describe, expect, it, vi } from 'vitest'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { RatioSettingsCard } from '../ratio-settings-card'

const mutateAsync = vi.fn(
  async (_request: { key: string; value: unknown }) => ({
    success: true,
  })
)

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), info: vi.fn(), success: vi.fn() },
}))

vi.mock('../../hooks/use-update-option', () => ({
  useUpdateOption: () => ({ mutateAsync, isPending: false }),
}))

vi.mock('@/features/model-pricing/api', () => ({
  buildPricingChanges: vi.fn(() => []),
  useModelPricing: () => ({
    data: undefined,
    isError: false,
    refetch: vi.fn(),
  }),
  useSaveModelPricing: () => ({
    mutateAsync: vi.fn(),
    isPending: false,
    isError: false,
    reset: vi.fn(),
  }),
}))

const modelDefaults = {
  ModelPrice: '{}',
  ModelRatio: '{}',
  CacheRatio: '{}',
  CreateCacheRatio: '{}',
  CompletionRatio: '{}',
  ImageRatio: '{}',
  AudioRatio: '{}',
  AudioCompletionRatio: '{}',
  ExposeRatioEnabled: false,
  BillingMode: '{}',
  BillingExpr: '{}',
  PluginBillingExpr: '{}',
}

const groupDefaults = {
  GroupRatio: '{"default":1,"vip":0.8}',
  TopupGroupRatio: '{}',
  UserUsableGroups: '{"default":"Standard access"}',
  GroupGroupRatio: '{}',
  AutoGroups: '[]',
  AutoGroupDescription: '',
  MaxTokenAutoGroups: 5,
  DefaultUseAutoGroup: false,
  GroupSpecialUsableGroup: '{}',
  GroupDefaultModel: '{}',
  GroupRetryTimes: '{}',
  ModelSquareVisibleGroups: '[]',
  ModelSquareGroupCategories: '{"default":"Basic","vip":"Premium"}',
}

function Fixture() {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  const queryClient = useMemo(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
    []
  )
  return (
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actions}>
        <div ref={setActions} />
        <RatioSettingsCard
          titleKey='Group Pricing'
          modelDefaults={modelDefaults}
          groupDefaults={groupDefaults}
          toolPricesDefault='{}'
          visibleTabs={['groups']}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

describe('group pricing save', () => {
  it('drops the category of a deleted pricing group from the saved categories', async () => {
    const user = userEvent.setup()
    render(<Fixture />)

    const vipRow = screen.getByDisplayValue('vip').closest('tr') as HTMLElement
    await user.click(within(vipRow).getByRole('button', { name: 'Delete' }))
    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))

    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        key: 'console_setting.model_square_group_categories',
        value: '{"default":"Basic"}',
      })
    )
  })
})
