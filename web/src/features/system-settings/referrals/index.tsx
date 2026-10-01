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
import { useTranslation } from 'react-i18next'
import { z } from 'zod'

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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
} from '../components/settings-form-layout'
import { SettingsPage } from '../components/settings-page'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const defaults = {
  'referral_setting.enabled': true,
  'referral_setting.percent': 5,
  'referral_setting.min_paid_cny': 20,
  'referral_setting.max_orders': 10,
  'referral_setting.delay_hours': 24,
  'referral_setting.currency_rates': '{"CNY":1,"USD":7.3}',
}

export function ReferralSettingsForm(props: { settings: typeof defaults }) {
  const { t } = useTranslation()
  const update = useUpdateOption()
  const schema = z.object({
    referral_setting: z.object({
      enabled: z.boolean(),
      percent: z.number().min(0).max(100),
      min_paid_cny: z.number().min(0).max(1000000000),
      max_orders: z.number().int().min(1).max(87600),
      delay_hours: z.number().int().min(1).max(87600),
      currency_rates: z
        .string()
        .max(4096)
        .refine((value) => {
          try {
            const rates: unknown = JSON.parse(value)
            if (!rates || typeof rates !== 'object' || Array.isArray(rates)) {
              return false
            }
            const entries = rates as Record<string, unknown>
            return (
              entries.CNY === 1 &&
              typeof entries.USD === 'number' &&
              entries.USD > 0 &&
              Object.entries(entries).every(
                ([currency, rate]) =>
                  /^[A-Z]{3}$/.test(currency) &&
                  typeof rate === 'number' &&
                  Number.isFinite(rate) &&
                  rate > 0 &&
                  rate <= 1000000
              )
            )
          } catch {
            return false
          }
        }, t('Provide positive currency rates with CNY equal to 1 and a USD rate.')),
    }),
  })
  const settings = props.settings
  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<z.infer<typeof schema>>({
      resolver: zodResolver(schema),
      defaultValues: {
        referral_setting: {
          enabled: settings['referral_setting.enabled'],
          percent: settings['referral_setting.percent'],
          min_paid_cny: settings['referral_setting.min_paid_cny'],
          max_orders: settings['referral_setting.max_orders'],
          delay_hours: settings['referral_setting.delay_hours'],
          currency_rates: settings['referral_setting.currency_rates'],
        },
      },
      onSubmit: async (_values, changes) => {
        for (const [key, value] of Object.entries(changes)) {
          await update.mutateAsync({ key, value: String(value) })
        }
      },
    })
  const numericFields = [
    {
      key: 'percent',
      label: t('Referral percentage'),
      min: 0,
      max: 100,
      step: 0.1,
    },
    {
      key: 'min_paid_cny',
      label: t('Exclusive minimum payment (CNY)'),
      min: 0,
      max: 1000000000,
      step: 0.01,
    },
    {
      key: 'max_orders',
      label: t('Qualifying orders per invitee'),
      min: 1,
      max: 87600,
      step: 1,
    },
    {
      key: 'delay_hours',
      label: t('Credit delay (hours)'),
      min: 1,
      max: 87600,
      step: 1,
    },
  ] as const
  return (
    <Form {...form}>
      <SettingsForm onSubmit={handleSubmit}>
        <SettingsPageFormActions
          onSave={handleSubmit}
          onReset={handleReset}
          isSaving={isSubmitting}
        />
        <FormDirtyIndicator isDirty={isDirty} />
        <FormNavigationGuard when={isDirty} />
        <p className='text-muted-foreground text-sm'>
          {t(
            'Only future orders use updated rules. Pending rewards retain their original rate and credit time. Reversed orders still count toward the limit.'
          )}
        </p>
        <FormField
          control={form.control}
          name='referral_setting.enabled'
          render={({ field }) => (
            <FormItem className='flex items-center justify-between rounded-lg border p-4'>
              <FormLabel>{t('Enable fiat referral rewards')}</FormLabel>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </FormItem>
          )}
        />
        <SettingsFormGrid>
          {numericFields.map((item) => (
            <FormField
              key={item.key}
              control={form.control}
              name={`referral_setting.${item.key}`}
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{item.label}</FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={item.min}
                      max={item.max}
                      step={item.step}
                      value={field.value}
                      onChange={(e) =>
                        field.onChange(
                          e.target.value === ''
                            ? Number.NaN
                            : Number(e.target.value)
                        )
                      }
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          ))}
        </SettingsFormGrid>
        <FormField
          control={form.control}
          name='referral_setting.currency_rates'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Currency conversion rates to CNY')}</FormLabel>
              <FormControl>
                <Textarea {...field} rows={4} />
              </FormControl>
              <FormDescription>
                {t(
                  'JSON: CNY value of one unit of each payment currency. Unlisted currencies do not qualify. USD also determines the reward balance conversion.'
                )}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
        <p className='text-muted-foreground text-sm'>
          {t(
            'Rewards are for platform usage only and cannot be withdrawn. Gift credits, redemption codes, subscriptions, crypto payments and manual credits are excluded.'
          )}
        </p>
      </SettingsForm>
    </Form>
  )
}

export function ReferralSettings() {
  return (
    <SettingsPage
      routePath='/_authenticated/system-settings/referrals'
      defaultSettings={defaults}
      defaultSection='referrals'
      getSectionMeta={() => ({ titleKey: 'Invitation settings' })}
      getSectionContent={(_section, settings) => (
        <ReferralSettingsForm settings={settings} />
      )}
    />
  )
}
