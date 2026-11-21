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

/** The part of `navigator.locks` that the purchase flow relies on. */
export type PurchaseLocks = {
  request<T>(
    name: string,
    options: { ifAvailable?: boolean },
    callback: (lock: object | null) => T
  ): Promise<Awaited<T>>
}

const heldLocks = new Map<string, Promise<void>>()

/**
 * Same-page stand-in for `navigator.locks`, which browsers expose only in
 * secure contexts. On plain-http deployments tabs cannot queue behind each
 * other, but a pending purchase replayed from two tabs is still settled once
 * because the server keys it by request id.
 */
export const pageLocks: PurchaseLocks = {
  async request<T>(
    name: string,
    options: { ifAvailable?: boolean },
    callback: (lock: object | null) => T
  ): Promise<Awaited<T>> {
    if (options.ifAvailable && heldLocks.has(name)) {
      return await callback(null)
    }
    while (heldLocks.has(name)) await heldLocks.get(name)
    let release!: () => void
    heldLocks.set(
      name,
      new Promise<void>((resolve) => {
        release = resolve
      })
    )
    try {
      return await callback({ name })
    } finally {
      heldLocks.delete(name)
      release()
    }
  },
}
