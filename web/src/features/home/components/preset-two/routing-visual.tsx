import { Bot, Code2, Laptop, Waypoints } from 'lucide-react'
import { AnimatePresence, motion } from 'motion/react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { getLobeIcon } from '@/lib/lobe-icon'

import { EASE_OUT } from './motion'

// Node coordinates live in the SVG viewBox (1000 x 420); the HTML labels
// reuse them as percentages so the curves always meet their nodes.
const VIEW_WIDTH = 1000
const VIEW_HEIGHT = 420
const GATEWAY = { x: 500, y: 210 }

const CLIENTS = [
  { y: 110, label: 'Your app', Icon: Laptop },
  { y: 210, label: 'AI agents', Icon: Bot },
  { y: 310, label: 'IDE & CLI', Icon: Code2 },
] as const

const PROVIDERS = [
  { y: 66, icon: 'OpenAI', name: 'OpenAI' },
  { y: 138, icon: 'Claude.Color', name: 'Claude' },
  { y: 210, icon: 'Gemini.Color', name: 'Gemini' },
  { y: 282, icon: 'DeepSeek.Color', name: 'DeepSeek' },
  { y: 354, icon: 'Qwen.Color', name: 'Qwen' },
] as const

const SAMPLE_TRAFFIC = [
  ['/v1/messages', 'claude-sonnet-4-5', 512],
  ['/v1/responses', 'gpt-5', 448],
  ['/v1/chat/completions', 'deepseek-chat', 386],
  ['/v1/embeddings', 'text-embedding-3-large', 92],
  ['/v1/chat/completions', 'qwen3-max', 301],
  ['/v1beta/models', 'gemini-2.5-pro', 467],
] as const

function nodePosition(x: number, y: number) {
  return {
    left: `${(x / VIEW_WIDTH) * 100}%`,
    top: `${(y / VIEW_HEIGHT) * 100}%`,
  }
}

type RoutingVisualProps = {
  brandName: string
  reduced: boolean
}

export function RoutingVisual(props: RoutingVisualProps) {
  const { t } = useTranslation()
  const [tick, setTick] = useState(0)

  useEffect(() => {
    if (props.reduced) return
    const timer = window.setInterval(() => setTick((value) => value + 1), 2400)
    return () => window.clearInterval(timer)
  }, [props.reduced])

  const clientPaths = CLIENTS.map(
    (client) =>
      `M 196 ${client.y} C 320 ${client.y}, 360 ${GATEWAY.y}, 428 ${GATEWAY.y}`
  )
  const providerPaths = PROVIDERS.map(
    (provider) =>
      `M 572 ${GATEWAY.y} C 660 ${GATEWAY.y}, 712 ${provider.y}, 812 ${provider.y}`
  )
  const trafficRows = [0, 1, 2].map((offset) => {
    const index = tick - offset
    const length = SAMPLE_TRAFFIC.length
    return {
      key: index,
      sample: SAMPLE_TRAFFIC[((index % length) + length) % length],
    }
  })

  return (
    <div className='relative overflow-hidden rounded-[28px] border border-white/10 bg-white/[0.025] shadow-[0_40px_120px_-40px_rgba(0,0,0,0.9)] backdrop-blur-xl'>
      <div
        aria-hidden='true'
        className='pointer-events-none absolute inset-x-0 top-0 h-px bg-gradient-to-r from-transparent via-white/40 to-transparent'
      />
      <div className='flex items-center justify-between border-b border-white/[0.07] px-5 py-3.5 text-[11px] text-white/45'>
        <div className='flex items-center gap-1.5' aria-hidden='true'>
          <span className='size-2.5 rounded-full bg-white/10' />
          <span className='size-2.5 rounded-full bg-white/10' />
          <span className='size-2.5 rounded-full bg-white/10' />
        </div>
        <span className='font-mono tracking-wide'>gateway · routing</span>
        <span className='inline-flex items-center gap-1.5 text-emerald-300/80'>
          <span className='relative flex size-1.5'>
            <span className='absolute inline-flex size-full animate-ping rounded-full bg-emerald-300/70 motion-reduce:animate-none' />
            <span className='relative inline-flex size-1.5 rounded-full bg-emerald-300' />
          </span>
          {t('Operational')}
        </span>
      </div>

      <div className='relative hidden aspect-[1000/420] md:block'>
        <svg
          aria-hidden='true'
          viewBox={`0 0 ${VIEW_WIDTH} ${VIEW_HEIGHT}`}
          className='absolute inset-0 size-full'
          fill='none'
        >
          <defs>
            <linearGradient id='preset-two-wire' x1='0' x2='1'>
              <stop offset='0%' stopColor='#ffffff' stopOpacity='0.04' />
              <stop offset='50%' stopColor='#e9d5ff' stopOpacity='0.35' />
              <stop offset='100%' stopColor='#a5f3fc' stopOpacity='0.08' />
            </linearGradient>
            <radialGradient id='preset-two-spark'>
              <stop offset='0%' stopColor='#ffffff' />
              <stop offset='45%' stopColor='#e9d5ff' />
              <stop offset='100%' stopColor='#a5f3fc' stopOpacity='0' />
            </radialGradient>
          </defs>
          {[...clientPaths, ...providerPaths].map((path, index) => (
            <motion.path
              key={path}
              d={path}
              stroke='url(#preset-two-wire)'
              strokeWidth={1.25}
              initial={props.reduced ? false : { pathLength: 0, opacity: 0 }}
              animate={{ pathLength: 1, opacity: 1 }}
              transition={{
                delay: props.reduced ? 0 : 0.6 + index * 0.08,
                duration: props.reduced ? 0 : 1.4,
                ease: EASE_OUT,
              }}
            />
          ))}
          {!props.reduced &&
            [...clientPaths, ...providerPaths].map((path, index) => (
              <circle key={`spark-${path}`} r={0} fill='url(#preset-two-spark)'>
                {/* Hidden until its motion starts so it never rests at the origin. */}
                <set
                  attributeName='r'
                  to='4'
                  begin={`${1.6 + index * 0.37}s`}
                />
                <animateMotion
                  dur={index < clientPaths.length ? '2.6s' : '2.2s'}
                  begin={`${1.6 + index * 0.37}s`}
                  repeatCount='indefinite'
                  path={path}
                />
              </circle>
            ))}
        </svg>

        {CLIENTS.map((client) => (
          <div
            key={client.label}
            className='absolute flex -translate-x-1/2 -translate-y-1/2 items-center gap-2.5 rounded-xl border border-white/10 bg-zinc-950/70 px-3.5 py-2.5 text-xs text-white/75 backdrop-blur-md'
            style={nodePosition(120, client.y)}
          >
            <client.Icon className='size-3.5 text-violet-200/80' />
            {t(client.label)}
          </div>
        ))}

        <div
          className='absolute -translate-x-1/2 -translate-y-1/2'
          style={nodePosition(GATEWAY.x, GATEWAY.y)}
        >
          {!props.reduced && (
            <motion.span
              aria-hidden='true'
              className='absolute inset-0 rounded-3xl border border-violet-200/30'
              animate={{ scale: [1, 1.45], opacity: [0.6, 0] }}
              transition={{ duration: 2.4, repeat: Infinity, ease: 'easeOut' }}
            />
          )}
          <div className='relative rounded-3xl'>
            <span
              aria-hidden='true'
              className='preset-two-orbit-border absolute inset-0'
            />
            <div className='flex w-36 flex-col items-center gap-2 rounded-3xl bg-zinc-950/85 px-4 py-5 text-center'>
              <span className='flex size-10 items-center justify-center rounded-2xl bg-gradient-to-br from-violet-200/25 to-cyan-200/10 ring-1 ring-white/15'>
                <Waypoints className='size-5 text-white' />
              </span>
              <span className='max-w-full truncate text-sm font-semibold text-white'>
                {props.brandName}
              </span>
              <span className='font-mono text-[10px] tracking-[0.2em] text-white/40 uppercase'>
                {t('AI Gateway')}
              </span>
            </div>
          </div>
        </div>

        {PROVIDERS.map((provider) => (
          <div
            key={provider.name}
            className='absolute flex -translate-x-1/2 -translate-y-1/2 items-center gap-2.5 rounded-xl border border-white/10 bg-zinc-950/70 py-2 pr-3.5 pl-2.5 text-xs text-white/80 backdrop-blur-md'
            style={nodePosition(880, provider.y)}
          >
            <span className='flex size-6 items-center justify-center rounded-lg bg-white/[0.06] text-white'>
              {getLobeIcon(provider.icon, 15)}
            </span>
            {provider.name}
          </div>
        ))}
      </div>

      <div className='border-t border-white/[0.07] px-5 py-4'>
        <p className='mb-3 text-[10px] font-medium tracking-[0.22em] text-white/35 uppercase'>
          {t('Sample traffic')}
        </p>
        <ul className='relative h-[6.5rem] space-y-1.5 overflow-hidden font-mono text-[11px]'>
          <AnimatePresence initial={false} mode='popLayout'>
            {trafficRows.map((row, index) => (
              <motion.li
                key={row.key}
                layout={!props.reduced}
                initial={props.reduced ? false : { opacity: 0, y: -14 }}
                animate={{ opacity: 1 - index * 0.28, y: 0 }}
                exit={{ opacity: 0, transition: { duration: 0.2 } }}
                transition={{ duration: 0.55, ease: EASE_OUT }}
                className='grid grid-cols-[auto_1fr_auto_auto] items-center gap-3 rounded-lg bg-white/[0.03] px-3 py-2 text-white/65'
              >
                <span className='rounded bg-violet-300/10 px-1.5 py-0.5 text-[10px] text-violet-200'>
                  POST
                </span>
                <span className='truncate'>
                  {row.sample[0]}
                  <span className='text-white/30'> · {row.sample[1]}</span>
                </span>
                <span className='text-emerald-300/85'>200</span>
                <span className='w-12 text-right text-white/40'>
                  {row.sample[2]}ms
                </span>
              </motion.li>
            ))}
          </AnimatePresence>
        </ul>
      </div>
    </div>
  )
}
