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
import { Link } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'
import { cn } from '@/lib/utils'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useTranslation()
  const { systemName, logo, loading, appearance } = useSystemConfig()
  const backgroundImage = appearance.backgroundImage
  const blurOpacity = Math.min(
    100,
    Math.max(0, appearance.backgroundBlurOpacity ?? 40)
  )
  const surfaceStyles = backgroundImage
    ? ({
        '--console-card-opacity': `${Math.min(85, blurOpacity + 15)}%`,
      } as React.CSSProperties)
    : undefined
  const glassSurfaceStyle = backgroundImage
    ? {
        backgroundColor:
          'color-mix(in oklch, var(--card) var(--console-card-opacity), transparent)',
      }
    : undefined

  return (
    <div
      className='relative isolate grid min-h-svh max-w-none'
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
      <Link
        to='/'
        className={cn(
          'absolute top-4 left-4 z-10 flex items-center gap-2 transition-opacity hover:opacity-80 sm:top-8 sm:left-8',
          backgroundImage && 'rounded-lg p-2 backdrop-blur-sm'
        )}
        style={glassSurfaceStyle}
      >
        <div className='relative h-8 w-8'>
          {loading ? (
            <Skeleton className='absolute inset-0 rounded-full' />
          ) : (
            <img
              src={logo}
              alt={t('Logo')}
              className='h-8 w-8 rounded-full object-cover'
            />
          )}
        </div>
        {loading ? (
          <Skeleton className='h-6 w-24' />
        ) : (
          <h1 className='text-xl font-medium'>{systemName}</h1>
        )}
      </Link>
      <div className='container relative z-0 flex items-center pt-16 sm:pt-0'>
        <div
          className={cn(
            'mx-auto flex w-full flex-col justify-center space-y-2 px-4 py-8 sm:w-[480px] sm:p-8',
            backgroundImage && 'rounded-lg backdrop-blur-sm'
          )}
          style={glassSurfaceStyle}
        >
          {children}
        </div>
      </div>
    </div>
  )
}
