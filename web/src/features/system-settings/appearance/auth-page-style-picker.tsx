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

import { PickerGroup, SwatchTile } from '@/components/appearance-pickers'

export type AuthPageStyle = 'centered' | 'split'

/** Mini page mocks for the two sign-in layouts. */
function CenteredPreview() {
  return (
    <div
      aria-hidden='true'
      className='absolute inset-2 flex items-center justify-center'
    >
      <div className='border-border/60 bg-background flex h-8 w-1/2 flex-col justify-center gap-1 rounded-sm border px-1.5'>
        <span className='bg-foreground/50 block h-[2px] w-full rounded-full' />
        <span className='bg-foreground/30 block h-[2px] w-2/3 rounded-full' />
      </div>
    </div>
  )
}

function SplitPreview() {
  return (
    <div aria-hidden='true' className='absolute inset-2 flex gap-1'>
      <div className='bg-chart-1/40 flex w-1/2 flex-col justify-between rounded-sm p-1'>
        <span className='bg-foreground/40 block h-[2px] w-1/2 rounded-full' />
        <span className='bg-foreground/25 block h-[2px] w-2/3 rounded-full' />
      </div>
      <div className='flex w-1/2 flex-col justify-center gap-1 px-1'>
        <span className='bg-foreground/50 block h-[2px] w-full rounded-full' />
        <span className='bg-foreground/30 block h-[2px] w-3/4 rounded-full' />
        <span className='bg-foreground/20 block h-2 w-full rounded-sm' />
      </div>
    </div>
  )
}

export function AuthPageStylePicker(props: {
  value: string
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const options = [
    { value: 'centered', label: t('Centered'), preview: CenteredPreview },
    { value: 'split', label: t('Split'), preview: SplitPreview },
  ]
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      label={t('Select login page style')}
      className='max-w-md grid-cols-2 gap-3'
    >
      {options.map((option) => (
        <SwatchTile
          key={option.value}
          value={option.value}
          label={option.label}
        >
          <option.preview />
        </SwatchTile>
      ))}
    </PickerGroup>
  )
}
