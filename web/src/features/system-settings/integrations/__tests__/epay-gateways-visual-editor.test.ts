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
import { describe, expect, test } from 'vitest'

import { parseEpayGateways, serializeEpayGateways } from '../epay-gateways'

describe('Epay gateway visual editor serialization', () => {
  test('preserves masked secrets and nested payment fees', () => {
    const gateways = parseEpayGateways(
      JSON.stringify([
        {
          id: 'primary',
          name: 'Primary',
          address: 'https://pay.example.com',
          merchant_id: '10001',
          key: '***',
          enabled: true,
          pay_methods: [
            {
              name: 'Alipay',
              type: 'alipay',
              fee: '0.30',
              fee_rate: '3',
            },
          ],
        },
      ])
    )

    expect(gateways).toHaveLength(1)
    expect(gateways[0].key).toBe('***')
    expect(gateways[0].pay_methods[0]).toMatchObject({
      type: 'alipay',
      fee: '0.30',
      fee_rate: '3',
    })
    expect(JSON.parse(serializeEpayGateways(gateways))).toEqual(gateways)
  })

  test('ignores malformed rows instead of replacing valid gateway data', () => {
    const gateways = parseEpayGateways(
      JSON.stringify([
        null,
        { id: 12 },
        {
          id: 'backup',
          name: 'Backup',
          address: 'https://backup.example.com',
          merchant_id: '20002',
          key: 'secret',
          enabled: false,
          pay_methods: [],
        },
      ])
    )

    expect(gateways.map((gateway) => gateway.id)).toEqual(['backup'])
  })
})
