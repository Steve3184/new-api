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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { type FiberFieldOptions, mountFiberField } from '../fiber-field'

// jsdom has no canvas backend, so the 2D context is the external boundary that
// gets faked. Each cleared frame records the strongest opacity it painted with.
const frames: number[] = []

function fakeContext(): CanvasRenderingContext2D {
  const context = {
    globalAlpha: 1,
    setTransform: vi.fn(),
    clearRect: vi.fn(() => frames.push(0)),
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    arc: vi.fn(),
    fill: vi.fn(),
    stroke: vi.fn(() => {
      const last = frames.length - 1
      frames[last] = Math.max(frames[last], context.globalAlpha)
    }),
  }
  return context as unknown as CanvasRenderingContext2D
}

// Animation frames are stepped by hand, 50ms apart.
let pending: FrameRequestCallback | null = null
let now = 0

function advance(seconds: number) {
  for (let step = 0; step < Math.round(seconds / 0.05); step++) {
    now += 50
    const callback = pending
    pending = null
    callback?.(now)
  }
}

function mount(size: DOMRect, options?: Partial<FiberFieldOptions>) {
  vi.spyOn(
    HTMLCanvasElement.prototype,
    'getBoundingClientRect'
  ).mockReturnValue(size)
  const canvas = document.createElement('canvas')
  mountFiberField(canvas, {
    look: { ringRadius: 0.5, halfLength: 0.9, twist: 2.75, lean: 0, roll: 0 },
    reduceMotion: false,
    readPalette: () => ({ colors: ['red', 'blue'], additive: true }),
    layout: (width, height) => ({
      x: width / 2,
      y: height / 2,
      span: Math.hypot(width, height),
    }),
    ...options,
  })
  return canvas
}

beforeEach(() => {
  frames.length = 0
  pending = null
  now = 0
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    fakeContext()
  )
  vi.spyOn(performance, 'now').mockImplementation(() => now)
  vi.stubGlobal('requestAnimationFrame', (callback: FrameRequestCallback) => {
    pending = callback
    return 1
  })
  vi.stubGlobal('cancelAnimationFrame', () => {
    pending = null
  })
})

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
})

describe('fiber field', () => {
  test('stays dark for its delay, then lights up', () => {
    mount(new DOMRect(0, 0, 720, 900), { delay: 1 })

    advance(0.9)
    expect(frames.length).toBeGreaterThan(10)
    expect(Math.max(...frames)).toBe(0)

    advance(0.6)
    expect(frames.at(-1)).toBeGreaterThan(0)
  })

  test('pauses while its canvas is off screen and resumes when it returns', () => {
    let report = (_visible: boolean) => {}
    vi.stubGlobal(
      'IntersectionObserver',
      class {
        constructor(
          callback: (entries: { isIntersecting: boolean }[]) => void
        ) {
          report = (visible) => callback([{ isIntersecting: visible }])
        }
        observe() {}
        disconnect() {}
      }
    )
    mount(new DOMRect(0, 0, 720, 900))
    advance(0.2)
    const painted = frames.length

    report(false)
    advance(0.2)
    expect(frames).toHaveLength(painted)

    report(true)
    advance(0.2)
    expect(frames.length).toBeGreaterThan(painted)
  })

  test('draws at device resolution until a page-sized canvas would exceed the pixel budget', () => {
    vi.stubGlobal('devicePixelRatio', 2)

    const panel = mount(new DOMRect(0, 0, 720, 900))
    expect([panel.width, panel.height]).toEqual([1440, 1800])

    const page = mount(new DOMRect(0, 0, 2560, 1600))
    expect(page.width * page.height).toBeLessThanOrEqual(5_000_000)
    expect(page.width / page.height).toBeCloseTo(1.6, 2)
  })
})
