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
import type { AxiosAdapter } from 'axios'
import { afterEach, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { getSystemOptions } from '../api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

it('an options read started after an update returns the updated value while an earlier read is still in flight', async () => {
  let storedValue = 'before update'
  let releaseResponses = () => {}
  const responsesReleased = new Promise<void>((resolve) => {
    releaseResponses = resolve
  })
  const adapter = vi.fn<AxiosAdapter>(async (config) => {
    const value = storedValue
    await responsesReleased
    return {
      data: { success: true, data: [{ key: 'option', value }] },
      status: 200,
      statusText: 'OK',
      headers: {},
      config,
    }
  })
  api.defaults.adapter = adapter

  const readBeforeUpdate = getSystemOptions()
  await vi.waitFor(() => expect(adapter).toHaveBeenCalledTimes(1))
  storedValue = 'after update'
  const readAfterUpdate = getSystemOptions()
  releaseResponses()

  expect((await readBeforeUpdate).data).toEqual([
    { key: 'option', value: 'before update' },
  ])
  expect((await readAfterUpdate).data).toEqual([
    { key: 'option', value: 'after update' },
  ])
})
