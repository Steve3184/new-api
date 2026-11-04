/* Copyright (C) 2023-2026 QuantumNous */
import { type KeyboardEvent, useId, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { toIntlLocale } from '@/i18n/languages'

import { contributionColors, type ContributionDay } from './calendar'

interface ContributionGridProps {
  days: ContributionDay[]
  describeDay: (day: ContributionDay) => string
  onSelect: (day: ContributionDay) => void
}

const DAY_OFFSETS: Record<string, number> = {
  ArrowLeft: -7,
  ArrowRight: 7,
  ArrowUp: -1,
  ArrowDown: 1,
}

export function ContributionGrid(props: ContributionGridProps) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const container = useRef<HTMLDivElement>(null)
  const tooltipId = useId()
  const [activeDate, setActiveDate] = useState<string>()
  const [hovered, setHovered] = useState<{
    day: ContributionDay
    x: number
    y: number
  }>()

  const weekCount = Math.ceil(props.days.length / 7)
  const monthFormatter = useMemo(
    () => new Intl.DateTimeFormat(locale, { month: 'short' }),
    [locale]
  )
  const weekdayFormatter = useMemo(
    () => new Intl.DateTimeFormat(locale, { weekday: 'short' }),
    [locale]
  )

  // The stored date can disappear when the parent's range or data changes, so a
  // stale value must never leave every cell with tabIndex -1.
  const lastAvailable = [...props.days].reverse().find((day) => day.available)
  const focusDate = props.days.some(
    (day) => day.available && day.date === activeDate
  )
    ? activeDate
    : lastAvailable?.date

  const focusCell = (date: string) => {
    setActiveDate(date)
    container.current
      ?.querySelector<HTMLButtonElement>(`[data-date="${date}"]`)
      ?.focus()
  }

  const showTooltipFor = (day: ContributionDay, cell: HTMLElement) => {
    setHovered({ day, x: cell.offsetLeft, y: cell.offsetTop })
  }

  // Walk from a starting index in a direction until an available day is found.
  const stepToAvailable = (from: number, offset: number) => {
    if (offset === 0) return undefined
    const direction = offset < 0 ? -1 : 1
    let index = from + offset
    while (props.days[index] !== undefined) {
      if (props.days[index].available) return props.days[index]
      index += direction
    }
    return undefined
  }

  const handleKeyDown = (event: KeyboardEvent, index: number) => {
    const offset = DAY_OFFSETS[event.key]
    let target: ContributionDay | undefined
    if (offset !== undefined) {
      target = stepToAvailable(index, offset)
    } else if (event.key === 'Home') {
      target = stepToAvailable(-1, 1)
    } else if (event.key === 'End') {
      target = stepToAvailable(props.days.length, -1)
    } else {
      return
    }
    event.preventDefault()
    if (target) focusCell(target.date)
  }

  return (
    <TooltipProvider delay={100}>
      <div
        ref={container}
        className='w-max min-w-full'
        role='group'
        aria-label={t('Usage by day')}
        onMouseLeave={() => setHovered(undefined)}
      >
        <div
          className='text-muted-foreground mb-2 ml-10 grid gap-[3px] text-xs'
          style={{ gridTemplateColumns: `repeat(${weekCount}, 12px)` }}
          aria-hidden='true'
        >
          {props.days
            .filter((day) => day.weekday === 0)
            .map((day, week) => {
              const date = new Date(day.timestamp)
              const previous = props.days[(week - 1) * 7]
              const showMonth =
                week === 0 ||
                (previous !== undefined &&
                  new Date(previous.timestamp).getMonth() !== date.getMonth())
              return (
                <span key={day.date} className='whitespace-nowrap'>
                  {showMonth ? monthFormatter.format(date) : ''}
                </span>
              )
            })}
        </div>
        <div className='flex gap-2'>
          <div
            className='text-muted-foreground grid w-8 shrink-0 grid-rows-7 gap-[3px] text-[10px]'
            aria-hidden='true'
          >
            {props.days.slice(0, 7).map((day) => (
              <span key={day.date} className='h-3 leading-3'>
                {day.weekday % 2 === 1
                  ? weekdayFormatter.format(new Date(day.timestamp))
                  : ''}
              </span>
            ))}
          </div>
          <div className='grid grid-flow-col grid-rows-7 gap-[3px]'>
            {props.days.map((day, index) => (
              <Button
                key={day.date}
                variant='ghost'
                size='icon-xs'
                className='hover:ring-foreground/40 size-3 min-w-0 rounded-[3px] p-0 hover:ring-2 focus-visible:ring-2'
                style={{
                  background: contributionColors[day.level],
                  opacity: day.available ? 1 : 0.2,
                }}
                disabled={!day.available}
                tabIndex={day.date === focusDate ? 0 : -1}
                data-date={day.date}
                aria-label={props.describeDay(day)}
                onKeyDown={(event) => handleKeyDown(event, index)}
                onFocus={(event) => {
                  setActiveDate(day.date)
                  showTooltipFor(day, event.currentTarget)
                  props.onSelect(day)
                }}
                onBlur={() => setHovered(undefined)}
                onPointerEnter={(event) =>
                  showTooltipFor(day, event.currentTarget)
                }
                onClick={() => props.onSelect(day)}
              />
            ))}
          </div>
        </div>
        <Tooltip
          open={Boolean(hovered)}
          triggerId={tooltipId}
          onOpenChange={(open) => {
            if (!open) setHovered(undefined)
          }}
        >
          <TooltipTrigger
            id={tooltipId}
            tabIndex={-1}
            render={<span />}
            className='pointer-events-none absolute size-px'
            style={{ left: hovered?.x ?? 0, top: hovered?.y ?? 0 }}
          />
          {hovered && (
            <TooltipContent>{props.describeDay(hovered.day)}</TooltipContent>
          )}
        </Tooltip>
      </div>
    </TooltipProvider>
  )
}
