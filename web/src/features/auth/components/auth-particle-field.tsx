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
import { useEffect, useRef } from 'react'

import { type FiberFieldLook, mountFiberField } from '@/lib/fiber-field'

// Lengths are in panel diagonals, so the form looks the same on any panel.
// The far end leans back into the top left corner, the near end spills out of
// the bottom right one.
const LOOK: FiberFieldLook = {
  ringRadius: 0.5,
  halfLength: 0.9,
  twist: 2.75,
  lean: -0.45,
  roll: -0.5,
}
// Chart tokens around a ring: the brand color carries the form and the next
// two accent it, which keeps every theme preset to one color family.
const HUE_CYCLE = [1, 2, 1, 3]
const FALLBACK_COLOR = 'oklch(0.72 0.18 250)'

/**
 * Decorative particle field for the split sign-in panel: a bundle of light
 * fibers converging on one gateway, see mountFiberField.
 *
 * Reads its palette from the active theme's chart colors and renders the
 * settled form as a still frame when the user asked for reduced motion.
 */
export function AuthParticleField(props: { className?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const field = mountFiberField(canvas, {
      look: LOOK,
      reduceMotion: window.matchMedia('(prefers-reduced-motion: reduce)')
        .matches,
      readPalette: () => {
        const styles = getComputedStyle(canvas)
        return {
          colors: HUE_CYCLE.map(
            (token) =>
              styles.getPropertyValue(`--chart-${token}`).trim() ||
              FALLBACK_COLOR
          ),
          additive: document.documentElement.classList.contains('dark'),
        }
      },
      layout: (width, height) => ({
        x: width * 0.56,
        y: height * 0.48,
        span: Math.hypot(width, height),
      }),
    })
    if (!field) return

    // The light/dark class and the theme preset are applied by provider
    // effects that run after this one, and can change while the page is open.
    const themeObserver = new MutationObserver(() => field.repaint())
    themeObserver.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['class'],
    })
    themeObserver.observe(document.body, {
      attributes: true,
      attributeFilter: ['data-theme-preset'],
    })

    return () => {
      themeObserver.disconnect()
      field.dispose()
    }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      aria-hidden='true'
      className={props.className}
      style={{ width: '100%', height: '100%' }}
    />
  )
}
