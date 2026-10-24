import { motion } from 'motion/react'
import { useTranslation } from 'react-i18next'

import { getLobeIcon } from '@/lib/lobe-icon'

import { IN_VIEW, riseVariants } from './motion'

const PROVIDERS = [
  ['OpenAI', 'OpenAI'],
  ['Claude.Color', 'Claude'],
  ['Gemini.Color', 'Gemini'],
  ['DeepSeek.Color', 'DeepSeek'],
  ['Qwen.Color', 'Qwen'],
  ['Doubao.Color', 'Doubao'],
  ['Moonshot', 'Moonshot'],
  ['Zhipu.Color', 'Zhipu'],
  ['Grok', 'Grok'],
  ['Mistral.Color', 'Mistral'],
  ['Minimax.Color', 'MiniMax'],
  ['Azure.Color', 'Azure'],
  ['Bedrock.Color', 'Bedrock'],
] as const

export function ProviderMarquee(props: { reduced: boolean }) {
  const { t } = useTranslation()

  return (
    <motion.section
      aria-label={t('Supported providers')}
      className='relative border-y border-white/[0.06] py-12'
      variants={riseVariants(props.reduced)}
      initial='hidden'
      whileInView='visible'
      viewport={IN_VIEW}
    >
      <p className='mb-8 text-center text-[11px] font-medium tracking-[0.3em] text-white/40 uppercase'>
        {t('One contract across the providers you already use')}
      </p>
      <div className='preset-two-marquee-host relative overflow-hidden [mask-image:linear-gradient(to_right,transparent,#000_12%,#000_88%,transparent)]'>
        <div className='preset-two-marquee flex w-max gap-4'>
          {[0, 1].map((copy) => (
            <div
              key={copy}
              aria-hidden={copy === 1 ? 'true' : undefined}
              className='flex gap-4'
            >
              {PROVIDERS.map(([icon, name]) => (
                <div
                  key={name}
                  className='flex items-center gap-3 rounded-2xl border border-white/[0.07] bg-white/[0.025] px-5 py-3.5 text-sm text-white/65 transition-colors duration-300 hover:border-white/20 hover:text-white'
                >
                  <span className='text-white'>{getLobeIcon(icon, 20)}</span>
                  {name}
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </motion.section>
  )
}
