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
import { useEffect, useCallback } from 'react'

import { DEFAULT_SYSTEM_NAME, DEFAULT_LOGO } from '@/lib/constants'
import { applyFaviconToDom, applySPAMetaToDom } from '@/lib/dom-utils'
import {
  useSystemConfigStore,
  type CurrencyConfig,
  type CurrencyDisplayType,
  type SystemConfig,
  DEFAULT_CURRENCY_CONFIG,
  DEFAULT_SITE_APPEARANCE,
  DEFAULT_SPA_META,
  type SiteAppearanceConfig,
} from '@/stores/system-config-store'

interface UseSystemConfigOptions {
  /** Automatically fetch config from backend (use only in root component) */
  autoLoad?: boolean
}

interface StatusApiResponse {
  success: boolean
  data: {
    system_name?: string
    logo?: string
    footer_html?: string
    demo_site_enabled?: boolean
    display_token_stat_enabled?: boolean
    display_in_currency?: boolean
    quota_display_type?: CurrencyDisplayType
    quota_per_unit?: number
    usd_exchange_rate?: number
    custom_currency_symbol?: string
    custom_currency_exchange_rate?: number
    site_appearance?: {
      background_image?: string
      background_blur_opacity?: number
      default_theme?: string
      default_theme_preset?: string
      default_theme_font?: string
      default_theme_radius?: string
      default_theme_scale?: string
      default_sidebar_variant?: string
      default_sidebar_layout?: string
      default_content_layout?: string
      default_direction?: string
      model_square_default_view?: string
      model_square_card_page_size?: number
      model_square_table_page_size?: number
    }
    spa_meta?: {
      description?: string
      og_type?: string
      og_description?: string
    }
  }
}

function enumValue<T extends string>(
  value: unknown,
  allowed: readonly T[],
  fallback: T
): T {
  return typeof value === 'string' && allowed.includes(value as T)
    ? (value as T)
    : fallback
}

function normalizeAppearance(
  value: StatusApiResponse['data']['site_appearance']
): SiteAppearanceConfig {
  const defaults = DEFAULT_SITE_APPEARANCE
  return {
    backgroundImage: value?.background_image?.trim() || '',
    backgroundBlurOpacity: Math.min(
      100,
      Math.max(0, toNumber(value?.background_blur_opacity, 40))
    ),
    defaultTheme: enumValue(
      value?.default_theme,
      ['system', 'light', 'dark'],
      defaults.defaultTheme
    ),
    defaultThemePreset: enumValue(
      value?.default_theme_preset,
      [
        'default',
        'anthropic',
        'simple-large',
        'underground',
        'rose-garden',
        'lake-view',
        'sunset-glow',
        'forest-whisper',
        'ocean-breeze',
        'lavender-dream',
      ],
      defaults.defaultThemePreset
    ),
    defaultThemeFont: enumValue(
      value?.default_theme_font,
      ['default', 'sans', 'serif'],
      defaults.defaultThemeFont
    ),
    defaultThemeRadius: enumValue(
      value?.default_theme_radius,
      ['default', 'none', 'sm', 'md', 'lg', 'xl'],
      defaults.defaultThemeRadius
    ),
    defaultThemeScale: enumValue(
      value?.default_theme_scale,
      ['default', 'sm', 'lg', 'xl'],
      defaults.defaultThemeScale
    ),
    defaultSidebarVariant: enumValue(
      value?.default_sidebar_variant,
      ['inset', 'floating', 'sidebar'],
      defaults.defaultSidebarVariant
    ),
    defaultSidebarLayout: enumValue(
      value?.default_sidebar_layout,
      ['expanded', 'icon', 'offcanvas'],
      defaults.defaultSidebarLayout
    ),
    defaultContentLayout: enumValue(
      value?.default_content_layout,
      ['full', 'centered'],
      defaults.defaultContentLayout
    ),
    defaultDirection: enumValue(
      value?.default_direction,
      ['ltr', 'rtl'],
      defaults.defaultDirection
    ),
    modelSquareDefaultView: enumValue(
      value?.model_square_default_view,
      ['card', 'table'],
      defaults.modelSquareDefaultView
    ),
    modelSquareCardPageSize: toNumber(
      value?.model_square_card_page_size,
      defaults.modelSquareCardPageSize
    ),
    modelSquareTablePageSize: toNumber(
      value?.model_square_table_page_size,
      defaults.modelSquareTablePageSize
    ),
  }
}

function toNumber(value: unknown, fallback: number): number {
  if (typeof value === 'number' && !Number.isNaN(value)) return value
  if (typeof value === 'string') {
    const parsed = Number(value)
    if (!Number.isNaN(parsed)) return parsed
  }
  return fallback
}

/**
 * Map `/api/status` response data to our persisted system config structure
 */
export function mapStatusDataToConfig(
  data: StatusApiResponse['data'] | undefined
): Partial<SystemConfig> {
  if (!data) return {}

  const quotaDisplayType =
    (data.quota_display_type as CurrencyDisplayType | undefined) ??
    DEFAULT_CURRENCY_CONFIG.quotaDisplayType

  const currency: CurrencyConfig = {
    displayInCurrency:
      data.display_in_currency ?? DEFAULT_CURRENCY_CONFIG.displayInCurrency,
    quotaDisplayType,
    quotaPerUnit: toNumber(
      data.quota_per_unit,
      DEFAULT_CURRENCY_CONFIG.quotaPerUnit
    ),
    usdExchangeRate: toNumber(
      data.usd_exchange_rate,
      DEFAULT_CURRENCY_CONFIG.usdExchangeRate
    ),
    customCurrencySymbol:
      data.custom_currency_symbol?.trim() ||
      DEFAULT_CURRENCY_CONFIG.customCurrencySymbol,
    customCurrencyExchangeRate: toNumber(
      data.custom_currency_exchange_rate,
      DEFAULT_CURRENCY_CONFIG.customCurrencyExchangeRate
    ),
  }

  return {
    systemName: data.system_name || DEFAULT_SYSTEM_NAME,
    logo: data.logo || DEFAULT_LOGO,
    footerHtml: data.footer_html,
    demoSiteEnabled: data.demo_site_enabled,
    displayTokenStatEnabled: data.display_token_stat_enabled,
    currency,
    appearance: normalizeAppearance(data.site_appearance),
    spaMeta: {
      description: data.spa_meta?.description ?? DEFAULT_SPA_META.description,
      ogType: data.spa_meta?.og_type || DEFAULT_SPA_META.ogType,
      ogDescription:
        data.spa_meta?.og_description ?? DEFAULT_SPA_META.ogDescription,
    },
  }
}

// Fetch system config from API
async function fetchSystemConfig(): Promise<Partial<SystemConfig>> {
  const response = await fetch('/api/status')
  if (!response.ok) throw new Error('Failed to fetch status')

  const data: StatusApiResponse = await response.json()
  if (!data.success) throw new Error('API returned error')

  return mapStatusDataToConfig(data.data)
}

// Preload image and return cleanup function
function preloadImage(
  src: string,
  onLoad: () => void,
  onError: () => void
): () => void {
  const img = new Image()
  img.onload = onLoad
  img.onerror = onError
  img.src = src

  return () => {
    img.onload = null
    img.onerror = null
  }
}

/**
 * System configuration hook with auto-loading and logo preloading
 *
 * @example
 * // Root component - auto-load from backend
 * useSystemConfig({ autoLoad: true })
 *
 * @example
 * // Other components - use cached config
 * const { systemName, logo, loading } = useSystemConfig()
 */
export function useSystemConfig(options: UseSystemConfigOptions = {}) {
  const { autoLoad = false } = options
  const {
    config,
    loading,
    loadedLogoUrl,
    setConfig,
    setLoadedLogoUrl,
    setLoading,
  } = useSystemConfigStore()

  // Load config from backend
  const loadConfig = useCallback(async () => {
    try {
      setLoading(true)
      const newConfig = await fetchSystemConfig()
      setConfig(newConfig)
    } catch (error) {
      // eslint-disable-next-line no-console
      console.error('Failed to load system config:', error)
    } finally {
      setLoading(false)
    }
  }, [setConfig, setLoading])

  useEffect(() => {
    if (autoLoad) loadConfig()
  }, [autoLoad, loadConfig])

  // Preload logo image when URL changes
  useEffect(() => {
    const { logo } = config

    // Skip if logo is already loaded
    if (!logo || logo === loadedLogoUrl) return

    // Preload new logo
    return preloadImage(
      logo,
      () => {
        setLoadedLogoUrl(logo)
        applyFaviconToDom(logo)
      },
      () => {
        if (logo !== DEFAULT_LOGO) {
          // eslint-disable-next-line no-console
          console.error('Failed to load logo:', logo)
        }
        // Mark as loaded even on error to prevent infinite retry
        setLoadedLogoUrl(logo)
      }
    )
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [config.logo, loadedLogoUrl, setLoadedLogoUrl])

  useEffect(() => {
    applySPAMetaToDom(config.spaMeta)
  }, [config.spaMeta])

  return {
    ...config,
    loading,
    logoLoaded: config.logo === loadedLogoUrl && !!loadedLogoUrl,
  }
}
