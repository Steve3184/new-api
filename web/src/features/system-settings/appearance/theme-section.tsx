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
import { zodResolver } from '@hookform/resolvers/zod'
import type { ReactNode } from 'react'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  ContentWidthPicker,
  DirectionPicker,
  FontPicker,
  PresetPicker,
  RadiusPicker,
  ScalePicker,
  SidebarLayoutPicker,
  SidebarVariantPicker,
  ThemePicker,
} from '@/components/appearance-pickers'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import type { ThemePreset } from '@/lib/theme-customization'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const themeSchema = z.object({
  defaultTheme: z.enum(['system', 'light', 'dark']),
  defaultThemeOverride: z.enum(['none', 'light', 'dark']),
  defaultThemePreset: z.string(),
  defaultThemeFont: z.enum(['default', 'sans', 'serif']),
  defaultThemeRadius: z.enum(['default', 'none', 'sm', 'md', 'lg', 'xl']),
  defaultThemeScale: z.enum(['default', 'sm', 'lg', 'xl']),
  defaultSidebarVariant: z.enum(['inset', 'floating', 'sidebar']),
  defaultSidebarLayout: z.enum(['expanded', 'icon', 'offcanvas']),
  defaultContentLayout: z.enum(['full', 'centered']),
  defaultDirection: z.enum(['ltr', 'rtl']),
})

export type ThemeFormValues = z.infer<typeof themeSchema>

const OPTION_KEYS: Record<keyof ThemeFormValues, string> = {
  defaultTheme: 'console_setting.default_theme',
  defaultThemeOverride: 'console_setting.default_theme_override',
  defaultThemePreset: 'console_setting.default_theme_preset',
  defaultThemeFont: 'console_setting.default_theme_font',
  defaultThemeRadius: 'console_setting.default_theme_radius',
  defaultThemeScale: 'console_setting.default_theme_scale',
  defaultSidebarVariant: 'console_setting.default_sidebar_variant',
  defaultSidebarLayout: 'console_setting.default_sidebar_layout',
  defaultContentLayout: 'console_setting.default_content_layout',
  defaultDirection: 'console_setting.default_direction',
}

const OVERRIDE_LABELS = {
  none: 'No override',
  light: 'Force light mode',
  dark: 'Force dark mode',
} as const

type PickerField = {
  name: keyof ThemeFormValues
  label: string
  render: (value: string, onChange: (value: string) => void) => ReactNode
}

export function ThemeSection(props: { defaultValues: ThemeFormValues }) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<ThemeFormValues>({
      resolver: zodResolver(themeSchema) as Resolver<ThemeFormValues>,
      defaultValues: props.defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [name, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key: OPTION_KEYS[name as keyof ThemeFormValues],
            value: String(value),
          })
        }
      },
    })

  const preset = form.watch('defaultThemePreset') as ThemePreset
  const pickers: PickerField[] = [
    {
      name: 'defaultTheme',
      label: 'Theme',
      render: (value, onChange) => (
        <ThemePicker value={value} onChange={onChange} />
      ),
    },
    {
      name: 'defaultThemePreset',
      label: 'Color preset',
      render: (value, onChange) => (
        <PresetPicker value={value} onChange={onChange} />
      ),
    },
    {
      name: 'defaultThemeFont',
      label: 'Font',
      render: (value, onChange) => (
        <FontPicker value={value} onChange={onChange} preset={preset} />
      ),
    },
    {
      name: 'defaultThemeRadius',
      label: 'Border radius',
      render: (value, onChange) => (
        <RadiusPicker value={value} onChange={onChange} />
      ),
    },
    {
      name: 'defaultThemeScale',
      label: 'Density',
      render: (value, onChange) => (
        <ScalePicker value={value} onChange={onChange} />
      ),
    },
    {
      name: 'defaultSidebarVariant',
      label: 'Sidebar',
      render: (value, onChange) => (
        <SidebarVariantPicker value={value} onChange={onChange} />
      ),
    },
    {
      // The drawer calls the expanded layout `default`.
      name: 'defaultSidebarLayout',
      label: 'Layout',
      render: (value, onChange) => (
        <SidebarLayoutPicker
          value={value === 'expanded' ? 'default' : value}
          onChange={(next) => onChange(next === 'default' ? 'expanded' : next)}
        />
      ),
    },
    {
      name: 'defaultContentLayout',
      label: 'Content width',
      render: (value, onChange) => (
        <ContentWidthPicker value={value} onChange={onChange} />
      ),
    },
    {
      name: 'defaultDirection',
      label: 'Direction',
      render: (value, onChange) => (
        <DirectionPicker value={value} onChange={onChange} />
      ),
    },
  ]

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Theme')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />
            <SettingsFormGrid>
              {pickers.map((picker) => (
                <SettingsFormGridItem key={picker.name} span='full'>
                  <FormField
                    control={form.control}
                    name={picker.name}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t(picker.label)}</FormLabel>
                        {picker.render(String(field.value), field.onChange)}
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGridItem>
              ))}
              <FormField
                control={form.control}
                name='defaultThemeOverride'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Theme override')}</FormLabel>
                    <FormControl>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue>
                            {t(OVERRIDE_LABELS[field.value])}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {Object.entries(OVERRIDE_LABELS).map(
                              ([value, label]) => (
                                <SelectItem key={value} value={value}>
                                  {t(label)}
                                </SelectItem>
                              )
                            )}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
