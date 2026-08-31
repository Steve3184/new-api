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
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

import { DEFAULT_SYSTEM_NAME, DEFAULT_LOGO } from '@/lib/constants'

export type CurrencyDisplayType = 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM'

export interface CurrencyConfig {
  /** Whether to render quota values as currency instead of raw units */
  displayInCurrency: boolean
  /** Currency presentation strategy configured by the admin */
  quotaDisplayType: CurrencyDisplayType
  /** Number of quota units that equal one USD */
  quotaPerUnit: number
  /** Exchange rate from USD to the configured local currency */
  usdExchangeRate: number
  /** Custom currency symbol configured by the admin (used when type === CUSTOM) */
  customCurrencySymbol: string
  /** Exchange rate from USD to the custom currency (used when type === CUSTOM) */
  customCurrencyExchangeRate: number
}

export type SiteTheme = 'system' | 'light' | 'dark'
export type SiteThemePreset =
  | 'default'
  | 'anthropic'
  | 'simple-large'
  | 'underground'
  | 'rose-garden'
  | 'lake-view'
  | 'sunset-glow'
  | 'forest-whisper'
  | 'ocean-breeze'
  | 'lavender-dream'

export interface SiteAppearanceConfig {
  backgroundImage: string
  backgroundBlurOpacity: number
  defaultTheme: SiteTheme
  defaultThemePreset: SiteThemePreset
  defaultThemeFont: 'default' | 'sans' | 'serif'
  defaultThemeRadius: 'default' | 'none' | 'sm' | 'md' | 'lg' | 'xl'
  defaultThemeScale: 'default' | 'sm' | 'lg' | 'xl'
  defaultSidebarVariant: 'inset' | 'floating' | 'sidebar'
  defaultSidebarLayout: 'expanded' | 'icon' | 'offcanvas'
  defaultContentLayout: 'full' | 'centered'
  defaultDirection: 'ltr' | 'rtl'
  modelSquareDefaultView: 'card' | 'table'
  modelSquareCardPageSize: number
  modelSquareTablePageSize: number
}

export interface SPAMetaConfig {
  description: string
  ogType: string
  ogDescription: string
}

export type HomepageStyle = 'default' | 'custom' | 'preset-1'

export interface HomepageConfig {
  style: HomepageStyle
  presetTitleMode: 'i18n' | 'english'
  presetSlaEnabled: boolean
  presetSlaText: string
}

export interface SystemConfig {
  systemName: string
  logo: string
  footerHtml?: string
  demoSiteEnabled?: boolean
  displayTokenStatEnabled?: boolean
  currency: CurrencyConfig
  appearance: SiteAppearanceConfig
  spaMeta: SPAMetaConfig
  homepage: HomepageConfig
}

export const DEFAULT_CURRENCY_CONFIG: CurrencyConfig = {
  displayInCurrency: true,
  quotaDisplayType: 'USD',
  quotaPerUnit: 500000,
  usdExchangeRate: 1,
  customCurrencySymbol: '¤',
  customCurrencyExchangeRate: 1,
}

export const DEFAULT_SITE_APPEARANCE: SiteAppearanceConfig = {
  backgroundImage: '',
  backgroundBlurOpacity: 40,
  defaultTheme: 'system',
  defaultThemePreset: 'default',
  defaultThemeFont: 'default',
  defaultThemeRadius: 'default',
  defaultThemeScale: 'default',
  defaultSidebarVariant: 'inset',
  defaultSidebarLayout: 'expanded',
  defaultContentLayout: 'full',
  defaultDirection: 'ltr',
  modelSquareDefaultView: 'card',
  modelSquareCardPageSize: 18,
  modelSquareTablePageSize: 20,
}

export const DEFAULT_SPA_META: SPAMetaConfig = {
  description: 'Unified AI API gateway and admin dashboard.',
  ogType: 'website',
  ogDescription: 'Unified AI API gateway and admin dashboard.',
}

export const DEFAULT_HOMEPAGE_CONFIG: HomepageConfig = {
  style: 'default',
  presetTitleMode: 'i18n',
  presetSlaEnabled: true,
  presetSlaText: '99% SLA guarantee',
}

interface SystemConfigState {
  config: SystemConfig
  loading: boolean
  loadedLogoUrl: string
  setConfig: (config: Partial<SystemConfig>) => void
  setLoadedLogoUrl: (url: string) => void
  setLoading: (loading: boolean) => void
}

/**
 * System configuration store with automatic persistence
 * Manages system name, logo, footer HTML and loading states
 */
export const useSystemConfigStore = create<SystemConfigState>()(
  persist(
    (set) => ({
      config: {
        systemName: DEFAULT_SYSTEM_NAME,
        logo: DEFAULT_LOGO,
        currency: { ...DEFAULT_CURRENCY_CONFIG },
        appearance: { ...DEFAULT_SITE_APPEARANCE },
        spaMeta: { ...DEFAULT_SPA_META },
        homepage: { ...DEFAULT_HOMEPAGE_CONFIG },
      },
      loading: true,
      loadedLogoUrl: DEFAULT_LOGO,
      setConfig: (newConfig) =>
        set((state) => ({
          config: {
            ...state.config,
            ...newConfig,
            currency: {
              ...state.config.currency,
              ...newConfig.currency,
            },
            appearance: {
              ...(state.config.appearance ?? DEFAULT_SITE_APPEARANCE),
              ...newConfig.appearance,
            },
            spaMeta: {
              ...(state.config.spaMeta ?? DEFAULT_SPA_META),
              ...newConfig.spaMeta,
            },
            homepage: {
              ...(state.config.homepage ?? DEFAULT_HOMEPAGE_CONFIG),
              ...newConfig.homepage,
            },
          },
        })),
      setLoadedLogoUrl: (url) => set({ loadedLogoUrl: url }),
      setLoading: (loading) => set({ loading }),
    }),
    {
      name: 'system-config-storage',
      merge: (persisted, current) => {
        const saved = persisted as Partial<SystemConfigState>
        return {
          ...current,
          ...saved,
          config: {
            ...current.config,
            ...saved.config,
            currency: {
              ...DEFAULT_CURRENCY_CONFIG,
              ...saved.config?.currency,
            },
            appearance: {
              ...DEFAULT_SITE_APPEARANCE,
              ...saved.config?.appearance,
            },
            spaMeta: {
              ...DEFAULT_SPA_META,
              ...saved.config?.spaMeta,
            },
            homepage: {
              ...DEFAULT_HOMEPAGE_CONFIG,
              ...saved.config?.homepage,
            },
          },
        }
      },
      partialize: (state) => ({
        config: state.config,
        loadedLogoUrl: state.loadedLogoUrl,
      }),
    }
  )
)

// Selector helpers for convenience
export const getSystemName = () =>
  useSystemConfigStore.getState().config.systemName

export const getLogo = () => useSystemConfigStore.getState().config.logo

export const getFooterHtml = () =>
  useSystemConfigStore.getState().config.footerHtml
