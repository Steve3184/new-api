import { motion, useScroll, useSpring } from 'motion/react'
import { Fragment, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import { EASE_OUT, IN_VIEW, cascadeVariants, riseVariants } from './motion'

type SnippetLanguage = 'curl' | 'python' | 'node'

const SNIPPET_TABS: ReadonlyArray<[SnippetLanguage, string]> = [
  ['curl', 'cURL'],
  ['python', 'Python'],
  ['node', 'Node.js'],
]

const STEPS = [
  ['Create an API key', 'Issue a scoped key from the console in seconds.'],
  [
    'Point your SDK at the gateway',
    'Swap the base URL. Your existing OpenAI, Claude, or Gemini client keeps working.',
  ],
  [
    'Ship with confidence',
    'Routing, retries, and metering apply to every request automatically.',
  ],
] as const

function buildSnippet(language: SnippetLanguage, baseUrl: string): string {
  if (language === 'python') {
    return [
      'import os',
      'from openai import OpenAI',
      '',
      'client = OpenAI(',
      `    base_url="${baseUrl}/v1",`,
      '    api_key=os.environ["API_KEY"],',
      ')',
      '',
      'reply = client.chat.completions.create(',
      '    model="claude-sonnet-4-5",',
      '    messages=[{"role": "user", "content": "Hello"}],',
      ')',
    ].join('\n')
  }
  if (language === 'node') {
    return [
      "import OpenAI from 'openai'",
      '',
      'const client = new OpenAI({',
      `  baseURL: '${baseUrl}/v1',`,
      '  apiKey: process.env.API_KEY,',
      '})',
      '',
      'const reply = await client.chat.completions.create({',
      "  model: 'gemini-2.5-pro',",
      "  messages: [{ role: 'user', content: 'Hello' }],",
      '})',
    ].join('\n')
  }
  return [
    `curl ${baseUrl}/v1/chat/completions \\`,
    '  -H "Authorization: Bearer $API_KEY" \\',
    '  -H "Content-Type: application/json" \\',
    "  -d '{",
    '    "model": "gpt-5",',
    '    "messages": [{"role": "user", "content": "Hello"}]',
    "  }'",
  ].join('\n')
}

type IntegrationSectionProps = {
  baseUrl: string
  reduced: boolean
}

export function IntegrationSection(props: IntegrationSectionProps) {
  const { t } = useTranslation()
  const [language, setLanguage] = useState<SnippetLanguage>('curl')
  const stepsRef = useRef<HTMLOListElement>(null)
  const { scrollYProgress } = useScroll({
    target: stepsRef,
    offset: ['start 80%', 'end 55%'],
  })
  const progress = useSpring(scrollYProgress, { stiffness: 120, damping: 30 })
  const snippet = buildSnippet(language, props.baseUrl)
  // Odd split segments are the quoted string literals we tint.
  let tokenOffset = 0
  const snippetTokens = snippet
    .split(/("[^"\n]*"|'[^'\n]*')/)
    .map((text, index) => {
      const token = { text, quoted: index % 2 === 1, offset: tokenOffset }
      tokenOffset += text.length
      return token
    })
  const rise = riseVariants(props.reduced)

  return (
    <section className='relative px-5 py-28 sm:px-8 lg:py-36'>
      <motion.div
        className='mx-auto grid max-w-6xl items-center gap-16 lg:grid-cols-[0.85fr_1.15fr] lg:gap-20'
        variants={cascadeVariants(props.reduced)}
        initial='hidden'
        whileInView='visible'
        viewport={IN_VIEW}
      >
        <motion.div variants={rise}>
          <p className='text-[11px] font-medium tracking-[0.3em] text-violet-200/70 uppercase'>
            {t('Drop-in integration')}
          </p>
          <h2 className='mt-5 text-4xl leading-[1.05] font-semibold tracking-[-0.03em] text-balance text-white sm:text-5xl sm:break-keep'>
            <span className='inline-block'>{t('Change one line.')}</span>{' '}
            <span className='preset-two-accent inline-block text-white/60'>
              {t('Keep everything else.')}
            </span>
          </h2>

          <ol ref={stepsRef} className='relative mt-12 space-y-9 pl-10'>
            <span
              aria-hidden='true'
              className='absolute top-1.5 bottom-1.5 left-[11px] w-px bg-white/10'
            />
            <motion.span
              aria-hidden='true'
              className='absolute top-1.5 bottom-1.5 left-[11px] w-px origin-top bg-gradient-to-b from-violet-200 to-cyan-200'
              style={{ scaleY: props.reduced ? 1 : progress }}
            />
            {STEPS.map(([title, description], index) => (
              <li key={title} className='relative'>
                <span className='absolute top-0 -left-10 flex size-6 items-center justify-center rounded-full border border-white/15 bg-[#050607] font-mono text-[10px] text-white/70'>
                  {index + 1}
                </span>
                <h3 className='text-base font-semibold text-white'>
                  {t(title)}
                </h3>
                <p className='mt-1.5 text-sm leading-relaxed text-white/50'>
                  {t(description)}
                </p>
              </li>
            ))}
          </ol>
        </motion.div>

        <motion.div
          variants={rise}
          className='relative overflow-hidden rounded-3xl border border-white/10 bg-[#0a0b0e]/90 shadow-[0_40px_120px_-50px_rgba(167,139,250,0.45)]'
        >
          <div className='flex items-center justify-between gap-3 border-b border-white/[0.07] px-4 py-3'>
            <Tabs
              value={language}
              onValueChange={(value) => setLanguage(value as SnippetLanguage)}
            >
              <TabsList className='h-8 bg-white/[0.04]'>
                {SNIPPET_TABS.map(([value, label]) => (
                  <TabsTrigger
                    key={value}
                    value={value}
                    className='px-3 text-xs text-white/50 hover:text-white data-active:bg-white/10 data-active:text-white dark:text-white/50 dark:hover:text-white dark:data-active:border-white/10 dark:data-active:bg-white/10 dark:data-active:text-white'
                  >
                    {label}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>
            <CopyButton
              value={snippet}
              className='size-8 rounded-lg text-white/50 hover:bg-white/10 hover:text-white'
              iconClassName='size-3.5'
            />
          </div>
          <motion.pre
            key={language}
            initial={props.reduced ? false : { opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EASE_OUT }}
            className='min-h-[19rem] overflow-x-auto p-6 font-mono text-[12.5px] leading-7 text-white/80'
          >
            <code>
              {snippetTokens.map((token) =>
                token.quoted ? (
                  <span key={token.offset} className='text-violet-200'>
                    {token.text}
                  </span>
                ) : (
                  <Fragment key={token.offset}>{token.text}</Fragment>
                )
              )}
            </code>
          </motion.pre>
          <div className='flex items-center justify-between border-t border-white/[0.07] px-6 py-3 font-mono text-[11px] text-white/35'>
            <span className='truncate'>{props.baseUrl}/v1</span>
            <span className='text-emerald-300/80'>200 OK</span>
          </div>
        </motion.div>
      </motion.div>
    </section>
  )
}
