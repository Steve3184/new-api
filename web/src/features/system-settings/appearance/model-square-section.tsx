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

const modelSquareSchema = z.object({
  modelSquareDefaultView: z.enum(['card', 'table']),
  modelSquareCardPageSize: z.coerce.number().int().min(6).max(96),
  modelSquareTablePageSize: z.coerce.number().int().min(5).max(100),
})

export type ModelSquareFormValues = z.infer<typeof modelSquareSchema>

const OPTION_KEYS: Record<keyof ModelSquareFormValues, string> = {
  modelSquareDefaultView: 'console_setting.model_square_default_view',
  modelSquareCardPageSize: 'console_setting.model_square_card_page_size',
  modelSquareTablePageSize: 'console_setting.model_square_table_page_size',
}

export function ModelSquareSection(props: {
  defaultValues: ModelSquareFormValues
}) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const schema = modelSquareSchema.extend({
    modelSquareCardPageSize: z.coerce
      .number()
      .int()
      .min(6)
      .max(96)
      .refine((value) => value % 6 === 0, t('Must be a multiple of 6')),
  })

  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<ModelSquareFormValues>({
      resolver: zodResolver(schema) as Resolver<ModelSquareFormValues>,
      defaultValues: props.defaultValues,
      onSubmit: async (_data, changedFields) => {
        for (const [name, value] of Object.entries(changedFields)) {
          await updateOption.mutateAsync({
            key: OPTION_KEYS[name as keyof ModelSquareFormValues],
            value: typeof value === 'number' ? value : String(value),
          })
        }
      },
    })

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Model Square')}>
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
                name='modelSquareDefaultView'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Default model square view')}</FormLabel>
                    <FormControl>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <SelectTrigger className='w-full'>
                          <SelectValue>
                            {field.value === 'table' ? t('Table') : t('Card')}
                          </SelectValue>
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            <SelectItem value='card'>{t('Card')}</SelectItem>
                            <SelectItem value='table'>{t('Table')}</SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
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
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
