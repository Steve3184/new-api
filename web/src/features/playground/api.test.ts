/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { describe, expect, test } from 'vitest'

import { normalizeVideoTaskResponse } from './api'

describe('normalizeVideoTaskResponse', () => {
  test('maps the generic task envelope to the playground video contract', () => {
    expect(
      normalizeVideoTaskResponse({
        code: 'success',
        data: {
          task_id: 'task-1',
          status: 'SUCCESS',
          progress: '100%',
          result_url: 'https://cdn.example/video.mp4',
          created_at: 10,
          finish_time: 20,
          properties: { origin_model_name: 'agnes-video-2.5-flash' },
        },
        success: true,
      })
    ).toEqual({
      id: 'task-1',
      object: 'video',
      model: 'agnes-video-2.5-flash',
      status: 'completed',
      progress: 100,
      created_at: 10,
      completed_at: 20,
      data: { url: 'https://cdn.example/video.mp4' },
    })
  })

  test('maps an unfinished task to a polling status', () => {
    expect(
      normalizeVideoTaskResponse({
        code: 'success',
        data: { task_id: 'task-2', status: 'NOT_START', progress: '10%' },
        success: true,
      })
    ).toMatchObject({ id: 'task-2', status: 'queued', progress: 10 })
  })
})
