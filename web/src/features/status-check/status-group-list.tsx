/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { ChevronRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { HeartbeatBar, type HeartbeatBeat } from '@/components/heartbeat-bar'
import {
  formatLatency,
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type {
  StatusDegradationCheckStatus,
  StatusDegradationModel,
  StatusGroup,
} from '@/features/performance-metrics/types'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'

const EMPTY_BEAT_CLASS = 'bg-muted-foreground/15'

const DEGRADATION_STATUS: Record<
  StatusDegradationCheckStatus,
  { colorClassName: string; labelKey: string }
> = {
  passed: { colorClassName: 'bg-emerald-500', labelKey: 'Not degraded' },
  degraded: { colorClassName: 'bg-red-500', labelKey: 'Model degraded' },
  error: { colorClassName: 'bg-amber-500', labelKey: 'Check failed' },
}

export function StatusGroupList(props: {
  groups: StatusGroup[]
  onSelectGroup: (group: StatusGroup) => void
}) {
  return (
    <ul className='bg-card divide-y overflow-hidden rounded-lg border'>
      {props.groups.map((group) => (
        <StatusGroupBlock
          key={group.group}
          group={group}
          onClick={() => props.onSelectGroup(group)}
        />
      ))}
    </ul>
  )
}

export function DegradationLegend() {
  const { t } = useTranslation()
  return (
    <div className='text-muted-foreground flex flex-wrap items-center gap-x-4 gap-y-1 text-xs'>
      <span>{t('Model degradation check: expected answer 21')}</span>
      {Object.values(DEGRADATION_STATUS).map((status) => (
        <span key={status.labelKey} className='flex items-center gap-1.5'>
          <span
            aria-hidden='true'
            className={cn('size-2 rounded-full', status.colorClassName)}
          />
          {t(status.labelKey)}
        </span>
      ))}
    </div>
  )
}

/**
 * One group: a summary line, its hourly health beats, then one line per
 * configured degradation check model.
 */
function StatusGroupBlock(props: { group: StatusGroup; onClick: () => void }) {
  const { t } = useTranslation()
  const hasData = props.group.request_count > 0
  const degradation = props.group.degradation ?? []
  const beats: HeartbeatBeat[] = props.group.beats.map((point) => {
    const time = formatTimestampToDate(point.ts)
    if (point.request_count === 0) {
      return {
        key: `${point.ts}`,
        colorClassName: EMPTY_BEAT_CLASS,
        label: `${time}: ${t('No data')}`,
      }
    }
    const rate = `${point.availability.toFixed(2)}%`
    return {
      key: `${point.ts}`,
      colorClassName: getSuccessRateDotClass(point.availability),
      label: `${time}: ${rate}`,
      detail: (
        <>
          <span className='font-medium'>{rate}</span>
          <span className='opacity-70'>{time}</span>
          {point.avg_ttft_ms > 0 && (
            <span className='opacity-70'>
              {t('First token')}: {formatLatency(point.avg_ttft_ms)}
            </span>
          )}
          {point.probe_filled && (
            <span className='opacity-70'>
              {t('From model degradation checks')}
            </span>
          )}
        </>
      ),
    }
  })

  return (
    <li className='min-w-0 space-y-2.5 px-4 py-3'>
      <button
        type='button'
        className='focus-visible:outline-primary group/header flex w-full min-w-0 items-center justify-between gap-3 rounded-md text-left focus-visible:outline-2 focus-visible:outline-offset-2'
        onClick={props.onClick}
        aria-label={t('View {{group}} status history', {
          group: props.group.group,
        })}
      >
        <span className='flex min-w-0 items-center gap-3'>
          <span
            className={cn(
              'inline-flex h-6 w-16 shrink-0 items-center justify-center rounded-full font-mono text-xs font-semibold tabular-nums',
              hasData
                ? [
                    getSuccessRateDotClass(props.group.availability),
                    'text-white',
                  ]
                : 'bg-muted text-muted-foreground'
            )}
          >
            {hasData ? `${props.group.availability.toFixed(1)}%` : '—'}
          </span>
          <span className='truncate font-medium group-hover/header:underline'>
            {props.group.group}
          </span>
        </span>
        <span className='text-muted-foreground flex shrink-0 items-center gap-3 text-xs'>
          <span className='font-mono tabular-nums'>
            {t('Cache')}{' '}
            {props.group.cache_input_tokens > 0
              ? `${props.group.cache_hit_rate.toFixed(1)}%`
              : '—'}
          </span>
          <span className='font-mono tabular-nums'>
            {t('First token')} {formatLatency(props.group.avg_ttft_ms)}
          </span>
          <ChevronRight className='size-4' aria-hidden='true' />
        </span>
      </button>
      <HeartbeatBar beats={beats} variant='lines' label={t('Availability')} />
      {degradation.length > 0 && (
        // Lines share one grid so every beat strip starts right after the
        // longest model name.
        <ul
          className='grid grid-cols-[fit-content(50%)_minmax(0,1fr)_auto] gap-x-3 gap-y-1.5'
          aria-label={t('Model degradation for {{group}}', {
            group: props.group.group,
          })}
        >
          {degradation.map((item) => (
            <DegradationLine key={`${item.model}:${item.effort}`} item={item} />
          ))}
        </ul>
      )}
    </li>
  )
}

function DegradationLine(props: { item: StatusDegradationModel }) {
  const { t } = useTranslation()
  const beats: HeartbeatBeat[] = props.item.checks.map((check) => {
    const status = DEGRADATION_STATUS[check.status] ?? DEGRADATION_STATUS.error
    const statusLabel = t(status.labelKey)
    const time = formatTimestampToDate(check.ts)
    return {
      key: `${check.ts}-${check.status}`,
      colorClassName: status.colorClassName,
      label: `${statusLabel}: ${time}`,
      detail: (
        <>
          <span className='font-medium'>{statusLabel}</span>
          <span className='opacity-70'>{time}</span>
          {check.answer && (
            <span className='opacity-70'>
              {t('Answer')}: {check.answer}
            </span>
          )}
          {check.latency_ms > 0 && (
            <span className='opacity-70'>
              {formatLatency(check.latency_ms)}
            </span>
          )}
        </>
      ),
    }
  })
  const judged = props.item.judged_count > 0

  return (
    <li className='col-span-3 grid grid-cols-subgrid items-center text-xs'>
      <span className='min-w-0 truncate'>
        <span className='font-mono'>{props.item.model}</span>{' '}
        <span className='text-muted-foreground'>
          ({props.item.effort || t('Model default')})
        </span>
      </span>
      <HeartbeatBar
        beats={beats}
        variant='lines'
        className='h-4'
        label={t('Degradation checks for {{model}}', {
          model: props.item.model,
        })}
      />
      <span
        className={cn(
          'min-w-12 text-right font-mono tabular-nums',
          judged
            ? getSuccessRateTextClass(props.item.pass_rate)
            : 'text-muted-foreground'
        )}
      >
        {judged ? `${props.item.pass_rate.toFixed(1)}%` : '—'}
      </span>
    </li>
  )
}
