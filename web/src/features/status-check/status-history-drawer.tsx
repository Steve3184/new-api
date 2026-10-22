/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { Activity, Gauge, TimerReset, Zap } from 'lucide-react'
import { lazy, type ReactNode, Suspense } from 'react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  SideDrawerSection,
  SideDrawerSectionHeader,
} from '@/components/drawer-layout'
import type { IconBadgeTone } from '@/components/ui/icon-badge'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Skeleton } from '@/components/ui/skeleton'
import type { StatusGroup } from '@/features/performance-metrics/types'

import type { StatusHistoryMetric } from './status-history-chart'

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

type HistorySection = {
  metric: StatusHistoryMetric
  titleKey: string
  descriptionKey: string
  icon: ReactNode
  iconTone: IconBadgeTone
}

const HISTORY_SECTIONS: HistorySection[] = [
  {
    metric: 'availability',
    titleKey: 'Availability',
    descriptionKey:
      'Hours without real requests use model degradation checks when enabled.',
    icon: <Activity aria-hidden='true' />,
    iconTone: 'chart-3',
  },
  {
    metric: 'ttft',
    titleKey: 'First token',
    descriptionKey: 'Average time to the first streamed token.',
    icon: <Zap aria-hidden='true' />,
    iconTone: 'chart-4',
  },
  {
    metric: 'latency',
    titleKey: 'Average latency',
    descriptionKey: 'Includes passive requests and flexible active probes.',
    icon: <TimerReset aria-hidden='true' />,
    iconTone: 'chart-1',
  },
  {
    metric: 'cache',
    titleKey: 'Cache hit rate',
    descriptionKey: 'Configured excluded models are omitted.',
    icon: <Gauge aria-hidden='true' />,
    iconTone: 'chart-2',
  },
]

export function StatusHistoryDrawer(props: StatusHistoryDrawerProps) {
  const { t } = useTranslation()
  const history = props.group?.history_24h ?? []

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='left'
        className={sideDrawerContentClassName('sm:max-w-xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{props.group?.group ?? t('Status Check')}</SheetTitle>
          <SheetDescription>
            {t('Hourly status data for the last 24 hours')}
          </SheetDescription>
        </SheetHeader>
        <div className={sideDrawerFormClassName()}>
          {HISTORY_SECTIONS.map((section) => (
            <SideDrawerSection key={section.metric}>
              <SideDrawerSectionHeader
                title={t(section.titleKey)}
                description={t(section.descriptionKey)}
                icon={section.icon}
                iconTone={section.iconTone}
              />
              <Suspense
                fallback={<Skeleton className='h-60 w-full rounded-lg' />}
              >
                <StatusHistoryLineChart
                  points={history}
                  metric={section.metric}
                />
              </Suspense>
            </SideDrawerSection>
          ))}
        </div>
      </SheetContent>
    </Sheet>
  )
}
