/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { useState } from 'react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../components/settings-page-context'
import { StatusCheckSection } from '../content/status-check-section'

const mutateAsync = vi.fn(
  async (_update: { key: string; value: string }) => ({ success: true })
)

vi.mock('sonner', () => ({ toast: { error: vi.fn(), success: vi.fn() } }))

vi.mock('../hooks/use-update-option', () => ({
  useUpdateOption: () => ({ mutateAsync, isPending: false }),
}))

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
  mutateAsync.mockClear()
  vi.mocked(toast.error).mockClear()
})

type SectionProps = {
  groups?: string
  flexibleMode?: string
  degradationMode?: string
}

function Harness(props: SectionProps) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setActions} />
      <SettingsPageProvider actionsContainer={actions}>
        <StatusCheckSection
          defaultValue={props.groups ?? '["default"]'}
          cacheExcludedModels='[]'
          announcement=''
          flexibleMode={props.flexibleMode ?? '{"groups":{}}'}
          degradationMode={props.degradationMode ?? '{"groups":{}}'}
          groupRatio='{"default":1,"vip":1}'
          modelRatio='{}'
        />
      </SettingsPageProvider>
    </>
  )
}

function renderSection(props: SectionProps = {}) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: ['gpt-5', 'gpt-5-mini'] },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <Harness {...props} />
    </QueryClientProvider>
  )
}

function savedFlexibleGroups(): Record<string, unknown> {
  const update = mutateAsync.mock.calls
    .map(([call]) => call)
    .find((call) => call.key === 'StatusCheckFlexibleMode')
  if (!update) throw new Error('StatusCheckFlexibleMode was not saved')
  return (JSON.parse(update.value) as { groups: Record<string, unknown> })
    .groups
}

describe('StatusCheckSection save', () => {
  it('saves a probe toggled in the group table', async () => {
    renderSection()
    fireEvent.click(
      await screen.findByRole('switch', {
        name: 'Availability probe for default',
      })
    )
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() =>
      expect(savedFlexibleGroups()).toEqual({
        default: expect.objectContaining({ enabled: true }),
      })
    )
  })

  it('saves continuous monitoring applied from the group sheet as an unlimited probe count', async () => {
    renderSection({
      flexibleMode:
        '{"groups":{"default":{"enabled":true,"idle_minutes":5,"max_consecutive_probes":40}}}',
    })
    fireEvent.click(await screen.findByRole('button', { name: 'Edit default' }))
    const sheet = await screen.findByRole('dialog')
    fireEvent.click(
      within(sheet).getByRole('switch', { name: 'Continuous monitoring' })
    )
    fireEvent.click(within(sheet).getByRole('button', { name: 'Apply' }))
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() =>
      expect(savedFlexibleGroups()).toEqual({
        default: {
          enabled: true,
          idle_minutes: 5,
          max_consecutive_probes: 0,
          model: '',
        },
      })
    )
  })

  it('applies a batch edit to every selected group and keeps each probe model', async () => {
    renderSection({
      groups: '["default","vip"]',
      flexibleMode:
        '{"groups":{"vip":{"enabled":false,"idle_minutes":30,"max_consecutive_probes":10,"model":"gpt-5"}}}',
    })
    fireEvent.click(await screen.findByRole('checkbox', { name: 'Select all' }))
    fireEvent.click(screen.getByRole('button', { name: 'Edit selected' }))
    const sheet = await screen.findByRole('dialog')
    fireEvent.click(
      within(sheet).getByRole('switch', { name: 'Enable availability probe' })
    )
    fireEvent.change(within(sheet).getByLabelText('Idle period (minutes)'), {
      target: { value: '5' },
    })
    fireEvent.click(
      within(sheet).getByRole('switch', { name: 'Continuous monitoring' })
    )
    fireEvent.click(within(sheet).getByRole('button', { name: 'Apply' }))
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() =>
      expect(savedFlexibleGroups()).toEqual({
        default: {
          enabled: true,
          idle_minutes: 5,
          max_consecutive_probes: 0,
          model: '',
        },
        vip: {
          enabled: true,
          idle_minutes: 5,
          max_consecutive_probes: 0,
          model: 'gpt-5',
        },
      })
    )
  })

  it('reports an out-of-range saved value instead of silently ignoring save', async () => {
    renderSection({
      degradationMode:
        '{"groups":{"default":{"enabled":true,"interval_minutes":3,"models":[]}}}',
    })
    fireEvent.click(await screen.findByRole('button', { name: /save/i }))

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Please fix the highlighted fields before saving'
      )
    )
    expect(mutateAsync).not.toHaveBeenCalled()
    const row = screen.getByRole('row', { name: /default/ })
    expect(within(row).getByText('Needs fixing')).toBeInTheDocument()
  })
})
