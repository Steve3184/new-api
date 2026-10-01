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

import {
  DEFAULT_CONFIG,
  DEFAULT_PARAMETER_ENABLED,
  STORAGE_KEYS,
} from '../../../constants'
import { getInitialPlaygroundConfig } from '../../state/playground-state-utils'
import { saveConfig } from '../../storage/storage'
import { buildChatCompletionPayload } from '../payload-builder'

describe('reasoning effort configuration and requests', () => {
  beforeEach(() => localStorage.clear())

  test.each(['none', 'low', 'medium', 'high', 'xhigh', 'max'] as const)(
    'persists %s and sends it explicitly for streaming and non-streaming requests',
    (effort) => {
      saveConfig({ ...DEFAULT_CONFIG, reasoning_effort: effort })
      const config = getInitialPlaygroundConfig()
      expect(config.reasoning_effort).toBe(effort)
      for (const stream of [true, false]) {
        const payload = buildChatCompletionPayload(
          [],
          { ...config, stream },
          DEFAULT_PARAMETER_ENABLED
        )
        expect(payload.reasoning_effort).toBe(effort)
        expect(payload.stream).toBe(stream)
      }
    }
  )

  test.each([undefined, null, 'default', 'unsupported', 3, {}])(
    'falls back for old or invalid saved effort %j without losing the model',
    (effort) => {
      localStorage.setItem(
        STORAGE_KEYS.CONFIG,
        JSON.stringify({ model: 'saved-model', reasoning_effort: effort })
      )
      const config = getInitialPlaygroundConfig()
      expect(config.model).toBe('saved-model')
      expect(config.reasoning_effort).toBe('medium')
      expect(
        buildChatCompletionPayload([], config, DEFAULT_PARAMETER_ENABLED)
      ).toHaveProperty('reasoning_effort', 'medium')
    }
  )

  test('resetting sends medium in subsequent requests', () => {
    saveConfig({ ...DEFAULT_CONFIG, reasoning_effort: 'max' })
    saveConfig({ ...getInitialPlaygroundConfig(), reasoning_effort: 'medium' })
    expect(
      buildChatCompletionPayload(
        [],
        getInitialPlaygroundConfig(),
        DEFAULT_PARAMETER_ENABLED
      )
    ).toHaveProperty('reasoning_effort', 'medium')
  })
})

test('fresh settings omit optional model parameters and explicit opt-in preserves zero values', () => {
  const payload = buildChatCompletionPayload(
    [],
    DEFAULT_CONFIG,
    DEFAULT_PARAMETER_ENABLED
  )
  for (const key of [
    'temperature',
    'top_p',
    'max_tokens',
    'frequency_penalty',
    'presence_penalty',
    'seed',
  ]) {
    expect(payload).not.toHaveProperty(key)
  }
  expect(
    buildChatCompletionPayload(
      [],
      { ...DEFAULT_CONFIG, top_p: 0 },
      { ...DEFAULT_PARAMETER_ENABLED, top_p: true }
    )
  ).toHaveProperty('top_p', 0)
})
