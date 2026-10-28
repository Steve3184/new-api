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
import type { ApiKeyTemplate } from '@/features/keys/types'

/** Token names are limited to 50 bytes by the server. */
const MAX_KEY_NAME_BYTES = 50

export type ChatPresetAutoKeySetting = ApiKeyTemplate & { enabled: boolean }

export type ChatPresetSetting = {
  standalone_menu?: boolean
  auto_key?: ChatPresetAutoKeySetting
}

/** Per-preset options keyed by the preset name used in Chats. */
export type ChatPresetSettingsMap = Record<string, ChatPresetSetting>

export function parseChatPresetSettings(value: string): ChatPresetSettingsMap {
  try {
    const parsed: unknown = JSON.parse(value || '{}')
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
      return parsed as ChatPresetSettingsMap
    }
  } catch {
    /* fall back to no settings */
  }
  return {}
}

/** The name the server uses when a preset key has no custom name. */
export function getDefaultChatPresetKeyName(presetName: string): string {
  const encoder = new TextEncoder()
  let name = ''
  for (const char of `app-${presetName.trim()}`) {
    if (encoder.encode(name + char).length > MAX_KEY_NAME_BYTES) break
    name += char
  }
  return name
}

/**
 * Keep the settings in step with the preset list: a renamed preset keeps its
 * settings, a removed one drops them.
 */
export function moveChatPresetSetting(
  settings: ChatPresetSettingsMap,
  from: string,
  to: string | null
): ChatPresetSettingsMap {
  if (!(from in settings) || from === to) return settings
  const next = { ...settings }
  const moved = next[from]
  delete next[from]
  if (to !== null) next[to] = moved
  return next
}
