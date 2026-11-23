/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import {
  cleanup,
  fireEvent,
  render,
  screen,
  within,
} from '@testing-library/react'
import { useState } from 'react'
import { afterEach, describe, expect, it, vi } from 'vitest'

import {
  type StatusCheckGroupRow,
  StatusCheckGroupTable,
} from '../content/status-check-group-table'

afterEach(() => {
  cleanup()
})

function row(
  group: string,
  overrides: {
    flexible?: Partial<StatusCheckGroupRow['flexible']>
    degradation?: Partial<StatusCheckGroupRow['degradation']>
  } = {}
): StatusCheckGroupRow {
  return {
    group,
    flexible: {
      enabled: true,
      idle_minutes: 5,
      max_consecutive_probes: 40,
      model: '',
      ...overrides.flexible,
    },
    degradation: {
      enabled: false,
      interval_minutes: 60,
      models: [],
      ...overrides.degradation,
    },
    invalid: false,
  }
}

function SelectableTable(props: {
  rows: StatusCheckGroupRow[]
  onEdit: (groups: string[]) => void
}) {
  const [selected, setSelected] = useState<string[]>([])
  return (
    <StatusCheckGroupTable
      rows={props.rows}
      selected={selected}
      onSelectedChange={setSelected}
      onProbeToggle={vi.fn()}
      onEdit={props.onEdit}
    />
  )
}

describe('StatusCheckGroupTable', () => {
  it.each([
    [
      'continuous',
      row('premium', { flexible: { max_consecutive_probes: 0 } }),
      'Every 5 min while idle · continuous',
    ],
    ['limited', row('premium'), 'After 5 min idle · up to 40 probes'],
    ['off', row('premium', { flexible: { enabled: false } }), 'Off'],
    [
      'covered by degradation',
      row('premium', { degradation: { enabled: true } }),
      'Covered by degradation detection',
    ],
  ])('summarizes a %s probe schedule in its row', (_, groupRow, summary) => {
    render(<SelectableTable rows={[groupRow]} onEdit={vi.fn()} />)

    const scheduleColumn = screen
      .getAllByRole('columnheader')
      .findIndex((header) => header.textContent === 'Schedule')
    const tableRow = screen.getByRole('row', { name: /premium/ })
    expect(within(tableRow).getAllByRole('cell')[scheduleColumn]).toHaveTextContent(
      summary
    )
  })

  it('disables the row probe switch while degradation detection covers the group', () => {
    render(
      <SelectableTable
        rows={[row('a', { degradation: { enabled: true } })]}
        onEdit={vi.fn()}
      />
    )

    expect(
      screen.getByRole('switch', { name: 'Availability probe for a' })
    ).toHaveAttribute('aria-disabled', 'true')
  })

  it('keeps the batch actions in place and enables them once a row is selected', () => {
    const onEdit = vi.fn()
    render(<SelectableTable rows={[row('a'), row('b')]} onEdit={onEdit} />)
    const editSelected = screen.getByRole('button', { name: 'Edit selected' })
    expect(editSelected).toBeDisabled()

    fireEvent.click(screen.getByRole('checkbox', { name: 'Select group b' }))
    expect(screen.getByRole('button', { name: 'Edit selected' })).toBe(
      editSelected
    )
    expect(editSelected).toBeEnabled()
    fireEvent.click(editSelected)

    expect(onEdit).toHaveBeenCalledWith(['b'])
  })

  it('selects every group from the header checkbox and clears the selection', () => {
    render(<SelectableTable rows={[row('a'), row('b')]} onEdit={vi.fn()} />)

    fireEvent.click(screen.getByRole('checkbox', { name: 'Select all' }))
    expect(screen.getByRole('checkbox', { name: 'Select group a' })).toBeChecked()
    expect(screen.getByRole('checkbox', { name: 'Select group b' })).toBeChecked()

    fireEvent.click(screen.getByRole('button', { name: 'Clear selection' }))
    expect(
      screen.getByRole('checkbox', { name: 'Select group a' })
    ).not.toBeChecked()
  })
})
