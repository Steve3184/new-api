/* Copyright (C) 2023-2026 QuantumNous */
import { fireEvent, render, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type { ContributionDay } from '../calendar'
import ContributionsScene from '../contributions-scene'
import { buildRamp, layoutIsoCalendar } from '../iso'

// jsdom has no canvas backend, so the 2D context is the external boundary that
// gets faked. Geometry, colours and hit testing stay real.
const painted: string[] = []

function fakeContext(): CanvasRenderingContext2D {
  const context = {
    fillStyle: '#000',
    setTransform: vi.fn(),
    clearRect: vi.fn(),
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    closePath: vi.fn(),
    fill: vi.fn(() => painted.push(String(context.fillStyle))),
    fillRect: vi.fn(),
    getImageData: () => ({ data: [20, 90, 200, 255] }),
    getContext: undefined,
  }
  return context as unknown as CanvasRenderingContext2D
}

const DAYS: ContributionDay[] = [
  {
    date: '2026-05-18',
    timestamp: new Date(2026, 4, 18).getTime(),
    tokens: 0,
    requests: 0,
    week: 0,
    weekday: 0,
    level: 0,
    available: true,
  },
  {
    date: '2026-05-20',
    timestamp: new Date(2026, 4, 20).getTime(),
    tokens: 900,
    requests: 3,
    week: 2,
    weekday: 2,
    level: 4,
    available: true,
  },
]

const describeDay = (day: ContributionDay) =>
  `${day.date}: ${day.tokens} tokens`

beforeEach(() => {
  painted.length = 0
})

afterEach(() => {
  vi.restoreAllMocks()
})

describe('contributions 3D scene', () => {
  it('paints three shaded faces for every day of the calendar', async () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
      fakeContext()
    )

    render(
      <ContributionsScene
        days={DAYS}
        describeDay={describeDay}
        onSelect={vi.fn()}
        onUnavailable={vi.fn()}
      />
    )

    await waitFor(() => expect(painted).toHaveLength(DAYS.length * 3))
    // Top and the two sides must differ, otherwise the cubes read as flat tiles.
    expect(new Set(painted).size).toBeGreaterThanOrEqual(3)
  })

  it('asks for the 2D fallback when the canvas cannot be drawn', () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(null)
    const onUnavailable = vi.fn()

    render(
      <ContributionsScene
        days={DAYS}
        describeDay={describeDay}
        onSelect={vi.fn()}
        onUnavailable={onUnavailable}
      />
    )

    expect(onUnavailable).toHaveBeenCalledTimes(1)
  })

  it('selects the day whose cube sits under the pointer and shows its tooltip', async () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
      fakeContext()
    )
    const onSelect = vi.fn()
    const { container } = render(
      <ContributionsScene
        days={DAYS}
        describeDay={describeDay}
        onSelect={onSelect}
        onUnavailable={vi.fn()}
      />
    )
    const canvas = container.querySelector('canvas')
    if (!canvas) throw new Error('Scene canvas is missing')

    // Rebuild the same geometry the component renders and aim at the busy day.
    const layout = layoutIsoCalendar(
      DAYS,
      buildRamp(
        { r: 20, g: 90, b: 200 },
        { r: 20, g: 90, b: 200 },
        { r: 20, g: 90, b: 200 }
      )
    )
    const target = layout.cubes.find((cube) => cube.day.tokens === 900)
    if (!target) throw new Error('Fixture cube is missing')
    const centre = {
      x: (target.top[0].x + target.top[2].x) / 2,
      y: (target.top[0].y + target.top[2].y) / 2,
    }
    // The canvas is scaled to its box, so map layout units back to client pixels.
    vi.spyOn(canvas, 'getBoundingClientRect').mockReturnValue(
      new DOMRect(0, 0, layout.width, layout.height)
    )

    fireEvent.pointerMove(canvas, {
      clientX: centre.x,
      clientY: centre.y,
      pointerType: 'mouse',
    })

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({ date: '2026-05-20', tokens: 900 })
    )
    await waitFor(() => {
      const tooltip = document.querySelector('[data-slot="tooltip-content"]')
      expect(tooltip).toHaveTextContent('2026-05-20: 900 tokens')
    })
  })

  it('ignores touch moves so a swipe never selects a day', async () => {
    vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
      fakeContext()
    )
    const onSelect = vi.fn()
    const { container } = render(
      <ContributionsScene
        days={DAYS}
        describeDay={describeDay}
        onSelect={onSelect}
        onUnavailable={vi.fn()}
      />
    )
    const canvas = container.querySelector('canvas')
    if (!canvas) throw new Error('Scene canvas is missing')

    fireEvent.pointerMove(canvas, {
      clientX: 40,
      clientY: 40,
      pointerType: 'touch',
    })

    expect(onSelect).not.toHaveBeenCalled()
  })
})
