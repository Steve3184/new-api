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
import { useTranslation } from 'react-i18next'

import { cn } from '@/lib/utils'

import { PickerGroup, SwatchTile, type PickerProps } from './picker-tile'

// `corner` is the CSS border-radius used to draw the preview corner.
const RADIUS_OPTIONS = [
  { value: 'default', label: 'Auto', corner: '1rem' },
  { value: 'none', label: '0', corner: '0' },
  { value: 'sm', label: '0.3', corner: '0.3rem' },
  { value: 'md', label: '0.5', corner: '0.5rem' },
  { value: 'lg', label: '0.75', corner: '0.75rem' },
  { value: 'xl', label: '1.0', corner: '1rem' },
]

export function RadiusPicker(props: PickerProps) {
  const { t } = useTranslation()
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      label={t('Select border radius')}
      className='grid-cols-6 gap-2'
    >
      {RADIUS_OPTIONS.map((option) => (
        <SwatchTile
          key={option.value}
          value={option.value}
          label={option.label}
          ariaLabel={
            option.value === 'default' ? t('System default') : option.label
          }
        >
          <span
            aria-hidden='true'
            className='border-foreground/70 absolute top-2.5 left-2.5 size-3.5 border-t-[1.5px] border-l-[1.5px]'
            style={{ borderTopLeftRadius: option.corner }}
          />
        </SwatchTile>
      ))}
    </PickerGroup>
  )
}

/** Rows drawn tighter or looser to show the relative density. */
function ScalePreview(props: { rows: number; rowGap: string }) {
  return (
    <div
      aria-hidden='true'
      className='absolute inset-2.5 flex flex-col justify-center'
      style={{ gap: props.rowGap }}
    >
      {Array.from({ length: props.rows }, (_, index) => 85 - index * 10).map(
        (width) => (
          <span
            key={width}
            className='bg-foreground/60 block h-[2px] rounded-full'
            style={{ width: `${width}%` }}
          />
        )
      )}
    </div>
  )
}

export function ScalePicker(props: PickerProps) {
  const { t } = useTranslation()
  const options = [
    { value: 'sm', label: t('Compact'), rows: 4, rowGap: '3px' },
    { value: 'default', label: t('Default'), rows: 3, rowGap: '6px' },
    { value: 'lg', label: t('Comfortable'), rows: 2, rowGap: '10px' },
    { value: 'xl', label: t('Super Large'), rows: 1, rowGap: '14px' },
  ]
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      label={t('Select interface density')}
      className='grid-cols-4 gap-3'
    >
      {options.map((option) => (
        <SwatchTile
          key={option.value}
          value={option.value}
          label={option.label}
          truncateLabel
        >
          <ScalePreview rows={option.rows} rowGap={option.rowGap} />
        </SwatchTile>
      ))}
    </PickerGroup>
  )
}

/** Mini page mock: `full` fills the width, `centered` clamps the body. */
function ContentLayoutPreview(props: { centered: boolean }) {
  return (
    <div aria-hidden='true' className='absolute inset-2 flex flex-col gap-1.5'>
      <span className='bg-foreground/40 block h-1.5 w-full rounded-sm' />
      <div
        className={cn(
          'flex flex-1 flex-col gap-1',
          props.centered ? 'mx-auto w-1/2' : 'w-full'
        )}
      >
        <span className='bg-foreground/60 block h-[2px] w-full rounded-full' />
        <span className='bg-foreground/60 block h-[2px] w-3/4 rounded-full' />
      </div>
    </div>
  )
}

export function ContentWidthPicker(props: PickerProps) {
  const { t } = useTranslation()
  const options = [
    { value: 'full', label: t('Full width') },
    { value: 'centered', label: t('Centered') },
  ]
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      label={t('Select content width')}
      className='grid-cols-2 gap-4'
    >
      {options.map((option) => (
        <SwatchTile
          key={option.value}
          value={option.value}
          label={option.label}
          truncateLabel
        >
          <ContentLayoutPreview centered={option.value === 'centered'} />
        </SwatchTile>
      ))}
    </PickerGroup>
  )
}
