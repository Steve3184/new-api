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
import { Textarea } from '@/components/ui/textarea'

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

const spaMetaSchema = z.object({
  description: z.string().max(500),
  ogType: z.string().regex(/^[A-Za-z][A-Za-z0-9._:-]{0,63}$/),
  ogDescription: z.string().max(500),
})

export type SPAMetaFormValues = z.infer<typeof spaMetaSchema>

export function SPAMetaSection({
  defaultValues,
}: {
  defaultValues: SPAMetaFormValues
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<SPAMetaFormValues>({
      resolver: zodResolver(spaMetaSchema) as Resolver<SPAMetaFormValues>,
      defaultValues,
      onSubmit: async (_data, changedFields) => {
        const keys: Record<keyof SPAMetaFormValues, string> = {
          description: 'console_setting.spa_meta_description',
          ogType: 'console_setting.spa_meta_og_type',
          ogDescription: 'console_setting.spa_meta_og_description',
        }
        for (const [name, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key: keys[name as keyof SPAMetaFormValues],
            value: String(value),
          })
        }
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('SPA Metadata')}>
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
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Meta description')}</FormLabel>
                    <FormControl>
                      <Textarea rows={4} maxLength={500} {...field} />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Description used by the SPA document and search previews'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='ogType'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Open Graph type')}</FormLabel>
                    <FormControl>
                      <Input placeholder='website' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='ogDescription'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Open Graph description')}</FormLabel>
                    <FormControl>
                      <Textarea rows={4} maxLength={500} {...field} />
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
