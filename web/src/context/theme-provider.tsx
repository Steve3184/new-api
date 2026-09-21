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
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react'

import {
  readThemePreference,
  THEME_STORAGE_KEYS,
  writeThemePreference,
} from '@/lib/theme-storage'
import { useSystemConfigStore } from '@/stores/system-config-store'

type Theme = 'dark' | 'light' | 'system'
type ResolvedTheme = Exclude<Theme, 'system'>

const DEFAULT_THEME = 'system'
const THEMES = new Set<Theme>(['dark', 'light', 'system'])

type ThemeProviderProps = {
  children: React.ReactNode
  defaultTheme?: Theme
  storageKey?: string
}

type ThemeProviderState = {
  defaultTheme: Theme
  isForced: boolean
  resolvedTheme: ResolvedTheme
  theme: Theme
  setTheme: (theme: Theme) => void
  resetTheme: () => void
}

const initialState: ThemeProviderState = {
  defaultTheme: DEFAULT_THEME,
  isForced: false,
  resolvedTheme: 'light',
  theme: DEFAULT_THEME,
  setTheme: () => null,
  resetTheme: () => null,
}

const ThemeContext = createContext<ThemeProviderState>(initialState)

function getSystemTheme(): ResolvedTheme {
  if (typeof window === 'undefined') return 'light'
  return window.matchMedia('(prefers-color-scheme: dark)').matches
    ? 'dark'
    : 'light'
}

function resolveTheme(theme: Theme): ResolvedTheme {
  return theme === 'system' ? getSystemTheme() : theme
}

function getStoredTheme(storageKey: string, fallback: Theme): Theme {
  return readThemePreference(storageKey, THEMES, fallback)
}

export function ThemeProvider({
  children,
  defaultTheme = DEFAULT_THEME,
  storageKey = THEME_STORAGE_KEYS.mode,
  ...props
}: ThemeProviderProps) {
  const configuredDefaultTheme = useSystemConfigStore(
    (state) => state.config.appearance?.defaultTheme
  )
  const configuredThemeOverride = useSystemConfigStore(
    (state) => state.config.appearance?.defaultThemeOverride
  )
  const forcedTheme: ResolvedTheme | null =
    configuredThemeOverride === 'light' || configuredThemeOverride === 'dark'
      ? configuredThemeOverride
      : null
  const effectiveDefaultTheme =
    forcedTheme ??
    (THEMES.has(configuredDefaultTheme as Theme)
      ? (configuredDefaultTheme as Theme)
      : defaultTheme)
  const previousForcedTheme = useRef<ResolvedTheme | null>(forcedTheme)
  const [theme, _setTheme] = useState<Theme>(
    () => forcedTheme ?? getStoredTheme(storageKey, effectiveDefaultTheme)
  )
  const [resolvedTheme, setResolvedTheme] = useState<ResolvedTheme>(() =>
    resolveTheme(
      forcedTheme ?? getStoredTheme(storageKey, effectiveDefaultTheme)
    )
  )

  useEffect(() => {
    if (forcedTheme) {
      _setTheme(forcedTheme)
    } else if (previousForcedTheme.current) {
      _setTheme(getStoredTheme(storageKey, effectiveDefaultTheme))
    } else if (getStoredTheme(storageKey, effectiveDefaultTheme) === effectiveDefaultTheme) {
      _setTheme(effectiveDefaultTheme)
    }
    previousForcedTheme.current = forcedTheme
  }, [effectiveDefaultTheme, forcedTheme, storageKey])

  useLayoutEffect(() => {
    const root = window.document.documentElement
    const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')

    const applyTheme = () => {
      const nextResolvedTheme = theme === 'system' ? getSystemTheme() : theme
      root.classList.remove('light', 'dark')
      root.classList.add(nextResolvedTheme)
      root.style.colorScheme = nextResolvedTheme
      setResolvedTheme(nextResolvedTheme)
    }

    applyTheme()

    mediaQuery.addEventListener('change', applyTheme)

    return () => mediaQuery.removeEventListener('change', applyTheme)
  }, [theme])

  const setTheme = useCallback(
    (nextTheme: Theme) => {
      if (forcedTheme) return
      writeThemePreference(storageKey, nextTheme)
      _setTheme(nextTheme)
    },
    [forcedTheme, storageKey]
  )

  const resetTheme = useCallback(() => {
    if (forcedTheme) {
      _setTheme(forcedTheme)
      return
    }
    writeThemePreference(storageKey, null)
    _setTheme(effectiveDefaultTheme)
  }, [effectiveDefaultTheme, forcedTheme, storageKey])

  const contextValue = useMemo(
    () => ({
      defaultTheme: effectiveDefaultTheme,
      isForced: forcedTheme !== null,
      resolvedTheme,
      resetTheme,
      theme,
      setTheme,
    }),
    [
      effectiveDefaultTheme,
      forcedTheme,
      resolvedTheme,
      resetTheme,
      theme,
      setTheme,
    ]
  )

  return (
    <ThemeContext value={contextValue} {...props}>
      {children}
    </ThemeContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useTheme = () => {
  const context = useContext(ThemeContext)

  if (!context) throw new Error('useTheme must be used within a ThemeProvider')

  return context
}
