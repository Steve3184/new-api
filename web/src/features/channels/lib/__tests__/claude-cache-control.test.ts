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
*/
import { describe, expect, test } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
} from '../channel-form'

describe('Claude stable prefix cache control setting', () => {
  test('persists the switch as disabled by default and enabled when selected', () => {
    const disabled = JSON.parse(
      transformFormDataToCreatePayload({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        type: 14,
      }).channel.settings ?? '{}'
    )
    expect(disabled.claude_cache_control).toBe(false)

    const enabled = JSON.parse(
      transformFormDataToCreatePayload({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        type: 14,
        claude_cache_control: true,
      }).channel.settings ?? '{}'
    )
    expect(enabled.claude_cache_control).toBe(true)
  })

  test('loads the persisted switch value and removes it from non-Claude channels', () => {
    const defaults = transformChannelToFormDefaults({
      id: 1,
      type: 14,
      key: '',
      status: 1,
      name: 'Anthropic channel',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      other: '',
      balance: 0,
      balance_updated_time: 0,
      models: 'claude-test',
      group: 'default',
      used_quota: 0,
      other_info: '',
      remark: '',
      max_input_tokens: 0,
      channel_info: {
        is_multi_key: false,
        multi_key_size: 0,
        multi_key_polling_index: 0,
        multi_key_mode: 'random',
      },
      setting: '{}',
      settings: '{"claude_cache_control":true}',
    })
    expect(defaults.claude_cache_control).toBe(true)

    const nonClaudeSettings = JSON.parse(
      transformFormDataToCreatePayload({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        type: 1,
        settings: '{"claude_cache_control":true}',
        claude_cache_control: true,
      }).channel.settings ?? '{}'
    )
    expect('claude_cache_control' in nonClaudeSettings).toBe(false)
  })
})
