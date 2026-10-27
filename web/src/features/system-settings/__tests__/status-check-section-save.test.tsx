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
} from '@testing-library/react'
import { useState } from 'react'
import { toast } from 'sonner'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { SettingsPageProvider } from '../components/settings-page-context'
import { StatusCheckSection } from '../content/status-check-section'

const mutateAsync = vi.fn(async () => ({ success: true }))

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

function Harness(props: { flexibleMode?: string; degradationMode?: string }) {
  const [actions, setActions] = useState<HTMLDivElement | null>(null)
  return (
    <>
      <div ref={setActions} />
      <SettingsPageProvider actionsContainer={actions}>
        <StatusCheckSection
          defaultValue='["default"]'
          cacheExcludedModels='[]'
          announcement=''
          flexibleMode={props.flexibleMode ?? '{"groups":{}}'}
          degradationMode={props.degradationMode ?? '{"groups":{}}'}
          groupRatio='{"default":1}'
          modelRatio='{}'
        />
      </SettingsPageProvider>
    </>
  )
}

function renderSection(
  props: { flexibleMode?: string; degradationMode?: string } = {}
) {
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

describe('StatusCheckSection save', () => {
  it('saves a changed per-group probe setting', async () => {
    renderSection()
    fireEvent.click(await screen.findByRole('switch', { name: 'default' }))
    fireEvent.click(screen.getByRole('button', { name: /save/i }))
    await waitFor(() =>
      expect(mutateAsync).toHaveBeenCalledWith({
        key: 'StatusCheckFlexibleMode',
        value: expect.stringContaining('"enabled":true'),
      })
    )
  })

  it('reports an out-of-range per-group value instead of silently ignoring save', async () => {
    renderSection()
    fireEvent.click(
      await screen.findByRole('switch', { name: 'Model degradation detection' })
    )
    const interval = screen.getByLabelText('Check interval (minutes)')
    fireEvent.change(interval, { target: { value: '3' } })
    fireEvent.click(screen.getByRole('button', { name: /save/i }))

    await waitFor(() =>
      expect(toast.error).toHaveBeenCalledWith(
        'Please fix the highlighted fields before saving'
      )
    )
    expect(mutateAsync).not.toHaveBeenCalled()
    expect(interval).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Enter a whole number from 5 to 10080'
    )
  })
})
