/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getSuccessRateDotClass } from '@/features/performance-metrics/lib/format'
import { cn } from '@/lib/utils'

const HISTORY_POINT_COUNT = 24
const MAX_BAR_COUNT = 160
const BAR_WIDTH_PX = 3
const BAR_GAP_PX = 4

type AvailabilityBarsProps = {
  rates: number[]
}

export function AvailabilityBars(props: AvailabilityBarsProps) {
  const { t } = useTranslation()
  const containerRef = useRef<HTMLDivElement>(null)
  const [barCount, setBarCount] = useState(HISTORY_POINT_COUNT)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width ?? 0
      const nextCount = Math.min(
        MAX_BAR_COUNT,
        Math.max(
          1,
          Math.floor((width + BAR_GAP_PX) / (BAR_WIDTH_PX + BAR_GAP_PX))
        )
      )
      setBarCount((current) => (current === nextCount ? current : nextCount))
    })

    observer.observe(container)
    return () => observer.disconnect()
  }, [])

  return (
    <div
      ref={containerRef}
      className='flex h-14 max-w-full min-w-0 items-end justify-end gap-[4px] overflow-hidden'
      aria-label={t('Availability')}
    >
      {Array.from({ length: barCount }, (_, index) => {
        const sourceIndex =
          barCount === 1
            ? HISTORY_POINT_COUNT - 1
            : Math.round((index * (HISTORY_POINT_COUNT - 1)) / (barCount - 1))
        const sourceLength = Math.min(props.rates.length, HISTORY_POINT_COUNT)
        const sourceOffset = props.rates.length - sourceLength
        const missingPoints = HISTORY_POINT_COUNT - sourceLength
        const rate =
          sourceIndex >= missingPoints
            ? props.rates[sourceOffset + sourceIndex - missingPoints]
            : undefined
        const hasRate = typeof rate === 'number'
        const height = hasRate ? Math.max(5, Math.round((rate / 100) * 54)) : 5
        return (
          <span
            key={index}
            className={cn(
              'w-[3px] shrink-0 rounded-t-sm transition-colors',
              hasRate ? getSuccessRateDotClass(rate) : 'bg-muted'
            )}
            style={{ height: `${height}px` }}
            title={hasRate ? `${rate.toFixed(2)}%` : t('No data')}
          />
        )
      })}
    </div>
  )
}
