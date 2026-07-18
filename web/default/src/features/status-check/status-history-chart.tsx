/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { VChart } from '@visactor/react-vchart'
import dayjs from 'dayjs'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import type { StatusHistoryPoint } from '@/features/performance-metrics/types'
import { useChartTheme } from '@/lib/use-chart-theme'
import { VCHART_OPTION } from '@/lib/vchart'

type StatusHistoryLineChartProps = {
  points: StatusHistoryPoint[]
  metric: 'latency' | 'cache'
}

export function StatusHistoryLineChart(props: StatusHistoryLineChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const isLatency = props.metric === 'latency'
  const textColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.68)'
      : 'rgba(15, 23, 42, 0.58)'
  const gridColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.12)'
      : 'rgba(15, 23, 42, 0.12)'

  const data = useMemo(
    () =>
      props.points
        .filter((point) =>
          isLatency ? point.ttft_sample_count > 0 : point.cache_sample_count > 0
        )
        .map((point) => ({
          time: dayjs.unix(point.ts).format('HH:mm'),
          value: isLatency ? point.avg_ttft_ms : point.cache_hit_rate,
        })),
    [isLatency, props.points]
  )

  const spec = useMemo(() => {
    if (data.length === 0) return null
    const lineColor = isLatency ? '#60a5fa' : '#22d3ee'
    return {
      type: 'line' as const,
      data: [{ id: `status-${props.metric}`, values: data }],
      xField: 'time',
      yField: 'value',
      smooth: true,
      line: {
        style: { stroke: lineColor, lineWidth: 2.5 },
      },
      point: {
        visible: true,
        style: {
          size: 5,
          fill: lineColor,
          stroke: resolvedTheme === 'dark' ? '#171717' : '#ffffff',
          lineWidth: 1.5,
        },
      },
      legends: { visible: false },
      tooltip: {
        mark: {
          title: { value: (datum: { time: string }) => datum.time },
          content: [
            {
              key: isLatency ? t('First-token latency') : t('Cache hit rate'),
              value: (datum: { value: number }) =>
                isLatency
                  ? `${Math.round(datum.value)} ms`
                  : `${datum.value.toFixed(2)}%`,
            },
          ],
        },
      },
      axes: [
        {
          orient: 'bottom',
          label: {
            style: { fill: textColor, fontSize: 10 },
            autoLimit: true,
          },
          tick: { visible: false },
        },
        {
          orient: 'left',
          min: 0,
          max: isLatency ? undefined : 100,
          nice: isLatency,
          label: {
            formatMethod: (value: number | string) =>
              isLatency ? `${value} ms` : `${value}%`,
            style: { fill: textColor, fontSize: 10 },
          },
          grid: {
            visible: true,
            style: { lineDash: [3, 3], stroke: gridColor },
          },
        },
      ],
    }
  }, [data, gridColor, isLatency, props.metric, resolvedTheme, t, textColor])

  if (data.length === 0) {
    return (
      <div className='text-muted-foreground flex h-60 items-center justify-center rounded-lg border text-sm'>
        {t('No history data available')}
      </div>
    )
  }

  return (
    <div
      className='h-60 rounded-lg border p-2'
      aria-label={t('Hourly history')}
    >
      {themeReady && spec && (
        <VChart
          key={`${props.metric}-${resolvedTheme}`}
          spec={{
            ...spec,
            theme: resolvedTheme === 'dark' ? 'dark' : 'light',
            background: 'transparent',
          }}
          option={VCHART_OPTION}
        />
      )}
    </div>
  )
}
