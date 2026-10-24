import { CalendarClock, Coins, KeyRound, ShieldCheck } from 'lucide-react'
import { motion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { EASE_OUT } from './motion'

const PROTOCOLS = [
  'Chat Completions',
  'Responses',
  'Claude Messages',
  'Gemini',
  'Embeddings',
  'Images',
  'Audio',
  'Realtime',
  'Rerank',
] as const

export function ProtocolChips(props: { reduced: boolean }) {
  return (
    <div className='flex flex-wrap gap-2'>
      {PROTOCOLS.map((protocol, index) => (
        <motion.span
          key={protocol}
          className='rounded-full border border-white/10 bg-white/[0.03] px-3 py-1.5 font-mono text-[11px] text-white/70 transition-colors duration-300 group-hover/card:border-violet-200/25'
          initial={props.reduced ? false : { opacity: 0, scale: 0.85 }}
          whileInView={{ opacity: 1, scale: 1 }}
          viewport={{ once: true }}
          transition={{
            delay: props.reduced ? 0 : 0.15 + index * 0.05,
            duration: props.reduced ? 0 : 0.5,
            ease: EASE_OUT,
          }}
        >
          {protocol}
        </motion.span>
      ))}
    </div>
  )
}

const CHANNEL_WEIGHTS = [
  ['A', ['62%', '48%', '70%', '62%']],
  ['B', ['28%', '44%', '18%', '28%']],
  ['C', ['10%', '8%', '12%', '10%']],
] as const

export function RoutingBars(props: { reduced: boolean }) {
  const { t } = useTranslation()

  return (
    <div className='space-y-3'>
      {CHANNEL_WEIGHTS.map(([label, widths], index) => (
        <div
          key={label}
          className='grid grid-cols-[4.5rem_1fr] items-center gap-3'
        >
          <span className='font-mono text-[10px] text-white/40'>
            {t('Channel')} {label}
          </span>
          <div className='h-1.5 overflow-hidden rounded-full bg-white/[0.06]'>
            <motion.div
              className='h-full rounded-full bg-gradient-to-r from-violet-300/80 to-cyan-200/80'
              initial={{ width: widths[0] }}
              animate={props.reduced ? undefined : { width: [...widths] }}
              transition={{
                duration: 7,
                delay: index * 0.3,
                repeat: Infinity,
                ease: 'easeInOut',
              }}
            />
          </div>
        </div>
      ))}
    </div>
  )
}

export function UsageSparkline(props: { reduced: boolean }) {
  const line =
    'M0 52 C 24 50, 36 38, 60 40 S 100 18, 124 26 S 168 44, 192 22 S 232 8, 260 12'

  return (
    <svg
      aria-hidden='true'
      viewBox='0 0 260 64'
      className='h-16 w-full overflow-visible'
      fill='none'
    >
      <defs>
        <linearGradient id='preset-two-spark-fill' x1='0' x2='0' y1='0' y2='1'>
          <stop offset='0%' stopColor='#e9d5ff' stopOpacity='0.28' />
          <stop offset='100%' stopColor='#e9d5ff' stopOpacity='0' />
        </linearGradient>
      </defs>
      <motion.path
        d={`${line} L 260 64 L 0 64 Z`}
        fill='url(#preset-two-spark-fill)'
        initial={props.reduced ? false : { opacity: 0 }}
        whileInView={{ opacity: 1 }}
        viewport={{ once: true }}
        transition={{ delay: props.reduced ? 0 : 0.8, duration: 0.8 }}
      />
      <motion.path
        d={line}
        stroke='#e9d5ff'
        strokeWidth={1.5}
        strokeLinecap='round'
        initial={props.reduced ? false : { pathLength: 0 }}
        whileInView={{ pathLength: 1 }}
        viewport={{ once: true }}
        transition={{ duration: props.reduced ? 0 : 1.6, ease: EASE_OUT }}
      />
      <circle cx='260' cy='12' r='3' fill='#a5f3fc' />
    </svg>
  )
}

export function GovernanceGrid() {
  const { t } = useTranslation()

  return (
    <div className='grid grid-cols-2 gap-2'>
      {(
        [
          [KeyRound, 'API keys'],
          [ShieldCheck, 'Model access'],
          [Coins, 'Quota limits'],
          [CalendarClock, 'Expiration'],
        ] as const
      ).map(([Icon, label]) => (
        <div
          key={label}
          className='flex items-center gap-2 rounded-xl border border-white/[0.07] bg-white/[0.02] px-3 py-2.5 text-[11px] text-white/60 transition-colors duration-300 group-hover/card:text-white/80'
        >
          <Icon className='size-3.5 text-cyan-100/70' />
          {t(label)}
        </div>
      ))}
    </div>
  )
}

const SIGNAL_BARS = [
  ['s1', 0.45],
  ['s2', 0.8],
  ['s3', 0.6],
  ['s4', 1],
  ['s5', 0.7],
  ['s6', 0.9],
  ['s7', 0.5],
  ['s8', 0.75],
  ['s9', 0.55],
  ['s10', 0.95],
  ['s11', 0.65],
  ['s12', 0.85],
] as const

export function SignalBars(props: { reduced: boolean }) {
  return (
    <div aria-hidden='true' className='flex h-16 items-end gap-1.5'>
      {SIGNAL_BARS.map(([id, height], index) => (
        <motion.span
          key={id}
          className='w-full origin-bottom rounded-sm bg-gradient-to-t from-cyan-300/20 to-violet-200/70'
          style={{ height: `${height * 100}%` }}
          animate={
            props.reduced ? undefined : { scaleY: [1, 0.45, 0.9, 0.6, 1] }
          }
          transition={{
            duration: 2.8,
            delay: index * 0.12,
            repeat: Infinity,
            ease: 'easeInOut',
          }}
        />
      ))}
    </div>
  )
}
