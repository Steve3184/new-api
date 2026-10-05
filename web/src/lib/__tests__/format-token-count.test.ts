import { describe, expect, it } from 'vitest'

import { toIntlLocale } from '@/i18n/languages'

import { formatTokenCount } from '../format'

describe('formatTokenCount', () => {
  it.each([
    [999, '999'],
    [1_000, '1.00K'],
    [12_345, '12.35K'],
    [1_234_567, '1.23M'],
    [1_234_567_890, '1.23B'],
    [1_234_567_890_123, '1.23T'],
  ])('formats %s with the expected token unit', (value, expected) => {
    expect(formatTokenCount(value, 'en-US')).toBe(expected)
  })

  it.each([
    ['zhCN', '1.23M'],
    ['zhTW', '1.23M'],
    ['en', '1.23M'],
    ['fr', '1,23M'],
    ['ru', '1,23M'],
    ['ja', '1.23M'],
    ['vi', '1,23M'],
  ])('keeps token units across interface language %s', (language, expected) => {
    expect(formatTokenCount(1_234_567, toIntlLocale(language))).toBe(expected)
  })

  it('falls back safely for an invalid interface language', () => {
    expect(formatTokenCount(1_234_567, toIntlLocale('invalid_locale!'))).toBe(
      formatTokenCount(1_234_567)
    )
  })
})
