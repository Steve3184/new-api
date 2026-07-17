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
import i18next from 'i18next'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
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
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const botProtectionSchema = z
  .object({
    CaptchaType: z.enum(['turnstile', 'hcaptcha', 'cap']),
    TurnstileCheckEnabled: z.boolean(),
    TurnstileSiteKey: z.string().optional(),
    TurnstileSecretKey: z.string().optional(),
    HCaptchaEnabled: z.boolean(),
    HCaptchaSiteKey: z.string().optional(),
    HCaptchaSecretKey: z.string().optional(),
    CapEnabled: z.boolean(),
    CapServerURL: z.string().optional(),
    CapAdminAPIKey: z.string().optional(),
    CapSiteKey: z.string().optional(),
    CapSecretKey: z.string().optional(),
    CapCheckinSiteKey: z.string().optional(),
    CapCheckinSecretKey: z.string().optional(),
    LoginCaptchaDifficulty: z.number().int().min(1).max(8),
    CheckinCaptchaDifficulty: z.number().int().min(1).max(8),
    ForceCheckinCaptcha: z.boolean(),
    ForceRedemptionCaptcha: z.boolean(),
  })
  .superRefine((values, context) => {
    if (values.CaptchaType === 'cap' && values.CapEnabled) {
      const requiredFields: Array<{
        key: 'CapServerURL' | 'CapAdminAPIKey' | 'CapSiteKey' | 'CapSecretKey'
        value: string | undefined
        message: string
      }> = [
        {
          key: 'CapServerURL',
          value: values.CapServerURL,
          message: 'Cap server URL is required',
        },
        {
          key: 'CapAdminAPIKey',
          value: values.CapAdminAPIKey,
          message: 'Cap API key is required',
        },
        {
          key: 'CapSiteKey',
          value: values.CapSiteKey,
          message: 'Login and registration site key is required',
        },
        {
          key: 'CapSecretKey',
          value: values.CapSecretKey,
          message: 'Login and registration secret key is required',
        },
      ]
      for (const field of requiredFields) {
        if (!field.value?.trim()) {
          context.addIssue({
            code: 'custom',
            path: [field.key],
            message: field.message,
          })
        }
      }
    }

    if (values.CaptchaType === 'hcaptcha' && values.HCaptchaEnabled) {
      if (!values.HCaptchaSiteKey?.trim()) {
        context.addIssue({
          code: 'custom',
          path: ['HCaptchaSiteKey'],
          message: 'hCaptcha site key is required',
        })
      }
      if (!values.HCaptchaSecretKey?.trim()) {
        context.addIssue({
          code: 'custom',
          path: ['HCaptchaSecretKey'],
          message: 'hCaptcha secret key is required',
        })
      }
    }

    if (values.ForceCheckinCaptcha) {
      if (values.CaptchaType === 'cap') {
        if (!values.CapEnabled) {
          context.addIssue({
            code: 'custom',
            path: ['CapEnabled'],
            message: 'Enable Cap before requiring it for check-in',
          })
        }
        if (!values.CapCheckinSiteKey?.trim()) {
          context.addIssue({
            code: 'custom',
            path: ['CapCheckinSiteKey'],
            message: 'Check-in site key is required',
          })
        }
        if (!values.CapCheckinSecretKey?.trim()) {
          context.addIssue({
            code: 'custom',
            path: ['CapCheckinSecretKey'],
            message: 'Check-in secret key is required',
          })
        }
      } else if (values.CaptchaType === 'hcaptcha') {
        if (!values.HCaptchaEnabled) {
          context.addIssue({
            code: 'custom',
            path: ['HCaptchaEnabled'],
            message: 'Enable hCaptcha before requiring it for check-in',
          })
        }
        if (!values.HCaptchaSiteKey?.trim()) {
          context.addIssue({
            code: 'custom',
            path: ['HCaptchaSiteKey'],
            message: 'hCaptcha site key is required',
          })
        }
        if (!values.HCaptchaSecretKey?.trim()) {
          context.addIssue({
            code: 'custom',
            path: ['HCaptchaSecretKey'],
            message: 'hCaptcha secret key is required',
          })
        }
      } else if (!values.TurnstileCheckEnabled) {
        context.addIssue({
          code: 'custom',
          path: ['TurnstileCheckEnabled'],
          message: 'Enable Turnstile before requiring it for check-in',
        })
      }
    }

    if (values.ForceRedemptionCaptcha) {
      if (values.CaptchaType === 'cap' && !values.CapEnabled) {
        context.addIssue({
          code: 'custom',
          path: ['CapEnabled'],
          message: i18next.t('Enable Cap before requiring it for redemption'),
        })
      } else if (values.CaptchaType === 'hcaptcha' && !values.HCaptchaEnabled) {
        context.addIssue({
          code: 'custom',
          path: ['HCaptchaEnabled'],
          message: i18next.t(
            'Enable hCaptcha before requiring it for redemption'
          ),
        })
      } else if (values.CaptchaType === 'turnstile') {
        if (!values.TurnstileCheckEnabled) {
          context.addIssue({
            code: 'custom',
            path: ['TurnstileCheckEnabled'],
            message: i18next.t(
              'Enable Turnstile before requiring it for redemption'
            ),
          })
        }
        if (!values.TurnstileSiteKey?.trim()) {
          context.addIssue({
            code: 'custom',
            path: ['TurnstileSiteKey'],
            message: i18next.t('Turnstile site key is required'),
          })
        }
        if (!values.TurnstileSecretKey?.trim()) {
          context.addIssue({
            code: 'custom',
            path: ['TurnstileSecretKey'],
            message: i18next.t('Turnstile secret key is required'),
          })
        }
      }
    }

    if (
      values.CapSiteKey &&
      values.CapSiteKey === values.CapCheckinSiteKey &&
      values.LoginCaptchaDifficulty !== values.CheckinCaptchaDifficulty
    ) {
      context.addIssue({
        code: 'custom',
        path: ['CapCheckinSiteKey'],
        message:
          'Use different Cap site keys when login and check-in difficulties differ',
      })
    }
  })

export type BotProtectionFormValues = z.infer<typeof botProtectionSchema>

type BotProtectionSectionProps = {
  defaultValues: BotProtectionFormValues
}

const SAVE_ORDER: Array<keyof BotProtectionFormValues> = [
  'TurnstileSiteKey',
  'TurnstileSecretKey',
  'HCaptchaSiteKey',
  'HCaptchaSecretKey',
  'CapServerURL',
  'CapAdminAPIKey',
  'CapSiteKey',
  'CapSecretKey',
  'CapCheckinSiteKey',
  'CapCheckinSecretKey',
  'LoginCaptchaDifficulty',
  'CheckinCaptchaDifficulty',
  'TurnstileCheckEnabled',
  'HCaptchaEnabled',
  'CapEnabled',
  'CaptchaType',
  'ForceCheckinCaptcha',
  'ForceRedemptionCaptcha',
]

// Password input that hides the "***" sentinel emitted by the server for
// already-configured secrets. Displays an empty field with an explanatory
// placeholder instead; the sentinel stays in React state so validation and
// the skip-unchanged save loop work without any special casing.
function SensitiveInput({
  placeholder,
  ...props
}: React.ComponentProps<typeof Input>) {
  const { t } = useTranslation()
  const isConfigured = props.value === '***'
  return (
    <Input
      {...props}
      value={isConfigured ? '' : props.value}
      placeholder={
        isConfigured
          ? t('Already configured — enter a new value to change')
          : placeholder
      }
    />
  )
}

export function BotProtectionSection(props: BotProtectionSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const form = useForm<BotProtectionFormValues>({
    resolver: zodResolver(botProtectionSchema),
    defaultValues: props.defaultValues,
  })

  useEffect(() => {
    form.reset(props.defaultValues)
  }, [form, props.defaultValues])

  const onSubmit = async (data: BotProtectionFormValues) => {
    for (const key of SAVE_ORDER) {
      const value = data[key]
      if (value === props.defaultValues[key]) continue
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  return (
    <SettingsSection title={t('Bot Protection')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <FormField
            control={form.control}
            name='CaptchaType'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Captcha Provider')}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='turnstile'>
                      Cloudflare Turnstile
                    </SelectItem>
                    <SelectItem value='cap'>Cap (PoW)</SelectItem>
                    <SelectItem value='hcaptcha'>hCaptcha</SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'Select the captcha service used for login, registration, password recovery, and check-in.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='space-y-5 border-t pt-5'>
            <h3 className='text-sm font-semibold'>Cloudflare Turnstile</h3>
            <FormField
              control={form.control}
              name='TurnstileCheckEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable Turnstile')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Protect login and registration with Cloudflare Turnstile'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='TurnstileSiteKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Site Key')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Your Turnstile site key')}
                        autoComplete='off'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='TurnstileSecretKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Secret Key')}</FormLabel>
                    <FormControl>
                      <SensitiveInput
                        type='password'
                        placeholder={t('Your Turnstile secret key')}
                        autoComplete='new-password'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <div className='space-y-5 border-t pt-5'>
            <h3 className='text-sm font-semibold'>hCaptcha</h3>
            <FormField
              control={form.control}
              name='HCaptchaEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable hCaptcha')}</FormLabel>
                    <FormDescription>
                      {t('Protect login and registration with hCaptcha')}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />
            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='HCaptchaSiteKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Site Key')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('Your hCaptcha site key')}
                        autoComplete='off'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='HCaptchaSecretKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Secret Key')}</FormLabel>
                    <FormControl>
                      <SensitiveInput
                        type='password'
                        placeholder={t('Your hCaptcha secret key')}
                        autoComplete='new-password'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <div className='space-y-5 border-t pt-5'>
            <h3 className='text-sm font-semibold'>Cap (PoW)</h3>
            <FormField
              control={form.control}
              name='CapEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable Cap')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Use a self-hosted Cap Standalone server for proof-of-work captcha challenges.'
                      )}
                    </FormDescription>
                  </SettingsSwitchContent>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </SettingsSwitchItem>
              )}
            />

            <div className='grid gap-4 md:grid-cols-2'>
              <FormField
                control={form.control}
                name='CapServerURL'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Cap Server URL')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder='https://cap.example.com'
                        autoComplete='url'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Public URL of your Cap Standalone instance.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='CapAdminAPIKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Cap API Key')}</FormLabel>
                    <FormControl>
                      <SensitiveInput
                        type='password'
                        placeholder={t(
                          'API key used to synchronize difficulty settings'
                        )}
                        autoComplete='new-password'
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Create this key in Cap under Settings > API Keys.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='grid gap-4 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='CapSiteKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Login and registration site key')}
                    </FormLabel>
                    <FormControl>
                      <Input autoComplete='off' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='CapSecretKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Login and registration secret key')}
                    </FormLabel>
                    <FormControl>
                      <SensitiveInput
                        type='password'
                        autoComplete='new-password'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='LoginCaptchaDifficulty'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>
                      {t('Login and registration difficulty')}
                    </FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={8}
                        value={field.value}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Cap supports difficulty values from 1 to 8.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <div className='grid gap-4 md:grid-cols-3'>
              <FormField
                control={form.control}
                name='CapCheckinSiteKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Check-in site key')}</FormLabel>
                    <FormControl>
                      <Input autoComplete='off' {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='CapCheckinSecretKey'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Check-in secret key')}</FormLabel>
                    <FormControl>
                      <SensitiveInput
                        type='password'
                        autoComplete='new-password'
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='CheckinCaptchaDifficulty'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Check-in difficulty')}</FormLabel>
                    <FormControl>
                      <Input
                        type='number'
                        min={1}
                        max={8}
                        value={field.value}
                        onChange={(event) =>
                          field.onChange(event.target.valueAsNumber)
                        }
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Use a separate Cap site key when this differs from login difficulty.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          </div>

          <FormField
            control={form.control}
            name='ForceCheckinCaptcha'
            render={({ field }) => (
              <SettingsSwitchItem className='border-t pt-5'>
                <SettingsSwitchContent>
                  <FormLabel>{t('Require captcha for check-in')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Require a new human verification every time a user checks in.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='ForceRedemptionCaptcha'
            render={({ field }) => (
              <SettingsSwitchItem className='border-t pt-5'>
                <SettingsSwitchContent>
                  <FormLabel>{t('Require captcha for redemption')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Require a new human verification every time a user redeems a code.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
