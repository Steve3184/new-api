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
import { render, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { AuthParticleField } from '../components/auth-particle-field'

// jsdom has no canvas backend, so the 2D context is the external boundary that
// gets faked. Each cleared frame records the blend mode it was painted with.
const frames: string[] = []

function fakeContext(): CanvasRenderingContext2D {
  const context = {
    globalCompositeOperation: 'source-over',
    setTransform: vi.fn(),
    clearRect: vi.fn(),
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    arc: vi.fn(),
    fill: vi.fn(),
    stroke: vi.fn(() => {
      frames[frames.length - 1] = context.globalCompositeOperation
    }),
  }
  context.clearRect.mockImplementation(() => frames.push('blank'))
  return context as unknown as CanvasRenderingContext2D
}

const pause = () => new Promise((resolve) => setTimeout(resolve, 40))

beforeEach(() => {
  frames.length = 0
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue(
    fakeContext()
  )
  vi.spyOn(
    HTMLCanvasElement.prototype,
    'getBoundingClientRect'
  ).mockReturnValue(new DOMRect(0, 0, 720, 900))
})

afterEach(() => {
  document.documentElement.classList.remove('dark')
  vi.restoreAllMocks()
})

describe('auth particle field', () => {
  test('holds one painted still frame when the user prefers reduced motion', async () => {
    render(<AuthParticleField />)

    expect(frames).toEqual(['multiply'])
    await pause()
    expect(frames).toHaveLength(1)
  })

  test('repaints the still frame with additive light when the theme turns dark', async () => {
    render(<AuthParticleField />)

    document.documentElement.classList.add('dark')

    await waitFor(() => expect(frames).toEqual(['multiply', 'lighter']))
  })

  test('keeps animating while mounted and stops once it unmounts', async () => {
    vi.spyOn(window, 'matchMedia').mockReturnValue({
      matches: false,
    } as MediaQueryList)
    const view = render(<AuthParticleField />)

    await waitFor(() => expect(frames.length).toBeGreaterThan(3))
    expect(frames).not.toContain('blank')

    view.unmount()
    const painted = frames.length
    await pause()
    expect(frames).toHaveLength(painted)
  })
})
