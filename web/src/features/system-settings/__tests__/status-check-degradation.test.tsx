/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { StatusCheckDegradationConfig } from '../content/status-check-degradation-config'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

function renderConfig(props: {
  enabled: boolean
  onChange: (update: unknown) => void
}) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <StatusCheckDegradationConfig
        group='vip'
        value={{
          enabled: props.enabled,
          interval_minutes: 60,
          models: [{ model: 'gpt-5', effort: 'high' }],
        }}
        onChange={props.onChange}
      />
    </QueryClientProvider>
  )
}

describe('StatusCheckDegradationConfig', () => {
  it('loads the models of its own group', async () => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: ['gpt-5', 'gpt-5-mini'] },
    })
    renderConfig({ enabled: true, onChange: vi.fn() })

    await waitFor(() =>
      expect(get).toHaveBeenCalledWith('/api/channel/models_enabled', {
        params: { group: 'vip' },
      })
    )
  })

  it('shows an effort control for each configured model', () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: [] },
    })
    renderConfig({ enabled: true, onChange: vi.fn() })

    const effort = screen.getByRole('combobox', {
      name: 'Reasoning effort for gpt-5',
    })
    expect(effort).toHaveTextContent('high')
    expect(effort).not.toHaveAttribute('aria-disabled', 'true')
  })

  it('enables detection when the switch is turned on', async () => {
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: true, data: [] },
    })
    const onChange = vi.fn()
    renderConfig({ enabled: false, onChange })

    expect(screen.getByLabelText('Check interval (minutes)')).toBeDisabled()
    await userEvent.click(
      screen.getByRole('switch', { name: 'Model degradation detection' })
    )

    expect(onChange).toHaveBeenCalledWith({ enabled: true })
  })
})
