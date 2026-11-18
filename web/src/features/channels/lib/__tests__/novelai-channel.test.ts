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

import { CHANNEL_TYPE_NOVELAI, CHANNEL_TYPE_OPTIONS } from '../../constants'
import { channelSchema } from '../../types'
import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
  type ChannelFormValues,
} from '../channel-form'
import {
  CHANNEL_TYPE_CONFIGS,
  getChannelTypeConfig,
} from '../channel-type-config'

const NOVELAI_MODELS = [
  'nai-diffusion-3',
  'nai-diffusion-furry-3',
  'nai-diffusion-4-curated-preview',
  'nai-diffusion-4-full',
  'nai-diffusion-4-5-curated',
  'nai-diffusion-4-5-full',
  'nai-diffusion-5-curated',
  'nai-diffusion-5-full',
]

function novelAIForm(overrides: Partial<ChannelFormValues> = {}) {
  return channelFormSchema.parse({
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'NovelAI',
    type: CHANNEL_TYPE_NOVELAI,
    key: 'test-token',
    models: NOVELAI_MODELS.join(','),
    ...overrides,
  })
}

function savedSettings(form: ChannelFormValues): Record<string, unknown> {
  const payload = transformFormDataToCreatePayload(
    channelFormSchema.parse(form)
  )
  return JSON.parse(payload.channel.settings ?? '{}')
}

describe('NovelAI channel form', () => {
  test('registers type 68 and exposes the supported diffusion models', () => {
    expect(CHANNEL_TYPE_OPTIONS).toContainEqual({
      value: CHANNEL_TYPE_NOVELAI,
      label: 'NovelAI',
    })
    expect(CHANNEL_TYPE_CONFIGS[CHANNEL_TYPE_NOVELAI]?.supportedModels).toEqual(
      NOVELAI_MODELS
    )
    expect(getChannelTypeConfig(CHANNEL_TYPE_NOVELAI).hints).toMatchObject({
      baseUrl: 'NovelAI image API base URL',
      key: 'NovelAI API token',
      models: 'NovelAI diffusion model names',
    })
  })

  test('defaults the 28-step limit to off and persists an enabled value', () => {
    expect(CHANNEL_FORM_DEFAULT_VALUES.novelai_max_28_steps).toBe(false)
    expect(
      savedSettings(novelAIForm({ novelai_max_28_steps: true }))
    ).toMatchObject({ novelai_max_28_steps: true })
  })

  test('loads the saved 28-step limit when editing a NovelAI channel', () => {
    const channel = channelSchema.parse({
      id: 68,
      name: 'NovelAI',
      type: CHANNEL_TYPE_NOVELAI,
      key: '',
      status: 1,
      created_time: 1,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      models: NOVELAI_MODELS.join(','),
      group: 'default',
      settings: JSON.stringify({ novelai_max_28_steps: true }),
    })

    expect(transformChannelToFormDefaults(channel).novelai_max_28_steps).toBe(
      true
    )
  })

  test('removes the NovelAI-only setting after switching provider types', () => {
    const settings = savedSettings(
      novelAIForm({
        type: 1,
        settings: JSON.stringify({ novelai_max_28_steps: true }),
      })
    )

    expect(settings).not.toHaveProperty('novelai_max_28_steps')
  })

  test('persists the enabled NovelAI setting in update payloads', () => {
    const payload = transformFormDataToUpdatePayload(
      novelAIForm({ novelai_max_28_steps: true }),
      68
    )

    expect(JSON.parse(payload.settings ?? '{}')).toMatchObject({
      novelai_max_28_steps: true,
    })
  })
})
