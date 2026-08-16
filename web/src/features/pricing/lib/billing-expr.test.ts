/* Copyright (C) 2023-2026 QuantumNous */
import { describe, expect, test } from 'vitest'

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
    expect(buildRequestRuleExpr(groups)).toBe(
      '(image_resolution() == "2K" ? 2 : 1) * (image_resolution() == "4K" ? 4 : 1)'
    )
  })

  test('parses canonical image resolution conditions', () => {
    expect(tryParseRequestRuleExpr(buildRequestRuleExpr(groups))).toEqual(
      groups
    )
  })

  test('preserves the base per-request price through visual parsing', () => {
    const expression = 'tier("base", p * 0 + c * 0 + req * 0.04)'
    const visualConfig = tryParseVisualConfig(expression)

    expect(visualConfig?.tiers[0]?.request_unit_cost).toBe(0.04)
    expect(generateExprFromVisualConfig(visualConfig)).toBe(expression)
  })
})
