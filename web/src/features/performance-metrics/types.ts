/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
export type PerformanceSummary = {
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
}

export type PerformanceSeriesPoint = {
  ts: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
}

export type PerformanceGroup = {
  group: string
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  series: PerformanceSeriesPoint[]
}

export type PerformanceMetricsData = {
  success: boolean
  message?: string
  data: {
    summary?: PerformanceSummary | null
    series?: PerformanceSeriesPoint[]
    window_start?: number
    window_end?: number
    model_name: string
    series_schema?: string
    groups: PerformanceGroup[]
  }
}

export type SuccessRatePoint = { ts: number; success_rate: number }

export type PerfModelSummary = {
  model_name: string
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_series?: SuccessRatePoint[]
  request_count?: number
}

export type PerfSummaryAllData = {
  success: boolean
  message?: string
  data: {
    summary?: PerformanceSummary | null
    window_start?: number
    window_end?: number
    models: PerfModelSummary[]
    recent_success_series?: Record<string, SuccessRatePoint[]>
  }
}

export type StatusGroup = {
  group: string
  availability: number
  avg_latency_ms: number
  avg_ttft_ms: number
  cache_hit_rate: number
  cache_sample_count: number
  cache_input_tokens: number
  request_count: number
  availability_24h: number[]
  history_24h: StatusHistoryPoint[]
  /** Hourly beats, oldest first; longer than 24 hours so wide rows fill up. */
  beats: StatusBeat[]
  degradation?: StatusDegradationModel[]
}

/** One hour of the status strip; `request_count` is 0 for hours without data. */
export type StatusBeat = {
  ts: number
  request_count: number
  availability: number
  avg_ttft_ms: number
  /** The hour had no real requests and uses degradation check samples. */
  probe_filled: boolean
}

export type StatusDegradationCheckStatus = 'passed' | 'degraded' | 'error'

export type StatusDegradationCheck = {
  ts: number
  status: StatusDegradationCheckStatus
  answer: string
  latency_ms: number
}

/** Configured model/effort row of a status group's degradation checks. */
export type StatusDegradationModel = {
  model: string
  effort: string
  /** Share of passed checks among judged checks; errors are excluded. */
  pass_rate: number
  judged_count: number
  checks: StatusDegradationCheck[]
}

/** One hour of a status group; `request_count` is 0 for hours without data. */
export type StatusHistoryPoint = {
  ts: number
  request_count: number
  availability: number
  /** The hour had no real requests and uses degradation check samples. */
  probe_filled: boolean
  avg_latency_ms: number
  avg_ttft_ms: number
  ttft_sample_count: number
  cache_hit_rate: number
  cache_sample_count: number
  cache_input_tokens: number
}

export type StatusCheckData = {
  success: boolean
  message?: string
  data: {
    groups: StatusGroup[]
  }
}
