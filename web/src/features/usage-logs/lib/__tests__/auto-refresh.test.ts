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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getUsageLogsAutoRefreshInterval,
  USAGE_LOGS_AUTO_REFRESH_INTERVAL_MS,
} from '../auto-refresh'

describe('usage logs auto refresh interval', () => {
  test('refreshes enabled common logs on the first page every five seconds', () => {
    assert.equal(
      getUsageLogsAutoRefreshInterval(true, 'common', 0),
      USAGE_LOGS_AUTO_REFRESH_INTERVAL_MS
    )
  })

  test('does not refresh disabled, paginated, or non-common logs', () => {
    assert.equal(getUsageLogsAutoRefreshInterval(false, 'common', 0), false)
    assert.equal(getUsageLogsAutoRefreshInterval(true, 'common', 1), false)
    assert.equal(getUsageLogsAutoRefreshInterval(true, 'drawing', 0), false)
  })
})
