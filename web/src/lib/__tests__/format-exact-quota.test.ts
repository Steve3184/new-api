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
import { beforeEach, describe, expect, test } from 'vitest'

import { useSystemConfigStore } from '@/stores/system-config-store'

import { formatExactQuota, formatQuota } from '../format'

describe('formatExactQuota', () => {
  beforeEach(() => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...state.config.currency,
          quotaDisplayType: 'TOKENS',
          quotaPerUnit: 500000,
          usdExchangeRate: 1,
        },
      },
    }))
  })

  test('shows token amounts in full where formatQuota abbreviates them', () => {
    expect(formatQuota(1_500_000)).toMatch(/k$/)
    // Token mode prints plain digits without grouping, as the usage logs do.
    expect(formatExactQuota(1_500_000, 'en')).toBe('1500000')
  })
})
