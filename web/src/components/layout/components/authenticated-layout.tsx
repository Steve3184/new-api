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
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { LayoutProvider } from '@/context/layout-provider'
import { SearchProvider } from '@/context/search-provider'
import { useSystemConfig } from '@/hooks/use-system-config'
import { getCookie } from '@/lib/cookies'
import { cn } from '@/lib/utils'
import { Link, useRouterState } from '@tanstack/react-router'
import { Store, Trophy } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const { t } = useTranslation()
  const pathname = useRouterState({ select: (state) => state.location.pathname })
  const { appearance } = useSystemConfig()
  const savedSidebarState = getCookie('sidebar_state')
  const defaultOpen = savedSidebarState
    ? savedSidebarState !== 'false'
    : appearance.defaultSidebarLayout === 'expanded'
  const backgroundImage = appearance.backgroundImage
  const blurOpacity = Math.min(
    100,
    Math.max(0, appearance.backgroundBlurOpacity ?? 40)
  )
  const surfaceStyles = backgroundImage
    ? ({
        '--console-surface-opacity': `${blurOpacity}%`,
        '--console-card-opacity': `${Math.min(85, blurOpacity + 15)}%`,
        '--console-sidebar-opacity': `${Math.min(80, blurOpacity + 10)}%`,
        '--console-list-opacity': `${Math.min(75, blurOpacity + 20)}%`,
      } as React.CSSProperties)
    : undefined

  return (
    <div
      className='relative isolate min-h-svh'
      data-console-background={backgroundImage ? 'image' : undefined}
      style={surfaceStyles}
    >
      {backgroundImage && (
        <div
          aria-hidden='true'
          className='pointer-events-none fixed inset-0 -z-10 bg-cover bg-center bg-no-repeat'
          style={{ backgroundImage: `url(${JSON.stringify(backgroundImage)})` }}
        />
      )}
      <LayoutProvider>
        <SearchProvider>
          <SidebarProvider
            defaultOpen={defaultOpen}
            className='flex-col has-data-[variant=inset]:bg-transparent'
          >
            <SkipToMain />
            <AppHeader
              className={cn(backgroundImage && 'backdrop-blur-sm')}
              style={
                backgroundImage
                  ? {
                      backgroundColor:
                        'color-mix(in oklch, var(--background) var(--console-surface-opacity), transparent)',
                    }
                  : undefined
              }
            />
            <div className='flex min-h-0 w-full flex-1'>
              <AppSidebar />
              <SidebarInset
                className={cn(
                  '@container/content',
                  'h-[calc(100svh-var(--app-header-height,0px))]',
                  'min-h-0 overflow-hidden',
                  'pb-14 md:pb-0',
                  'peer-data-[variant=inset]:h-[calc(100svh-var(--app-header-height,0px)-(var(--spacing)*4))]',
                  backgroundImage && 'backdrop-blur-sm'
                )}
                style={
                  backgroundImage
                    ? {
                        backgroundColor:
                          'color-mix(in oklch, var(--background) var(--console-surface-opacity), transparent)',
                      }
                    : undefined
                }
              >
                {props.children ?? <AnimatedOutlet />}
              </SidebarInset>
            </div>
            <nav className='bg-background/95 fixed inset-x-0 bottom-0 z-50 grid h-14 grid-cols-2 border-t backdrop-blur md:hidden'>
              <Link
                to='/pricing'
                className={cn(
                  'text-muted-foreground flex flex-col items-center justify-center gap-0.5 text-[11px]',
                  pathname.startsWith('/pricing') && 'text-primary'
                )}
                aria-label={t('Model Square')}
              >
                <Store className='size-4' aria-hidden='true' />
                {t('Model Square')}
              </Link>
              <Link
                to='/rankings'
                className={cn(
                  'text-muted-foreground flex flex-col items-center justify-center gap-0.5 text-[11px]',
                  pathname.startsWith('/rankings') && 'text-primary'
                )}
                aria-label={t('Rankings')}
              >
                <Trophy className='size-4' aria-hidden='true' />
                {t('Rankings')}
              </Link>
            </nav>
          </SidebarProvider>
        </SearchProvider>
      </LayoutProvider>
    </div>
  )
}
