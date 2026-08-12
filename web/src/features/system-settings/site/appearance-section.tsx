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
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { THEME_PRESETS } from '@/lib/theme-customization'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const appearanceSchema = z.object({
  backgroundImage: z.string(),
  backgroundBlurOpacity: z.coerce.number().min(0).max(100).default(40),
  defaultTheme: z.enum(['system', 'light', 'dark']),
  defaultThemePreset: z.string(),
  defaultThemeFont: z.enum(['default', 'sans', 'serif']),
  defaultThemeRadius: z.enum(['default', 'none', 'sm', 'md', 'lg', 'xl']),
  defaultThemeScale: z.enum(['default', 'sm', 'lg', 'xl']),
  defaultSidebarVariant: z.enum(['inset', 'floating', 'sidebar']),
  defaultSidebarLayout: z.enum(['expanded', 'icon', 'offcanvas']),
  defaultContentLayout: z.enum(['full', 'centered']),
  defaultDirection: z.enum(['ltr', 'rtl']),
  modelSquareDefaultView: z.enum(['card', 'table']),
  modelSquareCardPageSize: z.coerce.number().int().min(6).max(96),
  modelSquareTablePageSize: z.coerce.number().int().min(5).max(100),
})

export type AppearanceFormValues = z.infer<typeof appearanceSchema>

const OPTION_KEYS: Record<keyof AppearanceFormValues, string> = {
  backgroundImage: 'console_setting.background_image',
  backgroundBlurOpacity: 'console_setting.background_blur_opacity',
  defaultTheme: 'console_setting.default_theme',
  defaultThemePreset: 'console_setting.default_theme_preset',
  defaultThemeFont: 'console_setting.default_theme_font',
  defaultThemeRadius: 'console_setting.default_theme_radius',
  defaultThemeScale: 'console_setting.default_theme_scale',
  defaultSidebarVariant: 'console_setting.default_sidebar_variant',
  defaultSidebarLayout: 'console_setting.default_sidebar_layout',
  defaultContentLayout: 'console_setting.default_content_layout',
  defaultDirection: 'console_setting.default_direction',
  modelSquareDefaultView: 'console_setting.model_square_default_view',
  modelSquareCardPageSize: 'console_setting.model_square_card_page_size',
  modelSquareTablePageSize: 'console_setting.model_square_table_page_size',
}

type AppearanceSectionProps = { defaultValues: AppearanceFormValues }

export function AppearanceSection({ defaultValues }: AppearanceSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const schema = appearanceSchema.extend({
    backgroundImage: z
      .string()
      .refine(
        (value) =>
          !value ||
          (value.startsWith('/') && !value.startsWith('//')) ||
          /^https?:\/\//i.test(value),
        t('Use an HTTP(S) URL or a path starting with /')
      ),
    modelSquareCardPageSize: z.coerce
      .number()
      .int()
      .min(6)
      .max(96)
      .refine((value) => value % 6 === 0, t('Must be a multiple of 6')),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<AppearanceFormValues>({
      resolver: zodResolver(schema) as Resolver<AppearanceFormValues>,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [name, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key: OPTION_KEYS[name as keyof AppearanceFormValues],
            value: typeof value === 'number' ? value : String(value),
          })
        }
      },
    })

  const selectFields = [
    {
      name: 'defaultTheme',
      label: 'Default theme',
      options: [
        ['system', 'System'],
        ['light', 'Light'],
        ['dark', 'Dark'],
      ],
    },
    {
      name: 'defaultThemePreset',
      label: 'Default color preset',
      options: THEME_PRESETS.map((item) => [
        item.value,
        `preset.${item.value}`,
      ]),
    },
    {
      name: 'defaultThemeFont',
      label: 'Default font',
      options: [
        ['default', 'Auto'],
        ['sans', 'Sans'],
        ['serif', 'Serif'],
      ],
    },
    {
      name: 'defaultThemeRadius',
      label: 'Default border radius',
      options: [
        ['default', 'Auto'],
        ['none', 'None'],
        ['sm', 'Small'],
        ['md', 'Medium'],
        ['lg', 'Large'],
        ['xl', 'Extra large'],
      ],
    },
    {
      name: 'defaultThemeScale',
      label: 'Default density',
      options: [
        ['default', 'Default'],
        ['sm', 'Compact'],
        ['lg', 'Comfortable'],
        ['xl', 'Super Large'],
      ],
    },
    {
      name: 'defaultSidebarVariant',
      label: 'Default sidebar style',
      options: [
        ['inset', 'Inset'],
        ['floating', 'Floating'],
        ['sidebar', 'Sidebar'],
      ],
    },
    {
      name: 'defaultSidebarLayout',
      label: 'Default sidebar layout',
      options: [
        ['expanded', 'Expanded'],
        ['icon', 'Compact'],
        ['offcanvas', 'Full layout'],
      ],
    },
    {
      name: 'defaultContentLayout',
      label: 'Default content width',
      options: [
        ['full', 'Full width'],
        ['centered', 'Centered'],
      ],
    },
    {
      name: 'defaultDirection',
      label: 'Default direction',
      options: [
        ['ltr', 'Left to Right'],
        ['rtl', 'Right to Left'],
      ],
    },
    {
      name: 'modelSquareDefaultView',
      label: 'Default model square view',
      options: [
        ['card', 'Card'],
        ['table', 'Table'],
      ],
    },
  ] as const

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Appearance & Model Square')}>
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
              <FormField
                control={form.control}
                name='backgroundImage'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Background image')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='/_custom/img/background.webp'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Applied by the administrator and cannot be changed by users'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {selectFields.map((config) => (
                <FormField
                  key={config.name}
                  control={form.control}
                  name={config.name}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t(config.label)}</FormLabel>
                      <FormControl>
                        <Select
                          value={String(field.value)}
                          onValueChange={field.onChange}
                        >
                          <SelectTrigger className='w-full'>
                            <SelectValue placeholder={t(config.options.find(([v]) => v === field.value)?.[1] || String(field.value))} />
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            <SelectGroup>
                              {config.options.map(([value, label]) => (
                                <SelectItem key={value} value={value}>
                                  {t(label)}
                                </SelectItem>
                              ))}
                            </SelectGroup>
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ))}

              <FormField
                control={form.control}
                name='modelSquareCardPageSize'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Models per card page')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={6}
                        max={96}
                        step={6}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Use a multiple of 6')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='modelSquareTablePageSize'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Models per table page')}</FormLabel>
                    <FormControl>
                      <Input type='number' min={5} max={100} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='backgroundBlurOpacity'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Background blur opacity (%)')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={0}
                        max={100}
                        step={5}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Controls transparency of the background blur effect')}
                    </FormDescription>
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
