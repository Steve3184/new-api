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
import { Radio as RadioPrimitive } from '@base-ui/react/radio'
import { RadioGroup } from '@base-ui/react/radio-group'
import { CircleCheck } from 'lucide-react'
import { useId, type ReactElement, type ReactNode, type SVGProps } from 'react'

import { cn } from '@/lib/utils'

export type PickerIcon = (props: SVGProps<SVGSVGElement>) => ReactElement

/** Props shared by every appearance picker; the caller owns the value. */
export interface PickerProps {
  value: string
  onChange: (value: string) => void
  disabled?: boolean
  describedBy?: string
}

export function PickerGroup(props: {
  value: string
  onValueChange: (value: string) => void
  /** Accessible name of the whole group. */
  label: string
  /** Grid layout classes, for example `max-w-md grid-cols-3 gap-4`. */
  className: string
  disabled?: boolean
  describedBy?: string
  children: ReactNode
}) {
  return (
    <RadioGroup
      value={props.value}
      onValueChange={(value) => props.onValueChange(String(value))}
      disabled={props.disabled}
      className={cn('grid w-full', props.className)}
      aria-label={props.label}
      aria-describedby={props.describedBy}
    >
      {props.children}
    </RadioGroup>
  )
}

function CheckMark(props: { className: string }) {
  return (
    <CircleCheck
      className={cn(
        'fill-primary absolute top-0 right-0 translate-x-1/2 -translate-y-1/2 stroke-white',
        'group-data-unchecked:hidden',
        props.className
      )}
      aria-hidden='true'
    />
  )
}

/** A 48px-high tile whose preview is passed in as children. */
export function SwatchTile(props: {
  value: string
  label: string
  ariaLabel?: string
  truncateLabel?: boolean
  children: ReactNode
}) {
  return (
    <RadioPrimitive.Root
      value={props.value}
      className='group flex flex-col items-stretch outline-none'
      aria-label={props.ariaLabel ?? props.label}
    >
      <div
        className={cn(
          'ring-border relative h-12 rounded-md ring-[1px] transition',
          'group-data-checked:ring-primary group-data-checked:shadow-md',
          'group-focus-visible:ring-2',
          'group-hover:ring-primary/60'
        )}
      >
        <CheckMark className='z-10 size-5' />
        {props.children}
      </div>
      <div
        className={cn(
          'mt-1.5 text-center text-xs',
          props.truncateLabel && 'truncate'
        )}
      >
        {props.label}
      </div>
    </RadioPrimitive.Root>
  )
}

/** A tile that previews its option with an SVG illustration. */
export function IconTile(props: {
  value: string
  label: string
  icon: PickerIcon
  /** Theme previews carry their own colors, so they are not tinted. */
  plain?: boolean
}) {
  const Icon = props.icon
  const descriptionId = useId()
  return (
    <RadioPrimitive.Root
      value={props.value}
      className={cn('group outline-none', 'transition duration-200 ease-in')}
      aria-label={`Select ${props.label.toLowerCase()}`}
      aria-describedby={descriptionId}
    >
      <div
        className={cn(
          'ring-border relative rounded-md ring-[1px]',
          'group-data-checked:ring-primary group-data-checked:shadow-2xl',
          'group-focus-visible:ring-2'
        )}
        role='img'
        aria-hidden='false'
        aria-label={`${props.label} option preview`}
      >
        <CheckMark className='size-6' />
        <Icon
          className={cn(
            !props.plain &&
              'stroke-primary fill-primary group-data-unchecked:stroke-muted-foreground group-data-unchecked:fill-muted-foreground'
          )}
          aria-hidden='true'
        />
      </div>
      <div className='mt-1 text-xs' id={descriptionId} aria-live='polite'>
        {props.label}
      </div>
    </RadioPrimitive.Root>
  )
}
