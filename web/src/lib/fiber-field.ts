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

export type FiberFieldLook = {
  /** Ring radius, in spans. */
  ringRadius: number
  /** Distance from the waist to either ring, in spans. */
  halfLength: number
  /**
   * How far the near ring is turned against the far one, in radians. Half a
   * turn would pinch the waist to a point.
   */
  twist: number
  /** Tips the bundle out of the screen: negative brings the near end closer. */
  lean: number
  /**
   * Turns the bundle on the screen: 0 runs it top to bottom, a negative
   * quarter turn left to right.
   */
  roll: number
}

export type FiberFieldPalette = {
  /** One color per hue around a ring. The count must stay the same. */
  colors: string[]
  /** Light adds up on a dark surface; on a light one overlaps deepen instead. */
  additive: boolean
}

export type FiberFieldLayout = {
  /** Where the waist sits on the canvas. */
  x: number
  y: number
  /** The length every other measure scales with. */
  span: number
}

export type FiberFieldOptions = {
  look: FiberFieldLook
  /** Seconds to stay dark before the entrance starts. */
  delay?: number
  /** Paints the settled form once instead of animating. */
  reduceMotion: boolean
  readPalette: () => FiberFieldPalette
  layout: (width: number, height: number) => FiberFieldLayout
}

export type FiberField = {
  /** Reads the palette again and paints a frame with it. */
  repaint: () => void
  dispose: () => void
}

type Strand = {
  shell: number
  /** Where the strand sits on its ring, in radians. */
  angle: number
  hue: number
}

type Packet = {
  strand: number
  /** Position along the strand: 0 at the far ring, 1 at the near one. */
  travel: number
  /** Share of the strand covered per second. */
  speed: number
  size: number
  glow: number
}

// Concentric shells of strands. Neighbors spin in opposite directions so the
// aperture at the waist keeps shifting.
const SHELLS = [
  { radius: 1, strands: 46, spin: 0.05 },
  { radius: 0.72, strands: 34, spin: -0.07 },
  { radius: 0.46, strands: 22, spin: 0.1 },
]
const HUE_TURNS = 3
const PACKET_COUNT = 700

const FOCAL_LENGTH = 2
const FOCUS_DEPTH = 0.55
// The form enters this much more open than its look asks for.
const INTRO_SLACK = 1.55
const BREATH = 0.14
const INTRO_SECONDS = 2.4
const FADE_SECONDS = 0.9

const THREAD_STEPS = 10
const TRAIL_SECONDS = 0.5
const BOKEH = 5
const CULL_MARGIN = 48
// Backing store budget, so a page-sized canvas stays cheap on dense screens.
const MAX_PIXELS = 5_000_000

/**
 * Draws a bundle of light fibers on a canvas and keeps it moving.
 *
 * Strands run between two rings that are twisted against each other, so they
 * converge on a narrow waist and fan out again, and packets of light travel
 * along them from the far ring to the near one: many streams meeting at one
 * gateway. The form twists shut as it enters, then breathes, spins slowly and
 * tilts toward the pointer.
 *
 * Pauses while the page is hidden or the canvas is off screen or without a
 * size, and holds the settled form as a still frame under reduced motion.
 * Returns null when the canvas has no 2D context.
 */
export function mountFiberField(
  canvas: HTMLCanvasElement,
  options: FiberFieldOptions
): FiberField | null {
  const context = canvas.getContext('2d')
  if (!context) return null

  const look = options.look
  let palette = options.readPalette()
  const hues = palette.colors.length

  const strands: Strand[] = SHELLS.flatMap((shell, shellIndex) =>
    Array.from({ length: shell.strands }, (_, index) => {
      const turn = index / shell.strands
      return {
        shell: shellIndex,
        angle: turn * Math.PI * 2,
        hue: Math.floor(turn * hues * HUE_TURNS) % hues,
      }
    })
  )
  const packets: Packet[] = Array.from({ length: PACKET_COUNT }, () => ({
    strand: Math.floor(Math.random() * strands.length),
    travel: Math.random(),
    speed: 0.022 + Math.random() * 0.04,
    // Mostly fine dust with the occasional larger bead.
    size: 1 + Math.random() ** 3 * 2.4,
    glow: 0.45 + Math.random() * 0.55,
  }))
  // Grouped by hue so a frame only switches color a handful of times.
  packets.sort((a, b) => strands[a.strand].hue - strands[b.strand].hue)

  // Camera space ends of every strand: far xyz, then near xyz.
  const ends = new Float32Array(strands.length * 6)
  // Scratch result of locate(), reused to keep the frame loop allocation free.
  const point = { x: 0, y: 0, depth: 0, scale: 1 }
  const tilt = { x: 0, y: 0, targetX: 0, targetY: 0 }
  let place: FiberFieldLayout = { x: 0, y: 0, span: 0 }
  let width = 0
  let height = 0
  let frame = 0
  let last = 0
  let onScreen = true
  // Reduced motion skips the entrance and shows the settled form.
  let clock = options.reduceMotion ? INTRO_SECONDS : -(options.delay ?? 0)

  const locate = (strand: number, travel: number) => {
    const at = strand * 6
    point.depth = ends[at + 2] + (ends[at + 5] - ends[at + 2]) * travel
    point.scale = FOCAL_LENGTH / (FOCAL_LENGTH + point.depth / place.span)
    point.x =
      place.x + (ends[at] + (ends[at + 3] - ends[at]) * travel) * point.scale
    point.y =
      place.y +
      (ends[at + 1] + (ends[at + 4] - ends[at + 1]) * travel) * point.scale
  }

  const render = () => {
    if (width === 0 || height === 0) return
    context.clearRect(0, 0, width, height)
    context.globalCompositeOperation = palette.additive ? 'lighter' : 'multiply'

    const span = place.span
    const unit = span / 1400
    const age = Math.max(0, clock)
    const settle = 1 - (1 - Math.min(1, age / INTRO_SECONDS)) ** 4
    const fade = Math.min(1, age / FADE_SECONDS)
    const twist =
      look.twist -
      INTRO_SLACK * (1 - settle) +
      Math.sin(clock * 0.26) * BREATH * settle
    const yaw = tilt.x * 0.2 + Math.sin(clock * 0.13) * 0.05
    const pitch = tilt.y * 0.14

    // Model axes as the camera sees them: lean and roll pose the form, yaw
    // and pitch follow the pointer.
    const [right, down, forward] = [
      [1, 0, 0],
      [0, 1, 0],
      [0, 0, 1],
    ].map(([x, y, z]) => {
      const leanY = y * Math.cos(look.lean) - z * Math.sin(look.lean)
      const leanZ = y * Math.sin(look.lean) + z * Math.cos(look.lean)
      const rollX = x * Math.cos(look.roll) - leanY * Math.sin(look.roll)
      const rollY = x * Math.sin(look.roll) + leanY * Math.cos(look.roll)
      const yawX = rollX * Math.cos(yaw) + leanZ * Math.sin(yaw)
      const yawZ = leanZ * Math.cos(yaw) - rollX * Math.sin(yaw)
      return [
        yawX,
        rollY * Math.cos(pitch) - yawZ * Math.sin(pitch),
        rollY * Math.sin(pitch) + yawZ * Math.cos(pitch),
      ]
    })

    for (let index = 0; index < strands.length; index++) {
      const shell = SHELLS[strands[index].shell]
      const radius = span * look.ringRadius * shell.radius
      const angle = strands[index].angle + clock * shell.spin
      const farX = Math.cos(angle - twist / 2) * radius
      const farZ = Math.sin(angle - twist / 2) * radius
      const nearX = Math.cos(angle + twist / 2) * radius
      const nearZ = Math.sin(angle + twist / 2) * radius
      for (let axis = 0; axis < 3; axis++) {
        const reach = span * look.halfLength * down[axis]
        ends[index * 6 + axis] =
          farX * right[axis] + farZ * forward[axis] - reach
        ends[index * 6 + 3 + axis] =
          nearX * right[axis] + nearZ * forward[axis] + reach
      }
    }

    context.lineCap = 'butt'
    context.lineWidth = Math.max(0.5, unit * 0.7)
    for (let hue = 0; hue < hues; hue++) {
      context.strokeStyle = palette.colors[hue]
      for (let step = 0; step < THREAD_STEPS; step++) {
        const from = step / THREAD_STEPS
        const to = (step + 1) / THREAD_STEPS
        // Threads dissolve toward both rings, which keeps the waist in focus.
        context.globalAlpha =
          fade *
          (palette.additive ? 0.22 : 0.24) *
          (1 - Math.abs(from + to - 1) ** 1.5)
        context.beginPath()
        for (let index = 0; index < strands.length; index++) {
          if (strands[index].hue !== hue) continue
          locate(index, from)
          context.moveTo(point.x, point.y)
          locate(index, to)
          context.lineTo(point.x, point.y)
        }
        context.stroke()
      }
    }

    context.lineCap = 'round'
    let hue = -1
    for (const packet of packets) {
      locate(packet.strand, packet.travel)
      const x = point.x
      const y = point.y
      if (
        x < -CULL_MARGIN ||
        x > width + CULL_MARGIN ||
        y < -CULL_MARGIN ||
        y > height + CULL_MARGIN
      ) {
        continue
      }

      // Depth of field: packets away from the waist grow into faint discs,
      // and the ones behind it also sink into haze.
      const blur = (point.depth / (span * FOCUS_DEPTH)) ** 2
      const spread = 1 + blur * BOKEH
      const size = packet.size * unit * point.scale * spread
      const haze = point.depth > 0 ? 1 + blur * 2 : 1
      const edge = Math.min(1, packet.travel / 0.1, (1 - packet.travel) / 0.1)
      const alpha = (fade * edge * packet.glow) / (haze * spread ** 1.3)

      if (strands[packet.strand].hue !== hue) {
        hue = strands[packet.strand].hue
        context.strokeStyle = palette.colors[hue]
        context.fillStyle = palette.colors[hue]
      }
      // A long faint trail under a short brighter one tapers toward the tail.
      const trail = packet.speed * TRAIL_SECONDS
      context.lineWidth = size * 0.8
      context.globalAlpha = alpha * 0.2
      locate(packet.strand, packet.travel - trail)
      context.beginPath()
      context.moveTo(point.x, point.y)
      context.lineTo(x, y)
      context.stroke()
      context.globalAlpha = alpha * 0.35
      locate(packet.strand, packet.travel - trail * 0.3)
      context.beginPath()
      context.moveTo(point.x, point.y)
      context.lineTo(x, y)
      context.stroke()
      context.globalAlpha = alpha
      context.beginPath()
      context.arc(x, y, size * 0.7, 0, Math.PI * 2)
      context.fill()
    }

    context.globalAlpha = 1
    context.globalCompositeOperation = 'source-over'
  }

  const tick = (now: number) => {
    // Clamped so a throttled or resumed tab does not jump ahead.
    const delta = Math.min(0.05, Math.max(0, (now - last) / 1000))
    last = now
    clock += delta
    const ease = 1 - Math.exp(-delta * 2.5)
    tilt.x += (tilt.targetX - tilt.x) * ease
    tilt.y += (tilt.targetY - tilt.y) * ease
    for (const packet of packets) {
      packet.travel = (packet.travel + packet.speed * delta) % 1
    }
    render()
    frame = requestAnimationFrame(tick)
  }

  const start = () => {
    if (frame || options.reduceMotion || document.hidden || !onScreen) return
    if (width === 0 || height === 0) return
    last = performance.now()
    frame = requestAnimationFrame(tick)
  }
  const stop = () => {
    if (frame) cancelAnimationFrame(frame)
    frame = 0
  }

  const resize = () => {
    const rect = canvas.getBoundingClientRect()
    width = rect.width
    height = rect.height
    if (width === 0 || height === 0) {
      stop()
      return
    }
    const ratio = Math.min(
      window.devicePixelRatio || 1,
      2,
      Math.sqrt(MAX_PIXELS / (width * height))
    )
    canvas.width = Math.round(width * ratio)
    canvas.height = Math.round(height * ratio)
    context.setTransform(ratio, 0, 0, ratio, 0, 0)
    place = options.layout(width, height)
    // Setting canvas.width wiped the bitmap; repaint now so a resize mid
    // animation (or while rAF is throttled) never leaves a blank canvas.
    render()
    start()
  }

  const onPointerMove = (event: PointerEvent) => {
    tilt.targetX = (event.clientX / window.innerWidth) * 2 - 1
    tilt.targetY = (event.clientY / window.innerHeight) * 2 - 1
  }
  const onVisibility = () => {
    if (document.hidden) stop()
    else start()
  }

  resize()
  const resizeObserver = new ResizeObserver(() => resize())
  resizeObserver.observe(canvas)
  let viewObserver: IntersectionObserver | undefined
  if (!options.reduceMotion) {
    // Content usually sits above the canvas, so the pointer is tracked on
    // the window instead.
    window.addEventListener('pointermove', onPointerMove, { passive: true })
    document.addEventListener('visibilitychange', onVisibility)
    if (typeof IntersectionObserver !== 'undefined') {
      viewObserver = new IntersectionObserver((entries) => {
        onScreen = entries.at(-1)?.isIntersecting ?? true
        if (onScreen) start()
        else stop()
      })
      viewObserver.observe(canvas)
    }
  }

  return {
    repaint: () => {
      palette = options.readPalette()
      render()
    },
    dispose: () => {
      stop()
      resizeObserver.disconnect()
      viewObserver?.disconnect()
      window.removeEventListener('pointermove', onPointerMove)
      document.removeEventListener('visibilitychange', onVisibility)
    },
  }
}
