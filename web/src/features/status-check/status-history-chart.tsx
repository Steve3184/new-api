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

export type StatusHistoryMetric = 'availability' | 'ttft' | 'latency' | 'cache'

type StatusHistoryLineChartProps = {
  points: StatusHistoryPoint[]
  metric: StatusHistoryMetric
}

type MetricConfig = {
  labelKey: string
  color: string
  isPercent: boolean
  /** Returns the hour's value, or null when the hour has no sample. */
  value: (point: StatusHistoryPoint) => number | null
}

const METRIC_CONFIG: Record<StatusHistoryMetric, MetricConfig> = {
  availability: {
    labelKey: 'Availability',
    color: '#34d399',
    isPercent: true,
    value: (point) => (point.request_count > 0 ? point.availability : null),
  },
  ttft: {
    labelKey: 'First token',
    color: '#a78bfa',
    isPercent: false,
    value: (point) => (point.ttft_sample_count > 0 ? point.avg_ttft_ms : null),
  },
  latency: {
    labelKey: 'Average latency',
    color: '#60a5fa',
    isPercent: false,
    value: (point) => (point.request_count > 0 ? point.avg_latency_ms : null),
  },
  cache: {
    labelKey: 'Cache hit rate',
    color: '#22d3ee',
    isPercent: true,
    value: (point) =>
      point.cache_input_tokens > 0 ? point.cache_hit_rate : null,
  },
}

export function StatusHistoryLineChart(props: StatusHistoryLineChartProps) {
  const { t } = useTranslation()
  const { resolvedTheme, themeReady } = useChartTheme()
  const config = METRIC_CONFIG[props.metric]
  const textColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.68)'
      : 'rgba(15, 23, 42, 0.58)'
  const gridColor =
    resolvedTheme === 'dark'
      ? 'rgba(255, 255, 255, 0.12)'
      : 'rgba(15, 23, 42, 0.12)'

  // Every hour stays on the axis; hours without samples break the line.
  const data = useMemo(
    () =>
      props.points.map((point) => ({
        time: dayjs.unix(point.ts).format('HH:mm'),
        value: config.value(point),
      })),
    [config, props.points]
  )
  const hasData = data.some((datum) => datum.value !== null)

  const spec = useMemo(() => {
    if (!hasData) return null
    const lineColor = config.color
    const formatValue = (value: number) =>
      config.isPercent ? `${value.toFixed(2)}%` : `${Math.round(value)} ms`
    return {
      type: 'line' as const,
      data: [{ id: `status-${props.metric}`, values: data }],
      xField: 'time',
      yField: 'value',
      smooth: true,
      invalidType: 'break' as const,
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
              key: t(config.labelKey),
              value: (datum: { value: number | null }) =>
                datum.value === null ? '—' : formatValue(datum.value),
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
          max: config.isPercent ? 100 : undefined,
          nice: !config.isPercent,
          label: {
            formatMethod: (value: number | string) =>
              config.isPercent ? `${value}%` : `${value} ms`,
            style: { fill: textColor, fontSize: 10 },
          },
          grid: {
            visible: true,
            style: { lineDash: [3, 3], stroke: gridColor },
          },
        },
      ],
    }
  }, [
    config,
    data,
    gridColor,
    hasData,
    props.metric,
    resolvedTheme,
    t,
    textColor,
  ])

  if (!hasData) {
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
