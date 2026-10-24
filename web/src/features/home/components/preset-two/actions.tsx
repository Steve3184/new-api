import { Link } from '@tanstack/react-router'
import { ArrowRight, BookOpen } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

type PresetTwoActionsProps = {
  isAuthenticated: boolean
  docsUrl: string
  className?: string
}

const GHOST_BUTTON_CLASS =
  'h-12 rounded-full border-white/12 bg-white/[0.03] px-6 text-sm font-medium text-white/80 backdrop-blur-md hover:border-white/30 hover:bg-white/[0.08] hover:text-white dark:border-white/12 dark:bg-white/[0.03] dark:hover:bg-white/[0.08]'

/** Primary, pricing, and docs entry points shared by the hero and closing CTA. */
export function PresetTwoActions(props: PresetTwoActionsProps) {
  const { t } = useTranslation()
  const isExternalDocs = props.docsUrl.startsWith('http')

  return (
    <div className={cn('flex flex-wrap items-center gap-3', props.className)}>
      <Button
        className='group relative h-12 overflow-hidden rounded-full bg-white px-7 text-sm font-semibold text-zinc-950 shadow-[0_0_0_1px_rgba(255,255,255,0.4),0_12px_40px_-12px_rgba(233,213,255,0.65)] hover:bg-white [a]:hover:bg-white'
        render={<Link to={props.isAuthenticated ? '/dashboard' : '/sign-up'} />}
      >
        <span
          aria-hidden='true'
          className='preset-two-shine pointer-events-none absolute inset-y-0 left-0 w-1/3 bg-gradient-to-r from-transparent via-violet-200/70 to-transparent'
        />
        <span className='relative'>
          {t(props.isAuthenticated ? 'Go to Dashboard' : 'Get Started')}
        </span>
        <ArrowRight className='relative ml-1.5 size-4 transition-transform duration-300 group-hover:translate-x-1' />
      </Button>
      <Button
        variant='outline'
        className={GHOST_BUTTON_CLASS}
        render={<Link to='/pricing' />}
      >
        {t('View Pricing')}
      </Button>
      <Button
        variant='ghost'
        className='group h-12 rounded-full px-4 text-sm font-medium text-white/60 hover:bg-transparent hover:text-white dark:hover:bg-transparent'
        render={
          isExternalDocs ? (
            <a href={props.docsUrl} target='_blank' rel='noopener noreferrer' />
          ) : (
            <Link to={props.docsUrl} />
          )
        }
      >
        <BookOpen className='mr-1.5 size-4 opacity-70 transition-opacity group-hover:opacity-100' />
        {t('Docs')}
      </Button>
    </div>
  )
}
