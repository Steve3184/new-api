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

import {
  buildSettingJSON,
  CHANNEL_FORM_DEFAULT_VALUES,
  transformChannelToFormDefaults,
} from '../channel-form'

describe('simulated remote compact v2 channel setting', () => {
  test('persists the feature for non-OpenAI channel types', () => {
    const setting = JSON.parse(
      buildSettingJSON({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        type: 14,
        simulate_remote_compact_v2: true,
      })
    )

    expect(setting.simulate_remote_compact_v2).toBe(true)
  })

  test('loads the persisted feature into edit defaults', () => {
    const defaults = transformChannelToFormDefaults({
      id: 1,
      type: 14,
      key: '',
      status: 1,
      name: 'compact channel',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      other: '',
      balance: 0,
      balance_updated_time: 0,
      models: 'model-test',
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
      setting: '{"simulate_remote_compact_v2":true}',
      settings: '{}',
    })

    expect(defaults.simulate_remote_compact_v2).toBe(true)
  })
})
