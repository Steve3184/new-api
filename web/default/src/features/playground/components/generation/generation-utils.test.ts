/* Copyright (C) 2023-2026 QuantumNous */
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { filterGenerationGroups } from './generation-utils'

const groups = [
  { label: 'default', value: 'default', ratio: 1 },
  { label: 'image', value: 'image', ratio: 1 },
  { label: 'speech', value: 'speech', ratio: 1 },
]
const groupModels = {
  default: ['gpt-image-2', 'tts-1'],
  image: ['gpt-image-2'],
  speech: ['tts-1'],
}

describe('generation group filtering', () => {
  test('shows only groups that provide the selected model', () => {
    assert.deepEqual(
      filterGenerationGroups(
        groups,
        groupModels,
        [{ label: 'GPT Image 2', value: 'gpt-image-2' }],
        'gpt-image-2'
      ).map((group) => group.value),
      ['default', 'image']
    )
  })

  test('hides groups without any allowed generation model before selection', () => {
    assert.deepEqual(
      filterGenerationGroups(
        groups,
        groupModels,
        [{ label: 'TTS', value: 'tts-1' }],
        ''
      ).map((group) => group.value),
      ['default', 'speech']
    )
  })
})
