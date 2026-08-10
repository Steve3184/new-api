/* Copyright (C) 2023-2026 QuantumNous */
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  filterGenerationGroups,
  filterGenerationModelsForGroup,
  imageSizeFromResolution,
  normalizeImageAspectRatio,
  resolveGenerationGroup,
} from './generation-utils'

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
  test('shows the union of groups that provide any allowed generation model', () => {
    assert.deepEqual(
      filterGenerationGroups(groups, groupModels, [
        { label: 'GPT Image 2', value: 'gpt-image-2' },
        { label: 'TTS', value: 'tts-1' },
      ]).map((group) => group.value),
      ['default', 'image', 'speech']
    )
  })

  test('hides groups without any allowed generation model before selection', () => {
    assert.deepEqual(
      filterGenerationGroups(groups, groupModels, [
        { label: 'TTS', value: 'tts-1' },
      ]).map((group) => group.value),
      ['default', 'speech']
    )
  })

  test('falls back to an eligible group', () => {
    assert.equal(
      resolveGenerationGroup(
        groups,
        groupModels,
        [{ label: 'GPT Image 2', value: 'gpt-image-2' }],
        'speech'
      ),
      'default'
    )
  })

  test('keeps an eligible selected group when it has a different allowed model', () => {
    assert.equal(
      resolveGenerationGroup(
        groups,
        groupModels,
        [
          { label: 'GPT Image 2', value: 'gpt-image-2' },
          { label: 'TTS', value: 'tts-1' },
        ],
        'speech'
      ),
      'speech'
    )
  })

  test('limits the model list to models available in the selected group', () => {
    assert.deepEqual(
      filterGenerationModelsForGroup(
        [
          { label: 'GPT Image 2', value: 'gpt-image-2' },
          { label: 'DALL-E 3', value: 'dall-e-3' },
        ],
        {
          default: ['gpt-image-2', 'dall-e-3'],
          image: ['gpt-image-2'],
        },
        'image'
      ).map((model) => model.value),
      ['gpt-image-2']
    )
  })
})

describe('image size controls', () => {
  test('normalizes valid aspect ratios and rejects malformed values', () => {
    assert.equal(normalizeImageAspectRatio(' 32 : 18 '), '16:9')
    assert.equal(normalizeImageAspectRatio('5:4'), '5:4')
    assert.equal(normalizeImageAspectRatio('0:4'), null)
    assert.equal(normalizeImageAspectRatio('1.5:1'), null)
    assert.equal(normalizeImageAspectRatio('square'), null)
  })

  test('combines resolution and aspect ratio into provider-compatible dimensions', () => {
    assert.equal(imageSizeFromResolution(1024, '1:1'), '1024x1024')
    assert.equal(imageSizeFromResolution(1536, '3:2'), '1536x1024')
    assert.equal(imageSizeFromResolution(2048, '3:2'), '2040x1360')
    assert.equal(imageSizeFromResolution(2560, '16:9'), '2560x1440')
    assert.equal(imageSizeFromResolution(4096, '9:16'), '2304x4096')
    assert.equal(imageSizeFromResolution(2560, '5:4'), '2560x2048')
  })
})
