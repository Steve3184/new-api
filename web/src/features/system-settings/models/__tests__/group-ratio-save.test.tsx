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
import { fireEvent, render, screen } from '@testing-library/react'
import { useForm, type FieldErrors, type Resolver } from 'react-hook-form'
import { toast } from 'sonner'
import { describe, expect, test, vi } from 'vitest'

import { GroupRatioForm } from '../group-ratio-form'

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
  },
}))

vi.mock('../../components/settings-page-context', () => ({
  SettingsPageActionsPortal: (props: { children: React.ReactNode }) =>
    props.children,
}))

vi.mock('../group-ratio-visual-editor', () => ({
  GroupRatioVisualEditor: () => null,
}))

vi.mock('../group-special-usable-editor', () => ({
  GroupSpecialUsableRulesEditor: () => null,
}))

vi.mock('../group-coding-model-editor', () => ({
  GroupCodingModelEditor: () => null,
}))

type GroupFormValues = {
  GroupRatio: string
  TopupGroupRatio: string
  UserUsableGroups: string
  GroupGroupRatio: string
  AutoGroups: string
  AutoGroupDescription: string
  MaxTokenAutoGroups: number
  DefaultUseAutoGroup: boolean
  GroupSpecialUsableGroup: string
  GroupDefaultModel: string
  GroupRetryTimes: string
  ModelSquareVisibleGroups: string
}

const values: GroupFormValues = {
  GroupRatio: '{}',
  TopupGroupRatio: '{}',
  UserUsableGroups: '{}',
  GroupGroupRatio: '{}',
  AutoGroups: '[]',
  AutoGroupDescription: '',
  MaxTokenAutoGroups: 5,
  DefaultUseAutoGroup: false,
  GroupSpecialUsableGroup: '{}',
  GroupDefaultModel: '{}',
  GroupRetryTimes: '{}',
  ModelSquareVisibleGroups: '[]',
}

function TestForm(props: {
  errors?: FieldErrors<GroupFormValues>
  onSave: (values: GroupFormValues) => Promise<void>
}) {
  const resolver: Resolver<GroupFormValues> = async () => {
    if (props.errors) {
      return { values: {}, errors: props.errors }
    }
    return { values, errors: {} }
  }
  const form = useForm<GroupFormValues>({ defaultValues: values, resolver })

  return <GroupRatioForm form={form} onSave={props.onSave} isSaving={false} />
}

describe('group ratio save action', () => {
  test('shows the hidden validation error instead of ignoring the click', async () => {
    render(
      <TestForm
        errors={{
          GroupRetryTimes: {
            type: 'custom',
            message: 'Retry counts must be between 0 and 10',
          },
        }}
        onSave={vi.fn()}
      />
    )
    fireEvent.click(screen.getByRole('button', { name: 'Save group ratios' }))

    await vi.waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Retry counts must be between 0 and 10'
      )
    )
  })

  test('submits valid group ratio values from the portaled action', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined)

    render(<TestForm onSave={onSave} />)
    fireEvent.click(screen.getByRole('button', { name: 'Save group ratios' }))

    await vi.waitFor(() =>
      expect(onSave).toHaveBeenCalledWith(values, undefined)
    )
  })
})
