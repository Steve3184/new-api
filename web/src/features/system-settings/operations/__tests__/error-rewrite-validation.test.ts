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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  parseErrorRewriteRules,
  serializeErrorRewriteRules,
  validateErrorRewriteRules,
} from '../error-rewrite-utils'

describe('global error rewrite rule validation', () => {
  test('parses persisted rules and preserves message placeholders', () => {
    const rows = parseErrorRewriteRules(
      '[{"status_code":429,"message":"Model {model} is unavailable"}]'
    )

    assert.deepEqual(rows, [
      {
        id: 'error-rewrite-0',
        statusCode: '429',
        message: 'Model {model} is unavailable',
      },
    ])
    assert.equal(
      JSON.parse(serializeErrorRewriteRules(rows))[0].message,
      'Model {model} is unavailable'
    )
  })

  test('rejects out-of-range, fractional, empty, and duplicate rules', () => {
    const errors = validateErrorRewriteRules([
      { id: 'low', statusCode: '99', message: 'too low' },
      { id: 'fraction', statusCode: '500.5', message: 'fractional' },
      { id: 'empty', statusCode: '500', message: '  ' },
      { id: 'duplicate', statusCode: '500', message: 'duplicate' },
    ])

    assert.deepEqual(errors.low, ['invalid-status-code'])
    assert.deepEqual(errors.fraction, ['invalid-status-code'])
    assert.deepEqual(errors.empty, ['empty-message', 'duplicate-status-code'])
    assert.deepEqual(errors.duplicate, ['duplicate-status-code'])
  })

  test('serializes an empty editor as an empty JSON array', () => {
    assert.equal(serializeErrorRewriteRules([]), '[]')
  })
})
