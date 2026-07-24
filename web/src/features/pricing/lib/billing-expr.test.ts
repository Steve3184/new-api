/* Copyright (C) 2023-2026 QuantumNous */
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  MATCH_EQ,
  SOURCE_IMAGE_RESOLUTION,
  buildRequestRuleExpr,
  tryParseRequestRuleExpr,
  type RequestRuleGroup,
} from './billing-expr'
import { generateExprFromVisualConfig, tryParseVisualConfig } from './tier-expr'

describe('image resolution request pricing', () => {
  const groups: RequestRuleGroup[] = [
    {
      conditions: [
        {
          source: SOURCE_IMAGE_RESOLUTION,
          mode: MATCH_EQ,
          value: '2K',
        },
      ],
      multiplier: '2',
    },
    {
      conditions: [
        {
          source: SOURCE_IMAGE_RESOLUTION,
          mode: MATCH_EQ,
          value: '4K',
        },
      ],
      multiplier: '4',
    },
  ]

  test('builds canonical image resolution conditions', () => {
    assert.equal(
      buildRequestRuleExpr(groups),
      '(image_resolution() == "2K" ? 2 : 1) * (image_resolution() == "4K" ? 4 : 1)'
    )
  })

  test('parses canonical image resolution conditions', () => {
    assert.deepEqual(
      tryParseRequestRuleExpr(buildRequestRuleExpr(groups)),
      groups
    )
  })

  test('preserves the base per-request price through visual parsing', () => {
    const expression = 'tier("base", p * 0 + c * 0 + req * 0.04)'
    const visualConfig = tryParseVisualConfig(expression)

    assert.equal(visualConfig?.tiers[0]?.request_unit_cost, 0.04)
    assert.equal(generateExprFromVisualConfig(visualConfig), expression)
  })
})
