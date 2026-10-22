/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { useFormContext } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

export const emailTemplateKeys = [
  'EmailVerificationTemplate',
  'EmailPasswordResetTemplate',
  'EmailBindingTemplate',
  'EmailSecurityNotificationTemplate',
  'EmailNotificationTemplate',
] as const

type EmailTemplateKey = (typeof emailTemplateKeys)[number]

export function EmailTemplateEditor() {
  const { t } = useTranslation()
  const form = useFormContext<Record<EmailTemplateKey, string>>()
  const [selected, setSelected] = useState<EmailTemplateKey>(
    'EmailVerificationTemplate'
  )
  const options = [
    {
      value: 'EmailVerificationTemplate',
      label: t('Verification email template'),
      fields: ['Code', 'ValidMinutes'],
    },
    {
      value: 'EmailPasswordResetTemplate',
      label: t('Password reset email template'),
      fields: ['Greeting', 'Link', 'ActionText', 'ValidMinutes', 'Validity'],
    },
    {
      value: 'EmailBindingTemplate',
      label: t('Email binding template'),
      fields: ['Intro', 'Code', 'ValidMinutes', 'Validity', 'IsOldEmail'],
    },
    {
      value: 'EmailSecurityNotificationTemplate',
      label: t('Account security email template'),
      fields: ['Event', 'Intro', 'Advice'],
    },
    {
      value: 'EmailNotificationTemplate',
      label: t('Notification email template'),
      fields: ['Title', 'Content'],
    },
  ]
  const current = options.find((option) => option.value === selected)!

  return (
    <div data-settings-form-span='full' className='grid min-w-0 gap-4'>
      <FormItem>
        <Label htmlFor='email-template-type'>{t('Email template')}</Label>
        <Select
          items={options}
          value={selected}
          onValueChange={(value: EmailTemplateKey | null) => {
            if (value) setSelected(value)
          }}
        >
          <SelectTrigger id='email-template-type' className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            {options.map((option) => (
              <SelectItem key={option.value} value={option.value}>
                {option.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </FormItem>
      <FormField
        key={selected}
        control={form.control}
        name={selected}
        render={({ field }) => (
          <FormItem>
            <FormLabel>{current.label}</FormLabel>
            <FormControl>
              <Textarea
                rows={18}
                className='font-mono text-xs'
                spellCheck={false}
                {...field}
              />
            </FormControl>
            <FormDescription>
              {t('Available placeholders:')}{' '}
              {['SystemName', ...current.fields].map((name, index) => (
                <span key={name}>
                  {index > 0 && ', '}
                  <code>{`{{.${name}}}`}</code>
                </span>
              ))}
            </FormDescription>
            {selected !== 'EmailVerificationTemplate' && (
              <FormDescription>
                {t('Leave blank to use the localized default template')}
              </FormDescription>
            )}
            <FormMessage />
          </FormItem>
        )}
      />
    </div>
  )
}
