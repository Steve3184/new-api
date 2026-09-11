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
import type { PerfMetricsBucketTime } from '@/stores/system-config-store'

/**
 * Label a perf-metrics sample for a chart axis.
 *
 * The API always returns a fixed window of samples, so the admin's aggregation
 * bucket only changes how many samples land in that window. A minute-level
 * bucket packs several samples into one hour, so those labels must carry
 * minutes — otherwise neighbouring points collapse onto the same axis tick. An
 * hourly bucket spans a full day, so it needs the date instead; adding minutes
 * there would only repeat `:00`.
 */
export function formatPerfBucketLabel(
  timestampMs: number,
  bucket: PerfMetricsBucketTime
): string {
  const withMinutes = bucket !== 'hour'
  return new Date(timestampMs).toLocaleString(undefined, {
    month: withMinutes ? undefined : 'short',
    day: withMinutes ? undefined : 'numeric',
    hour: '2-digit',
    ...(withMinutes ? { minute: '2-digit' } : {}),
  })
}
