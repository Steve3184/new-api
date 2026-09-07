import { Link } from '@tanstack/react-router'
import {
  Activity,
  ArrowDown,
  ArrowRight,
  BookOpen,
  Braces,
  Gauge,
  Route,
  ShieldCheck,
} from 'lucide-react'
import { motion, useReducedMotion, type Variants } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Footer } from '@/components/layout/components/footer'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'
import { useSystemConfig } from '@/hooks/use-system-config'
import { useAuthStore } from '@/stores/auth-store'

const FALLBACK_BACKGROUND =
  'https://images.unsplash.com/photo-1515694346937-94d85e41e6f0?auto=format&fit=crop&w=2400&q=88'

const EASE_OUT = [0.16, 1, 0.3, 1] as const

function revealVariants(reduced: boolean): Variants {
  return {
    hidden: reduced ? { opacity: 1 } : { opacity: 0, y: 28 },
    visible: {
      opacity: 1,
      y: 0,
      transition: reduced
        ? { duration: 0 }
        : { duration: 0.75, ease: EASE_OUT },
    },
  }
}

function staggerVariants(reduced: boolean): Variants {
  return {
    hidden: {},
    visible: {
      transition: reduced
        ? { duration: 0 }
        : { delayChildren: 0.12, staggerChildren: 0.1 },
    },
  }
}

function SignalMark(props: { active?: boolean }) {
  return (
    <span
      aria-hidden='true'
      className='relative flex size-2 items-center justify-center'
    >
      {props.active && (
        <span className='absolute size-2 animate-ping rounded-full bg-emerald-300/60 motion-reduce:animate-none' />
      )}
      <span
        className={`relative size-1.5 rounded-full ${props.active ? 'bg-emerald-300' : 'bg-white/35'}`}
      />
    </span>
  )
}

function DocsButton(props: { href: string }) {
  const { t } = useTranslation()
  const isExternal = props.href.startsWith('http')

  return (
    <Button
      variant='outline'
      className='group h-11 rounded-lg border-white/20 bg-white/[0.04] px-5 text-sm font-medium text-white/85 hover:border-white/45 hover:bg-white/[0.1] hover:text-white'
      render={
        isExternal ? (
          <a href={props.href} target='_blank' rel='noopener noreferrer' />
        ) : (
          <Link to={props.href} />
        )
      }
    >
      <BookOpen className='mr-2 size-4 text-white/65 transition-colors group-hover:text-white' />
      {t('Docs')}
    </Button>
  )
}

export function PresetOne() {
  const { t } = useTranslation()
  const shouldReduceMotion = useReducedMotion() ?? false
  const { status } = useStatus()
  const { systemName, appearance, homepage } = useSystemConfig()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const docsUrl =
    (status?.docs_link as string | undefined) || 'https://docs.newapi.pro'
  const backgroundImage = appearance.backgroundImage || FALLBACK_BACKGROUND
  const brandName = systemName || 'New API'
  const configuredServerAddress =
    typeof status?.server_address === 'string'
      ? status.server_address.trim().replace(/\/+$/, '')
      : ''
  const publicBaseUrl = configuredServerAddress || globalThis.location.origin
  const exampleEndpoint = `${publicBaseUrl}/v1/chat/completions`
  const heroSubtitle =
    homepage.presetTitleMode === 'english' ? 'AI Gateway' : t('AI Gateway')
  const reveal = revealVariants(shouldReduceMotion)
  const stagger = staggerVariants(shouldReduceMotion)
  const viewport = { once: true, amount: 0.2 }

  return (
    <PublicLayout showMainContainer={false} headerProps={{ floating: true }}>
      <div
        data-motion-mode={shouldReduceMotion ? 'reduced' : 'full'}
        className='relative min-h-svh overflow-hidden bg-[#202a30] text-white'
      >
        <motion.div
          aria-hidden='true'
          className='absolute inset-x-0 top-0 h-[100svh] scale-[1.03] bg-cover bg-center bg-no-repeat'
          style={{ backgroundImage: `url(${JSON.stringify(backgroundImage)})` }}
          animate={
            shouldReduceMotion
              ? undefined
              : { scale: [1.03, 1.075], x: ['0%', '-0.8%'] }
          }
          transition={{
            duration: 18,
            ease: 'easeInOut',
            repeat: Infinity,
            repeatType: 'mirror',
          }}
        />
        <div
          aria-hidden='true'
          className='absolute inset-x-0 top-0 h-[100svh] bg-[#10181e]/55'
        />
        <div
          aria-hidden='true'
          className='absolute inset-x-0 top-0 h-[100svh] bg-black/20 mix-blend-multiply'
        />

        <main className='relative z-10'>
          <section className='mx-auto flex min-h-[calc(100svh-1.5rem)] max-w-[1230px] flex-col justify-between px-5 pt-28 pb-5 sm:px-8 sm:pt-32 lg:px-0'>
            <motion.div
              className='grid flex-1 items-center gap-12 pb-16 sm:pb-20 lg:grid-cols-[minmax(0,1.45fr)_minmax(17rem,0.55fr)] lg:gap-20'
              variants={stagger}
              initial='hidden'
              animate='visible'
            >
              <div className='max-w-3xl'>
                <motion.p
                  variants={reveal}
                  className='mb-6 text-[10px] font-medium tracking-[0.36em] text-white/60 uppercase sm:text-[11px]'
                >
                  {brandName} <span className='px-1.5 text-white/35'>/</span>{' '}
                  {t('AI infrastructure')}
                </motion.p>

                <motion.h1
                  variants={reveal}
                  className='max-w-4xl text-6xl leading-[0.92] font-semibold text-white sm:text-7xl lg:text-8xl xl:text-9xl'
                >
                  <span className='block'>{brandName}</span>
                  <span className='mt-2 block text-white/70'>
                    {heroSubtitle}
                  </span>
                </motion.h1>

                <motion.p
                  variants={reveal}
                  className='mt-8 max-w-xl text-base leading-relaxed text-white/70 sm:text-lg'
                >
                  {t('One endpoint. Every model. Built for what comes next.')}
                </motion.p>

                <motion.div
                  variants={reveal}
                  className='mt-8 flex flex-wrap items-center gap-2.5 sm:mt-9'
                >
                  <Button
                    className='group h-11 rounded-lg bg-white px-5 text-sm font-semibold text-slate-950 hover:bg-white/90'
                    render={
                      <Link to={isAuthenticated ? '/dashboard' : '/sign-up'} />
                    }
                  >
                    {t(isAuthenticated ? 'Go to Dashboard' : 'Get Started')}
                    <ArrowRight className='ml-2 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
                  </Button>
                  <Button
                    variant='outline'
                    className='h-11 rounded-lg border-white/20 bg-white/[0.04] px-5 text-sm font-medium text-white/85 hover:border-white/45 hover:bg-white/[0.1] hover:text-white'
                    render={<Link to='/pricing' />}
                  >
                    {t('View Pricing')}
                  </Button>
                  <DocsButton href={docsUrl} />
                </motion.div>

                {homepage.presetSlaEnabled && homepage.presetSlaText && (
                  <motion.p
                    variants={reveal}
                    className='mt-7 text-[11px] tracking-[0.08em] text-white/55'
                  >
                    {homepage.presetSlaText}
                  </motion.p>
                )}
              </div>

              <motion.aside
                variants={reveal}
                aria-label={t('Gateway status')}
                className='hidden border-y border-white/20 py-5 lg:block'
              >
                <div className='flex items-center justify-between text-[10px] font-medium tracking-[0.18em] text-white/45 uppercase'>
                  <span>{t('Live routing')}</span>
                  <span className='inline-flex items-center gap-2 text-emerald-200/80'>
                    <SignalMark active />
                    {t('Operational')}
                  </span>
                </div>
                <div className='mt-6 space-y-5'>
                  {(
                    [
                      [Route, t('Unified endpoint'), '/v1'],
                      [Gauge, t('Adaptive routing'), t('Active')],
                      [ShieldCheck, t('Policy controls'), t('Enforced')],
                      [Activity, t('Request signals'), t('Streaming')],
                    ] as const
                  ).map(([StatusIcon, label, value], index) => {
                    return (
                      <motion.div
                        key={String(label)}
                        className='grid grid-cols-[1.5rem_1fr_auto] items-center gap-3 border-t border-white/10 pt-4'
                        initial={
                          shouldReduceMotion ? false : { opacity: 0, x: 16 }
                        }
                        animate={{ opacity: 1, x: 0 }}
                        transition={{
                          delay: shouldReduceMotion ? 0 : 0.58 + index * 0.1,
                          duration: shouldReduceMotion ? 0 : 0.55,
                          ease: EASE_OUT,
                        }}
                      >
                        <StatusIcon className='size-4 text-cyan-100/65' />
                        <span className='text-xs text-white/55'>{label}</span>
                        <span className='font-mono text-[10px] text-white/85'>
                          {value}
                        </span>
                      </motion.div>
                    )
                  })}
                </div>
                <div className='mt-6 flex items-center gap-2 border-t border-white/10 pt-4 font-mono text-[10px] text-white/40'>
                  <Braces className='size-3.5 text-cyan-100/60' />
                  <span>POST {exampleEndpoint}</span>
                </div>
              </motion.aside>
            </motion.div>

            <motion.div
              className='border-t border-white/25'
              initial={shouldReduceMotion ? false : { opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{
                delay: shouldReduceMotion ? 0 : 0.8,
                duration: 0.6,
              }}
            >
              <div className='flex flex-wrap items-center justify-between gap-x-8 gap-y-4 py-4 text-[10px] font-medium tracking-[0.16em] text-white/50 uppercase sm:py-5'>
                <span className='shrink-0'>{t('Supported routes')}</span>
                <div className='flex flex-wrap items-center gap-x-5 gap-y-2 sm:gap-x-7'>
                  <span>/01 OpenAI</span>
                  <span>/02 Responses</span>
                  <span>/03 Claude</span>
                  <span>/04 Gemini</span>
                </div>
                <a
                  href='#overview'
                  className='group ml-auto inline-flex shrink-0 items-center gap-2 text-white/65 transition-colors hover:text-white'
                >
                  {t('Explore')}
                  <ArrowDown className='size-3.5 transition-transform duration-200 group-hover:translate-y-0.5' />
                </a>
              </div>
            </motion.div>
          </section>

          <section
            id='overview'
            className='border-t border-white/10 bg-[#11191f]/95'
          >
            <motion.div
              className='mx-auto grid max-w-[1230px] gap-12 px-5 py-20 sm:px-8 lg:grid-cols-[0.9fr_1.1fr] lg:gap-24 lg:px-0 lg:py-28'
              variants={stagger}
              initial='hidden'
              whileInView='visible'
              viewport={viewport}
            >
              <motion.div variants={reveal}>
                <p className='text-[10px] font-medium tracking-[0.3em] text-cyan-200/70 uppercase'>
                  {t('One endpoint, any model')}
                </p>
                <h2 className='mt-4 max-w-lg text-3xl leading-tight font-semibold text-white sm:text-4xl'>
                  {t('A clearer path from idea to production.')}
                </h2>
              </motion.div>
              <div className='grid gap-8 sm:grid-cols-3'>
                {[
                  [
                    t('Connect'),
                    t('Use one familiar API for the models you already run.'),
                  ],
                  [
                    t('Compose'),
                    t('Keep routing, limits, and access in one place.'),
                  ],
                  [
                    t('Observe'),
                    t('See the signals that matter as traffic grows.'),
                  ],
                ].map(([title, description]) => (
                  <motion.div
                    key={title}
                    variants={reveal}
                    className='group border-t border-white/20 pt-4 transition-colors hover:border-cyan-200/60'
                  >
                    <h3 className='text-sm font-semibold text-white'>
                      {title}
                    </h3>
                    <p className='mt-2 text-xs leading-relaxed text-white/50 transition-colors group-hover:text-white/70'>
                      {description}
                    </p>
                  </motion.div>
                ))}
              </div>
            </motion.div>
          </section>

          <section className='border-t border-white/10 bg-[#11191f]/95'>
            <motion.div
              className='mx-auto grid max-w-[1230px] gap-14 px-5 py-20 sm:px-8 lg:grid-cols-[0.8fr_1.2fr] lg:gap-24 lg:px-0 lg:py-28'
              variants={stagger}
              initial='hidden'
              whileInView='visible'
              viewport={viewport}
            >
              <motion.div variants={reveal}>
                <p className='text-[10px] font-medium tracking-[0.3em] text-cyan-200/70 uppercase'>
                  01 / {t('Control')}
                </p>
                <h2 className='mt-4 max-w-md text-3xl leading-tight font-semibold text-white sm:text-4xl'>
                  {t('The routing layer your stack was missing.')}
                </h2>
                <p className='mt-5 max-w-md text-sm leading-relaxed text-white/55'>
                  {t(
                    'Keep model choice, spend controls, reliability, and observability in one operational surface.'
                  )}
                </p>
              </motion.div>

              <motion.div
                variants={reveal}
                className='divide-y divide-white/15 border-y border-white/15'
              >
                {[
                  [
                    'Smart routing',
                    'Balance latency, cost, and availability across providers.',
                  ],
                  [
                    'Usage clarity',
                    'See spend, quotas, and performance signals as requests move.',
                  ],
                  [
                    'Policy by design',
                    'Apply groups, limits, and access rules before traffic leaves.',
                  ],
                  [
                    'Developer first',
                    'Keep one stable API contract while your model mix evolves.',
                  ],
                ].map(([title, description], index) => (
                  <motion.div
                    key={title}
                    className='group grid gap-3 py-5 sm:grid-cols-[3rem_1fr_1.5fr] sm:items-center sm:gap-6'
                    initial={shouldReduceMotion ? false : { opacity: 0, x: 18 }}
                    whileInView={{ opacity: 1, x: 0 }}
                    viewport={{ once: true, amount: 0.5 }}
                    transition={{
                      delay: shouldReduceMotion ? 0 : index * 0.07,
                      duration: shouldReduceMotion ? 0 : 0.55,
                      ease: EASE_OUT,
                    }}
                  >
                    <span className='text-[10px] tracking-[0.2em] text-white/35 transition-colors group-hover:text-cyan-100/75'>
                      0{index + 1}
                    </span>
                    <h3 className='text-sm font-semibold text-white'>
                      {t(title)}
                    </h3>
                    <p className='text-xs leading-relaxed text-white/50'>
                      {t(description)}
                    </p>
                  </motion.div>
                ))}
              </motion.div>
            </motion.div>
          </section>

          <section className='border-t border-white/10 bg-[#0d151b]'>
            <motion.div
              className='mx-auto grid max-w-[1230px] gap-12 px-5 py-20 sm:px-8 lg:grid-cols-[0.9fr_1.1fr] lg:gap-24 lg:px-0 lg:py-28'
              variants={stagger}
              initial='hidden'
              whileInView='visible'
              viewport={viewport}
            >
              <motion.div
                variants={reveal}
                className='flex flex-col justify-between'
              >
                <div>
                  <p className='text-[10px] font-medium tracking-[0.3em] text-cyan-200/70 uppercase'>
                    02 / {t('Drop-in integration')}
                  </p>
                  <h2 className='mt-4 max-w-md text-3xl leading-tight font-semibold text-white sm:text-4xl'>
                    {t('Build the route once. Move faster everywhere.')}
                  </h2>
                  <p className='mt-5 max-w-md text-sm leading-relaxed text-white/55'>
                    {t(
                      'Start with a clean endpoint and add providers as your product grows.'
                    )}
                  </p>
                </div>
                <div className='mt-9 flex flex-wrap gap-2.5'>
                  <Button
                    className='group h-11 rounded-lg bg-white px-5 text-sm font-semibold text-slate-950 hover:bg-white/90'
                    render={
                      <Link to={isAuthenticated ? '/dashboard' : '/sign-up'} />
                    }
                  >
                    {t('Start routing')}
                    <ArrowRight className='ml-2 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
                  </Button>
                  <Button
                    variant='outline'
                    className='h-11 rounded-lg border-white/20 bg-white/[0.04] px-5 text-sm font-medium text-white/80 hover:border-white/45 hover:bg-white/[0.1] hover:text-white'
                    render={<Link to='/pricing' />}
                  >
                    {t('Explore plans')}
                  </Button>
                </div>
              </motion.div>

              <motion.div
                variants={reveal}
                className='relative overflow-hidden border-y border-white/15 py-5 sm:py-7'
              >
                {!shouldReduceMotion && (
                  <motion.span
                    aria-hidden='true'
                    className='absolute inset-x-0 top-0 h-px bg-cyan-200/70'
                    animate={{ x: ['-100%', '100%'] }}
                    transition={{
                      duration: 4.5,
                      repeat: Infinity,
                      ease: 'linear',
                    }}
                  />
                )}
                <div className='mb-5 flex items-center justify-between text-[10px] tracking-[0.2em] text-white/40 uppercase'>
                  <span>{t('One endpoint, any model')}</span>
                  <span className='text-emerald-200/80'>200 OK</span>
                </div>
                <pre className='overflow-x-auto text-xs leading-7 text-cyan-100/75'>
                  <code>
                    {[
                      `curl ${exampleEndpoint} \\`,
                      '  -H "Authorization: Bearer $API_KEY" \\',
                      '  -H "Content-Type: application/json" \\',
                      "  -d '{",
                      '    "model": "your-model",',
                      '    "messages": [{"role":"user","content":"Hello"}]',
                      "  }'",
                    ].join('\n')}
                  </code>
                </pre>
              </motion.div>
            </motion.div>
          </section>

          <section className='border-t border-white/10 bg-[#11191f]/95'>
            <motion.div
              className='mx-auto flex max-w-[1230px] flex-col gap-8 px-5 py-20 sm:px-8 lg:flex-row lg:items-end lg:justify-between lg:px-0 lg:py-28'
              variants={stagger}
              initial='hidden'
              whileInView='visible'
              viewport={viewport}
            >
              <motion.div variants={reveal}>
                <p className='text-[10px] font-medium tracking-[0.3em] text-cyan-200/70 uppercase'>
                  03 / {t('Ready when you are')}
                </p>
                <h2 className='mt-4 max-w-xl text-3xl leading-tight font-semibold text-white sm:text-5xl'>
                  {t('One endpoint, any model')}
                </h2>
              </motion.div>
              <motion.div
                variants={reveal}
                className='flex flex-wrap gap-2.5 lg:pb-1'
              >
                <Button
                  className='group h-11 rounded-lg bg-white px-5 text-sm font-semibold text-slate-950 hover:bg-white/90'
                  render={
                    <Link to={isAuthenticated ? '/dashboard' : '/sign-up'} />
                  }
                >
                  {t(isAuthenticated ? 'Go to Dashboard' : 'Get Started')}
                  <ArrowRight className='ml-2 size-4 transition-transform duration-200 group-hover:translate-x-0.5' />
                </Button>
                <DocsButton href={docsUrl} />
              </motion.div>
            </motion.div>
          </section>
          <Footer className='relative border-white/10 bg-[#0d151b] text-white [&_*]:border-white/10 [&_a]:text-white/55 [&_a:hover]:text-white [&_span]:text-white/40' />
        </main>
      </div>
    </PublicLayout>
  )
}
