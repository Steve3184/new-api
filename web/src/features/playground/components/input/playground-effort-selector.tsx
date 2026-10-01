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
import { BrainIcon, ChevronDownIcon, RotateCcwIcon } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Popover,
  PopoverContent,
  PopoverDescription,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover'
import { Slider } from '@/components/ui/slider'
import { cn } from '@/lib/utils'

import { REASONING_EFFORTS } from '../../constants'
import type { PlaygroundConfig } from '../../types'

const EFFORT_STEPS = REASONING_EFFORTS

type PlaygroundEffortSelectorProps = {
  value: PlaygroundConfig['reasoning_effort']
  onChange: (value: PlaygroundConfig['reasoning_effort']) => void
  disabled?: boolean
}

export function PlaygroundEffortSelector(props: PlaygroundEffortSelectorProps) {
  const { t } = useTranslation()
  const isDefault = props.value === 'medium'
  const label = props.value
  const selectedIndex = EFFORT_STEPS.indexOf(props.value)

  return (
    <Popover>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='ghost'
            size='sm'
            disabled={props.disabled}
            aria-label={t('Reasoning effort')}
            className={cn(
              'h-8 gap-1.5 rounded-full px-2.5 text-xs',
              isDefault
                ? 'text-muted-foreground'
                : 'bg-primary/10 text-primary hover:bg-primary/15'
            )}
          />
        }
      >
        <BrainIcon aria-hidden='true' className='size-4' />
        <span className='max-w-20 truncate font-medium'>{label}</span>
        <ChevronDownIcon aria-hidden='true' className='size-3 opacity-60' />
      </PopoverTrigger>
      <PopoverContent
        side='top'
        align='center'
        sideOffset={10}
        collisionPadding={12}
        className='border-border/50 w-72 gap-3 rounded-2xl border p-4 shadow-xl'
      >
        <div className='relative flex flex-col items-center gap-1 px-7'>
          <PopoverTitle className='text-primary text-sm font-semibold'>
            {label}
          </PopoverTitle>
          <span className='text-muted-foreground text-xs'>
            {t('Reasoning effort')}
          </span>
          <Button
            type='button'
            size='icon'
            variant='ghost'
            aria-label={t('Reset to medium')}
            title={t('Reset to medium')}
            disabled={props.disabled || isDefault}
            onClick={() => props.onChange('medium')}
            className='text-muted-foreground absolute -top-1 -right-1 size-7 rounded-full'
          >
            <RotateCcwIcon aria-hidden='true' className='size-3.5' />
          </Button>
        </div>
        <div>
          <div className='relative'>
            <Slider
              min={0}
              max={EFFORT_STEPS.length - 1}
              step={1}
              largeStep={1}
              value={[selectedIndex]}
              disabled={props.disabled}
              thumbProps={{
                getAriaLabel: () => t('Reasoning effort'),
                getAriaValueText: () => label,
              }}
              onValueChange={(value) => {
                const index = Array.isArray(value) ? value[0] : value
                props.onChange(EFFORT_STEPS[index])
              }}
              className='[&_[data-slot=slider-track]]:bg-muted-foreground/15 py-1 [&_[data-slot=slider-thumb]]:size-7 [&_[data-slot=slider-thumb]]:border-0 [&_[data-slot=slider-thumb]]:shadow-md [&_[data-slot=slider-track]]:h-6'
            />
            <div
              aria-hidden='true'
              className='pointer-events-none absolute inset-x-3.5 top-1/2 flex -translate-y-1/2 justify-between'
            >
              {EFFORT_STEPS.map((effort, index) => (
                <span
                  key={effort}
                  className={cn(
                    'size-1 rounded-full',
                    index <= selectedIndex
                      ? 'bg-primary-foreground/60'
                      : 'bg-muted-foreground/40',
                    index === selectedIndex && 'invisible'
                  )}
                />
              ))}
            </div>
          </div>
        </div>
        <PopoverDescription className='text-center text-[11px] leading-relaxed'>
          {t('Available effort levels depend on the model.')}
        </PopoverDescription>
      </PopoverContent>
    </Popover>
  )
}
