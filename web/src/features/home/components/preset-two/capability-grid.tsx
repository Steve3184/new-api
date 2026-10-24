import { Coins, Receipt, Tags, Wallet } from 'lucide-react'
import { motion } from 'motion/react'
import { useRef, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import {
  GovernanceGrid,
  ProtocolChips,
  RoutingBars,
  SignalBars,
  UsageSparkline,
} from './capability-visuals'
import { cascadeVariants, riseVariants } from './motion'

type SpotlightCardProps = {
  title: string
  description: string
  index: number
  reduced: boolean
  className?: string
  children: ReactNode
}

function SpotlightCard(props: SpotlightCardProps) {
  const cardRef = useRef<HTMLDivElement>(null)

  return (
    <motion.article
      ref={cardRef}
      variants={riseVariants(props.reduced)}
      onPointerMove={(event) => {
        const card = cardRef.current
        if (!card) return
        const rect = card.getBoundingClientRect()
        card.style.setProperty('--card-x', `${event.clientX - rect.left}px`)
        card.style.setProperty('--card-y', `${event.clientY - rect.top}px`)
      }}
      className={cn(
        'group/card relative isolate flex flex-col justify-between gap-8 overflow-hidden rounded-3xl border border-white/[0.07] bg-gradient-to-b from-white/[0.04] to-white/[0.01] p-7 [--card-x:50%] [--card-y:0%] sm:p-8',
        props.className
      )}
    >
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 -z-10 bg-[radial-gradient(420px_circle_at_var(--card-x)_var(--card-y),rgba(233,213,255,0.1),transparent_65%)] opacity-0 transition-opacity duration-500 group-hover/card:opacity-100'
      />
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-0 rounded-3xl bg-[radial-gradient(280px_circle_at_var(--card-x)_var(--card-y),rgba(233,213,255,0.55),transparent_70%)] [mask-composite:exclude] p-px opacity-0 transition-opacity duration-500 [mask:linear-gradient(#000_0_0)_content-box,linear-gradient(#000_0_0)] group-hover/card:opacity-100'
      />
      <div>{props.children}</div>
      <div>
        <span className='font-mono text-[10px] tracking-[0.2em] text-white/30'>
          {String(props.index).padStart(2, '0')}
        </span>
        <h3 className='mt-2 text-lg font-semibold tracking-tight text-white'>
          {props.title}
        </h3>
        <p className='mt-2 max-w-md text-sm leading-relaxed text-white/50'>
          {props.description}
        </p>
      </div>
    </motion.article>
  )
}

const BILLING_TRAITS = [
  [Wallet, 'Pay as you go'],
  [Tags, 'Per-model pricing'],
  [Coins, 'Real-time balance'],
  [Receipt, 'Usage logs'],
] as const

export function CapabilityGrid(props: { reduced: boolean }) {
  const { t } = useTranslation()

  return (
    <section id='capabilities' className='relative px-5 py-28 sm:px-8 lg:py-36'>
      <motion.div
        className='mx-auto max-w-6xl'
        variants={cascadeVariants(props.reduced)}
        initial='hidden'
        whileInView='visible'
        viewport={{ once: true, amount: 0.1 }}
      >
        <motion.div
          variants={riseVariants(props.reduced)}
          className='mx-auto max-w-3xl text-center'
        >
          <p className='text-[11px] font-medium tracking-[0.3em] text-violet-200/70 uppercase'>
            {t('Capabilities')}
          </p>
          <h2 className='mt-5 text-4xl leading-[1.05] font-semibold tracking-[-0.03em] text-balance text-white sm:text-5xl sm:break-keep'>
            {t('Everything between your product and the model.')}
          </h2>
          <p className='mt-5 text-base leading-relaxed text-white/50'>
            {t(
              'Protocol translation, routing, metering, and governance, composed into one operational surface.'
            )}
          </p>
        </motion.div>

        <div className='mt-16 grid gap-4 lg:grid-cols-3'>
          <SpotlightCard
            index={1}
            reduced={props.reduced}
            className='lg:col-span-2'
            title={t('Every protocol, one contract')}
            description={t(
              'Serve OpenAI, Claude, and Gemini compatible APIs from the same keys, so clients never need to know which upstream answers.'
            )}
          >
            <ProtocolChips reduced={props.reduced} />
          </SpotlightCard>
          <SpotlightCard
            index={2}
            reduced={props.reduced}
            title={t('Routing that heals itself')}
            description={t(
              'Automatic retries and failover across upstreams keep your requests flowing when a provider degrades.'
            )}
          >
            <RoutingBars reduced={props.reduced} />
          </SpotlightCard>
          <SpotlightCard
            index={3}
            reduced={props.reduced}
            title={t('Built for streaming')}
            description={t(
              'Streaming, tool calls, and multimodal inputs work the way your SDK already expects.'
            )}
          >
            <UsageSparkline reduced={props.reduced} />
          </SpotlightCard>
          <SpotlightCard
            index={4}
            reduced={props.reduced}
            title={t('Governance built in')}
            description={t(
              'Create a key per project, then limit which models it can call and how much it can spend.'
            )}
          >
            <GovernanceGrid />
          </SpotlightCard>
          <SpotlightCard
            index={5}
            reduced={props.reduced}
            title={t('Signals, not noise')}
            description={t(
              'Latency, errors, and spend surface in real time as traffic grows.'
            )}
          >
            <SignalBars reduced={props.reduced} />
          </SpotlightCard>
          <SpotlightCard
            index={6}
            reduced={props.reduced}
            className='lg:col-span-3 lg:flex-row-reverse lg:items-end'
            title={t('Pay only for what you use')}
            description={t(
              'Transparent per-model pricing, a live balance, and a detailed log for every call. Top up whenever you need, with no commitment.'
            )}
          >
            <div className='flex flex-wrap gap-2 lg:justify-end'>
              {BILLING_TRAITS.map(([Icon, label]) => (
                <span
                  key={label}
                  className='inline-flex items-center gap-2 rounded-full border border-white/10 bg-white/[0.03] px-3.5 py-1.5 text-xs text-white/65'
                >
                  <Icon className='size-3.5 text-cyan-100/60' />
                  {t(label)}
                </span>
              ))}
            </div>
          </SpotlightCard>
        </div>
      </motion.div>
    </section>
  )
}
