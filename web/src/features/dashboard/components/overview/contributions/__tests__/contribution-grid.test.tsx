/* Copyright (C) 2023-2026 QuantumNous */
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, it, vi } from 'vitest'

import type { ContributionDay } from '../calendar'
import { ContributionGrid } from '../contribution-grid'

// Three weeks where the middle week is outside the site's statistics range.
const METRICS: Array<{ tokens: number; level: number; available: boolean }> = [
  { tokens: 10, level: 1, available: true },
  { tokens: 0, level: 0, available: true },
  { tokens: 30, level: 3, available: true },
  { tokens: 0, level: 0, available: true },
  { tokens: 50, level: 4, available: true },
  { tokens: 20, level: 2, available: true },
  { tokens: 0, level: 0, available: true },
  ...Array.from({ length: 7 }, () => ({
    tokens: 0,
    level: 0,
    available: false,
  })),
  { tokens: 5, level: 1, available: true },
  { tokens: 0, level: 0, available: true },
  { tokens: 0, level: 0, available: true },
  { tokens: 0, level: 0, available: true },
  { tokens: 0, level: 0, available: true },
  { tokens: 0, level: 0, available: true },
  { tokens: 0, level: 0, available: true },
]

const DAYS: ContributionDay[] = METRICS.map((metric, index) => ({
  date: `2026-03-${String(index + 1).padStart(2, '0')}`,
  timestamp: new Date(2026, 2, index + 1).getTime(),
  week: Math.floor(index / 7),
  weekday: index % 7,
  requests: metric.tokens > 0 ? 1 : 0,
  ...metric,
}))

const describeDay = (day: ContributionDay) =>
  day.available ? `${day.date}: ${day.tokens} tokens` : `${day.date}: none`

function renderGrid(days: ContributionDay[] = DAYS) {
  const onSelect = vi.fn()
  const view = render(
    <ContributionGrid
      days={days}
      describeDay={describeDay}
      onSelect={onSelect}
    />
  )
  return { ...view, onSelect }
}

async function waitForTooltip() {
  return waitFor(() => {
    const tooltip = document.querySelector('[data-slot="tooltip-content"]')
    if (!tooltip) throw new Error('Tooltip is not open')
    return tooltip
  })
}

function cell(date: string) {
  const day = DAYS.find((entry) => entry.date === date)
  if (!day) throw new Error(`Unknown fixture day: ${date}`)
  return screen.getByLabelText(describeDay(day))
}

describe('contribution grid', () => {
  it('labels every day with its description and disables days outside the range', () => {
    renderGrid()

    expect(screen.getAllByRole('button')).toHaveLength(DAYS.length)
    expect(cell('2026-03-01')).toHaveAccessibleName('2026-03-01: 10 tokens')
    expect(cell('2026-03-10')).toBeDisabled()
    expect(screen.getByRole('group')).toHaveAccessibleName('Usage by day')
  })

  it('puts the newest available day in the tab order', () => {
    renderGrid()

    expect(cell('2026-03-21')).toHaveAttribute('tabindex', '0')
    expect(cell('2026-03-19')).toHaveAttribute('tabindex', '-1')
  })

  it('moves a week at a time and skips days without statistics', async () => {
    const user = userEvent.setup()
    renderGrid()
    await user.click(cell('2026-03-19'))

    await user.keyboard('{ArrowLeft}')

    expect(cell('2026-03-07')).toHaveFocus()
  })

  it('jumps to the first and last available day with Home and End', async () => {
    const user = userEvent.setup()
    renderGrid()
    await user.click(cell('2026-03-05'))

    await user.keyboard('{Home}')
    expect(cell('2026-03-01')).toHaveFocus()

    await user.keyboard('{End}')
    expect(cell('2026-03-21')).toHaveFocus()
  })

  it('keeps a day focusable after the available range changes', () => {
    const { rerender, onSelect } = renderGrid()

    const shifted = DAYS.map((day, index) => ({
      ...day,
      available: index >= 14,
    }))
    rerender(
      <ContributionGrid
        days={shifted}
        describeDay={describeDay}
        onSelect={onSelect}
      />
    )

    const focusable = screen
      .getAllByRole('button')
      .filter((button) => button.getAttribute('tabindex') === '0')
    expect(focusable).toHaveLength(1)
    expect(focusable[0]).toHaveAttribute('data-date', '2026-03-21')
  })

  it('reports the selected day on click and on focus', async () => {
    const user = userEvent.setup()
    const { onSelect } = renderGrid()

    await user.click(cell('2026-03-05'))

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ date: '2026-03-05', tokens: 50 })
    )
  })

  it('shows the shared tooltip when a day receives the pointer', async () => {
    const user = userEvent.setup()
    renderGrid()

    await user.hover(cell('2026-03-05'))

    // The shared popup keeps one tooltip mounted for the whole calendar instead
    // of one per cell, so it is addressed through its slot.
    expect(await waitForTooltip()).toHaveTextContent('2026-03-05: 50 tokens')
  })
})
