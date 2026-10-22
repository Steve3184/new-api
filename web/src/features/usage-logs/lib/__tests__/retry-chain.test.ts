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
import { afterEach, describe, expect, test } from 'vitest'

import { LOG_TYPE_ENUM } from '../../constants'
import {
  collapseRetryChainLogs,
  HIDE_RETRY_FAILURES_STORAGE_KEY,
  readHideRetryFailuresPreference,
  saveHideRetryFailuresPreference,
} from '../retry-chain'

const consume = (id: number, requestId: string) => ({
  id,
  type: LOG_TYPE_ENUM.CONSUME,
  request_id: requestId,
})
const failure = (id: number, requestId: string) => ({
  id,
  type: LOG_TYPE_ENUM.ERROR,
  request_id: requestId,
})

describe('collapseRetryChainLogs', () => {
  test('recovered retry chain keeps only the successful log', () => {
    const logs = [consume(3, 'req-a'), failure(2, 'req-a'), failure(1, 'req-a')]

    expect(collapseRetryChainLogs(logs).map((log) => log.id)).toEqual([3])
  })

  test('fully failed retry chain keeps only the newest error', () => {
    const logs = [failure(3, 'req-a'), failure(2, 'req-a'), failure(1, 'req-a')]

    expect(collapseRetryChainLogs(logs).map((log) => log.id)).toEqual([3])
  })

  test('logs from separate requests and logs without request id are kept', () => {
    const logs = [
      failure(4, 'req-b'),
      consume(3, 'req-a'),
      failure(2, ''),
      failure(1, ''),
    ]

    expect(collapseRetryChainLogs(logs).map((log) => log.id)).toEqual([
      4, 3, 2, 1,
    ])
  })
})

describe('hide retry failures preference', () => {
  afterEach(() => {
    localStorage.removeItem(HIDE_RETRY_FAILURES_STORAGE_KEY)
  })

  test('defaults to enabled when nothing is saved', () => {
    expect(readHideRetryFailuresPreference()).toBe(true)
  })

  test('restores a saved disabled preference', () => {
    saveHideRetryFailuresPreference(false)

    expect(readHideRetryFailuresPreference()).toBe(false)
  })
})
