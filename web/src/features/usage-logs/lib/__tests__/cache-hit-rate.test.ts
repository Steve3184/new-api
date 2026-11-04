/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, it } from 'vitest'

import { cacheHitRate, formatCacheHitRate } from '../format'

const log = (prompt: number, other: Record<string, unknown>) => ({
  prompt_tokens: prompt,
  other: JSON.stringify(other),
})

describe('cache hit rate', () => {
  it('returns nothing when the request reported no cache read', () => {
    expect(cacheHitRate(log(1000, { cache_tokens: 0 }))).toBeUndefined()
    expect(cacheHitRate(log(1000, {}))).toBeUndefined()
    expect(cacheHitRate(log(0, { cache_tokens: 0 }))).toBeUndefined()
  })

  it('divides by the prompt for providers whose prompt already includes the cache', () => {
    expect(cacheHitRate(log(1000, { cache_tokens: 800 }))).toBeCloseTo(0.8, 5)
  })

  it('adds cache reads and writes to the input side for Claude semantics', () => {
    // Anthropic reports 1000 uncached input plus 3000 read and 1000 written.
    expect(
      cacheHitRate(
        log(1000, {
          claude: true,
          cache_tokens: 3000,
          cache_creation_tokens: 1000,
        })
      )
    ).toBeCloseTo(0.6, 5)
  })

  it('prefers the split cache write figures when Claude reports them', () => {
    expect(
      cacheHitRate(
        log(1000, {
          claude: true,
          cache_tokens: 1000,
          cache_creation_tokens: 9999,
          cache_creation_tokens_5m: 600,
          cache_creation_tokens_1h: 400,
        })
      )
    ).toBeCloseTo(1 / 3, 5)
  })

  it('never exceeds a full cache hit', () => {
    expect(cacheHitRate(log(100, { cache_tokens: 500 }))).toBe(1)
  })

  it('survives an unparsable other payload', () => {
    expect(cacheHitRate({ prompt_tokens: 10, other: '{oops' })).toBeUndefined()
  })
})

describe('cache hit rate formatting', () => {
  it('keeps one decimal place and localizes the percent sign', () => {
    expect(formatCacheHitRate(0.8125, 'en')).toBe('81.3%')
    expect(formatCacheHitRate(0.5, 'zh-CN')).toBe('50%')
  })

  it('prints a dash when there is no cache usage', () => {
    expect(formatCacheHitRate(undefined, 'en')).toBe('-')
  })
})
