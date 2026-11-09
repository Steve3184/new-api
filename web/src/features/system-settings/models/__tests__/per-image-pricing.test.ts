/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, it } from 'vitest'

import {
  buildPerImageExpression,
  readPerImagePrice,
} from '../per-image-pricing'

describe('per-image pricing expression', () => {
  it('builds the fixed price times the validated image count', () => {
    expect(buildPerImageExpression('0.04')).toBe(
      'tier("image", fixed(0.04)) * image_count'
    )
  })

  it('reads the unit price back from a stored expression', () => {
    expect(readPerImagePrice('tier("image", fixed(0.04)) * image_count')).toBe(
      '0.04'
    )
    // The expression editor may keep grouping parentheses from an earlier edit.
    expect(readPerImagePrice('(tier("image", fixed(1.5)) * image_count)')).toBe(
      '1.5'
    )
  })

  it('ignores expressions that are not priced per image', () => {
    expect(readPerImagePrice('tier("base", p * 2 + c * 8)')).toBeUndefined()
    // A token price multiplied by the image count is rejected by the engine,
    // so it must not reopen the per-image tab either.
    expect(
      readPerImagePrice('tier("image", p * 2) * image_count')
    ).toBeUndefined()
    expect(
      readPerImagePrice('tier("image", fixed(0.04)) * image_count * 2')
    ).toBeUndefined()
  })
})
