/* Copyright (C) 2023-2026 QuantumNous */
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ErrorState } from '@/components/error-state'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'
import { getUserQuotaDates } from '@/features/dashboard/api'
import { useStatus } from '@/hooks/use-status'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'
import { useContributionsStore } from '@/stores/contributions-store'

import {
  buildContributionCalendar,
  contributionColors,
  contributionRange,
  localDateKey,
  type ContributionDay,
} from './calendar'
import { ContributionGrid } from './contribution-grid'

// Three.js is only needed in 3D mode, so it stays out of the overview bundle.
const ContributionsScene = lazy(() => import('./contributions-scene'))

const DEFAULT_MAX_RANGE_DAYS = 365
// These mirror ContributionGrid: cells are 3 spacing units (`size-3`) with
// 3px gaps, after a weekday label column of 8 units plus a 2-unit gap. The
// spacing unit follows the theme's density scale, so it is read from a probe;
// 12px is the default-scale cell used when nothing can be measured.
const DEFAULT_CELL_SIZE = 12
const CELL_GAP = 3
const CELL_UNITS = 3
const WEEKDAY_LABEL_UNITS = 10
// A 2D grid longer than a year is fetched in whole quarters, so widening the
// window does not request the data again for every added week.
const FETCH_WEEKS_STEP = 13

export function ContributionsCard() {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const userId = useAuthStore((state) => state.auth.user?.id)
  const { status } = useStatus()
  const view = useContributionsStore((state) => state.view)
  const setView = useContributionsStore((state) => state.setView)
  const [webglUnavailable, setWebglUnavailable] = useState(false)
  const [selectedDate, setSelectedDate] = useState<string>()
  // Week columns that fit the card; unknown until it has been measured.
  const [gridWeeks, setGridWeeks] = useState<number>()
  const contentRef = useRef<HTMLDivElement>(null)
  const cellProbeRef = useRef<HTMLSpanElement>(null)

  // A density change also changes the card's padding, so watching the content
  // box covers both a resize and a new cell size.
  useEffect(() => {
    const content = contentRef.current
    if (!content) return
    const observer = new ResizeObserver((entries) => {
      const width = entries[0]?.contentRect.width ?? 0
      if (width <= 0) return
      const cellSize =
        cellProbeRef.current?.getBoundingClientRect().width || DEFAULT_CELL_SIZE
      const labelsWidth = (cellSize / CELL_UNITS) * WEEKDAY_LABEL_UNITS
      const next = Math.max(
        1,
        Math.floor((width - labelsWidth + CELL_GAP) / (cellSize + CELL_GAP))
      )
      setGridWeeks((current) => (current === next ? current : next))
    })
    observer.observe(content)
    return () => observer.disconnect()
  }, [])

  const today = localDateKey(new Date())
  const maxDays = status?.log_max_range_days || DEFAULT_MAX_RANGE_DAYS
  // 3D always shows the rolling year; the 2D grid shows the weeks that fit.
  const yearRange = useMemo(
    () => contributionRange(new Date(`${today}T23:59:59`), maxDays),
    [today, maxDays]
  )
  const gridRange = useMemo(
    () =>
      gridWeeks === undefined
        ? yearRange
        : contributionRange(new Date(`${today}T23:59:59`), maxDays, gridWeeks),
    [today, maxDays, gridWeeks, yearRange]
  )
  // One request serves both views, so it starts at whichever begins earlier.
  const range = useMemo(
    () =>
      gridWeeks === undefined || gridRange.start >= yearRange.start
        ? yearRange
        : contributionRange(
            new Date(`${today}T23:59:59`),
            maxDays,
            Math.ceil(gridWeeks / FETCH_WEEKS_STEP) * FETCH_WEEKS_STEP
          ),
    [today, maxDays, gridWeeks, gridRange, yearRange]
  )
  // A failed /api/status must not leave the card loading forever: fall back to
  // the site default range and let the server reject an out-of-range request.
  const query = useQuery({
    queryKey: [
      'dashboard',
      'contributions',
      userId,
      range.start_timestamp,
      range.end_timestamp,
    ],
    queryFn: async () =>
      requireServerSuccess(
        await getUserQuotaDates({
          start_timestamp: range.start_timestamp,
          end_timestamp: range.end_timestamp,
        })
      ),
    enabled: Boolean(userId),
    staleTime: 300000,
    refetchInterval: 300000,
    // Keep the calendar on screen while a wider window loads more weeks.
    placeholderData: keepPreviousData,
  })
  const yearDays = useMemo(
    () => buildContributionCalendar(query.data?.data ?? [], yearRange),
    [query.data, yearRange]
  )
  // The 2D grid ends at today; the rest of this week would only be blank cells.
  const gridDays = useMemo(() => {
    const days =
      gridRange === yearRange
        ? yearDays
        : buildContributionCalendar(query.data?.data ?? [], gridRange)
    return days.filter((day) => day.date <= today)
  }, [query.data, gridRange, yearRange, yearDays, today])
  const show3d = view === '3d' && !webglUnavailable
  // In 3D the grid stays mounted as the keyboard view of the same year.
  const days = show3d ? yearDays : gridDays
  const shownRange = show3d ? yearRange : gridRange
  const selected = days.find(
    (day) => day.date === selectedDate && day.available
  )
  const dateFormatter = useMemo(
    () => new Intl.DateTimeFormat(locale, { dateStyle: 'medium' }),
    [locale]
  )

  const describeDay = (day: ContributionDay) => {
    const date = dateFormatter.format(new Date(day.timestamp))
    if (!day.available) {
      return t('{{date}}: No statistics available', { date })
    }
    return t('{{date}}: {{tokens}} tokens', {
      date,
      tokens: formatNumber(day.tokens, locale),
    })
  }

  const onSelect = (day: ContributionDay) => setSelectedDate(day.date)

  return (
    <Card className='h-full min-w-0' aria-label={t('Usage calendar')}>
      <CardHeader className='flex flex-row flex-wrap items-center justify-between gap-2'>
        <h2 className='text-base font-semibold'>{t('Usage calendar')}</h2>
        <ToggleGroup
          size='sm'
          variant='outline'
          value={[show3d ? '3d' : '2d']}
          aria-label={t('Usage view')}
          onValueChange={(values) => {
            const [value] = values
            if (value === '2d' || value === '3d') setView(value)
          }}
        >
          <ToggleGroupItem value='2d'>2D</ToggleGroupItem>
          <ToggleGroupItem value='3d' disabled={webglUnavailable}>
            3D
          </ToggleGroupItem>
        </ToggleGroup>
      </CardHeader>
      <CardContent ref={contentRef} className='space-y-3'>
        <span
          ref={cellProbeRef}
          aria-hidden='true'
          className='pointer-events-none invisible absolute size-3'
        />
        {query.isPending && (
          <Skeleton
            className='h-40 w-full'
            aria-label={t('Loading usage data')}
          />
        )}
        {query.isError && (
          <ErrorState
            className='min-h-40'
            title={t('Failed to load usage data')}
            onRetry={() => void query.refetch()}
          />
        )}
        {query.isSuccess && (
          <>
            {/* The 2D grid is sized to the card, so it never scrolls; clipping
                only covers the moment before the card has been measured. */}
            <div className={cn('pb-2', !show3d && 'overflow-hidden')}>
              {show3d && (
                <Suspense
                  fallback={<Skeleton className='h-[300px] min-w-[700px]' />}
                >
                  <ContributionsScene
                    days={yearDays}
                    describeDay={describeDay}
                    onSelect={onSelect}
                    onUnavailable={() => setWebglUnavailable(true)}
                  />
                </Suspense>
              )}
              <div className={cn(show3d && 'sr-only focus-within:not-sr-only')}>
                <ContributionGrid
                  days={days}
                  describeDay={describeDay}
                  onSelect={onSelect}
                />
              </div>
            </div>
            <div className='text-muted-foreground flex flex-wrap items-center justify-between gap-x-3 gap-y-1 text-xs'>
              <span role='status' className='text-left'>
                {selected
                  ? describeDay(selected)
                  : t('Select a day to view token usage.')}
              </span>
              <div className='flex items-center gap-1' aria-hidden='true'>
                <span className='mr-1'>{t('Less')}</span>
                {contributionColors.map((color) => (
                  <span
                    key={color}
                    className='size-3 rounded-[3px]'
                    style={{ background: color }}
                  />
                ))}
                <span className='ml-1'>{t('More')}</span>
              </div>
            </div>
            {shownRange.coverageStart > shownRange.start && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Statistics available from {{date}} due to the site query limit.',
                  { date: dateFormatter.format(shownRange.coverageStart) }
                )}
              </p>
            )}
            {!days.some((day) => day.tokens > 0 || day.requests > 0) && (
              <p className='text-muted-foreground text-xs'>
                {t('No usage recorded in this period.')}
              </p>
            )}
          </>
        )}
        {webglUnavailable && (
          <p className='text-muted-foreground text-xs'>
            {t('3D is unavailable on this device. Showing 2D instead.')}
          </p>
        )}
      </CardContent>
    </Card>
  )
}
