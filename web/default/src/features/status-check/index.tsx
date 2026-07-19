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
import { Markdown } from '@/components/ui/markdown'
import { Skeleton } from '@/components/ui/skeleton'
import { getStatusCheck } from '@/features/performance-metrics/api'
import {
  formatLatency,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { StatusGroup } from '@/features/performance-metrics/types'
import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

import { AvailabilityBars } from './availability-bars'
import { StatusHistoryDrawer } from './status-history-drawer'

const STATUS_WINDOW_HOURS = 24
const STATUS_REFRESH_INTERVAL_MS = 30 * 1000

export function StatusCheck() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [selectedGroupName, setSelectedGroupName] = useState<string | null>(
    null
  )
  const query = useQuery({
    queryKey: ['status-check', STATUS_WINDOW_HOURS],
    queryFn: getStatusCheck,
    staleTime: STATUS_REFRESH_INTERVAL_MS,
    refetchInterval: STATUS_REFRESH_INTERVAL_MS,
    refetchOnWindowFocus: false,
    retry: false,
  })
  const groups = query.data?.data.groups ?? []
  const selectedGroup =
    groups.find((group) => group.group === selectedGroupName) ?? null
  const announcement =
    typeof status?.status_check_announcement === 'string'
      ? status.status_check_announcement
      : ''

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
          <div className='min-w-0 space-y-4'>
            {announcement && (
              <Markdown
                breaks
                className='bg-muted/30 rounded-md border px-4 py-3 text-sm'
              >
                {announcement}
              </Markdown>
            )}
            <div className='text-muted-foreground flex items-center gap-2 text-sm'>
              <HeartPulse className='size-4' />
              <span>{t('Passive relay metrics from the last 24 hours')}</span>
            </div>
            <StatusGroupsContent
              loading={query.isLoading}
              groups={groups}
              onSelectGroup={(group) => setSelectedGroupName(group.group)}
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
          if (!open) setSelectedGroupName(null)
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
      <div className='grid min-w-0 grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
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
    <div className='grid min-w-0 grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-3'>
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
  return (
    <button
      type='button'
      className='focus-visible:outline-primary w-full max-w-full min-w-0 overflow-hidden rounded-lg text-left focus-visible:outline-2 focus-visible:outline-offset-2'
      onClick={props.onClick}
      aria-label={t('View {{group}} status history', {
        group: props.group.group,
      })}
    >
      <Card className='hover:border-primary/50 focus-within:border-primary max-w-full min-w-0 overflow-hidden transition-colors'>
        <CardHeader className='min-w-0 border-b pb-3'>
          <div className='flex min-w-0 items-center justify-between gap-3'>
            <CardTitle className='min-w-0 truncate text-base'>
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
        <CardContent className='min-w-0 space-y-3 overflow-hidden pt-2'>
          <AvailabilityBars rates={props.group.availability_24h} />
          <div className='grid grid-cols-2 gap-3'>
            <Metric
              label={t('First-token latency')}
              value={formatLatency(props.group.avg_ttft_ms)}
            />
            <Metric
              label={t('Cache hit rate')}
              value={
                props.group.cache_input_tokens > 0
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
    <div className='bg-muted/40 min-w-0 rounded-lg px-3 py-2'>
      <div className='text-muted-foreground truncate text-xs'>
        {props.label}
      </div>
      <div className='mt-1 truncate font-mono text-sm font-semibold tabular-nums'>
        {props.value}
      </div>
    </div>
  )
}
