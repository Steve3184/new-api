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

import {
  resolveThemeFont,
  THEME_PRESETS,
  type ThemePreset,
} from '@/lib/theme-customization'

import { PickerGroup, SwatchTile, type PickerProps } from './picker-tile'

const DEFAULT_PRESET_GRADIENT =
  'linear-gradient(135deg, oklch(0.68 0.2 25) 0%, oklch(0.8 0.17 85) 25%, oklch(0.72 0.18 155) 50%, oklch(0.66 0.19 245) 75%, oklch(0.68 0.2 315) 100%)'

export function PresetPicker(props: PickerProps) {
  const { t } = useTranslation()
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      label={t('Select color preset')}
      className='grid-cols-4 gap-3'
    >
      {THEME_PRESETS.map((preset) => (
        <SwatchTile
          key={preset.value}
          value={preset.value}
          label={t(`preset.${preset.value}`)}
          truncateLabel
        >
          <div
            aria-hidden='true'
            className='absolute inset-0 rounded-md'
            style={{
              background:
                preset.value === 'default'
                  ? DEFAULT_PRESET_GRADIENT
                  : `linear-gradient(135deg, ${preset.swatches[0]} 0%, ${preset.swatches[1] ?? preset.swatches[0]} 100%)`,
            }}
          />
        </SwatchTile>
      ))}
    </PickerGroup>
  )
}

// Auto previews the font that the active preset resolves to.
const FONT_OPTIONS = [
  { value: 'default', label: 'Auto', family: undefined },
  { value: 'sans', label: 'Sans', family: 'var(--font-public-sans)' },
  { value: 'serif', label: 'Serif', family: 'var(--font-serif)' },
]

export function FontPicker(props: PickerProps & { preset: ThemePreset }) {
  const { t } = useTranslation()
  const autoFamily =
    resolveThemeFont('default', props.preset) === 'serif'
      ? 'var(--font-serif)'
      : 'sans-serif'
  return (
    <PickerGroup
      value={props.value}
      onValueChange={props.onChange}
      disabled={props.disabled}
      label={t('Select body font')}
      className='grid-cols-3 gap-4'
    >
      {FONT_OPTIONS.map((option) => (
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
            className='text-foreground absolute inset-0 flex items-center justify-center text-lg leading-none font-medium'
            style={{ fontFamily: option.family ?? autoFamily }}
          >
            Aa
          </span>
        </SwatchTile>
      ))}
    </PickerGroup>
  )
}
