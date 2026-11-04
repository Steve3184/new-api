/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, it } from 'vitest'

import type { ContributionDay } from '../calendar'
import {
  buildRamp,
  hitTestIso,
  isoCubeHeight,
  ISO_SLOPE,
  layoutIsoCalendar,
  parseRgb,
} from '../iso'

const RAMP = buildRamp(
  { r: 255, g: 255, b: 255 },
  { r: 20, g: 90, b: 200 },
  { r: 230, g: 230, b: 232 }
)

function day(
  week: number,
  weekday: number,
  tokens: number,
  level: number,
  available = true
): ContributionDay {
  return {
    date: `2026-01-${String(week * 7 + weekday + 1).padStart(2, '0')}`,
    timestamp: new Date(2026, 0, week * 7 + weekday + 1).getTime(),
    tokens,
    requests: tokens > 0 ? 1 : 0,
    week,
    weekday,
    level,
    available,
  }
}

describe('iso cube heights', () => {
  it('keeps an idle day at the flat base height', () => {
    expect(isoCubeHeight(0, 1000)).toBe(2)
    expect(isoCubeHeight(0, 0)).toBe(2)
  })

  it('gives every active day a visible minimum', () => {
    expect(isoCubeHeight(1, 1_000_000)).toBe(8)
  })

  it('compresses the scale so the busiest day reaches the top', () => {
    // A quarter of the peak (square root of 0.25) still reaches mid height.
    expect(isoCubeHeight(250, 1000)).toBe(36)
    expect(isoCubeHeight(1000, 1000)).toBe(120)
    expect(isoCubeHeight(250, 1000)).toBeGreaterThan(isoCubeHeight(25, 1000))
  })

  it('does not overflow above the busiest day', () => {
    expect(isoCubeHeight(5000, 1000)).toBe(120)
  })
})

describe('iso layout', () => {
  const days = [day(0, 0, 0, 0), day(1, 2, 1000, 4), day(2, 6, 400, 2)]
  const layout = layoutIsoCalendar(days, RAMP)

  it('steps one column right per week and one column left per weekday', () => {
    const stepY = 13 * ISO_SLOPE
    const [origin, nextWeek, nextDay] = layoutIsoCalendar(
      [day(0, 0, 0, 0), day(1, 0, 0, 0), day(0, 1, 0, 0)],
      RAMP
    ).cubes.map((cube) => cube.top[1])

    expect(nextWeek.x - origin.x).toBeCloseTo(13, 5)
    expect(nextWeek.y - origin.y).toBeCloseTo(stepY, 5)
    expect(nextDay.x - origin.x).toBeCloseTo(-13, 5)
    expect(nextDay.y - origin.y).toBeCloseTo(stepY, 5)
  })

  it('draws faces back to front for correct overlap', () => {
    const depths = layout.cubes.map((cube) => cube.depth)
    expect(depths).toEqual([...depths].sort((a, b) => a - b))
  })

  it('darkens the side faces of every cube', () => {
    const cube = layout.cubes[0]
    expect(cube.topColor).not.toBe(cube.leftColor)
    expect(cube.leftColor).not.toBe(cube.rightColor)
    // Level 0 is the neutral wash: 60% of the muted grey over white.
    expect(cube.leftColor).toBe('rgb(187,187,188)')
  })

  it('sizes the canvas to contain the whole calendar', () => {
    const right = Math.max(
      ...layout.cubes.flatMap((cube) => cube.top.map((p) => p.x))
    )
    const bottom = Math.max(
      ...layout.cubes.flatMap((cube) => cube.right.map((p) => p.y))
    )
    expect(right).toBeLessThan(layout.width)
    expect(bottom).toBeLessThan(layout.height)
    expect(
      layout.cubes.every((cube) => cube.top.every((p) => p.x > 0 && p.y > 0))
    ).toBe(true)
  })
})

describe('iso hit testing', () => {
  const days = [day(2, 3, 1000, 4), day(5, 1, 20, 1)]
  const layout = layoutIsoCalendar(days, RAMP)

  it('returns the day under a point on its top face', () => {
    const target = layout.cubes.find((cube) => cube.day.tokens === 20)
    if (!target) throw new Error('Fixture cube is missing')
    const centre = {
      x: (target.top[0].x + target.top[2].x) / 2,
      y: (target.top[0].y + target.top[2].y) / 2,
    }

    expect(hitTestIso(layout.cubes, centre)?.tokens).toBe(20)
  })

  it('returns the day under a point on its side face', () => {
    const target = layout.cubes.find((cube) => cube.day.tokens === 20)
    if (!target) throw new Error('Fixture cube is missing')
    const centre = {
      x: (target.right[0].x + target.right[3].x) / 2,
      y: (target.right[0].y + target.right[3].y) / 2,
    }

    expect(hitTestIso(layout.cubes, centre)?.tokens).toBe(20)
  })

  it('prefers the nearer cube where two columns overlap', () => {
    const near = layout.cubes.at(-1)
    if (!near) throw new Error('Fixture cube is missing')
    const centre = {
      x: (near.top[0].x + near.top[2].x) / 2,
      y: (near.top[0].y + near.top[2].y) / 2,
    }

    expect(hitTestIso(layout.cubes, centre)).toBe(near.day)
  })

  it('returns nothing outside the calendar', () => {
    expect(hitTestIso(layout.cubes, { x: -50, y: -50 })).toBeUndefined()
  })
})

describe('theme ramp', () => {
  it('tints the surface from a neutral wash up to the full primary', () => {
    const surface = { r: 255, g: 255, b: 255 }
    const ramp = buildRamp(
      surface,
      { r: 0, g: 0, b: 100 },
      { r: 200, g: 200, b: 200 }
    )

    expect(ramp[0].g).toBeCloseTo(222, 5)
    expect(ramp[4]).toEqual({ r: 0, g: 0, b: 100 })
    // Every step must be further from the surface than the previous one.
    const distance = (color: { r: number; g: number; b: number }) =>
      Math.abs(color.r - surface.r) + Math.abs(color.g - surface.g)
    expect(distance(ramp[2])).toBeGreaterThan(distance(ramp[1]))
    expect(distance(ramp[3])).toBeGreaterThan(distance(ramp[2]))
  })

  it('reads the rgb form the theme resolver returns', () => {
    expect(parseRgb('rgba(12, 34, 56, 1.00)')).toEqual({ r: 12, g: 34, b: 56 })
    expect(parseRgb('oklch(0.6 0.2 250)')).toBeUndefined()
    expect(parseRgb(undefined)).toBeUndefined()
  })
})
