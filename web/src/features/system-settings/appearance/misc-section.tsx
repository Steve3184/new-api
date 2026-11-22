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

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsSwitchField,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const miscSchema = z.object({
  backgroundImage: z.string(),
  backgroundBlurOpacity: z.coerce.number().min(0).max(100).default(40),
  legalBackgroundEnabled: z.boolean(),
})

export type MiscAppearanceFormValues = z.infer<typeof miscSchema>

const OPTION_KEYS: Record<keyof MiscAppearanceFormValues, string> = {
  backgroundImage: 'console_setting.background_image',
  backgroundBlurOpacity: 'console_setting.background_blur_opacity',
  legalBackgroundEnabled: 'console_setting.legal_background_enabled',
}

export function MiscAppearanceSection(props: {
  defaultValues: MiscAppearanceFormValues
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const schema = miscSchema.extend({
    backgroundImage: z
      .string()
      .refine(
        (value) =>
          !value ||
          (value.startsWith('/') && !value.startsWith('//')) ||
          /^https?:\/\//i.test(value),
        t('Use an HTTP(S) URL or a path starting with /')
      ),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<MiscAppearanceFormValues>({
      resolver: zodResolver(schema) as Resolver<MiscAppearanceFormValues>,
      defaultValues: props.defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [name, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key: OPTION_KEYS[name as keyof MiscAppearanceFormValues],
            value: typeof value === 'number' ? value : String(value),
          })
        }
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Miscellaneous')}>
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
                name='legalBackgroundEnabled'
                render={({ field }) => (
                  <SettingsSwitchField
                    controlId='legal-background-enabled'
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    label={t('Show background image on legal pages')}
                    description={t(
                      'Apply the configured background image to the user agreement and privacy policy.'
                    )}
                  />
                )}
              />
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
