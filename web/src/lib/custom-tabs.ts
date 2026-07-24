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
import {
  Bell,
  BookOpen,
  Box,
  ChartNoAxesCombined,
  CircleHelp,
  CreditCard,
  Database,
  ExternalLink,
  FileText,
  Globe,
  Key,
  Link,
  MessageSquare,
  Settings,
  Shield,
  Terminal,
  User,
  Users,
  Wallet,
} from 'lucide-react'

export type CustomTabCategory = 'chat' | 'general' | 'personal' | 'admin'

export type CustomTab = {
  id: string
  label: string
  url: string
  icon: string
  category: CustomTabCategory
  external: boolean
}

export const CUSTOM_TAB_CATEGORIES: CustomTabCategory[] = [
  'chat',
  'general',
  'personal',
  'admin',
]

export const CUSTOM_TAB_ICONS = {
  Bell,
  BookOpen,
  Box,
  ChartNoAxesCombined,
  CircleHelp,
  CreditCard,
  Database,
  ExternalLink,
  FileText,
  Globe,
  Key,
  Link,
  MessageSquare,
  Settings,
  Shield,
  Terminal,
  User,
  Users,
  Wallet,
} as const

export function parseCustomTabs(raw: string | undefined): CustomTab[] {
  if (!raw) return []
  try {
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) return []
    return parsed.filter((value): value is CustomTab => {
      if (!value || typeof value !== 'object') return false
      const tab = value as Partial<CustomTab>
      return Boolean(
        typeof tab.id === 'string' &&
        typeof tab.label === 'string' &&
        typeof tab.url === 'string' &&
        typeof tab.icon === 'string' &&
        typeof tab.external === 'boolean' &&
        CUSTOM_TAB_CATEGORIES.includes(tab.category as CustomTabCategory)
      )
    })
  } catch {
    return []
  }
}

export function isValidCustomTabURL(value: string): boolean {
  const normalizedURL = value.trim()
  if (!normalizedURL) return false
  if (normalizedURL.startsWith('/')) return true

  try {
    const parsedURL = new URL(normalizedURL)
    return parsedURL.protocol === 'http:' || parsedURL.protocol === 'https:'
  } catch {
    return false
  }
}

export function getCustomTabIcon(name: string) {
  return CUSTOM_TAB_ICONS[name as keyof typeof CUSTOM_TAB_ICONS] || Globe
}
