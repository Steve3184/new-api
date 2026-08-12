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
import { afterEach, describe, test } from 'node:test'

import { api } from '@/lib/api'

import { getTokenGroupNames, migrateTokenGroup } from './api'

type ApiMethod = (url: string, data?: unknown) => Promise<{ data: unknown }>
type MockableApi = {
  get: ApiMethod
  post: ApiMethod
}

const apiClient = api as unknown as MockableApi
const originalGet = apiClient.get
const originalPost = apiClient.post

afterEach(() => {
  apiClient.get = originalGet
  apiClient.post = originalPost
})

describe('token group migration API', () => {
  test('loads source and target group choices from the admin endpoint', async () => {
    apiClient.get = async (url) => {
      assert.equal(url, '/api/token/group-names')
      return {
        data: {
          success: true,
          data: {
            source_groups: ['ClaudeA'],
            target_groups: ['ClaudeB'],
          },
        },
      }
    }

    const result = await getTokenGroupNames()

    assert.deepEqual(result.data, {
      source_groups: ['ClaudeA'],
      target_groups: ['ClaudeB'],
    })
  })

  test('posts the selected source and target groups without renaming fields', async () => {
    apiClient.post = async (url, data) => {
      assert.equal(url, '/api/token/group/migrate')
      assert.deepEqual(data, {
        source_group: 'ClaudeA',
        target_group: 'ClaudeB',
      })
      return {
        data: {
          success: true,
          data: {
            source_group: 'ClaudeA',
            target_group: 'ClaudeB',
            migrated: 12,
          },
        },
      }
    }

    const result = await migrateTokenGroup('ClaudeA', 'ClaudeB')

    assert.equal(result.data?.migrated, 12)
  })
})
