import { motion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { PresetTwoActions } from './actions'
import { IN_VIEW, riseVariants } from './motion'

type ClosingCtaProps = {
  brandName: string
  isAuthenticated: boolean
  docsUrl: string
  reduced: boolean
}

export function ClosingCta(props: ClosingCtaProps) {
  const { t } = useTranslation()

  return (
    <section className='px-5 pb-28 sm:px-8 lg:pb-36'>
      <motion.div
        className='relative mx-auto max-w-6xl rounded-[32px]'
        variants={riseVariants(props.reduced)}
        initial='hidden'
        whileInView='visible'
        viewport={IN_VIEW}
      >
        <span
          aria-hidden='true'
          className='preset-two-orbit-border absolute inset-0'
        />
        <div className='relative isolate overflow-hidden rounded-[32px] bg-[#08090c] px-6 py-20 text-center sm:px-12 lg:py-28'>
          {/* The grain only blends with what shares this layer, so the layer
              carries its own opaque base under the drifting colors. */}
          <div
            aria-hidden='true'
            className='absolute inset-0 -z-10 bg-[#08090c]'
          >
            <div className='preset-two-flow absolute top-[42%] -left-[22%] h-[80%] w-[70%] rounded-full bg-violet-600/75 blur-[70px]' />
            <div className='preset-two-flow absolute -top-[46%] -right-[20%] h-[80%] w-[60%] rounded-full bg-cyan-400/40 blur-[80px] [--flow-delay:-11s] [--flow-duration:31s]' />
            <div className='preset-two-flow absolute -right-[18%] -bottom-[56%] h-[78%] w-[58%] rounded-full bg-fuchsia-500/50 blur-[80px] [--flow-delay:-5s] [--flow-direction:alternate-reverse] [--flow-duration:27s]' />
            <div className='absolute inset-0 bg-[radial-gradient(ellipse_62%_64%_at_50%_46%,rgba(8,9,12,0.8),transparent_78%)]' />
            <div className='preset-two-frost absolute inset-0 opacity-70 mix-blend-overlay' />
          </div>
          <p className='text-[11px] font-medium tracking-[0.3em] text-white/45 uppercase'>
            {props.brandName}
          </p>
          <h2 className='mx-auto mt-6 max-w-3xl text-4xl leading-[1.05] font-semibold tracking-[-0.035em] text-balance text-white sm:text-6xl sm:break-keep'>
            <span className='inline-block'>
              {t('Your models. Your rules.')}
            </span>{' '}
            <span className='preset-two-accent preset-two-gradient-text -mb-[0.2em] inline-block pb-[0.2em]'>
              {t('One gateway.')}
            </span>
          </h2>
          <p className='mx-auto mt-6 max-w-xl text-base leading-relaxed text-white/55'>
            {t(
              'Sign up, create a key, and make your first request in minutes, with every model behind the same endpoint.'
            )}
          </p>
          <PresetTwoActions
            className='mt-10 justify-center'
            isAuthenticated={props.isAuthenticated}
            docsUrl={props.docsUrl}
          />
        </div>
      </motion.div>
    </section>
  )
}
