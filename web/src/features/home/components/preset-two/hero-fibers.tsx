import { type RefObject, useEffect, useRef } from 'react'

import { type FiberFieldLook, mountFiberField } from '@/lib/fiber-field'

// The bundle lies across the hero: streams come in from the left edge, meet
// at the gateway and fan out to the right, with that end nearer the viewer.
const LOOK: FiberFieldLook = {
  ringRadius: 0.52,
  halfLength: 0.8,
  twist: 2.85,
  lean: -0.22,
  roll: -Math.PI / 2,
}
const PALETTE = {
  colors: ['#a78bfa', '#67e8f9', '#a78bfa', '#fde68a'],
  additive: true,
}
const MIN_SPAN = 900
const MAX_SPAN = 1800

type HeroFibersProps = {
  /** The element whose center the fibers converge on. */
  anchorRef: RefObject<HTMLElement | null>
  /** Seconds to stay dark before the fibers light up. */
  delay: number
  reduced: boolean
  className?: string
}

/**
 * Backdrop of light fibers for the hero, converging behind the gateway node
 * of the routing diagram. The canvas has to fill the positioned ancestor it
 * shares with the anchor.
 */
export function HeroFibers(props: HeroFibersProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    const field = mountFiberField(canvas, {
      look: LOOK,
      delay: props.delay,
      reduceMotion: props.reduced,
      readPalette: () => PALETTE,
      layout: (width, height) => {
        const anchor = props.anchorRef.current
        let y = height * 0.68
        if (anchor?.offsetParent) {
          // Offsets ignore transforms, so the card's entrance animation does
          // not drag the waist along.
          y = anchor.offsetHeight / 2
          let node: Element | null = anchor
          while (node instanceof HTMLElement && !node.contains(canvas)) {
            y += node.offsetTop
            node = node.offsetParent
          }
        }
        return {
          x: width / 2,
          y,
          span: Math.min(Math.max(width, MIN_SPAN), MAX_SPAN),
        }
      },
    })
    return () => field?.dispose()
  }, [props.anchorRef, props.delay, props.reduced])

  return (
    <canvas ref={canvasRef} aria-hidden='true' className={props.className} />
  )
}
