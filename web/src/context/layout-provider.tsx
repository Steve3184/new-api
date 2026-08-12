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
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
} from 'react'

import { getCookie, removeCookie, setCookie } from '@/lib/cookies'
import { useSystemConfigStore } from '@/stores/system-config-store'

export type Collapsible = 'offcanvas' | 'icon' | 'none'
export type Variant = 'inset' | 'sidebar' | 'floating'

// Cookie constants following the pattern from sidebar.tsx
const LAYOUT_COLLAPSIBLE_COOKIE_NAME = 'layout_collapsible'
const LAYOUT_VARIANT_COOKIE_NAME = 'layout_variant'
const LAYOUT_COOKIE_MAX_AGE = 60 * 60 * 24 * 7 // 7 days

// Default values
const DEFAULT_VARIANT = 'inset'
const DEFAULT_COLLAPSIBLE = 'icon'

type LayoutContextType = {
  resetLayout: () => void

  defaultCollapsible: Collapsible
  collapsible: Collapsible
  setCollapsible: (collapsible: Collapsible) => void
  resetCollapsible: () => void

  defaultVariant: Variant
  variant: Variant
  setVariant: (variant: Variant) => void
  resetVariant: () => void
}

const LayoutContext = createContext<LayoutContextType | null>(null)

type LayoutProviderProps = {
  children: React.ReactNode
}

export function LayoutProvider({ children }: LayoutProviderProps) {
  const appearance = useSystemConfigStore((state) => state.config.appearance)
  const defaultVariant: Variant = ['inset', 'floating', 'sidebar'].includes(
    appearance?.defaultSidebarVariant
  )
    ? appearance.defaultSidebarVariant
    : DEFAULT_VARIANT
  const defaultCollapsible: Collapsible =
    appearance?.defaultSidebarLayout === 'offcanvas'
      ? 'offcanvas'
      : DEFAULT_COLLAPSIBLE
  const [collapsible, _setCollapsible] = useState<Collapsible>(() => {
    const saved = getCookie(LAYOUT_COLLAPSIBLE_COOKIE_NAME)
    return (saved as Collapsible) || defaultCollapsible
  })

  const [variant, _setVariant] = useState<Variant>(() => {
    const saved = getCookie(LAYOUT_VARIANT_COOKIE_NAME)
    return (saved as Variant) || defaultVariant
  })

  useEffect(() => {
    if (!getCookie(LAYOUT_COLLAPSIBLE_COOKIE_NAME)) {
      _setCollapsible(defaultCollapsible)
    }
    if (!getCookie(LAYOUT_VARIANT_COOKIE_NAME)) _setVariant(defaultVariant)
  }, [defaultCollapsible, defaultVariant])

  const setCollapsible = (newCollapsible: Collapsible) => {
    _setCollapsible(newCollapsible)
    setCookie(
      LAYOUT_COLLAPSIBLE_COOKIE_NAME,
      newCollapsible,
      LAYOUT_COOKIE_MAX_AGE
    )
  }

  const setVariant = (newVariant: Variant) => {
    _setVariant(newVariant)
    setCookie(LAYOUT_VARIANT_COOKIE_NAME, newVariant, LAYOUT_COOKIE_MAX_AGE)
  }

  const resetCollapsible = useCallback(() => {
    removeCookie(LAYOUT_COLLAPSIBLE_COOKIE_NAME)
    _setCollapsible(defaultCollapsible)
  }, [defaultCollapsible])

  const resetVariant = useCallback(() => {
    removeCookie(LAYOUT_VARIANT_COOKIE_NAME)
    _setVariant(defaultVariant)
  }, [defaultVariant])

  const resetLayout = useCallback(() => {
    resetCollapsible()
    resetVariant()
  }, [resetCollapsible, resetVariant])

  const contextValue = useMemo<LayoutContextType>(
    () => ({
      resetLayout,
      defaultCollapsible,
      collapsible,
      setCollapsible,
      resetCollapsible,
      defaultVariant,
      variant,
      setVariant,
      resetVariant,
    }),
    [
      resetLayout,
      defaultCollapsible,
      collapsible,
      resetCollapsible,
      defaultVariant,
      variant,
      resetVariant,
    ]
  )

  return <LayoutContext value={contextValue}>{children}</LayoutContext>
}

// Define the hook for the provider
// eslint-disable-next-line react-refresh/only-export-components
export function useLayout() {
  const context = useContext(LayoutContext)
  if (!context) {
    throw new Error('useLayout must be used within a LayoutProvider')
  }
  return context
}
