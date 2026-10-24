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

import { formatStatusBucketRange } from './lib/status-buckets'

export type StatusHistoryMetric = 'availability' | 'ttft' | 'latency' | 'cache'

type StatusHistoryLineChartProps = {
  points: StatusHistoryPoint[]
  /** Metrics bucket length that each point covers. */
  bucketSeconds: number
  metric: StatusHistoryMetric
}

// Above this many points (e.g. 5-minute buckets over 24 hours) markers would
// crowd the curve, so they shrink to dots that keep isolated samples visible.
const DENSE_POINT_COUNT = 48

type MetricConfig = {
  labelKey: string
  color: string
  isPercent: boolean
  /** Returns the bucket's value, or null when the bucket has no sample. */
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

  // Every bucket of the window stays on the axis; buckets without samples
  // break the curve. Timestamps keep the categories unique.
  const data = useMemo(
    () =>
      props.points.map((point) => ({
        time: String(point.ts),
        range: formatStatusBucketRange(point.ts, props.bucketSeconds),
        value: config.value(point),
      })),
    [config, props.bucketSeconds, props.points]
  )
  const hasData = data.some((datum) => datum.value !== null)

  const spec = useMemo(() => {
    if (!hasData) return null
    const lineColor = config.color
    const dense = data.length > DENSE_POINT_COUNT
    const formatValue = (value: number) =>
      config.isPercent ? `${value.toFixed(2)}%` : `${Math.round(value)} ms`
    const tooltipPattern = {
      title: { value: (datum: { range: string }) => datum.range },
      content: [
        {
          key: t(config.labelKey),
          value: (datum: { value: number | null }) =>
            datum.value === null ? '—' : formatValue(datum.value),
        },
      ],
    }
    return {
      type: 'line' as const,
      data: [{ id: `status-${props.metric}`, values: data }],
      xField: 'time',
      yField: 'value',
      invalidType: 'break' as const,
      line: {
        style: {
          stroke: lineColor,
          lineWidth: dense ? 1.75 : 2.5,
          curveType: 'monotone' as const,
        },
      },
      point: {
        visible: true,
        style: dense
          ? { size: 2.5, fill: lineColor, lineWidth: 0 }
          : {
              size: 5,
              fill: lineColor,
              stroke: resolvedTheme === 'dark' ? '#171717' : '#ffffff',
              lineWidth: 1.5,
            },
      },
      legends: { visible: false },
      // The crosshair (dimension) tooltip otherwise falls back to the raw
      // timestamp category and an unformatted value.
      tooltip: { mark: tooltipPattern, dimension: tooltipPattern },
      axes: [
        {
          orient: 'bottom',
          label: {
            formatMethod: (value: number | string) =>
              dayjs.unix(Number(value)).format('HH:mm'),
            style: { fill: textColor, fontSize: 10 },
            autoLimit: true,
            autoHide: true,
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
      aria-label={t('Status history')}
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
