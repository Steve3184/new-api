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

type Particle = {
  x: number
  y: number
  vx: number
  vy: number
  radius: number
  hue: number
}

const LINK_DISTANCE = 130
const POINTER_RADIUS = 160
// Spread particles so the field looks the same on any panel size.
const DENSITY = 1 / 14000
const MAX_PARTICLES = 90
const MIN_PARTICLES = 28

/**
 * Decorative particle field for the split sign-in panel.
 *
 * Reads its palette from the active theme's chart colors, pauses while off
 * screen or hidden, and renders a still frame when the user asked for reduced
 * motion. Particles drift slowly and link to nearby neighbors, and the cursor
 * nudges them, so the panel feels alive without pulling attention from the
 * form.
 */
export function AuthParticleField(props: { className?: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    const context = canvas?.getContext('2d')
    if (!canvas || !context) return

    const reduceMotion = window.matchMedia(
      '(prefers-reduced-motion: reduce)'
    ).matches
    const styles = getComputedStyle(canvas)
    const palette = [1, 2, 3, 4]
      .map((index) => styles.getPropertyValue(`--chart-${index}`).trim())
      .filter(Boolean)
    const linkColor = styles.getPropertyValue('--muted-foreground').trim()
    const colors = palette.length > 0 ? palette : ['oklch(0.72 0.18 250)']

    const pointer = { x: -1000, y: -1000, active: false }
    let particles: Particle[] = []
    let frame = 0
    let width = 0
    let height = 0
    let visible = true

    const resize = () => {
      const ratio = Math.min(window.devicePixelRatio || 1, 2)
      const rect = canvas.getBoundingClientRect()
      width = rect.width
      height = rect.height
      if (width === 0 || height === 0) return
      canvas.width = Math.round(width * ratio)
      canvas.height = Math.round(height * ratio)
      context.setTransform(ratio, 0, 0, ratio, 0, 0)

      const count = Math.round(width * height * DENSITY)
      particles = Array.from(
        { length: Math.min(MAX_PARTICLES, Math.max(MIN_PARTICLES, count)) },
        (_, index) => ({
          x: Math.random() * width,
          y: Math.random() * height,
          // Slow drift keeps the field calm; the pointer adds the motion.
          vx: (Math.random() - 0.5) * 0.18,
          vy: (Math.random() - 0.5) * 0.18,
          radius: 1 + Math.random() * 1.8,
          hue: index % colors.length,
        })
      )
      // Setting canvas.width wiped the bitmap; repaint now so a resize mid
      // animation (or while rAF is throttled) never leaves a blank panel.
      draw()
    }

    const advance = () => {
      for (const particle of particles) {
        particle.x += particle.vx
        particle.y += particle.vy

        if (pointer.active) {
          const dx = particle.x - pointer.x
          const dy = particle.y - pointer.y
          const distance = Math.hypot(dx, dy)
          if (distance < POINTER_RADIUS && distance > 0.001) {
            const push = (POINTER_RADIUS - distance) / POINTER_RADIUS
            particle.x += (dx / distance) * push * 1.6
            particle.y += (dy / distance) * push * 1.6
          }
        }

        if (particle.x < -20) particle.x = width + 20
        if (particle.x > width + 20) particle.x = -20
        if (particle.y < -20) particle.y = height + 20
        if (particle.y > height + 20) particle.y = -20
      }
    }

    const draw = () => {
      if (width === 0 || height === 0) return
      context.clearRect(0, 0, width, height)
      context.globalCompositeOperation = 'lighter'

      for (const particle of particles) {
        context.beginPath()
        context.fillStyle = colors[particle.hue]
        context.globalAlpha = 0.85
        context.arc(particle.x, particle.y, particle.radius, 0, Math.PI * 2)
        context.fill()
      }

      context.lineWidth = 1
      context.strokeStyle = linkColor || 'currentColor'
      for (let i = 0; i < particles.length; i++) {
        for (let j = i + 1; j < particles.length; j++) {
          const a = particles[i]
          const b = particles[j]
          const distance = Math.hypot(a.x - b.x, a.y - b.y)
          if (distance > LINK_DISTANCE) continue
          context.globalAlpha = (1 - distance / LINK_DISTANCE) * 0.18
          context.beginPath()
          context.moveTo(a.x, a.y)
          context.lineTo(b.x, b.y)
          context.stroke()
        }
      }

      context.globalAlpha = 1
      context.globalCompositeOperation = 'source-over'
    }

    const tick = () => {
      advance()
      draw()
      frame = visible ? requestAnimationFrame(tick) : 0
    }

    const start = () => {
      if (!frame && visible) frame = requestAnimationFrame(tick)
    }

    const onPointerMove = (event: PointerEvent) => {
      const rect = canvas.getBoundingClientRect()
      pointer.x = event.clientX - rect.left
      pointer.y = event.clientY - rect.top
      pointer.active = true
    }
    const onPointerLeave = () => {
      pointer.active = false
    }
    const onVisibility = () => {
      visible = !document.hidden
      if (!visible && frame) {
        cancelAnimationFrame(frame)
        frame = 0
      } else if (visible && !reduceMotion) {
        start()
      }
    }

    resize()
    const observer = new ResizeObserver(() => resize())
    observer.observe(canvas)

    if (reduceMotion) {
      // A single still frame keeps the panel from looking empty.
      draw()
    } else {
      start()
      const host = canvas.parentElement
      host?.addEventListener('pointermove', onPointerMove)
      host?.addEventListener('pointerleave', onPointerLeave)
      document.addEventListener('visibilitychange', onVisibility)
    }

    return () => {
      if (frame) cancelAnimationFrame(frame)
      observer.disconnect()
      const host = canvas.parentElement
      host?.removeEventListener('pointermove', onPointerMove)
      host?.removeEventListener('pointerleave', onPointerLeave)
      document.removeEventListener('visibilitychange', onVisibility)
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
