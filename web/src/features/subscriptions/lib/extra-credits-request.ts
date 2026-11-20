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
import { extraCreditsFromWalletQuota } from './extra-credits'

/**
 * Persistent purchases whose outcome is uncertain, scoped by authenticated
 * user and subscription. Reopening a tab must recover the same transaction.
 */
export const EXTRA_CREDITS_PENDING_CHANGED =
  'new-api:extra-credits-pending-changed'

export const EXTRA_CREDITS_PENDING_STORAGE_KEY =
  'new-api:subscription-extra-credits-pending'

export const MIN_REQUEST_ID_LENGTH = 16
export const MAX_REQUEST_ID_LENGTH = 64

/** The exact request body to replay until the server settles the purchase. */
export type ExtraCreditsPurchasePayload = {
  request_id: string
  wallet_quota: number
  expected_rate: number
}

type PendingMap = Record<string, ExtraCreditsPurchasePayload>
export const EXTRA_CREDITS_RESOLVED_STORAGE_KEY = `${EXTRA_CREDITS_PENDING_STORAGE_KEY}:resolved`
export type ExtraCreditsPurchaseOutcome = ExtraCreditsPurchasePayload & {
  status: 'purchased' | 'rejected'
  resolved_at: number
}
type OutcomeMap = Record<string, ExtraCreditsPurchaseOutcome>
const mirrorKey = `${EXTRA_CREDITS_PENDING_STORAGE_KEY}:tab`

// A storage clear is not a receipt. Keep the exact known bodies and terminal
// outcomes for this document's lifetime, independently of mounted components.
let pagePending: PendingMap = {}
let pageOutcomes: OutcomeMap = {}

export function resetExtraCreditsPageForTests() {
  pagePending = {}
  pageOutcomes = {}
}

export function sameBody(
  a: ExtraCreditsPurchasePayload,
  b: ExtraCreditsPurchasePayload
) {
  return (
    a.request_id === b.request_id &&
    a.wallet_quota === b.wallet_quota &&
    a.expected_rate === b.expected_rate
  )
}

function readPurchaseMap<T>(storage: Storage, key: string): Record<string, T> {
  const raw = storage.getItem(key)
  if (!raw) return {}
  const map: unknown = JSON.parse(raw)
  if (!map || typeof map !== 'object' || Array.isArray(map)) {
    throw new Error('Invalid purchase storage')
  }
  return map as Record<string, T>
}

export function readExtraCreditsPurchaseOutcomes(
  userId: number,
  subscriptionId: number,
  shared: OutcomeMap | null = readPurchaseMap(
    window.localStorage,
    EXTRA_CREDITS_RESOLVED_STORAGE_KEY
  )
): OutcomeMap {
  const prefix = subscriptionId ? `${userId}:${subscriptionId}:` : `${userId}:`
  const outcomes: OutcomeMap = {}
  const known =
    shared === null
      ? pageOutcomes
      : {
          ...readPurchaseMap<ExtraCreditsPurchaseOutcome>(
            window.sessionStorage,
            EXTRA_CREDITS_RESOLVED_STORAGE_KEY
          ),
          ...shared,
          ...pageOutcomes,
        }
  for (const [key, outcome] of Object.entries(known)) {
    if (!key.startsWith(prefix)) continue
    const scope = `${userId}:${Number(key.split(':')[1])}`
    if (
      !isValidPayload(outcome) ||
      key !== `${scope}:${outcome.request_id}` ||
      !['purchased', 'rejected'].includes(outcome.status)
    ) {
      if (!subscriptionId) continue
      throw new Error('Invalid purchase result')
    }
    outcomes[key] = outcome
  }
  return outcomes
}

/** Prefer the oldest unresolved body known to this tab. Retain other IDs in
 * the tab mirror so resolving the first reveals the next, even after reload. */
export function readExtraCreditsPurchaseState(
  userId: number,
  subscriptionId: number
) {
  const sharedOutcomes = readPurchaseMap<ExtraCreditsPurchaseOutcome>(
    window.localStorage,
    EXTRA_CREDITS_RESOLVED_STORAGE_KEY
  )
  const outcomes = readExtraCreditsPurchaseOutcomes(
    userId,
    subscriptionId,
    sharedOutcomes
  )
  const shared = readPurchaseMap<ExtraCreditsPurchasePayload>(
    window.localStorage,
    EXTRA_CREDITS_PENDING_STORAGE_KEY
  )
  const mirror = readPurchaseMap<ExtraCreditsPurchasePayload>(
    window.sessionStorage,
    mirrorKey
  )
  const purchases = new Map<number, ExtraCreditsPurchasePayload>()
  let mirrorChanged = false
  let sharedChanged = false
  let invalidPending = false
  for (const source of [pagePending, mirror, shared]) {
    for (const [key, body] of Object.entries(source)) {
      if (!key.startsWith(`${userId}:`)) continue
      const id = Number(key.split(':')[1])
      if (
        !isFinitePositiveInteger(id) ||
        (subscriptionId && id !== subscriptionId)
      ) {
        continue
      }
      const scope = extraCreditsPendingKey(userId, id)
      if (
        !isValidPayload(body) ||
        (key !== scope && key !== `${scope}:${body.request_id}`)
      ) {
        invalidPending = true
        continue
      }
      const requestKey = `${scope}:${body.request_id}`
      const outcome = outcomes[requestKey]
      if (outcome && sameBody(body, outcome)) {
        delete pagePending[requestKey]
        if (source === mirror) {
          delete mirror[key]
          mirrorChanged = true
        }
        // If only this tab knows the receipt, don't leave an independent tab
        // a replayable shared body even if copying the receipt fails again.
        const saved = sharedOutcomes[requestKey]
        if (
          source === shared &&
          (saved?.status !== outcome.status || !sameBody(saved, outcome))
        ) {
          delete shared[key]
          sharedChanged = true
        }
        continue
      }
      if (outcome) delete outcomes[requestKey]
      const known = pagePending[requestKey]
      if (known && !sameBody(known, body)) {
        if (subscriptionId) throw new Error('Conflicting pending purchase')
        continue
      }
      pagePending[requestKey] = body
      if (!purchases.has(id)) purchases.set(id, body)
      if (
        !shared[requestKey] &&
        shared[scope]?.request_id !== body.request_id
      ) {
        shared[shared[scope] ? requestKey : scope] = body
        sharedChanged = true
      }
      if (
        !mirror[requestKey] &&
        mirror[scope]?.request_id !== body.request_id
      ) {
        mirror[requestKey] = body
        mirrorChanged = true
      }
    }
  }
  // An invalid sibling must not hide a valid retry, but don't start a new
  // order if the only record for this subscription is unreadable.
  if (subscriptionId && invalidPending && !purchases.has(subscriptionId)) {
    throw new Error('Invalid pending purchase')
  }
  Object.assign(pageOutcomes, outcomes)
  let repairOutcomes = false
  for (const [key, outcome] of Object.entries(outcomes)) {
    const saved = sharedOutcomes[key]
    if (saved?.status === outcome.status && sameBody(saved, outcome)) continue
    sharedOutcomes[key] = outcome
    repairOutcomes = true
  }
  if (repairOutcomes) {
    try {
      window.localStorage.setItem(
        EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
        JSON.stringify(sharedOutcomes)
      )
    } catch {
      // A full store may still allow removal of the matching pending body.
    }
  }
  if (sharedChanged && !writeMap(window.localStorage, shared)) {
    throw new Error('Cannot restore shared purchase')
  }
  if (mirrorChanged) {
    try {
      window.sessionStorage.setItem(mirrorKey, JSON.stringify(mirror))
    } catch {
      // Storage may be full. The page still knows these outcomes, and every
      // POST must independently persist and verify its exact body first.
    }
  }
  return {
    pending: purchases.get(subscriptionId) ?? null,
    purchases: Array.from(purchases, ([subscriptionId, body]) => ({
      subscriptionId,
      body,
    })),
    outcomes,
  }
}

/** Call only after a matched receipt or a definitive handler refusal. Results
 * are keyed by request, so a later order never resurrects an earlier refusal. */
export function resolvePendingExtraCreditsPurchase(
  userId: number,
  subscriptionId: number,
  payload: ExtraCreditsPurchasePayload,
  status: 'purchased' | 'rejected' = 'purchased'
): boolean {
  if (!userId || !subscriptionId || !isValidPayload(payload)) return false
  const key = `${userId}:${subscriptionId}:${payload.request_id}`
  const outcome = (pageOutcomes[key] = {
    ...payload,
    status,
    resolved_at: Date.now(),
  })
  delete pagePending[key]
  let persisted = true
  for (const name of ['localStorage', 'sessionStorage'] as const) {
    try {
      const storage = window[name]
      const map = readPurchaseMap<ExtraCreditsPurchaseOutcome>(
        storage,
        EXTRA_CREDITS_RESOLVED_STORAGE_KEY
      )
      map[key] = outcome
      storage.setItem(EXTRA_CREDITS_RESOLVED_STORAGE_KEY, JSON.stringify(map))
      const saved = readPurchaseMap<ExtraCreditsPurchaseOutcome>(
        storage,
        EXTRA_CREDITS_RESOLVED_STORAGE_KEY
      )[key]
      if (saved?.status !== status || !sameBody(saved, payload)) {
        persisted = false
      }
    } catch {
      persisted = false
    }
  }
  // Removing the shared pending body may still work when saving the result
  // did not. A session-only refusal cannot protect an independent tab.
  const cleared = clearPendingExtraCreditsPurchase(
    userId,
    subscriptionId,
    payload
  )
  return persisted && cleared
}

function defaultStorage(): Storage | null {
  try {
    return typeof window === 'undefined' ? null : window.localStorage
  } catch {
    // Storage can be blocked by browser privacy settings.
    return null
  }
}

function readMap(storage: Storage | null): PendingMap {
  if (!storage) return {}
  try {
    const raw = storage.getItem(EXTRA_CREDITS_PENDING_STORAGE_KEY)
    if (!raw) return {}
    const parsed: unknown = JSON.parse(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return {}
    }
    return parsed as PendingMap
  } catch {
    return {}
  }
}

function writeMap(storage: Storage | null, map: PendingMap): boolean {
  if (!storage) return false
  try {
    const keys = Object.keys(map)
    if (keys.length === 0) {
      storage.removeItem(EXTRA_CREDITS_PENDING_STORAGE_KEY)
      return true
    }
    storage.setItem(EXTRA_CREDITS_PENDING_STORAGE_KEY, JSON.stringify(map))
    return true
  } catch {
    // Full or blocked storage: the caller must not send an unsaved purchase.
    return false
  }
}

/**
 * One pending purchase per authenticated user *and* subscription, so a
 * different account signing in on the same tab can never replay, or read, a
 * purchase id that belongs to someone else.
 */
export function extraCreditsPendingKey(
  userId: number,
  subscriptionId: number
): string {
  return `${userId}:${subscriptionId}`
}

function isFinitePositiveInteger(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value > 0
}

function isValidPayload(value: unknown): value is ExtraCreditsPurchasePayload {
  if (!value || typeof value !== 'object') return false
  const payload = value as Record<string, unknown>
  return (
    typeof payload.request_id === 'string' &&
    payload.request_id.length >= MIN_REQUEST_ID_LENGTH &&
    payload.request_id.length <= MAX_REQUEST_ID_LENGTH &&
    typeof payload.wallet_quota === 'number' &&
    typeof payload.expected_rate === 'number' &&
    extraCreditsFromWalletQuota(payload.wallet_quota, payload.expected_rate) !==
      null
  )
}

/**
 * The unsettled purchase for this user and subscription, if any. A successful
 * purchase clears its entry, so finding one means an earlier attempt may or
 * may not have reached the server: replay it verbatim instead of charging
 * with a new id.
 */
export function readPendingExtraCreditsPurchase(
  userId: number,
  subscriptionId: number,
  storage: Storage | null = defaultStorage()
): ExtraCreditsPurchasePayload | null {
  if (!userId || !subscriptionId) return null
  const payload =
    readMap(storage)[extraCreditsPendingKey(userId, subscriptionId)]
  return isValidPayload(payload) ? payload : null
}

/**
 * Persists the body and reads it back. Returns false when storage is missing,
 * blocked or full; the purchase must then not be sent, because a lost response
 * could not be retried with the same request id after a reload.
 */
export function savePendingExtraCreditsPurchase(
  userId: number,
  subscriptionId: number,
  payload: ExtraCreditsPurchasePayload,
  storage: Storage | null = defaultStorage()
): boolean {
  if (!userId || !subscriptionId || !storage || !isValidPayload(payload)) {
    return false
  }
  const scope = extraCreditsPendingKey(userId, subscriptionId)
  const key = `${scope}:${payload.request_id}`
  let undoNewBody = false
  try {
    const state =
      storage === defaultStorage()
        ? readExtraCreditsPurchaseState(userId, subscriptionId)
        : null
    const map = readPurchaseMap<ExtraCreditsPurchasePayload>(
      storage,
      EXTRA_CREDITS_PENDING_STORAGE_KEY
    )
    let sharedKey = scope
    if (state) {
      if (
        state.outcomes[key] ||
        (state.pending && !sameBody(state.pending, payload))
      ) {
        return false
      }
      // Keep a different tab's unresolved legacy slot intact during replay.
      if (
        map[scope] &&
        !sameBody(map[scope], payload) &&
        !state.outcomes[`${scope}:${map[scope].request_id}`]
      ) {
        sharedKey = key
      }
    } else if (map[scope] && !sameBody(map[scope], payload)) {
      return false
    }
    map[sharedKey] = payload
    if (!writeMap(storage, map)) return false
    undoNewBody = !!state && !state.pending
    const saved = readPurchaseMap<ExtraCreditsPurchasePayload>(
      storage,
      EXTRA_CREDITS_PENDING_STORAGE_KEY
    )[sharedKey]
    if (!saved || !sameBody(saved, payload)) {
      throw new Error('Shared purchase not saved')
    }
    if (storage === defaultStorage()) {
      const mirror = readPurchaseMap<ExtraCreditsPurchasePayload>(
        window.sessionStorage,
        mirrorKey
      )
      mirror[key] = payload
      window.sessionStorage.setItem(mirrorKey, JSON.stringify(mirror))
      const savedMirror = readPurchaseMap<ExtraCreditsPurchasePayload>(
        window.sessionStorage,
        mirrorKey
      )[key]
      if (!savedMirror || !sameBody(savedMirror, payload)) {
        throw new Error('Purchase mirror not saved')
      }
      pagePending[key] = { ...payload }
    }
    window.dispatchEvent(new Event(EXTRA_CREDITS_PENDING_CHANGED))
    return true
  } catch {
    if (undoNewBody) {
      clearPendingExtraCreditsPurchase(userId, subscriptionId, payload, storage)
    }
    return false
  }
}

/**
 * Drops the entry once the server definitively settled or refused it. With
 * `requestId`, only that exact purchase is removed, so a late response for an
 * older purchase never discards a newer unsettled one.
 */
export function clearPendingExtraCreditsPurchase(
  userId: number,
  subscriptionId: number,
  request?: string | ExtraCreditsPurchasePayload,
  storage: Storage | null = defaultStorage()
): boolean {
  if (!userId || !subscriptionId || !storage) return false
  const scope = extraCreditsPendingKey(userId, subscriptionId)
  const remove = (map: PendingMap) => {
    for (const [key, body] of Object.entries(map)) {
      if (
        (key === scope || key.startsWith(`${scope}:`)) &&
        (!request ||
          (typeof request === 'string'
            ? body?.request_id === request
            : body && sameBody(body, request)))
      ) {
        delete map[key]
      }
    }
  }
  try {
    const map = readPurchaseMap<ExtraCreditsPurchasePayload>(
      storage,
      EXTRA_CREDITS_PENDING_STORAGE_KEY
    )
    remove(map)
    const cleared = writeMap(storage, map)
    if (storage === defaultStorage()) {
      remove(pagePending)
      const mirror = readPurchaseMap<ExtraCreditsPurchasePayload>(
        window.sessionStorage,
        mirrorKey
      )
      remove(mirror)
      window.sessionStorage.setItem(mirrorKey, JSON.stringify(mirror))
    }
    return cleared
  } catch {
    return false
  } finally {
    window.dispatchEvent(new Event(EXTRA_CREDITS_PENDING_CHANGED))
  }
}
