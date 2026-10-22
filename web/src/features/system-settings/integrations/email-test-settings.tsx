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
import { useMutation } from '@tanstack/react-query'
import { Loader2, Send } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import { FormDescription, FormItem } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { sendSMTPTestEmail } from '../api'
import type { SMTPTestEmailType } from '../types'

export function EmailTestSettings(props: { disabled: boolean }) {
  const { t } = useTranslation()
  const [recipient, setRecipient] = useState('')
  const [type, setType] = useState<SMTPTestEmailType>('smtp')
  const [securityEvent, setSecurityEvent] = useState('password_updated')
  const valid = z.string().email().safeParse(recipient.trim()).success
  const mutation = useMutation({
    mutationFn: sendSMTPTestEmail,
    onSuccess: (data) => {
      if (data.success) toast.success(t('Test email sent successfully'))
      else toast.error(data.message || t('Request failed'))
    },
  })
  const types = [
    { value: 'smtp', label: t('SMTP connection test') },
    { value: 'verification', label: t('Email verification code') },
    { value: 'password_reset', label: t('Password reset') },
    { value: 'email_binding', label: t('New email confirmation') },
    { value: 'email_change', label: t('Current email confirmation') },
    { value: 'security', label: t('Account security notification') },
    { value: 'notification', label: t('General notification') },
    { value: 'low_balance', label: t('Low balance reminder') },
    {
      value: 'subscription_low_balance',
      label: t('Low subscription quota reminder'),
    },
    { value: 'channel_disabled', label: t('Channel disabled') },
    { value: 'channel_enabled', label: t('Channel enabled') },
    { value: 'channel_test', label: t('Channel test completed') },
    { value: 'upstream_update', label: t('Upstream model update') },
  ]
  const events = [
    { value: 'password_updated', label: t('Password updated') },
    { value: 'password_reset', label: t('Password reset') },
    { value: 'email_change_requested', label: t('Email change requested') },
    { value: 'email_changed', label: t('Email address changed') },
    { value: 'email_confirmed', label: t('Email address confirmed') },
    { value: 'login_linked', label: t('Login account linked') },
    { value: 'login_unlinked', label: t('Login account unlinked') },
    { value: 'wechat_linked', label: t('WeChat account linked') },
    { value: 'twofa_enabled', label: t('Two-factor authentication enabled') },
    { value: 'twofa_disabled', label: t('Two-factor authentication disabled') },
    {
      value: 'twofa_reset',
      label: t('Two-factor authentication reset by an administrator'),
    },
    {
      value: 'login_twofa_enabled',
      label: t('Sign-in two-factor authentication enabled'),
    },
    {
      value: 'login_twofa_disabled',
      label: t('Sign-in two-factor authentication disabled'),
    },
    {
      value: 'backup_codes_regenerated',
      label: t('Recovery codes regenerated'),
    },
    { value: 'passkey_registered', label: t('Passkey registered') },
    { value: 'passkey_removed', label: t('Passkey removed') },
    { value: 'passkey_reset', label: t('Passkey reset by an administrator') },
  ]
  const pending = mutation.isPending || props.disabled

  return (
    <div data-settings-form-span='full' className='min-w-0'>
      <div className='grid gap-3 sm:grid-cols-2'>
        <FormItem>
          <Label htmlFor='smtp-test-type'>{t('Email type')}</Label>
          <Select
            items={types}
            value={type}
            onValueChange={(value: SMTPTestEmailType | null) => {
              if (value) setType(value)
            }}
            disabled={mutation.isPending}
          >
            <SelectTrigger id='smtp-test-type' className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent alignItemWithTrigger={false}>
              {types.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </FormItem>
        {type === 'security' && (
          <FormItem>
            <Label htmlFor='smtp-test-security-event'>
              {t('Security event')}
            </Label>
            <Select
              items={events}
              value={securityEvent}
              onValueChange={(value) => {
                if (value) setSecurityEvent(value)
              }}
              disabled={mutation.isPending}
            >
              <SelectTrigger id='smtp-test-security-event' className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                {events.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </FormItem>
        )}
      </div>
      <div className='mt-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_auto] md:items-end'>
        <FormItem>
          <Label htmlFor='smtp-test-recipient'>
            {t('Test recipient email')}
          </Label>
          <Input
            id='smtp-test-recipient'
            aria-describedby='smtp-test-description'
            autoComplete='off'
            type='email'
            value={recipient}
            onChange={(event) => setRecipient(event.target.value)}
            disabled={mutation.isPending}
          />
        </FormItem>
        <Button
          type='button'
          variant='secondary'
          className='shrink-0'
          disabled={pending || !valid}
          onClick={() =>
            mutation.mutate({
              email: recipient.trim(),
              type,
              ...(type === 'security' && { security_event: securityEvent }),
            })
          }
        >
          {mutation.isPending ? <Loader2 className='animate-spin' /> : <Send />}
          {mutation.isPending ? t('Sending...') : t('Send test email')}
        </Button>
      </div>
      <FormDescription id='smtp-test-description' className='mt-2'>
        {t('Save SMTP settings before sending a test email')}
      </FormDescription>
      <FormDescription className='mt-1'>
        {t(
          'Test emails use sample data and do not change accounts or issue valid verification codes'
        )}
      </FormDescription>
    </div>
  )
}
