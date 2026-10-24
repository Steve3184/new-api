import { animate, motion, useInView } from 'motion/react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'
import { cn } from '@/lib/utils'

import { DEFAULT_STATS } from '../../constants'
import { EASE_OUT, IN_VIEW, cascadeVariants, riseVariants } from './motion'

function CountUp(props: { value: number; reduced: boolean }) {
  const { i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const ref = useRef<HTMLSpanElement>(null)
  const inView = useInView(ref, { once: true, amount: 0.6 })
  const [current, setCurrent] = useState(props.reduced ? props.value : 0)

  useEffect(() => {
    if (!inView || props.reduced) return
    const controls = animate(0, props.value, {
      duration: 1.8,
      ease: EASE_OUT,
      onUpdate: (latest) => setCurrent(Math.round(latest)),
    })
    return () => controls.stop()
  }, [inView, props.reduced, props.value])

  return <span ref={ref}>{formatNumber(current, locale)}</span>
}

export function MetricsBand(props: { reduced: boolean }) {
  const { t } = useTranslation()

  return (
    <section className='relative px-5 sm:px-8'>
      <motion.div
        className='mx-auto grid max-w-6xl grid-cols-2 overflow-hidden rounded-3xl border border-white/[0.07] bg-white/[0.015] lg:grid-cols-4'
        variants={cascadeVariants(props.reduced)}
        initial='hidden'
        whileInView='visible'
        viewport={IN_VIEW}
      >
        {DEFAULT_STATS.map((stat, index) => (
          <motion.div
            key={stat.description}
            variants={riseVariants(props.reduced)}
            className={cn(
              'border-white/[0.07] px-6 py-10 sm:px-10 lg:py-12',
              index % 2 === 1 && 'border-l',
              index >= 2 && 'border-t lg:border-t-0',
              index === 2 && 'lg:border-l'
            )}
          >
            <div className='flex items-baseline text-5xl font-semibold tracking-[-0.04em] text-white tabular-nums sm:text-6xl'>
              <CountUp value={Number(stat.value)} reduced={props.reduced} />
              <span className='preset-two-gradient-text ml-0.5 font-serif text-4xl font-normal italic sm:text-5xl'>
                {stat.suffix}
              </span>
            </div>
            <p className='mt-3 text-sm text-white/45'>{t(stat.description)}</p>
          </motion.div>
        ))}
      </motion.div>
    </section>
  )
}
