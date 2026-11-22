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

import { IconDir } from '@/assets/custom/icon-dir'
import { IconLayoutCompact } from '@/assets/custom/icon-layout-compact'
import { IconLayoutDefault } from '@/assets/custom/icon-layout-default'
import { IconLayoutFull } from '@/assets/custom/icon-layout-full'
import { IconSidebarFloating } from '@/assets/custom/icon-sidebar-floating'
import { IconSidebarInset } from '@/assets/custom/icon-sidebar-inset'
import { IconSidebarSidebar } from '@/assets/custom/icon-sidebar-sidebar'
import { IconThemeDark } from '@/assets/custom/icon-theme-dark'
import { IconThemeLight } from '@/assets/custom/icon-theme-light'
import { IconThemeSystem } from '@/assets/custom/icon-theme-system'

import {
  IconTile,
  PickerGroup,
  type PickerIcon,
  type PickerProps,
} from './picker-tile'

const LtrIcon: PickerIcon = (props) => <IconDir dir='ltr' {...props} />
const RtlIcon: PickerIcon = (props) => <IconDir dir='rtl' {...props} />

export function ThemePicker(props: PickerProps) {
  const { t } = useTranslation()
  const options = [
    { value: 'system', label: t('System'), icon: IconThemeSystem },
    { value: 'light', label: t('Light'), icon: IconThemeLight },
    { value: 'dark', label: t('Dark'), icon: IconThemeDark },
  ]
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      describedBy={props.describedBy}
      label={t('Select theme preference')}
      className='max-w-md grid-cols-3 gap-4'
    >
      {options.map((option) => (
        <IconTile key={option.value} {...option} plain />
      ))}
    </PickerGroup>
  )
}

export function SidebarVariantPicker(props: PickerProps) {
  const { t } = useTranslation()
  const options = [
    { value: 'inset', label: t('Inset'), icon: IconSidebarInset },
    { value: 'floating', label: t('Floating'), icon: IconSidebarFloating },
    { value: 'sidebar', label: t('Sidebar'), icon: IconSidebarSidebar },
  ]
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      describedBy={props.describedBy}
      label={t('Select sidebar style')}
      className='max-w-md grid-cols-3 gap-4'
    >
      {options.map((option) => (
        <IconTile key={option.value} {...option} />
      ))}
    </PickerGroup>
  )
}

/** Values are `default`, `icon` and `offcanvas`. */
export function SidebarLayoutPicker(props: PickerProps) {
  const { t } = useTranslation()
  const options = [
    { value: 'default', label: t('Default'), icon: IconLayoutDefault },
    { value: 'icon', label: t('Compact'), icon: IconLayoutCompact },
    { value: 'offcanvas', label: t('Full layout'), icon: IconLayoutFull },
  ]
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      describedBy={props.describedBy}
      label={t('Select layout style')}
      className='max-w-md grid-cols-3 gap-4'
    >
      {options.map((option) => (
        <IconTile key={option.value} {...option} />
      ))}
    </PickerGroup>
  )
}

export function DirectionPicker(props: PickerProps) {
  const { t } = useTranslation()
  const options = [
    { value: 'ltr', label: t('Left to Right'), icon: LtrIcon },
    { value: 'rtl', label: t('Right to Left'), icon: RtlIcon },
  ]
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      describedBy={props.describedBy}
      label={t('Select site direction')}
      className='max-w-md grid-cols-3 gap-4'
    >
      {options.map((option) => (
        <IconTile key={option.value} {...option} />
      ))}
    </PickerGroup>
  )
}
