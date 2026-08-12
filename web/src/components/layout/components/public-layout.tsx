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
import { useSystemConfig } from '@/hooks/use-system-config'
import { cn } from '@/lib/utils'

import type { TopNavLink } from '../types'
import { PublicHeader, type PublicHeaderProps } from './public-header'

type PublicLayoutProps = {
  children: React.ReactNode
  showMainContainer?: boolean
  navContent?: React.ReactNode
  headerProps?: Omit<PublicHeaderProps, 'navContent'>
  navLinks?: TopNavLink[]
  showThemeSwitch?: boolean
  showAuthButtons?: boolean
  showNotifications?: boolean
  logo?: React.ReactNode
  siteName?: string
  backgroundMode?: 'none' | 'hero'
}

export function PublicLayout(props: PublicLayoutProps) {
  const { appearance } = useSystemConfig()
  const showHeroBackground =
    props.backgroundMode === 'hero' && !!appearance.backgroundImage

  return (
    <div className='bg-background text-foreground relative min-h-svh overflow-x-clip'>
      {showHeroBackground && (
        <div className='pointer-events-none absolute inset-x-0 top-0 h-[min(62svh,760px)] overflow-hidden'>
          <div
            aria-hidden='true'
            className='absolute inset-0 bg-cover bg-center bg-no-repeat'
            style={{
              backgroundImage: `url(${JSON.stringify(appearance.backgroundImage)})`,
            }}
          />
          <div
            aria-hidden='true'
            className='from-background/20 via-background/35 to-background absolute inset-0 bg-linear-to-b'
          />
        </div>
      )}
      <PublicHeader
        navContent={props.navContent}
        navLinks={props.navLinks}
        showThemeSwitch={props.showThemeSwitch}
        showAuthButtons={props.showAuthButtons}
        showNotifications={props.showNotifications}
        logo={props.logo}
        siteName={props.siteName}
        {...props.headerProps}
        className={cn(
          showHeroBackground && 'border-0 bg-transparent',
          props.headerProps?.className
        )}
      />

      {props.showMainContainer !== false ? (
        <main className='relative container px-4 py-6 pt-20 md:px-4'>
          {props.children}
        </main>
      ) : (
        props.children
      )}
    </div>
  )
}
