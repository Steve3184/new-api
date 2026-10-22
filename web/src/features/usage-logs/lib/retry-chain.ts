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
import { LOG_TYPE_ENUM } from '../constants'

export const HIDE_RETRY_FAILURES_STORAGE_KEY = 'usage_logs_hide_retry_failures'

type RetryChainLog = {
  type?: number
  request_id?: string
}

/**
 * Every attempt of an automatic retry chain writes its own error log under
 * the same request_id as the final outcome. Keeps only the final log of each
 * chain: the non-error logs when the chain recovered, otherwise the newest
 * error. Logs must be ordered newest first, as the logs API returns them.
 */
export function collapseRetryChainLogs<T extends RetryChainLog>(
  logs: T[]
): T[] {
  const recoveredRequestIds = new Set<string>()
  for (const log of logs) {
    if (log.request_id && log.type !== LOG_TYPE_ENUM.ERROR) {
      recoveredRequestIds.add(log.request_id)
    }
  }

  const keptErrorRequestIds = new Set<string>()
  return logs.filter((log) => {
    if (!log.request_id || log.type !== LOG_TYPE_ENUM.ERROR) return true
    if (recoveredRequestIds.has(log.request_id)) return false
    if (keptErrorRequestIds.has(log.request_id)) return false
    keptErrorRequestIds.add(log.request_id)
    return true
  })
}

export function readHideRetryFailuresPreference(): boolean {
  if (typeof window === 'undefined') return true
  return localStorage.getItem(HIDE_RETRY_FAILURES_STORAGE_KEY) !== 'false'
}

export function saveHideRetryFailuresPreference(enabled: boolean): void {
  localStorage.setItem(HIDE_RETRY_FAILURES_STORAGE_KEY, String(enabled))
}
