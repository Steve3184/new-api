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
          <div aria-hidden='true' className='absolute inset-0 -z-10'>
            <div className='preset-two-aurora absolute -bottom-1/2 left-1/2 h-[30rem] w-[52rem] -translate-x-1/2 rounded-full bg-violet-500/25 blur-[120px]' />
            <div className='absolute inset-0 bg-[radial-gradient(rgba(255,255,255,0.08)_1px,transparent_1px)] [mask-image:radial-gradient(ellipse_60%_60%_at_50%_100%,#000,transparent)] bg-[size:22px_22px]' />
          </div>
          <p className='text-[11px] font-medium tracking-[0.3em] text-white/45 uppercase'>
            {props.brandName}
          </p>
          <h2 className='mx-auto mt-6 max-w-3xl text-4xl leading-[1.05] font-semibold tracking-[-0.035em] text-balance text-white sm:text-6xl sm:break-keep'>
            <span className='inline-block'>
              {t('Your models. Your rules.')}
            </span>{' '}
            <span className='preset-two-accent preset-two-gradient-text inline-block'>
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
