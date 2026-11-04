/* Copyright (C) 2023-2026 QuantumNous */
import { CONTRIBUTION_LEVEL_ALPHA, type ContributionDay } from './calendar'

/**
 * Isometric geometry for the 3D contribution view.
 *
 * The projection matches the established isometric contribution look: every day
 * is a cube drawn as a top rhombus plus two extruded side faces, viewed at 30°.
 * The side faces are the top colour darkened by fixed factors, so the shading
 * never depends on a light rig.
 */
// Screen slope of one iso step. A true 30° projection (0.577) is noticeably
// steep on a wide, short card, so the year grid is flattened a little: the
// calendar stays readable while the cubes still read as isometric.
export const ISO_SLOPE = 0.45
// Side faces carry the cube's volume. A wider spread than the reference's
// 0.8367/0.7 keeps neighbouring columns readable at this scale.
const LEFT_DARKEN = 0.78
const RIGHT_DARKEN = 0.58
const CELL = 13
const PAD = 16
const MIN_HEIGHT = 2
const MIN_ACTIVE_HEIGHT = 8
const MAX_HEIGHT = 120

export type Rgb = { r: number; g: number; b: number }
export type Point = { x: number; y: number }

export interface IsoCube {
  day: ContributionDay
  depth: number
  top: [Point, Point, Point, Point]
  left: [Point, Point, Point, Point]
  right: [Point, Point, Point, Point]
  topColor: string
  leftColor: string
  rightColor: string
}

export interface IsoLayout {
  cubes: IsoCube[]
  width: number
  height: number
}

/** Parse the `rgb()`/`rgba()` form the theme resolver produces. */
export function parseRgb(value: string | undefined): Rgb | undefined {
  if (!value) return undefined
  const match = value.match(/rgba?\(([^)]+)\)/)
  if (!match) return undefined
  const [r, g, b] = match[1].split(',').map((part) => Number(part.trim()))
  if ([r, g, b].some((channel) => !Number.isFinite(channel))) return undefined
  return { r, g, b }
}

function darken(color: Rgb, factor: number): string {
  const channel = (value: number) =>
    Math.max(0, Math.min(255, Math.round(value * factor)))
  return `rgb(${channel(color.r)},${channel(color.g)},${channel(color.b)})`
}

function mix(from: Rgb, to: Rgb, weight: number): Rgb {
  return {
    r: from.r + (to.r - from.r) * weight,
    g: from.g + (to.g - from.g) * weight,
    b: from.b + (to.b - from.b) * weight,
  }
}

/**
 * Height of one column: a flat base for idle days, then a linear share of the
 * busiest day so the tallest bar always reaches the top of the scale.
 */
export function isoCubeHeight(tokens: number, maxTokens: number): number {
  if (!(tokens > 0) || !(maxTokens > 0)) return MIN_HEIGHT
  const share = Math.min(1, tokens / maxTokens)
  return Math.round(
    MIN_ACTIVE_HEIGHT + share * (MAX_HEIGHT - MIN_ACTIVE_HEIGHT)
  )
}

/**
 * Place every day as an isometric cube.
 *
 * Column `w` and row `d` sit at `x = (w - d) * CELL`, `y = (w + d) * CELL*ISO_SLOPE`,
 * which keeps the rhombi tiling edge to edge at the chosen screen slope.
 */
export function layoutIsoCalendar(
  days: ContributionDay[],
  ramp: Rgb[]
): IsoLayout {
  const maxWeek = Math.max(0, ...days.map((day) => day.week))
  const maxTokens = Math.max(0, ...days.map((day) => day.tokens))
  const stepY = CELL * ISO_SLOPE
  const spanX = (maxWeek + 6) * CELL
  const depth = (maxWeek + 6) * stepY
  const originX = PAD + CELL + 6 * CELL
  const originY = PAD + MAX_HEIGHT + stepY
  const cubes: IsoCube[] = []

  for (const day of days) {
    const height = day.available
      ? isoCubeHeight(day.tokens, maxTokens)
      : MIN_HEIGHT
    const x = originX + (day.week - day.weekday) * CELL
    const baseY = originY + (day.week + day.weekday) * stepY
    const topY = baseY - height
    const base = ramp[day.available ? day.level : 0] ?? ramp[0]
    cubes.push({
      day,
      depth: day.week + day.weekday,
      top: [
        { x, y: topY - stepY },
        { x: x + CELL, y: topY },
        { x, y: topY + stepY },
        { x: x - CELL, y: topY },
      ],
      left: [
        { x: x - CELL, y: topY },
        { x, y: topY + stepY },
        { x, y: baseY + stepY },
        { x: x - CELL, y: baseY },
      ],
      right: [
        { x, y: topY + stepY },
        { x: x + CELL, y: topY },
        { x: x + CELL, y: baseY },
        { x, y: baseY + stepY },
      ],
      topColor: `rgb(${Math.round(base.r)},${Math.round(base.g)},${Math.round(base.b)})`,
      leftColor: darken(base, LEFT_DARKEN),
      rightColor: darken(base, RIGHT_DARKEN),
    })
  }

  // Painter's order: nearer cubes (larger week + weekday) are drawn last.
  cubes.sort((a, b) => a.depth - b.depth)

  return {
    cubes,
    width: spanX + 2 * CELL + 2 * PAD,
    height: originY + depth + stepY + PAD,
  }
}

/**
 * Build the five-step colour ramp the way the 2D grid paints it: the theme
 * primary tinted by alpha over the card surface, plus a neutral wash for days
 * without usage. Compositing here keeps both views identical.
 */
export function buildRamp(background: Rgb, primary: Rgb, muted: Rgb): Rgb[] {
  return CONTRIBUTION_LEVEL_ALPHA.map((alpha, level) =>
    level === 0 ? mix(background, muted, 0.6) : mix(background, primary, alpha)
  )
}

function inPolygon(point: Point, polygon: Point[]): boolean {
  let inside = false
  for (let i = 0, j = polygon.length - 1; i < polygon.length; j = i++) {
    const a = polygon[i]
    const b = polygon[j]
    const straddles = a.y > point.y !== b.y > point.y
    if (
      straddles &&
      point.x < ((b.x - a.x) * (point.y - a.y)) / (b.y - a.y) + a.x
    ) {
      inside = !inside
    }
  }
  return inside
}

/** Topmost cube under a canvas point, searched from the front of the scene. */
export function hitTestIso(
  cubes: IsoCube[],
  point: Point
): ContributionDay | undefined {
  for (let index = cubes.length - 1; index >= 0; index--) {
    const cube = cubes[index]
    if (
      inPolygon(point, cube.top) ||
      inPolygon(point, cube.left) ||
      inPolygon(point, cube.right)
    ) {
      return cube.day
    }
  }
  return undefined
}
