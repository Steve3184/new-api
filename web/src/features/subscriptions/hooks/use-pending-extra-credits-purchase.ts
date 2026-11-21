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
import { useEffect, useRef, useState } from 'react'

import {
  EXTRA_CREDITS_PENDING_CHANGED,
  EXTRA_CREDITS_PENDING_STORAGE_KEY,
  EXTRA_CREDITS_RESOLVED_STORAGE_KEY,
  readExtraCreditsPurchaseOutcomes,
  readExtraCreditsPurchaseState,
  sameBody,
  type ExtraCreditsPurchaseOutcome,
  type ExtraCreditsPurchasePayload,
} from '../lib/extra-credits-request'
import { pageLocks, type PurchaseLocks } from '../lib/page-locks'

/** Read the shared transaction under its purchase lock and follow tab updates. */
export function usePendingExtraCreditsPurchase(
  userId: number,
  subscriptionId: number,
  enabled = true,
  onResolved?: (outcomes: ExtraCreditsPurchaseOutcome[]) => void
) {
  const listener = useRef(onResolved)
  listener.current = onResolved
  const refresh = useRef<() => void>(undefined)
  // Web Locks exist only in secure contexts, and many self-hosted sites run on
  // plain http. There the lock is per page: two tabs may replay the same pending
  // body, which the server settles once by its request id.
  const [locks] = useState<PurchaseLocks>(() => {
    try {
      return navigator.locks ?? pageLocks
    } catch {
      return pageLocks
    }
  })
  const [unavailable, setUnavailable] = useState(false)
  const [state, setState] = useState<{
    userId: number
    subscriptionId: number
    pending: ExtraCreditsPurchasePayload | null
    purchases: { subscriptionId: number; body: ExtraCreditsPurchasePayload }[]
  } | null>(null)

  useEffect(() => {
    let active = true
    if (!enabled || !userId) return
    // Establish the notification baseline before queuing behind an in-flight
    // purchase. The pending body itself is only trusted under the lock.
    let seen: Record<string, ExtraCreditsPurchaseOutcome> = {}
    if (listener.current) {
      try {
        seen = readExtraCreditsPurchaseOutcomes(userId, subscriptionId)
      } catch {
        seen = readExtraCreditsPurchaseOutcomes(userId, subscriptionId, null)
      }
    }
    const read = async () => {
      try {
        const changed = await locks.request(
          'extra-credits-pending-store',
          {},
          async () => {
            if (!active) return
            let outcomes: Record<string, ExtraCreditsPurchaseOutcome>
            try {
              const snapshot = readExtraCreditsPurchaseState(
                userId,
                subscriptionId
              )
              setState({
                userId,
                subscriptionId,
                pending: snapshot.pending,
                purchases: snapshot.purchases,
              })
              setUnavailable(false)
              outcomes = snapshot.outcomes
            } catch {
              // Block submissions, but a matched receipt in this document
              // still needs to refresh the balances even if getItem failed.
              setUnavailable(true)
              outcomes = readExtraCreditsPurchaseOutcomes(
                userId,
                subscriptionId,
                null
              )
              setState((previous) => {
                if (
                  previous?.userId !== userId ||
                  previous.subscriptionId !== subscriptionId
                ) {
                  return previous
                }
                const purchases = previous.purchases.filter(
                  ({ subscriptionId: id, body }) => {
                    const outcome =
                      outcomes[`${userId}:${id}:${body.request_id}`]
                    return !outcome || !sameBody(outcome, body)
                  }
                )
                if (purchases.length === previous.purchases.length) {
                  return previous
                }
                return {
                  ...previous,
                  purchases,
                  pending:
                    purchases.find((p) => p.subscriptionId === subscriptionId)
                      ?.body ?? null,
                }
              })
            }
            if (!listener.current) return
            const results: ExtraCreditsPurchaseOutcome[] = []
            for (const [key, outcome] of Object.entries(outcomes)) {
              const prior = seen[key]
              if (
                prior?.status !== outcome.status ||
                !sameBody(prior, outcome)
              ) {
                results.push(outcome)
              }
              seen[key] = outcome
            }
            return results
          }
        )
        if (active && changed?.length) listener.current?.(changed)
      } catch {
        if (active) setUnavailable(true)
      }
    }
    const onStorage = (event: StorageEvent) => {
      if (
        event.key === null ||
        event.key === EXTRA_CREDITS_PENDING_STORAGE_KEY ||
        event.key === EXTRA_CREDITS_RESOLVED_STORAGE_KEY
      ) {
        void read()
      }
    }
    const onLocalChange = () => void read()
    refresh.current = onLocalChange
    void read()
    window.addEventListener('storage', onStorage)
    window.addEventListener(EXTRA_CREDITS_PENDING_CHANGED, onLocalChange)
    return () => {
      active = false
      refresh.current = undefined
      window.removeEventListener('storage', onStorage)
      window.removeEventListener(EXTRA_CREDITS_PENDING_CHANGED, onLocalChange)
    }
  }, [enabled, userId, subscriptionId, locks])

  const current =
    enabled &&
    state?.userId === userId &&
    state.subscriptionId === subscriptionId
      ? state
      : null
  return {
    pending: current?.pending ?? null,
    purchases: current?.purchases ?? [],
    ready: !!current && !unavailable,
    locks: unavailable ? null : locks,

    markUnavailable: () => setUnavailable(true),
    refresh: () => refresh.current?.(),
  }
}
