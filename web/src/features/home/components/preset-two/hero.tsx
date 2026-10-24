import { Sparkles } from 'lucide-react'
import { motion } from 'motion/react'
import { useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { PresetTwoActions } from './actions'
import { EASE_OUT, cascadeVariants, riseVariants } from './motion'
import { RoutingVisual } from './routing-visual'

type PresetTwoHeroProps = {
  brandName: string
  titleMode: 'i18n' | 'english'
  isAuthenticated: boolean
  docsUrl: string
  slaText: string
  reduced: boolean
}

const TITLE_LEAD = 'One endpoint.'
const TITLE_ACCENT = 'Every frontier model.'

export function PresetTwoHero(props: PresetTwoHeroProps) {
  const { t } = useTranslation()
  const sectionRef = useRef<HTMLElement>(null)
  const rise = riseVariants(props.reduced)
  const titleLead = props.titleMode === 'english' ? TITLE_LEAD : t(TITLE_LEAD)
  const titleAccent =
    props.titleMode === 'english' ? TITLE_ACCENT : t(TITLE_ACCENT)

  return (
    <section
      ref={sectionRef}
      onPointerMove={(event) => {
        const section = sectionRef.current
        if (!section || props.reduced) return
        const rect = section.getBoundingClientRect()
        section.style.setProperty('--spot-x', `${event.clientX - rect.left}px`)
        section.style.setProperty('--spot-y', `${event.clientY - rect.top}px`)
      }}
      className='relative isolate overflow-hidden px-5 pt-36 pb-24 [--spot-x:50%] [--spot-y:30%] sm:px-8 sm:pt-44 lg:pb-32'
    >
      <div aria-hidden='true' className='absolute inset-0 -z-10'>
        <div className='preset-two-aurora absolute -top-[22%] left-[8%] h-[38rem] w-[38rem] rounded-full bg-violet-500/25 blur-[120px]' />
        <div className='preset-two-aurora absolute top-[2%] right-[2%] h-[30rem] w-[30rem] rounded-full bg-cyan-400/15 blur-[120px] [animation-delay:-7s]' />
        <div className='preset-two-aurora absolute top-[38%] left-[38%] h-[24rem] w-[24rem] rounded-full bg-amber-300/10 blur-[110px] [animation-delay:-13s]' />
        <div className='absolute inset-0 bg-[linear-gradient(to_right,rgba(255,255,255,0.045)_1px,transparent_1px),linear-gradient(to_bottom,rgba(255,255,255,0.045)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_70%_55%_at_50%_0%,#000_40%,transparent_100%)] bg-[size:72px_72px]' />
        <div className='absolute inset-0 bg-[radial-gradient(520px_circle_at_var(--spot-x)_var(--spot-y),rgba(233,213,255,0.09),transparent_60%)]' />
        <div className='absolute inset-x-0 bottom-0 h-56 bg-gradient-to-b from-transparent to-[#050607]' />
      </div>

      <motion.div
        className='mx-auto flex max-w-5xl flex-col items-center text-center'
        variants={cascadeVariants(props.reduced, 0.12)}
        initial='hidden'
        animate='visible'
      >
        <motion.div
          variants={rise}
          className='relative inline-flex rounded-full'
        >
          <span
            aria-hidden='true'
            className='preset-two-orbit-border absolute inset-0'
          />
          <span className='inline-flex items-center gap-2 rounded-full bg-white/[0.04] px-4 py-1.5 text-xs text-white/70 backdrop-blur-md'>
            <Sparkles className='size-3.5 text-violet-200' />
            <span className='font-medium text-white'>{props.brandName}</span>
            <span aria-hidden='true' className='h-3 w-px bg-white/15' />
            {t('One API key for every leading model')}
          </span>
        </motion.div>

        <motion.h1
          variants={rise}
          className='mt-9 text-[clamp(2.75rem,8vw,6.5rem)] leading-[1.02] font-semibold tracking-[-0.045em] text-white sm:break-keep'
        >
          <span
            lang={props.titleMode === 'english' ? 'en' : undefined}
            className='block'
          >
            {titleLead}
          </span>
          <span
            lang={props.titleMode === 'english' ? 'en' : undefined}
            className='preset-two-accent preset-two-gradient-text block pb-2 tracking-[-0.03em]'
          >
            {titleAccent}
          </span>
        </motion.h1>

        <motion.p
          variants={rise}
          className='mt-7 max-w-2xl text-base leading-relaxed text-pretty text-white/60 sm:text-lg'
        >
          {t(
            'Route, meter, and govern every AI request through one gateway that speaks the OpenAI, Claude, and Gemini protocols natively.'
          )}
        </motion.p>

        <motion.div variants={rise}>
          <PresetTwoActions
            className='mt-10 justify-center'
            isAuthenticated={props.isAuthenticated}
            docsUrl={props.docsUrl}
          />
        </motion.div>

        {props.slaText && (
          <motion.p
            variants={rise}
            className='mt-6 inline-flex items-center gap-2 text-xs tracking-wide text-white/45'
          >
            <span className='size-1 rounded-full bg-emerald-300/80' />
            {props.slaText}
          </motion.p>
        )}
      </motion.div>

      <motion.div
        className='mx-auto mt-16 max-w-5xl [perspective:1600px] sm:mt-20'
        initial={
          props.reduced
            ? false
            : { opacity: 0, y: 70, rotateX: 16, scale: 0.96 }
        }
        animate={{ opacity: 1, y: 0, rotateX: 0, scale: 1 }}
        transition={{
          delay: props.reduced ? 0 : 0.55,
          duration: props.reduced ? 0 : 1.3,
          ease: EASE_OUT,
        }}
      >
        <RoutingVisual brandName={props.brandName} reduced={props.reduced} />
      </motion.div>
    </section>
  )
}
