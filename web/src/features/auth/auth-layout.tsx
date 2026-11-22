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
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { cn } from '@/lib/utils'

import { AuthParticleField } from './components/auth-particle-field'

type AuthLayoutProps = {
  children: React.ReactNode
}

export function AuthLayout({ children }: AuthLayoutProps) {
  const { appearance } = useSystemConfig()
  if (appearance.authPageStyle === 'split') {
    return <SplitAuthLayout>{children}</SplitAuthLayout>
  }
  return <CenteredAuthLayout>{children}</CenteredAuthLayout>
}

function BrandMark(props: { className?: string; translucent?: boolean }) {
  const { t } = useTranslation()
  const { systemName, logo, loading } = useSystemConfig()
  return (
    <div
      className={cn(
        'flex items-center gap-2',
        props.translucent && 'rounded-lg p-2 backdrop-blur-sm'
      )}
      style={
        props.translucent
          ? {
              backgroundColor:
                'color-mix(in oklch, var(--card) 60%, transparent)',
            }
          : undefined
      }
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
    </div>
  )
}

function CenteredAuthLayout({ children }: AuthLayoutProps) {
  const { systemName, appearance } = useSystemConfig()
  const backgroundImage = appearance.authBackgroundEnabled
    ? appearance.backgroundImage
    : ''
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
      data-auth-layout='centered'
      aria-label={systemName}
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
          'absolute top-4 left-4 z-10 transition-opacity hover:opacity-80 sm:top-8 sm:left-8'
        )}
      >
        <BrandMark translucent={Boolean(backgroundImage)} />
      </Link>
      <div className='relative z-0 container flex items-center pt-16 sm:pt-0'>
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

/**
 * Two-column sign-in: a branded illustration panel beside the form. Below the
 * large breakpoint the illustration is dropped so the form keeps the full
 * width, matching the centered layout on phones.
 */
function SplitAuthLayout({ children }: AuthLayoutProps) {
  const { appearance } = useSystemConfig()
  const illustration = appearance.authSplitImage
  // A split layout is all about the illustration panel; keep the configured
  // background image available there instead of behind the form.
  const backgroundImage =
    appearance.authBackgroundEnabled && !illustration
      ? appearance.backgroundImage
      : ''
  // A configured illustration wins; otherwise the background image, then the
  // built-in particle field.
  const panelImage = illustration || backgroundImage

  return (
    <div
      className='grid min-h-svh max-w-none lg:grid-cols-2'
      data-auth-layout='split'
    >
      <div className='bg-muted relative hidden overflow-hidden lg:flex lg:flex-col'>
        {panelImage ? (
          <img
            src={panelImage}
            alt=''
            aria-hidden='true'
            className='absolute inset-0 h-full w-full object-cover'
          />
        ) : (
          <BuiltInIllustration />
        )}
        <div className='relative z-10 flex flex-1 flex-col justify-between p-8 xl:p-12'>
          <Link to='/' className='w-fit transition-opacity hover:opacity-80'>
            <BrandMark translucent={Boolean(panelImage)} />
          </Link>
          <LegalLinks className='text-muted-foreground' />
        </div>
      </div>

      <div className='relative flex flex-col'>
        <div className='flex items-center justify-between p-6 lg:hidden'>
          <Link to='/' className='transition-opacity hover:opacity-80'>
            <BrandMark />
          </Link>
        </div>
        <div className='flex flex-1 items-center justify-center px-6 pb-10 lg:px-8 lg:py-12'>
          <div className='mx-auto flex w-full flex-col justify-center space-y-2 sm:w-[420px]'>
            {children}
          </div>
        </div>
        <div className='p-6 lg:hidden'>
          <LegalLinks className='text-muted-foreground text-center' />
        </div>
      </div>
    </div>
  )
}

/** Animated particle field drawn from the active theme colors. */
function BuiltInIllustration() {
  return (
    <div
      aria-hidden='true'
      className='from-muted via-background to-secondary absolute inset-0 overflow-hidden bg-gradient-to-br'
    >
      <div className='bg-chart-1/20 absolute top-[10%] left-[8%] size-[38%] rounded-full blur-3xl' />
      <div className='bg-chart-2/20 absolute right-[6%] bottom-[8%] size-[42%] rounded-full blur-3xl' />
      <div className='bg-chart-4/15 absolute top-[40%] left-[30%] size-[30%] rounded-full blur-3xl' />
      <AuthParticleField className='absolute inset-0' />
    </div>
  )
}

function LegalLinks(props: { className?: string }) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const links = [
    status?.user_agreement_enabled
      ? { label: t('User Agreement'), href: '/user-agreement' }
      : null,
    status?.privacy_policy_enabled
      ? { label: t('Privacy Policy'), href: '/privacy-policy' }
      : null,
  ].filter((link): link is { label: string; href: string } => link !== null)
  if (links.length === 0) return null

  return (
    <nav
      aria-label={t('Legal documents')}
      className={cn('flex flex-wrap gap-x-6 gap-y-2 text-xs', props.className)}
    >
      {links.map((link) => (
        <a
          key={link.href}
          href={link.href}
          className='hover:text-primary underline-offset-4 hover:underline'
        >
          {link.label}
        </a>
      ))}
    </nav>
  )
}
