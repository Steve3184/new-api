import { animate, motion, useInView } from 'motion/react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { cn } from '@/lib/utils'

import { DEFAULT_STATS } from '../../constants'
import { usePlatformStats } from '../../hooks'
import { EASE_OUT, IN_VIEW, cascadeVariants, riseVariants } from './motion'

type Stat = {
  key: string
  value: number
  /** Grouped integer, or compact notation whose unit joins the suffix. */
  notation: 'integer' | 'compact'
  suffix: string
  /** Caption, as an i18n key. */
  description: string
}

function StatFigure(props: { stat: Stat; reduced: boolean }) {
  const { i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const ref = useRef<HTMLSpanElement>(null)
  const inView = useInView(ref, { once: true, amount: 0.6 })
  const [counted, setCounted] = useState(0)
  // Where the last count stopped, so a refreshed figure moves on from there.
  const reached = useRef(0)

  useEffect(() => {
    if (!inView || props.reduced) return
    const controls = animate(reached.current, props.stat.value, {
      duration: 1.8,
      ease: EASE_OUT,
      onUpdate: (latest) => {
        reached.current = latest
        setCounted(latest)
      },
    })
    return () => controls.stop()
  }, [inView, props.reduced, props.stat.value])

  const notation = props.stat.notation
  const formatter = useMemo(
    () =>
      new Intl.NumberFormat(
        locale,
        notation === 'compact'
          ? { notation: 'compact', maximumFractionDigits: 1 }
          : { maximumFractionDigits: 0 }
      ),
    [locale, notation]
  )

  // Compact notation brings its own unit (K, M, 万 ...), which is set in the
  // accent style together with the suffix.
  const parts = formatter.formatToParts(
    props.reduced ? props.stat.value : counted
  )
  const unit = parts.find((part) => part.type === 'compact')?.value ?? ''
  const digits = parts
    .filter((part) => part.type !== 'compact' && part.type !== 'literal')
    .map((part) => part.value)
    .join('')
  const accent = unit + props.stat.suffix

  return (
    <>
      <span ref={ref}>{digits}</span>
      {/* Upright on purpose: gradient text is clipped to the glyph box, so an
          italic unit such as M loses the part that leans past it. */}
      {accent && (
        <span className='preset-two-gradient-text ml-0.5 font-serif text-4xl font-normal sm:text-5xl'>
          {accent}
        </span>
      )}
    </>
  )
}

export function MetricsBand(props: { reduced: boolean }) {
  const { t } = useTranslation()
  const ref = useRef<HTMLElement>(null)
  // Starts loading a little before the band scrolls into view.
  const near = useInView(ref, { once: true, margin: '0px 0px 400px 0px' })
  const platform = usePlatformStats(near)

  // One live figure per slot, in the order of the capability it stands in for.
  const live: (Stat | undefined)[] = [
    platform.vendors === undefined
      ? undefined
      : {
          key: 'vendors',
          value: platform.vendors,
          notation: 'integer',
          suffix: '',
          description: 'providers connected',
        },
    platform.models === undefined
      ? undefined
      : {
          key: 'models',
          value: platform.models,
          notation: 'integer',
          suffix: '',
          description: 'models available',
        },
    platform.traffic && {
      key: 'rpm',
      value: platform.traffic.rpm,
      notation: 'compact',
      suffix: '',
      description: 'requests per minute, live',
    },
    platform.traffic && {
      key: 'tpm',
      value: platform.traffic.tpm,
      notation: 'compact',
      suffix: '',
      description: 'tokens per minute, live',
    },
  ]

  return (
    <section ref={ref} className='relative px-5 sm:px-8'>
      <motion.div
        className='mx-auto grid max-w-6xl grid-cols-2 overflow-hidden rounded-3xl border border-white/[0.07] bg-white/[0.015] lg:grid-cols-4'
        variants={cascadeVariants(props.reduced)}
        initial='hidden'
        whileInView='visible'
        viewport={IN_VIEW}
      >
        {DEFAULT_STATS.map((capability, index) => {
          // A slot whose live figure is missing keeps what the gateway
          // supports in general.
          const stat: Stat = live[index] ?? {
            key: capability.description,
            value: Number(capability.value),
            notation: 'integer',
            suffix: capability.suffix,
            description: capability.description,
          }
          return (
            <motion.div
              // The slot stays mounted whichever figure fills it: a cell added
              // after the band has revealed itself would never be told to appear.
              key={capability.description}
              variants={riseVariants(props.reduced)}
              className={cn(
                'border-white/[0.07] px-6 py-10 sm:px-10 lg:py-12',
                index % 2 === 1 && 'border-l',
                index >= 2 && 'border-t lg:border-t-0',
                index === 2 && 'lg:border-l'
              )}
            >
              <div className='flex items-baseline text-5xl font-semibold tracking-[-0.04em] text-white tabular-nums sm:text-6xl'>
                {platform.settled ? (
                  <StatFigure
                    key={stat.key}
                    stat={stat}
                    reduced={props.reduced}
                  />
                ) : (
                  <span className='text-white/15'>–</span>
                )}
              </div>
              <p className='mt-3 min-h-5 text-sm text-white/45'>
                {platform.settled && t(stat.description)}
              </p>
            </motion.div>
          )
        })}
      </motion.div>
    </section>
  )
}
