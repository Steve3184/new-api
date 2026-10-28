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
import { t } from 'i18next'

import {
  fetchChatPresetKey,
  fetchTokenKey,
  getApiKeys,
} from '@/features/keys/api'
import { API_KEY_STATUS } from '@/features/keys/constants'
import {
  requireServerSuccess,
  createServerError,
} from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import type { ChatPreset } from '../lib/chat-links'

export async function fetchActiveChatKey() {
  const result = await getApiKeys({ p: 1, size: 50 })
  if (!result.success) {
    throw createServerError(result, t('Failed to load API keys'))
  }

  const items = result.data?.items ?? []
  const active = items.find((item) => item.status === API_KEY_STATUS.ENABLED)
  if (!active) {
    throw new Error('No enabled API keys found. Create or enable one first.')
  }

  const keyResult = await fetchTokenKey(active.id)
  if (!keyResult.success || !keyResult.data?.key) {
    throw createServerError(keyResult, t('Failed to load API keys'))
  }

  return `sk-${keyResult.data.key}`
}

/**
 * Resolve the API key a chat preset opens with: its own server-created key
 * when the preset has one, otherwise the user's first enabled key.
 */
export async function fetchKeyForChatPreset(
  preset: Pick<ChatPreset, 'name' | 'autoKey'>
) {
  if (!preset.autoKey) return fetchActiveChatKey()

  const result = await fetchChatPresetKey(preset.name)
  if (!result.success || !result.data?.key) {
    throw createServerError(
      result,
      t('Failed to prepare the API key for this chat preset')
    )
  }
  return `sk-${result.data.key}`
}

/**
 * Get the API key used to open a chat preset link
 */
export function useChatPresetKey(
  preset: Pick<ChatPreset, 'name' | 'autoKey'> | undefined,
  enabled: boolean
) {
  const userId = useAuthStore((state) => state.auth.user?.id)
  const presetKey = preset?.autoKey ? preset.name : null

  return useQuery({
    queryKey: ['chat-active-key', userId, presetKey],
    queryFn: async () =>
      requireServerSuccess(
        await fetchKeyForChatPreset({
          name: presetKey ?? '',
          autoKey: presetKey !== null,
        })
      ),
    enabled: enabled && Boolean(userId) && Boolean(preset),
    staleTime: 5 * 60 * 1000,
    gcTime: 10 * 60 * 1000,
  })
}
