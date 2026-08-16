import { describe, expect, test } from 'vitest'

import { isValidCustomTabURL } from './custom-tabs'

describe('custom tab URL validation', () => {
  test('accepts internal paths and HTTP URLs independently of open behavior', () => {
    expect(isValidCustomTabURL('/dashboard/overview')).toBe(true)
    expect(isValidCustomTabURL('https://example.com')).toBe(true)
    expect(isValidCustomTabURL(' http://example.com/path ')).toBe(true)
  })

  test('rejects missing schemes and unsupported protocols', () => {
    expect(isValidCustomTabURL('example.com')).toBe(false)
    expect(isValidCustomTabURL('ftp://example.com')).toBe(false)
    expect(isValidCustomTabURL('')).toBe(false)
  })
})
