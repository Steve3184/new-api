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
import { BrainIcon, ChevronDownIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { cn } from '@/lib/utils'

import { REASONING_EFFORTS } from '../../constants'
import type { PlaygroundConfig } from '../../types'

type PlaygroundEffortSelectorProps = {
  value: PlaygroundConfig['reasoning_effort']
  onChange: (value: PlaygroundConfig['reasoning_effort']) => void
  disabled?: boolean
}

export function PlaygroundEffortSelector(props: PlaygroundEffortSelectorProps) {
  const { t } = useTranslation()
  const isDefault = props.value === 'default'

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='sm'
            disabled={props.disabled}
            aria-label={t('Reasoning effort')}
            className={cn(
              'h-8 gap-1.5 rounded-md px-2 text-xs',
              isDefault
                ? 'text-muted-foreground'
                : 'bg-primary/10 text-primary hover:bg-primary/15'
            )}
          />
        }
      >
        <BrainIcon aria-hidden='true' className='size-4' />
        <span className='max-w-20 truncate font-medium'>
          {isDefault ? t('Default') : props.value}
        </span>
        <ChevronDownIcon aria-hidden='true' className='size-3 opacity-60' />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        side='top'
        align='start'
        className='w-56 rounded-xl p-1.5'
      >
        <DropdownMenuGroup>
          <DropdownMenuLabel className='flex items-center gap-2 px-2 py-2'>
            <BrainIcon aria-hidden='true' className='text-primary size-4' />
            {t('Reasoning effort')}
          </DropdownMenuLabel>
          <p className='text-muted-foreground px-2 pb-2 text-xs leading-relaxed'>
            {t('Available effort levels depend on the model.')}
          </p>
          <DropdownMenuSeparator />
          <DropdownMenuRadioGroup
            value={props.value}
            onValueChange={(value) =>
              props.onChange(value as PlaygroundConfig['reasoning_effort'])
            }
          >
            <DropdownMenuRadioItem
              closeOnClick
              value='default'
              className='rounded-lg px-2 py-2'
            >
              {t('Default')}
            </DropdownMenuRadioItem>
            {REASONING_EFFORTS.map((effort, index) => (
              <DropdownMenuRadioItem
                closeOnClick
                key={effort}
                value={effort}
                className='data-checked:bg-primary/10 data-checked:text-primary rounded-lg px-2 py-2'
              >
                <span
                  aria-hidden='true'
                  className='flex h-4 w-8 items-end gap-0.5'
                >
                  {REASONING_EFFORTS.slice(1).map((level, bar) => (
                    <span
                      key={level}
                      className={cn(
                        'h-3 w-1 rounded-full',
                        bar < index ? 'bg-primary' : 'bg-muted-foreground/20'
                      )}
                    />
                  ))}
                </span>
                <span className='font-medium'>{effort}</span>
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
        </DropdownMenuGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
