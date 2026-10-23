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
import { Markdown } from '@/components/ui/markdown'
import { Skeleton } from '@/components/ui/skeleton'
import { getStatusCheck } from '@/features/performance-metrics/api'
import type { StatusGroup } from '@/features/performance-metrics/types'
import { useStatus } from '@/hooks/use-status'
import { cn } from '@/lib/utils'

import { DegradationLegend, StatusGroupList } from './status-group-list'
import { StatusHistoryDrawer } from './status-history-drawer'

const STATUS_WINDOW_HOURS = 24
const STATUS_REFRESH_INTERVAL_MS = 30 * 1000
const DEFAULT_BUCKET_SECONDS = 3600

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
  const bucketSeconds =
    query.data?.data.bucket_seconds || DEFAULT_BUCKET_SECONDS
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
              <span>
                {t(
                  'Relay metrics and active check results from the last 24 hours'
                )}
              </span>
            </div>
            <StatusGroupsContent
              loading={query.isLoading}
              groups={groups}
              bucketSeconds={bucketSeconds}
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
        bucketSeconds={bucketSeconds}
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
  bucketSeconds: number
  onSelectGroup: (group: StatusGroup) => void
}) {
  const { t } = useTranslation()
  if (props.loading) {
    return (
      <div className='bg-border grid grid-cols-1 gap-px overflow-hidden rounded-lg border xl:grid-cols-2'>
        {[0, 1, 2, 3].map((item) => (
          <div key={item} className='bg-card flex items-center gap-4 px-4 py-3'>
            <Skeleton className='h-6 w-16 rounded-full' />
            <Skeleton className='h-5 w-32' />
            <Skeleton className='h-5 flex-1' />
          </div>
        ))}
      </div>
    )
  }
  if (props.groups.length === 0) {
    return (
      <div className='text-muted-foreground rounded-lg border py-12 text-center text-sm'>
        {t('No groups configured')}
      </div>
    )
  }
  const hasDegradation = props.groups.some(
    (group) => (group.degradation?.length ?? 0) > 0
  )
  return (
    <div className='min-w-0 space-y-2'>
      <StatusGroupList
        groups={props.groups}
        bucketSeconds={props.bucketSeconds}
        onSelectGroup={props.onSelectGroup}
      />
      {hasDegradation && <DegradationLegend />}
    </div>
  )
}
