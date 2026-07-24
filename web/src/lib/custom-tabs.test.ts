import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { isValidCustomTabURL } from './custom-tabs'

describe('custom tab URL validation', () => {
  test('accepts internal paths and HTTP URLs independently of open behavior', () => {
    assert.equal(isValidCustomTabURL('/dashboard/overview'), true)
    assert.equal(isValidCustomTabURL('https://example.com'), true)
    assert.equal(isValidCustomTabURL(' http://example.com/path '), true)
  })

  test('rejects missing schemes and unsupported protocols', () => {
    assert.equal(isValidCustomTabURL('example.com'), false)
    assert.equal(isValidCustomTabURL('ftp://example.com'), false)
    assert.equal(isValidCustomTabURL(''), false)
  })
})
