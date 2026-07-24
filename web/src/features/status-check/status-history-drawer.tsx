/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { Gauge, TimerReset } from 'lucide-react'
import { lazy, Suspense } from 'react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  SideDrawerSection,
  SideDrawerSectionHeader,
} from '@/components/drawer-layout'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import type { StatusGroup } from '@/features/performance-metrics/types'

const StatusHistoryLineChart = lazy(() =>
  import('./status-history-chart').then((module) => ({
    default: module.StatusHistoryLineChart,
  }))
)

type StatusHistoryDrawerProps = {
  group: StatusGroup | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function StatusHistoryDrawer(props: StatusHistoryDrawerProps) {
  const { t } = useTranslation()
  const history = props.group?.history_24h ?? []

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{props.group?.group ?? t('Status Check')}</SheetTitle>
          <SheetDescription>
            {t('First-token latency and cache hit rate over the last 24 hours')}
          </SheetDescription>
        </SheetHeader>
        <div className={sideDrawerFormClassName()}>
          <SideDrawerSection>
            <SideDrawerSectionHeader
              title={t('First-token latency')}
              description={t('Synchronous requests are excluded.')}
              icon={<TimerReset aria-hidden='true' />}
              iconTone='chart-1'
            />
            <Suspense
              fallback={<Skeleton className='h-60 w-full rounded-lg' />}
            >
              <StatusHistoryLineChart points={history} metric='latency' />
            </Suspense>
          </SideDrawerSection>
          <SideDrawerSection>
            <SideDrawerSectionHeader
              title={t('Cache hit rate')}
              description={t('Configured excluded models are omitted.')}
              icon={<Gauge aria-hidden='true' />}
              iconTone='chart-2'
            />
            <Suspense
              fallback={<Skeleton className='h-60 w-full rounded-lg' />}
            >
              <StatusHistoryLineChart points={history} metric='cache' />
            </Suspense>
          </SideDrawerSection>
        </div>
      </SheetContent>
    </Sheet>
  )
}
