/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useQuery } from '@tanstack/react-query'
import { HeartPulse, RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { getStatusCheck } from '@/features/performance-metrics/api'
import {
  formatLatency,
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { StatusGroup } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

import { StatusHistoryDrawer } from './status-history-drawer'

const STATUS_WINDOW_HOURS = 24

export function StatusCheck() {
  const { t } = useTranslation()
  const [selectedGroup, setSelectedGroup] = useState<StatusGroup | null>(null)
  const query = useQuery({
    queryKey: ['status-check', STATUS_WINDOW_HOURS],
    queryFn: getStatusCheck,
    staleTime: 60 * 1000,
    refetchOnWindowFocus: false,
    retry: false,
  })
  const groups = query.data?.data.groups ?? []

  return (
    <>
      <SectionPageLayout>
        <SectionPageLayout.Title>{t('Status Check')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <Button
            variant='outline'
            size='sm'
            onClick={() => void query.refetch()}
            disabled={query.isFetching}
          >
            <RefreshCw className={cn(query.isFetching && 'animate-spin')} />
            {t('Refresh')}
          </Button>
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='space-y-4'>
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <HeartPulse className='size-4' />
              <span>{t('Passive relay metrics from the last 24 hours')}</span>
            </div>
            <StatusGroupsContent
              loading={query.isLoading}
              groups={groups}
              onSelectGroup={setSelectedGroup}
            />
            {query.isError && (
              <p className='text-destructive text-sm'>{t('Request failed')}</p>
            )}
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
      <StatusHistoryDrawer
        group={selectedGroup}
        open={selectedGroup !== null}
        onOpenChange={(open) => {
          if (!open) setSelectedGroup(null)
        }}
      />
    </>
  )
}

function StatusGroupsContent(props: {
  loading: boolean
  groups: StatusGroup[]
  onSelectGroup: (group: StatusGroup) => void
}) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
        {[0, 1, 2].map((item) => (
          <Card key={item}>
            <CardHeader>
              <Skeleton className='h-5 w-32' />
            </CardHeader>
            <CardContent className='space-y-4'>
              <Skeleton className='h-8 w-full' />
              <Skeleton className='h-10 w-full' />
            </CardContent>
          </Card>
        ))}
      </div>
    )
  }
  if (props.groups.length === 0) {
    return (
      <Card>
        <CardContent className='text-muted-foreground py-12 text-center text-sm'>
          {t('No groups configured')}
        </CardContent>
      </Card>
    )
  }
  return (
    <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
      {props.groups.map((group) => (
        <StatusGroupCard
          key={group.group}
          group={group}
          onClick={() => props.onSelectGroup(group)}
        />
      ))}
    </div>
  )
}

function StatusGroupCard(props: { group: StatusGroup; onClick: () => void }) {
  const { t } = useTranslation()
  const hasData = props.group.request_count > 0
  const bars = props.group.availability_24h
  return (
    <button
      type='button'
      className='focus-visible:outline-primary w-full rounded-lg text-left focus-visible:outline-2 focus-visible:outline-offset-2'
      onClick={props.onClick}
      aria-label={t('View {{group}} status history', {
        group: props.group.group,
      })}
    >
      <Card className='hover:border-primary/50 focus-within:border-primary overflow-hidden transition-colors'>
        <CardHeader className='border-b pb-3'>
          <div className='flex items-center justify-between gap-3'>
            <CardTitle className='truncate text-base'>
              {props.group.group}
            </CardTitle>
            <span
              className={cn(
                'shrink-0 font-mono text-sm font-semibold tabular-nums',
                hasData
                  ? getSuccessRateTextClass(props.group.availability)
                  : 'text-muted-foreground'
              )}
            >
              {hasData ? `${props.group.availability.toFixed(2)}%` : '—'}
            </span>
          </div>
        </CardHeader>
        <CardContent className='space-y-3 pt-2'>
          <div
            className='flex h-14 items-end gap-1'
            aria-label={t('Availability')}
          >
            {Array.from({ length: 24 }, (_, index) => {
              const rate = bars[index - (24 - bars.length)]
              const hasRate = typeof rate === 'number'
              const height = hasRate
                ? Math.max(5, Math.round((rate / 100) * 54))
                : 5
              return (
                <span
                  key={index}
                  className={cn(
                    'w-full rounded-t-sm transition-colors',
                    hasRate ? getSuccessRateDotClass(rate) : 'bg-muted'
                  )}
                  style={{ height: `${height}px` }}
                  title={hasRate ? `${rate.toFixed(2)}%` : t('No data')}
                />
              )
            })}
          </div>
          <div className='grid grid-cols-2 gap-3'>
            <Metric
              label={t('First-token latency')}
              value={formatLatency(props.group.avg_ttft_ms)}
            />
            <Metric
              label={t('Cache hit rate')}
              value={
                props.group.cache_sample_count > 0
                  ? `${props.group.cache_hit_rate.toFixed(2)}%`
                  : '—'
              }
            />
          </div>
        </CardContent>
      </Card>
    </button>
  )
}

function Metric(props: { label: string; value: string }) {
  return (
    <div className='bg-muted/40 rounded-lg px-3 py-2'>
      <div className='text-muted-foreground text-xs'>{props.label}</div>
      <div className='mt-1 font-mono text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}
