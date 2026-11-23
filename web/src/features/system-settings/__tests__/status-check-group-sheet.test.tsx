/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import type { DegradationGroupConfig } from '../content/status-check-degradation-options'
import type { FlexibleGroupConfig } from '../content/status-check-flexible-options'
import { StatusCheckGroupSheet } from '../content/status-check-group-sheet'

afterEach(() => {
  cleanup()
  vi.restoreAllMocks()
})

const limitedProbe: FlexibleGroupConfig = {
  enabled: true,
  idle_minutes: 5,
  max_consecutive_probes: 40,
  model: '',
}

const degradationOff: DegradationGroupConfig = {
  enabled: false,
  interval_minutes: 60,
  models: [],
}

function renderSheet(
  props: {
    groups?: string[]
    flexible?: FlexibleGroupConfig
    degradation?: DegradationGroupConfig
  } = {}
) {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: ['gpt-5'] },
  })
  const onApply = vi.fn()
  const onOpenChange = vi.fn()
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <StatusCheckGroupSheet
        open
        onOpenChange={onOpenChange}
        groups={props.groups ?? ['default']}
        flexible={props.flexible ?? limitedProbe}
        degradation={props.degradation ?? degradationOff}
        onApply={onApply}
      />
    </QueryClientProvider>
  )
  return { onApply, onOpenChange }
}

describe('StatusCheckGroupSheet', () => {
  it('replaces the probe limit with a daily ceiling when continuous monitoring is turned on', () => {
    const { onApply } = renderSheet()

    fireEvent.click(screen.getByRole('switch', { name: 'Continuous monitoring' }))

    expect(
      screen.queryByLabelText('Maximum consecutive probes')
    ).not.toBeInTheDocument()
    expect(
      screen.getByText(
        'At most 288 probes a day, only while the group has no traffic.'
      )
    ).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    expect(onApply).toHaveBeenCalledWith({
      flexible: { ...limitedProbe, max_consecutive_probes: 0 },
      degradation: degradationOff,
    })
  })

  it('restores the default probe limit when continuous monitoring is turned off', () => {
    renderSheet({ flexible: { ...limitedProbe, max_consecutive_probes: 0 } })

    fireEvent.click(screen.getByRole('switch', { name: 'Continuous monitoring' }))

    expect(screen.getByLabelText('Maximum consecutive probes')).toHaveValue(40)
  })

  it('keeps the sheet open and marks the field when the idle period is out of range', () => {
    const { onApply, onOpenChange } = renderSheet()
    const idle = screen.getByLabelText('Idle period (minutes)')

    fireEvent.change(idle, { target: { value: '0' } })
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))

    expect(onApply).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(idle).toHaveAttribute('aria-invalid', 'true')
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Enter a whole number from 1 to 1440'
    )
  })

  it('disables the probe switch while degradation detection covers the group', () => {
    renderSheet({ degradation: { ...degradationOff, enabled: true } })

    expect(
      screen.getByRole('switch', { name: 'Enable availability probe' })
    ).toHaveAttribute('aria-disabled', 'true')
  })

  it('edits only shared probe settings when several groups are selected', () => {
    const { onApply } = renderSheet({ groups: ['default', 'vip'] })

    expect(screen.queryByLabelText('Probe model')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('switch', { name: 'Model degradation detection' })
    ).not.toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: 'Apply' }))
    expect(onApply).toHaveBeenCalledWith({
      flexible: {
        enabled: true,
        idle_minutes: 5,
        max_consecutive_probes: 40,
      },
    })
  })
})
