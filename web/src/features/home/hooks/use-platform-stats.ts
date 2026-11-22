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
import { useQuery } from '@tanstack/react-query'

import { getPricing } from '@/features/pricing/api'
import { useStatus } from '@/hooks/use-status'
import { resolveModelProvider } from '@/lib/model-provider'
import { getModuleAccessFromStatus } from '@/lib/nav-modules'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import { getPlatformTraffic } from '../api'

export type PlatformStats = {
  /** Models the visitor can see on the pricing page, when there are any. */
  models?: number
  /**
   * Distinct vendors behind those models, when any can be told. A model
   * without a configured vendor is attributed by its name.
   */
  vendors?: number
  /** Requests and tokens served in the last minute, across all users. */
  traffic?: { rpm: number; tpm: number }
  /** False while a first answer is still on its way. */
  settled: boolean
}

const TRAFFIC_REFRESH_MS = 15 * 1000
// A homepage figure that fails to load is simply left out.
const QUIET = { retry: false, meta: { errorToast: false } } as const

/**
 * Live figures about this deployment for the homepage.
 *
 * The catalog counts come from the pricing page's data and the traffic rates
 * from the rankings module, so nothing is requested that the visitor could
 * not open themselves. A figure without data stays undefined.
 */
export function usePlatformStats(enabled: boolean): PlatformStats {
  const { status } = useStatus()
  const signedIn = useAuthStore((state) => Boolean(state.auth.user))
  const snapshot = status as Record<string, unknown> | null
  const pricingAccess = getModuleAccessFromStatus(snapshot, 'pricing')
  const rankingsAccess = getModuleAccessFromStatus(snapshot, 'rankings')

  const pricing = useQuery({
    queryKey: ['pricing'],
    queryFn: async () => requireServerSuccess(await getPricing()),
    staleTime: 5 * 60 * 1000,
    enabled:
      enabled &&
      pricingAccess.enabled &&
      (!pricingAccess.requireAuth || signedIn),
    ...QUIET,
  })
  const traffic = useQuery({
    queryKey: ['platform-traffic'],
    queryFn: async () => requireServerSuccess(await getPlatformTraffic()),
    refetchInterval: TRAFFIC_REFRESH_MS,
    enabled:
      enabled &&
      rankingsAccess.enabled &&
      (!rankingsAccess.requireAuth || signedIn),
    ...QUIET,
  })

  const models = pricing.data?.data ?? []
  const vendorNames = new Map(
    (pricing.data?.vendors ?? []).map((vendor) => [vendor.id, vendor.name])
  )
  const vendors = new Set<string>()
  for (const model of models) {
    const configured = model.vendor_id
      ? vendorNames.get(model.vendor_id)
      : undefined
    const vendor = configured ?? resolveModelProvider(model.model_name)?.name
    if (vendor) vendors.add(vendor.toLowerCase())
  }

  return {
    models: models.length || undefined,
    vendors: vendors.size || undefined,
    traffic: traffic.data?.data,
    settled: enabled && !pricing.isLoading && !traffic.isLoading,
  }
}
