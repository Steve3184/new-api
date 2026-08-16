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
  channelFormSchema,
  transformChannelToFormDefaults,
} from '../channel-form'

describe('stream first response timeout channel setting', () => {
  test('omits disabled timeout and persists a configured timeout', () => {
    const disabled = JSON.parse(buildSettingJSON(CHANNEL_FORM_DEFAULT_VALUES))
    expect('stream_first_response_timeout' in disabled).toBe(false)

    const configured = JSON.parse(
      buildSettingJSON({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        stream_first_response_timeout: 45,
      })
    )
    expect(configured.stream_first_response_timeout).toBe(45)
  })

  test('loads the persisted timeout into edit defaults', () => {
    const defaults = transformChannelToFormDefaults({
      id: 1,
      type: 1,
      key: '',
      status: 1,
      name: 'streaming channel',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      other: '',
      balance: 0,
      balance_updated_time: 0,
      models: 'gpt-5',
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
      setting: '{"stream_first_response_timeout":30}',
      settings: '{}',
    })

    expect(defaults.stream_first_response_timeout).toBe(30)
  })

  test('rejects negative and oversized values', () => {
    expect(
      channelFormSchema.safeParse({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        stream_first_response_timeout: -1,
      }).success
    ).toBe(false)
    expect(
      channelFormSchema.safeParse({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        stream_first_response_timeout: 86401,
      }).success
    ).toBe(false)
  })
})
