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
import { zodResolver } from '@hookform/resolvers/zod'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { describe, expect, it, vi } from 'vitest'
import { z } from 'zod'

import { SettingsPageProvider } from '../../components/settings-page-context'
import { positiveIntegerSchema } from '../../utils/numeric-field'
import { GroupRatioForm } from '../group-ratio-form'

const defaults = {
  GroupRatio: '{"default":1,"vip":0.8}',
  TopupGroupRatio: '{"vip":1.2}',
  UserUsableGroups: '{"default":"Standard access","vip":"Premium access"}',
  GroupGroupRatio: '{}',
  AutoGroups: '["default","vip"]',
  AutoGroupDescription: '',
  MaxTokenAutoGroups: 5,
  DefaultUseAutoGroup: false,
  GroupSpecialUsableGroup: '{}',
  GroupDefaultModel: '{}',
  GroupRetryTimes: '{}',
  ModelSquareVisibleGroups: '{}',
  ModelSquareGroupCategories: '{}',
}

const schema = z.object({
  GroupRatio: z.string(),
  TopupGroupRatio: z.string(),
  UserUsableGroups: z.string(),
  GroupGroupRatio: z.string(),
  AutoGroups: z.string(),
  AutoGroupDescription: z.string(),
  MaxTokenAutoGroups: positiveIntegerSchema('Enter a positive integer'),
  DefaultUseAutoGroup: z.boolean(),
  GroupSpecialUsableGroup: z.string(),
  GroupDefaultModel: z.string(),
  GroupRetryTimes: z.string(),
  ModelSquareVisibleGroups: z.string(),
  ModelSquareGroupCategories: z.string(),
})

function Fixture(props: {
  onSave?: (values: typeof defaults) => Promise<void>
  initial?: Partial<typeof defaults>
  isSaving?: boolean
}) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  const queryClient = useMemo(
    () => new QueryClient({ defaultOptions: { queries: { retry: false } } }),
    []
  )
  const form = useForm({
    defaultValues: { ...defaults, ...props.initial },
    resolver: zodResolver(schema),
  })

  return (
    <QueryClientProvider client={queryClient}>
      <SettingsPageProvider actionsContainer={actions}>
        <div ref={setActions} />
        <GroupRatioForm
          form={form}
          onSave={props.onSave ?? (async () => {})}
          isSaving={props.isSaving ?? false}
        />
      </SettingsPageProvider>
    </QueryClientProvider>
  )
}

describe('group settings workspace', () => {
  it('does not offer per-group subscription switches because plan bindings define subscription groups', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)

    expect(
      screen.queryByRole('switch', { name: /^Subscription group: / })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('switch', { name: /^Automatically create API key/ })
    ).not.toBeInTheDocument()
    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0][0]).not.toHaveProperty('SubscriptionGroups')
  })

  it('preserves pricing edits when switching between visual and JSON editors', async () => {
    const user = userEvent.setup()
    render(<Fixture />)

    const ratio = screen.getByDisplayValue('0.8')
    await user.clear(ratio)
    await user.type(ratio, '0.6')
    await user.click(screen.getByRole('button', { name: 'Switch to JSON' }))
    expect(screen.getByText('Group ratios')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Switch to Visual' }))

    expect(screen.getByDisplayValue('0.6')).toBeVisible()
  })

  it('adds a pricing group and saves the updated settings', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)

    const addGroupButtons = screen.getAllByRole('button', { name: 'Add group' })
    await user.click(addGroupButtons[0])
    expect(screen.getByDisplayValue('group_1')).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))

    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(JSON.parse(onSave.mock.calls[0][0].GroupRatio)).toEqual({
      default: 1,
      vip: 0.8,
      group_1: 1,
    })
  })

  it('keeps auto group settings and descriptions in the submitted values', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)

    await user.clear(
      screen.getByRole('textbox', { name: 'Auto group description' })
    )
    await user.type(
      screen.getByRole('textbox', { name: 'Auto group description' }),
      'Balanced routing\nFallback enabled'
    )
    await user.click(
      screen.getByRole('switch', { name: 'Default to auto groups' })
    )
    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))

    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0][0]).toEqual(
      expect.objectContaining({
        AutoGroupDescription: 'Balanced routing\nFallback enabled',
        DefaultUseAutoGroup: true,
      })
    )
  })

  it('shows empty state guidance and disables save during a pending request', async () => {
    render(
      <Fixture
        isSaving
        initial={{
          GroupRatio: '{}',
          TopupGroupRatio: '{}',
          UserUsableGroups: '{}',
          AutoGroups: '[]',
        }}
      />
    )

    expect(
      screen.getByText('No groups yet. Add a group to get started.')
    ).toBeVisible()
    expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
    expect(
      screen.getByText(
        'Each rule reads as a sentence: users of one group pay a special ratio when billed as another group. Without a rule, the billing group base ratio applies.'
      )
    ).toBeVisible()
  })

  it('prepends consecutive new groups and saves them without changing existing settings', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)

    await user.click(screen.getAllByRole('button', { name: 'Add group' })[0])
    await user.click(screen.getAllByRole('button', { name: 'Add group' })[0])
    expect(
      screen
        .getAllByRole<HTMLInputElement>('textbox', { name: 'Group name' })
        .map((input) => input.value)
    ).toEqual(['group_2', 'group_1', 'default', 'vip'])

    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    const saved = onSave.mock.calls[0][0]
    expect(JSON.parse(saved.GroupRatio)).toEqual({
      group_2: 1,
      group_1: 1,
      default: 1,
      vip: 0.8,
    })
    expect(JSON.parse(saved.UserUsableGroups)).toEqual({
      group_2: '',
      group_1: '',
      default: 'Standard access',
      vip: 'Premium access',
    })
    expect(JSON.parse(saved.TopupGroupRatio)).toEqual({ vip: 1.2 })
    expect(saved.AutoGroups).toBe(defaults.AutoGroups)
  })

  it('fits group descriptions to one line and grows or shrinks with edited lines', () => {
    render(<Fixture />)
    const description = screen.getByDisplayValue('Standard access')
    expect(description).toHaveAttribute('rows', '1')
    expect(description).toHaveClass('min-h-0', 'field-sizing-content')
    expect(description).not.toHaveClass('min-h-16')

    fireEvent.change(description, {
      target: { value: 'Standard access\nImages supported\nVideo supported' },
    })
    expect(description).toHaveAttribute('rows', '3')
    fireEvent.change(description, { target: { value: 'Standard access' } })
    expect(description).toHaveAttribute('rows', '1')
    fireEvent.change(description, { target: { value: '' } })
    expect(description).toHaveAttribute('rows', '1')
  })
})

describe('model plaza group categories', () => {
  it('edits categories in their own card instead of a pricing table column', () => {
    render(
      <Fixture initial={{ ModelSquareGroupCategories: '{"vip":"Premium"}' }} />
    )

    expect(
      screen.queryByRole('columnheader', { name: 'Model plaza category' })
    ).not.toBeInTheDocument()
    const name = screen.getByRole('textbox', { name: 'Category name' })
    expect(name).toHaveValue('Premium')
    const categoryRow = name.closest('tr')
    expect(categoryRow).not.toBeNull()
    expect(within(categoryRow as HTMLElement).getByText('vip')).toBeVisible()
    expect(
      within(categoryRow as HTMLElement).queryByRole('textbox', {
        name: 'Group name',
      })
    ).not.toBeInTheDocument()
  })

  it('saves groups picked for a new category as the category map', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(<Fixture onSave={onSave} />)

    await user.click(screen.getByRole('button', { name: 'Add category' }))
    await user.type(
      screen.getByRole('textbox', { name: 'Category name' }),
      'Premium'
    )
    await user.click(screen.getByRole('combobox', { name: 'Select groups' }))
    await user.click(screen.getByRole('option', { name: 'vip' }))
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))

    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(
      JSON.parse(onSave.mock.calls[0][0].ModelSquareGroupCategories)
    ).toEqual({ vip: 'Premium' })
  })

  it('offers a group already in one category to no other category', async () => {
    const user = userEvent.setup()
    render(
      <Fixture initial={{ ModelSquareGroupCategories: '{"vip":"Premium"}' }} />
    )

    await user.click(screen.getByRole('button', { name: 'Add category' }))
    const groupSelects = screen.getAllByRole('combobox', {
      name: 'Select groups',
    })
    expect(groupSelects).toHaveLength(2)
    await user.click(groupSelects[1])

    expect(screen.getByRole('option', { name: 'default' })).toBeVisible()
    expect(
      screen.queryByRole('option', { name: 'vip' })
    ).not.toBeInTheDocument()
  })

  it('keeps groups while a category name is cleared but leaves the unnamed category out of saved settings', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    render(
      <Fixture
        onSave={onSave}
        initial={{ ModelSquareGroupCategories: '{"vip":"Premium"}' }}
      />
    )

    const name = screen.getByRole('textbox', { name: 'Category name' })
    await user.clear(name)

    expect(name).toHaveAttribute('aria-invalid', 'true')
    expect(name).toHaveAccessibleDescription('Category name is required')
    expect(
      within(name.closest('tr') as HTMLElement).getByText('vip')
    ).toBeVisible()
    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))
    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(
      JSON.parse(onSave.mock.calls[0][0].ModelSquareGroupCategories)
    ).toEqual({})
  })

  it('marks duplicate category names as invalid', async () => {
    const user = userEvent.setup()
    render(
      <Fixture initial={{ ModelSquareGroupCategories: '{"vip":"Premium"}' }} />
    )

    await user.click(screen.getByRole('button', { name: 'Add category' }))
    const names = screen.getAllByRole('textbox', { name: 'Category name' })
    await user.type(names[1], 'Premium')

    for (const name of names) {
      expect(name).toHaveAttribute('aria-invalid', 'true')
      expect(name).toHaveAccessibleDescription(
        'Duplicate category names: Premium'
      )
    }
  })

  it('saves the loaded category value unchanged after a group is picked and removed again', async () => {
    const user = userEvent.setup()
    const onSave = vi.fn(async (_values: typeof defaults) => {})
    const initialCategories = '{"default":"Zeta","vip":"Alpha"}'
    render(
      <Fixture
        onSave={onSave}
        initial={{ ModelSquareGroupCategories: initialCategories }}
      />
    )

    const alphaRow = screen
      .getByDisplayValue('Alpha')
      .closest('tr') as HTMLElement
    await user.click(
      within(alphaRow).getByRole('combobox', { name: 'Select groups' })
    )
    await user.click(screen.getByRole('option', { name: 'vip' }))
    await user.click(screen.getByRole('option', { name: 'vip' }))
    await user.keyboard('{Escape}')
    await user.click(screen.getByRole('button', { name: 'Save group ratios' }))

    await waitFor(() => expect(onSave).toHaveBeenCalled())
    expect(onSave.mock.calls[0][0].ModelSquareGroupCategories).toBe(
      JSON.stringify(JSON.parse(initialCategories), null, 2)
    )
  })

  it('warns when a categorized group is missing from the pricing table', () => {
    render(
      <Fixture initial={{ ModelSquareGroupCategories: '{"legacy":"Old"}' }} />
    )

    expect(screen.getByText('Not in pricing table: legacy')).toBeVisible()
  })

  it('shows the group category in the group details sheet', async () => {
    const user = userEvent.setup()
    render(
      <Fixture initial={{ ModelSquareGroupCategories: '{"vip":"Premium"}' }} />
    )

    await user.click(screen.getAllByRole('button', { name: 'Details' })[1])
    const sheet = await screen.findByRole('dialog')

    expect(within(sheet).getByText('Model plaza category')).toBeVisible()
    expect(within(sheet).getByText('Premium')).toBeVisible()
  })

  it('shows Uncategorized in the details of an uncategorized group named like an object property', async () => {
    const user = userEvent.setup()
    render(
      <Fixture
        initial={{
          GroupRatio: '{"constructor":1}',
          TopupGroupRatio: '{}',
          UserUsableGroups: '{}',
          AutoGroups: '[]',
        }}
      />
    )

    await user.click(screen.getByRole('button', { name: 'Details' }))
    const sheet = await screen.findByRole('dialog')

    expect(within(sheet).getByText('Uncategorized')).toBeVisible()
  })
})
