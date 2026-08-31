/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Route } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { getLobeIcon } from '@/lib/lobe-icon'
import { cn } from '@/lib/utils'

interface ModelBadgeProps {
  modelName: string
  actualModel?: string
  className?: string
  modelIcon?: string
  providerIcon?: string
}

function ModelBadgeContent(props: ModelBadgeProps) {
  const configuredIcon = props.modelIcon
    ? getLobeIcon(props.modelIcon, 18)
    : null
  const providerIcon = props.providerIcon
    ? getLobeIcon(props.providerIcon, 18)
    : null
  const displayIcon = configuredIcon || providerIcon

  return (
    <StatusBadge
      copyText={props.modelName}
      size='sm'
      showDot={!displayIcon}
      autoColor={!displayIcon ? props.modelName : undefined}
      className={cn(
        'border-border/60 bg-muted/30 h-6 max-w-none gap-1.5 rounded-md border px-2 [font-family:var(--font-body)]',
        displayIcon && 'text-foreground',
        props.className
      )}
    >
      <span className='flex max-w-none items-center gap-1.5'>
        {displayIcon && (
          <span
            className='flex h-[18px] w-[18px] shrink-0 items-center justify-center'
            title={props.modelIcon || props.providerIcon || props.modelName}
            aria-label={
              props.modelIcon || props.providerIcon || props.modelName
            }
          >
            {displayIcon}
          </span>
        )}
        <span className='whitespace-nowrap'>{props.modelName}</span>
      </span>
    </StatusBadge>
  )
}

export function ModelBadge(props: ModelBadgeProps) {
  const { t } = useTranslation()

  if (!props.actualModel) {
    return <ModelBadgeContent {...props} />
  }

  return (
    <Popover>
      <PopoverTrigger
        render={
          <button type='button' className='inline-flex items-center gap-1' />
        }
      >
        <ModelBadgeContent {...props} />
        <Route className='text-muted-foreground size-3 shrink-0' />
      </PopoverTrigger>
      <PopoverContent className='w-72'>
        <div className='space-y-2'>
          <div className='flex items-start justify-between gap-3'>
            <span className='text-muted-foreground text-xs'>
              {t('Request Model:')}
            </span>
            <span className='truncate font-mono text-xs font-medium'>
              {props.modelName}
            </span>
          </div>
          <div className='flex items-start justify-between gap-3'>
            <span className='text-muted-foreground text-xs'>
              {t('Actual Model:')}
            </span>
            <span className='truncate font-mono text-xs font-medium'>
              {props.actualModel}
            </span>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}
