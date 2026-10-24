import { motion, useReducedMotion, useScroll, useSpring } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useAuthStore } from '@/stores/auth-store'

import { CapabilityGrid } from './capability-grid'
import { ClosingCta } from './closing-cta'
import { PresetTwoHero } from './hero'
import { IntegrationSection } from './integration-section'
import { MetricsBand } from './metrics-band'
import { ProviderMarquee } from './provider-marquee'

export function PresetTwo() {
  const { i18n } = useTranslation()
  const shouldReduceMotion = useReducedMotion() ?? false
  const { status } = useStatus()
  const { systemName, homepage } = useSystemConfig()
  const isAuthenticated = useAuthStore((state) => !!state.auth.user)
  const { scrollYProgress } = useScroll()
  const scrollProgress = useSpring(scrollYProgress, {
    stiffness: 140,
    damping: 30,
  })
  const brandName = systemName || 'New API'
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const configuredServerAddress =
    typeof status?.server_address === 'string'
      ? status.server_address.trim().replace(/\/+$/, '')
      : ''
  const baseUrl = configuredServerAddress || globalThis.location.origin
  // CJK glyphs have no true italic; the accent font switches to upright.
  const script = /^(zh|ja)/i.test(i18n.resolvedLanguage || i18n.language)
    ? 'cjk'
    : 'latin'
  const slaText = homepage.presetSlaEnabled ? homepage.presetSlaText : ''

  return (
    <PublicLayout
      showMainContainer={false}
      headerProps={{ floating: true, animateFloatingEntrance: true }}
    >
      <div
        data-motion-mode={shouldReduceMotion ? 'reduced' : 'full'}
        data-script={script}
        className='relative min-h-svh overflow-hidden bg-[#050607] text-white selection:bg-violet-300/30'
      >
        {!shouldReduceMotion && (
          <motion.div
            aria-hidden='true'
            className='fixed inset-x-0 top-0 z-[60] h-px origin-left bg-gradient-to-r from-violet-200 via-cyan-200 to-amber-100'
            style={{ scaleX: scrollProgress }}
          />
        )}
        <div
          aria-hidden='true'
          className='preset-two-grain pointer-events-none fixed inset-0 z-[1] opacity-[0.05] mix-blend-overlay'
        />

        <main className='relative z-[2]'>
          <PresetTwoHero
            brandName={brandName}
            titleMode={homepage.presetTitleMode}
            isAuthenticated={isAuthenticated}
            docsUrl={docsUrl}
            slaText={slaText}
            reduced={shouldReduceMotion}
          />
          <ProviderMarquee reduced={shouldReduceMotion} />
          <CapabilityGrid reduced={shouldReduceMotion} />
          <MetricsBand reduced={shouldReduceMotion} />
          <IntegrationSection baseUrl={baseUrl} reduced={shouldReduceMotion} />
          <ClosingCta
            brandName={brandName}
            isAuthenticated={isAuthenticated}
            docsUrl={docsUrl}
            reduced={shouldReduceMotion}
          />
          <Footer className='relative border-white/[0.06] bg-[#050607] text-white [&_*]:border-white/[0.06] [&_a]:text-white/50 [&_a:hover]:text-white [&_span]:text-white/35' />
        </main>
      </div>
    </PublicLayout>
  )
}
