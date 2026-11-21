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

import { pageLocks } from '../page-locks'

describe('pageLocks', () => {
  test('runs queued requests for the same name one after another', async () => {
    const order: string[] = []
    let releaseFirst!: () => void
    const first = pageLocks.request('queue', {}, async () => {
      order.push('first:start')
      await new Promise<void>((resolve) => {
        releaseFirst = resolve
      })
      order.push('first:end')
    })
    const second = pageLocks.request('queue', {}, async () => {
      order.push('second')
    })

    await Promise.resolve()
    expect(order).toEqual(['first:start'])

    releaseFirst()
    await Promise.all([first, second])
    expect(order).toEqual(['first:start', 'first:end', 'second'])
  })

  test('reports the lock as unavailable to ifAvailable requests while it is held', async () => {
    let releaseHolder!: () => void
    const holder = pageLocks.request(
      'busy',
      {},
      () =>
        new Promise<void>((resolve) => {
          releaseHolder = resolve
        })
    )

    const whileHeld = await pageLocks.request(
      'busy',
      { ifAvailable: true },
      async (lock) => lock
    )
    expect(whileHeld).toBeNull()

    releaseHolder()
    await holder
    const afterRelease = await pageLocks.request(
      'busy',
      { ifAvailable: true },
      async (lock) => lock
    )
    expect(afterRelease).not.toBeNull()
  })
})
