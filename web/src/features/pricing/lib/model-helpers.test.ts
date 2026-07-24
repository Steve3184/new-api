/* Copyright (C) 2023-2026 QuantumNous */
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { getDisplayGroupRatio } from './model-helpers'

const model = {
  id: 1,
  model_name: 'test-model',
  quota_type: 0 as const,
  model_ratio: 1,
  completion_ratio: 1,
  enable_groups: ['default', 'vip'],
  group_ratio: { default: 1, vip: 0.5 },
}

describe('model square display group ratio', () => {
  test('uses the lowest enabled group ratio without a filter', () => {
    assert.equal(getDisplayGroupRatio(model), 0.5)
  })

  test('uses the selected group ratio when a filter is active', () => {
    assert.equal(getDisplayGroupRatio(model, 'default'), 1)
  })

  test('supports models enabled for every group', () => {
    assert.equal(
      getDisplayGroupRatio(
        { ...model, enable_groups: ['all'], group_ratio: { default: 1, vip: 0.5 } },
        undefined
      ),
      0.5
    )
  })

  test('falls back to the base ratio without usable group prices', () => {
    assert.equal(
      getDisplayGroupRatio({ ...model, group_ratio: {} }),
      1
    )
  })
})
