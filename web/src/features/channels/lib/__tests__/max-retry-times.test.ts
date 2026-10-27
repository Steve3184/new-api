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

describe('max retry times channel setting', () => {
  test('omits the disabled default and persists a configured value', () => {
    const disabled = JSON.parse(buildSettingJSON(CHANNEL_FORM_DEFAULT_VALUES))
    expect('max_retry_times' in disabled).toBe(false)

    const configured = JSON.parse(
      buildSettingJSON({ ...CHANNEL_FORM_DEFAULT_VALUES, max_retry_times: 1 })
    )
    expect(configured.max_retry_times).toBe(1)
  })

  test('loads the persisted value into edit defaults', () => {
    const defaults = transformChannelToFormDefaults({
      id: 1,
      type: 1,
      key: '',
      status: 1,
      name: 'retrying channel',
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
      setting: '{"max_retry_times":2}',
      settings: '{}',
    })

    expect(defaults.max_retry_times).toBe(2)
  })

  test('rejects negative and oversized values', () => {
    expect(
      channelFormSchema.safeParse({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        max_retry_times: -1,
      }).success
    ).toBe(false)
    expect(
      channelFormSchema.safeParse({
        ...CHANNEL_FORM_DEFAULT_VALUES,
        max_retry_times: 11,
      }).success
    ).toBe(false)
  })
})
