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
import { memo, type ReactNode, useEffect, useRef, useState } from 'react'

import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

export type HeartbeatBeat = {
  key: string
  /** Background color class of the beat, e.g. `bg-emerald-500`. */
  colorClassName: string
  /** Accessible name of the beat. */
  label: string
  /** Tooltip body; defaults to the label. */
  detail?: ReactNode
}

type HeartbeatBarProps = {
  beats: HeartbeatBeat[]
  /** Accessible name of the whole bar. */
  label: string
  /**
   * Fixed number of slots. Older beats are dropped and missing slots are
   * padded on the left, so bars of different rows stay aligned.
   */
  slots?: number
  /**
   * `compact` renders thin square beats that share the width. `lines` renders
   * Uptime Kuma style narrow lines of fixed width and shows as many of the
   * newest beats as fit. Override the height with `className`.
   */
  variant?: 'compact' | 'lines'
  className?: string
}

const DEFAULT_EMPTY_SLOTS = 30
const LINE_WIDTH_PX = 4
const LINE_GAP_PX = 3

/**
 * Uptime-style row of status beats, newest on the right.
 */
export const HeartbeatBar = memo(function HeartbeatBar(
  props: HeartbeatBarProps
) {
  const isLines = props.variant === 'lines'
  const containerRef = useRef<HTMLDivElement>(null)
  const [fittingSlots, setFittingSlots] = useState<number | null>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!isLines || !container) return
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width ?? 0
      const next = Math.max(
        0,
        Math.floor((width + LINE_GAP_PX) / (LINE_WIDTH_PX + LINE_GAP_PX))
      )
      setFittingSlots((current) => (current === next ? current : next))
    })
    observer.observe(container)
    return () => observer.disconnect()
  }, [isLines])

  let slots = props.slots
  if (isLines) {
    slots = fittingSlots ?? props.beats.length
  } else if (slots === undefined && props.beats.length === 0) {
    slots = DEFAULT_EMPTY_SLOTS
  }
  const beats = slots === undefined ? props.beats : props.beats.slice(-slots)
  const emptyCount = slots === undefined ? 0 : slots - beats.length
  const beatClassName = isLines
    ? 'w-[4px] shrink-0 rounded-full'
    : 'min-w-0 flex-1 rounded-[1px]'

  return (
    <TooltipProvider delay={100}>
      <div
        ref={containerRef}
        role='img'
        aria-label={props.label}
        className={cn(
          'flex w-full min-w-0 overflow-hidden',
          isLines ? 'h-6 gap-[3px]' : 'h-3 gap-0.5',
          props.className
        )}
      >
        {Array.from({ length: emptyCount }, (_, index) => (
          <span
            key={`empty-${index}`}
            aria-hidden='true'
            className={cn(beatClassName, 'bg-muted-foreground/15')}
          />
        ))}
        {beats.map((beat) => (
          <Tooltip key={beat.key}>
            <TooltipTrigger
              render={
                <span
                  aria-label={beat.label}
                  className={cn(beatClassName, beat.colorClassName)}
                />
              }
            />
            <TooltipContent>{beat.detail ?? beat.label}</TooltipContent>
          </Tooltip>
        ))}
      </div>
    </TooltipProvider>
  )
})
