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
import { useMemo } from 'react'

import type { SystemStatus } from '@/features/auth/types'
import { useStatus } from '@/hooks/use-status'

import {
  type ChatPreset,
  parseChatConfig,
  type RawChatConfig,
  type RawChatPresetSettings,
} from '../lib/chat-links'

function getStoredStatus(): Record<string, unknown> | undefined {
  if (typeof window === 'undefined') return undefined
  try {
    const raw = window.localStorage.getItem('status')
    if (!raw) return undefined
    return JSON.parse(raw) ?? undefined
  } catch {
    return undefined
  }
}

function extractServerAddress(status: SystemStatus | null) {
  const fromStatus =
    (status?.server_address as string | undefined) ??
    (status?.serverAddress as string | undefined) ??
    status?.data?.server_address ??
    (status?.data as Record<string, unknown> | undefined)?.serverAddress

  if (fromStatus && typeof fromStatus === 'string') {
    return fromStatus
  }

  if (typeof window !== 'undefined') {
    return window.location.origin
  }

  return ''
}

function extractChats(status: SystemStatus | null): RawChatConfig {
  const raw =
    status?.Chats ?? status?.chats ?? status?.data?.Chats ?? status?.data?.chats

  const stored = getStoredStatus()
  return ((raw ?? stored?.chats ?? stored?.Chats) as RawChatConfig)
}

function extractChatPresetSettings(
  status: SystemStatus | null
): RawChatPresetSettings {
  return (
    status?.chat_preset_settings ??
    status?.data?.chat_preset_settings ??
    getStoredStatus()?.chat_preset_settings
  )
}

export function useChatPresets(): {
  chatPresets: ChatPreset[]
  serverAddress: string
} {
  const { status } = useStatus()

  const serverAddress = useMemo(() => extractServerAddress(status), [status])

  const chatPresets = useMemo(() => {
    return parseChatConfig(
      extractChats(status),
      extractChatPresetSettings(status)
    )
  }, [status])

  return {
    chatPresets,
    serverAddress,
  }
}
