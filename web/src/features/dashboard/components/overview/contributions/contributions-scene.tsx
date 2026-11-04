/* Copyright (C) 2023-2026 QuantumNous */
import { type PointerEvent, useEffect, useId, useRef, useState } from 'react'

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { resolveThemeColor } from '@/lib/theme-color'

import type { ContributionDay } from './calendar'
import {
  buildRamp,
  hitTestIso,
  layoutIsoCalendar,
  parseRgb,
  type IsoLayout,
  type Rgb,
} from './iso'

type ContributionsSceneProps = {
  days: ContributionDay[]
  describeDay: (day: ContributionDay) => string
  onSelect: (day: ContributionDay) => void
  onUnavailable: () => void
}

const FALLBACK_PRIMARY: Rgb = { r: 59, g: 130, b: 246 }
const FALLBACK_MUTED: Rgb = { r: 228, g: 228, b: 231 }
const FALLBACK_SURFACE: Rgb = { r: 255, g: 255, b: 255 }

/** Theme tokens resolve to `oklch()`/`color-mix()`, which canvas cannot paint. */
function themeRamp(): Rgb[] {
  const primary = parseRgb(resolveThemeColor('--primary')) ?? FALLBACK_PRIMARY
  const muted = parseRgb(resolveThemeColor('--muted')) ?? FALLBACK_MUTED
  const surface = parseRgb(resolveThemeColor('--card')) ?? FALLBACK_SURFACE
  return buildRamp(surface, primary, muted)
}

function drawLayout(
  context: CanvasRenderingContext2D,
  layout: IsoLayout,
  scale: number
) {
  context.setTransform(scale, 0, 0, scale, 0, 0)
  context.clearRect(0, 0, layout.width, layout.height)
  for (const cube of layout.cubes) {
    for (const [face, color] of [
      [cube.left, cube.leftColor],
      [cube.right, cube.rightColor],
      [cube.top, cube.topColor],
    ] as const) {
      context.beginPath()
      context.moveTo(face[0].x, face[0].y)
      context.lineTo(face[1].x, face[1].y)
      context.lineTo(face[2].x, face[2].y)
      context.lineTo(face[3].x, face[3].y)
      context.closePath()
      context.fillStyle = color
      context.fill()
    }
  }
}

export default function ContributionsScene(props: ContributionsSceneProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const callbacks = useRef(props)
  const tooltipId = useId()
  const [tooltip, setTooltip] = useState<{
    day: ContributionDay
    x: number
    y: number
  }>()
  const [layout, setLayout] = useState<IsoLayout>()

  useEffect(() => {
    callbacks.current = props
  }, [props])

  useEffect(() => {
    setTooltip(undefined)
    setLayout(layoutIsoCalendar(props.days, themeRamp()))
  }, [props.days])

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas || !layout) return
    const context = canvas.getContext('2d')
    if (!context) {
      callbacks.current.onUnavailable()
      return
    }

    const draw = () => {
      const ratio = Math.min(window.devicePixelRatio || 1, 2)
      canvas.width = Math.round(layout.width * ratio)
      canvas.height = Math.round(layout.height * ratio)
      drawLayout(context, layoutIsoCalendar(props.days, themeRamp()), ratio)
    }

    draw()
    const themeObserver = new MutationObserver(draw)
    for (const element of [document.documentElement, document.body]) {
      themeObserver.observe(element, {
        attributes: true,
        attributeFilter: ['class', 'style', 'data-theme-preset'],
      })
    }
    return () => themeObserver.disconnect()
  }, [layout, props.days])

  const dayAt = (event: { clientX: number; clientY: number }) => {
    const canvas = canvasRef.current
    if (!canvas || !layout) return undefined
    const rect = canvas.getBoundingClientRect()
    if (!rect.width || !rect.height) return undefined
    return hitTestIso(layout.cubes, {
      x: ((event.clientX - rect.left) / rect.width) * layout.width,
      y: ((event.clientY - rect.top) / rect.height) * layout.height,
    })
  }

  const showTooltip = (
    day: ContributionDay | undefined,
    event: { clientX: number; clientY: number }
  ) => {
    if (!day) {
      setTooltip(undefined)
      return
    }
    const rect = canvasRef.current?.getBoundingClientRect()
    setTooltip({
      day,
      x: event.clientX - (rect?.left ?? 0),
      y: event.clientY - (rect?.top ?? 0),
    })
  }

  const selectAt = (event: PointerEvent<HTMLCanvasElement>) => {
    const day = dayAt(event)
    showTooltip(day, event)
    if (day) props.onSelect(day)
  }

  return (
    <div aria-hidden='true' className='relative w-full'>
      <canvas
        ref={canvasRef}
        className='block h-auto w-full max-w-full cursor-crosshair'
        style={{
          aspectRatio: layout ? `${layout.width} / ${layout.height}` : '4 / 3',
        }}
        onPointerMove={(event) => {
          if (event.pointerType !== 'touch') selectAt(event)
        }}
        onPointerLeave={() => setTooltip(undefined)}
        onPointerDown={selectAt}
      />
      <TooltipProvider delay={100}>
        <Tooltip
          open={Boolean(tooltip)}
          triggerId={tooltipId}
          onOpenChange={(open) => {
            if (!open) setTooltip(undefined)
          }}
        >
          <TooltipTrigger
            id={tooltipId}
            tabIndex={-1}
            render={<span />}
            className='pointer-events-none absolute size-px'
            style={{ left: tooltip?.x ?? 0, top: tooltip?.y ?? 0 }}
          />
          {tooltip && (
            <TooltipContent className='pointer-events-none'>
              {props.describeDay(tooltip.day)}
            </TooltipContent>
          )}
        </Tooltip>
      </TooltipProvider>
    </div>
  )
}
