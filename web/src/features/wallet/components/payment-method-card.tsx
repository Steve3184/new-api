import { Loader2 } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { cn } from '@/lib/utils'

import { getPaymentIcon } from '../lib/ui'

interface Props {
  name: string
  type: string
  icon?: string
  iconNode?: ReactNode
  selected?: boolean
  disabled?: boolean
  loading?: boolean
  description?: string
  disabledReason?: string
  onClick: () => void
}

export function PaymentMethodCard(props: Props) {
  const { t } = useTranslation()
  const name = t(props.name)
  const button = (
    <Button
      variant='outline'
      type='button'
      onClick={props.onClick}
      disabled={props.disabled || props.loading}
      title={props.disabledReason}
      aria-label={
        props.disabledReason ? `${name}. ${props.disabledReason}` : name
      }
      aria-pressed={props.selected}
      className={cn(
        'min-h-14 min-w-0 justify-start gap-2 rounded-lg px-3 py-2 text-left',
        props.selected && 'border-primary bg-primary/10'
      )}
    >
      {props.loading ? (
        <Loader2 className='h-4 w-4 animate-spin' />
      ) : (
        props.iconNode ||
        getPaymentIcon(props.type, 'h-4 w-4', props.icon, name)
      )}
      <span className='flex min-w-0 flex-col items-start gap-0.5'>
        <span className='max-w-full truncate'>{name}</span>
        {props.description && (
          <span className='text-muted-foreground max-w-full truncate text-[11px] leading-4 font-normal'>
            {props.description}
          </span>
        )}
      </span>
    </Button>
  )
  if (!props.disabledReason) return button
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger render={button} />
        <TooltipContent>{props.disabledReason}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
