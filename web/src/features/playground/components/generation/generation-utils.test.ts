/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, test } from 'vitest'

import {
  filterGenerationGroups,
  filterGenerationModelsForGroup,
  imageSizeFromResolution,
  normalizeImageAspectRatio,
  resolveGenerationGroup,
} from './generation-utils'

const groups = [
  { label: 'default', value: 'default', ratio: 1 },
  { label: 'auto', value: 'auto', ratio: 1 },
  { label: 'image', value: 'image', ratio: 1 },
  { label: 'speech', value: 'speech', ratio: 1 },
]
const groupModels = {
  default: ['gpt-image-2', 'tts-1'],
  auto: ['gpt-image-2'],
  image: ['gpt-image-2'],
  speech: ['tts-1'],
}

describe('generation group filtering', () => {
  test('shows eligible generation groups without exposing the default group', () => {
    expect(
      filterGenerationGroups(groups, groupModels, [
        { label: 'GPT Image 2', value: 'gpt-image-2' },
        { label: 'TTS', value: 'tts-1' },
      ]).map((group) => group.value)
    ).toEqual(['image', 'speech'])
  })

  test('hides groups without any allowed generation model before selection', () => {
    expect(
      filterGenerationGroups(groups, groupModels, [
        { label: 'TTS', value: 'tts-1' },
      ]).map((group) => group.value)
    ).toEqual(['speech'])
  })

  test('falls back to an eligible group', () => {
    expect(
      resolveGenerationGroup(
        groups,
        groupModels,
        [{ label: 'GPT Image 2', value: 'gpt-image-2' }],
        'speech'
      )
    ).toBe('image')
  })

  test('keeps an eligible selected group when it has a different allowed model', () => {
    expect(
      resolveGenerationGroup(
        groups,
        groupModels,
        [
          { label: 'GPT Image 2', value: 'gpt-image-2' },
          { label: 'TTS', value: 'tts-1' },
        ],
        'speech'
      )
    ).toBe('speech')
  })

  test('does not fall back to default when no selectable group is available', () => {
    const defaultOnlyGroups = [groups[0]]
    const models = [{ label: 'GPT Image 2', value: 'gpt-image-2' }]

    expect(
      resolveGenerationGroup(
        defaultOnlyGroups,
        groupModels,
        models,
        'default'
      )
    ).toBe('')
    expect(
      filterGenerationModelsForGroup(models, groupModels, '')
    ).toEqual([])
  })

  test('limits the model list to models available in the selected group', () => {
    expect(
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
      ).map((model) => model.value)
    ).toEqual(['gpt-image-2'])
  })
})

describe('image size controls', () => {
  test('normalizes valid aspect ratios and rejects malformed values', () => {
    expect(normalizeImageAspectRatio(' 32 : 18 ')).toBe('16:9')
    expect(normalizeImageAspectRatio('5:4')).toBe('5:4')
    expect(normalizeImageAspectRatio('0:4')).toBeNull()
    expect(normalizeImageAspectRatio('1.5:1')).toBeNull()
    expect(normalizeImageAspectRatio('square')).toBeNull()
  })

  test('combines resolution and aspect ratio into provider-compatible dimensions', () => {
    expect(imageSizeFromResolution(1024, '1:1')).toBe('1024x1024')
    expect(imageSizeFromResolution(1536, '3:2')).toBe('1536x1024')
    expect(imageSizeFromResolution(2048, '3:2')).toBe('2040x1360')
    expect(imageSizeFromResolution(2560, '16:9')).toBe('2560x1440')
    expect(imageSizeFromResolution(4096, '9:16')).toBe('2304x4096')
    expect(imageSizeFromResolution(2560, '5:4')).toBe('2560x2048')
  })
})
