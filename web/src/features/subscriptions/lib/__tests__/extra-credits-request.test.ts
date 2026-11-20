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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  clearPendingExtraCreditsPurchase,
  EXTRA_CREDITS_PENDING_STORAGE_KEY,
  EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
  readPendingExtraCreditsPurchase,
  savePendingExtraCreditsPurchase,
  readExtraCreditsPurchaseState,
  resetExtraCreditsPageForTests,
  resolvePendingExtraCreditsPurchase,
} from '../extra-credits-request'

const payload = {
  request_id: 'abcdefghijklmnop-1234',
  wallet_quota: 1_000_000,
  expected_rate: 2,
}

beforeEach(() => {
  resetExtraCreditsPageForTests()
  window.sessionStorage.clear()
  window.localStorage.clear()
})

describe('pending extra-credits purchase storage', () => {
  test('a fresh tab cannot recover a refusal when the shared result write fails', () => {
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    const set = Storage.prototype.setItem
    const writes = vi
      .spyOn(Storage.prototype, 'setItem')
      .mockImplementation(function (this: Storage, key, value) {
        if (
          this === localStorage &&
          key === EXTRA_CREDITS_RESOLVED_STORAGE_KEY
        ) {
          throw new Error('full')
        }
        set.call(this, key, value)
      })
    try {
      expect(
        resolvePendingExtraCreditsPurchase(1, 9, payload, 'rejected')
      ).toBe(false)
      expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
      expect(
        JSON.parse(
          sessionStorage.getItem(EXTRA_CREDITS_RESOLVED_STORAGE_KEY) ?? '{}'
        )[`1:9:${payload.request_id}`].status
      ).toBe('rejected')
      sessionStorage.clear()
      resetExtraCreditsPageForTests()
      expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
    } finally {
      writes.mockRestore()
    }
  })

  test.each([false, true])(
    'repairs a missing shared order before the tab closes (other order=%s)',
    (hasOther) => {
      const other = { ...payload, request_id: 'independent-tab-order' }
      expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
      localStorage.clear()
      if (hasOther) {
        localStorage.setItem(
          EXTRA_CREDITS_PENDING_STORAGE_KEY,
          JSON.stringify({ '1:9': other })
        )
      }
      expect(readExtraCreditsPurchaseState(1, 9).pending).toEqual(payload)
      sessionStorage.clear()
      resetExtraCreditsPageForTests()
      if (hasOther) {
        expect(readExtraCreditsPurchaseState(1, 9).pending).toEqual(other)
        expect(resolvePendingExtraCreditsPurchase(1, 9, other)).toBe(true)
      }
      expect(readExtraCreditsPurchaseState(1, 9).pending).toEqual(payload)
    }
  )

  test('repairs a session-only refusal once shared storage recovers without losing another order', () => {
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    expect(savePendingExtraCreditsPurchase(1, 10, payload)).toBe(true)
    const set = Storage.prototype.setItem
    const writes = vi
      .spyOn(Storage.prototype, 'setItem')
      .mockImplementation(function (this: Storage, key, value) {
        if (this === localStorage) throw new Error('temporarily full')
        set.call(this, key, value)
      })
    try {
      expect(
        resolvePendingExtraCreditsPurchase(1, 9, payload, 'rejected')
      ).toBe(false)
    } finally {
      writes.mockRestore()
    }
    resetExtraCreditsPageForTests()
    expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
    sessionStorage.clear()
    resetExtraCreditsPageForTests()
    expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
    expect(readExtraCreditsPurchaseState(1, 10).pending).toEqual(payload)
  })

  test.each([{ wallet_quota: 2 }, { expected_rate: 3 }])(
    'a mismatched stored receipt (%o) cannot settle or block the real order',
    (changed) => {
      expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
      const key = `1:9:${payload.request_id}`
      localStorage.setItem(
        EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
        JSON.stringify({
          [key]: { ...payload, ...changed, status: 'rejected', resolved_at: 1 },
        })
      )
      const state = readExtraCreditsPurchaseState(1, 9)
      expect(state.pending).toEqual(payload)
      expect(state.outcomes[key]).toBeUndefined()
      expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
      expect(resolvePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
      sessionStorage.clear()
      resetExtraCreditsPageForTests()
      expect(readExtraCreditsPurchaseState(1, 9).outcomes[key]).toMatchObject({
        ...payload,
        status: 'purchased',
      })
    }
  )

  test.each([1, Number.MAX_SAFE_INTEGER])(
    'an invalid grant for wallet_quota=%s cannot hide a valid retry for the same subscription',
    (quota) => {
      const bad = {
        request_id: 'invalid-grant-purchase',
        wallet_quota: quota,
        expected_rate: quota === 1 ? 0.000001 : 2,
      }
      expect(savePendingExtraCreditsPurchase(1, 9, bad)).toBe(false)
      localStorage.setItem(
        EXTRA_CREDITS_PENDING_STORAGE_KEY,
        JSON.stringify({ '1:9': bad, [`1:9:${payload.request_id}`]: payload })
      )
      expect(readExtraCreditsPurchaseState(1, 9).pending).toEqual(payload)
      expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    }
  )

  test.each(['purchased', 'rejected'] as const)(
    'never replays a %s order after both outcome writes fail',
    (status) => {
      expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
      const write = vi
        .spyOn(Storage.prototype, 'setItem')
        .mockImplementation(() => {
          throw new Error('quota exceeded')
        })
      try {
        expect(resolvePendingExtraCreditsPurchase(1, 9, payload, status)).toBe(
          false
        )
      } finally {
        write.mockRestore()
      }
      expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
      expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(false)
    }
  )

  test('replays the tab order before a different shared order without losing either', () => {
    const other = {
      ...payload,
      request_id: 'independent-tab-order',
      expected_rate: 3,
    }
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    window.localStorage.setItem(
      EXTRA_CREDITS_PENDING_STORAGE_KEY,
      JSON.stringify({ '1:9': other })
    )
    expect(readExtraCreditsPurchaseState(1, 9).pending).toEqual(payload)
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    window.localStorage.clear()
    resetExtraCreditsPageForTests()
    expect(readExtraCreditsPurchaseState(1, 9).pending).toEqual(payload)
    expect(resolvePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    expect(readExtraCreditsPurchaseState(1, 9).pending).toEqual(other)
    expect(savePendingExtraCreditsPurchase(1, 9, other)).toBe(true)
  })

  test('keeps each definitive result even after another purchase resolves', () => {
    savePendingExtraCreditsPurchase(1, 9, payload)
    expect(resolvePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    const newer = { ...payload, request_id: 'second-completed-request-1234' }
    savePendingExtraCreditsPurchase(1, 9, newer)
    expect(resolvePendingExtraCreditsPurchase(1, 9, newer)).toBe(true)
    window.localStorage.setItem(
      EXTRA_CREDITS_PENDING_STORAGE_KEY,
      JSON.stringify({ '1:9': payload })
    )
    expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
  })

  test('does not resurrect a tab retry after observing a remote refusal and losing shared storage', () => {
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    window.localStorage.setItem(
      EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
      JSON.stringify({
        [`1:9:${payload.request_id}`]: {
          ...payload,
          status: 'rejected',
          resolved_at: 1,
        },
      })
    )
    expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
    window.localStorage.clear()
    resetExtraCreditsPageForTests()
    expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
  })

  test('a definitive outcome dominates its pending record when cleanup fails', () => {
    savePendingExtraCreditsPurchase(1, 9, payload)
    const remove = vi
      .spyOn(Storage.prototype, 'removeItem')
      .mockImplementation(() => {
        throw new Error('blocked')
      })
    expect(resolvePendingExtraCreditsPurchase(1, 9, payload)).toBe(false)
    expect(readExtraCreditsPurchaseState(1, 9).pending).toBeNull()
    remove.mockRestore()
  })

  test('keeps an uncertain purchase after the original tab session ends', () => {
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    window.sessionStorage.clear()
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(payload)
    expect(
      window.localStorage.getItem(EXTRA_CREDITS_PENDING_STORAGE_KEY)
    ).not.toBeNull()
  })
  test('a late response never removes a newer pending purchase', () => {
    savePendingExtraCreditsPurchase(1, 9, payload)
    const newer = { ...payload, request_id: 'newer-purchase-abcdefghijkl' }
    clearPendingExtraCreditsPurchase(1, 9, payload.request_id)
    savePendingExtraCreditsPurchase(1, 9, newer)
    clearPendingExtraCreditsPurchase(1, 9, payload.request_id)
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(newer)
  })

  test('refuses to save when storage is unavailable', () => {
    expect(savePendingExtraCreditsPurchase(1, 9, payload, null)).toBe(false)
  })

  test('never overwrites an unsettled purchase with a different request id', () => {
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    const other = { ...payload, request_id: 'second-purchase-abcdefghij' }
    expect(savePendingExtraCreditsPurchase(1, 9, other)).toBe(false)
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(payload)
  })

  test('never changes the amount of an existing request id', () => {
    expect(savePendingExtraCreditsPurchase(1, 9, payload)).toBe(true)
    expect(
      savePendingExtraCreditsPurchase(1, 9, {
        ...payload,
        wallet_quota: 2_000_000,
      })
    ).toBe(false)
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(payload)
  })

  test('refuses to save when the stored purchases cannot be read', () => {
    // An earlier, possibly charged purchase may be hidden behind the read
    // error; saving over it would let a new id charge twice.
    window.localStorage.setItem(
      EXTRA_CREDITS_PENDING_STORAGE_KEY,
      JSON.stringify({ '1:9': payload })
    )
    const getItem = vi
      .spyOn(Storage.prototype, 'getItem')
      .mockImplementation(() => {
        throw new Error('storage temporarily unavailable')
      })
    try {
      const other = { ...payload, request_id: 'second-purchase-abcdefghij' }
      expect(savePendingExtraCreditsPurchase(1, 9, other)).toBe(false)
    } finally {
      getItem.mockRestore()
    }
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(payload)
  })
  test('returns the exact saved body for the same user and subscription', () => {
    savePendingExtraCreditsPurchase(1, 9, payload)
    expect(readPendingExtraCreditsPurchase(1, 9)).toEqual(payload)
  })

  test('scopes pending bodies by user and by subscription', () => {
    savePendingExtraCreditsPurchase(1, 9, payload)
    expect(readPendingExtraCreditsPurchase(2, 9)).toBeNull()
    expect(readPendingExtraCreditsPurchase(1, 10)).toBeNull()
  })

  test('clearing one entry keeps the others and drops the key when empty', () => {
    savePendingExtraCreditsPurchase(1, 9, payload)
    savePendingExtraCreditsPurchase(1, 10, payload)
    clearPendingExtraCreditsPurchase(1, 9)
    expect(readPendingExtraCreditsPurchase(1, 10)).toEqual(payload)
    clearPendingExtraCreditsPurchase(1, 10)
    expect(
      window.localStorage.getItem(EXTRA_CREDITS_PENDING_STORAGE_KEY)
    ).toBeNull()
  })

  test('ignores corrupt or out-of-range stored bodies', () => {
    window.localStorage.setItem(EXTRA_CREDITS_PENDING_STORAGE_KEY, '{oops')
    expect(readPendingExtraCreditsPurchase(1, 9)).toBeNull()
    for (const bad of [
      { ...payload, request_id: 'short' },
      { ...payload, wallet_quota: 2 ** 53 },
      { ...payload, expected_rate: 0.0000001 },
      { ...payload, expected_rate: 1.1234567 },
    ]) {
      window.localStorage.setItem(
        EXTRA_CREDITS_PENDING_STORAGE_KEY,
        JSON.stringify({ '1:9': bad })
      )
      expect(readPendingExtraCreditsPurchase(1, 9)).toBeNull()
    }
  })

  test('does nothing without an authenticated user', () => {
    savePendingExtraCreditsPurchase(0, 9, payload)
    expect(
      window.localStorage.getItem(EXTRA_CREDITS_PENDING_STORAGE_KEY)
    ).toBeNull()
  })
})
