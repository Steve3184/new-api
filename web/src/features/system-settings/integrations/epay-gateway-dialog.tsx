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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
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

import type { EpayGatewayData } from './epay-gateways'
import { PaymentMethodsVisualEditor } from './payment-methods-visual-editor'

const EPAY_GATEWAY_FORM_ID = 'epay-gateway-form'

type EpayGatewayDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSave: (gateway: EpayGatewayData) => void
  editData: EpayGatewayData | null
  existingIds: string[]
}

export function EpayGatewayDialog(props: EpayGatewayDialogProps) {
  const { t } = useTranslation()
  const schema = z.object({
    id: z
      .string()
      .trim()
      .min(1, t('Gateway ID is required'))
      .max(100, t('Gateway ID must not exceed 100 characters'))
      .refine(
        (value) =>
          value === props.editData?.id || !props.existingIds.includes(value),
        t('Gateway ID must be unique')
      ),
    name: z.string().trim().min(1, t('Gateway name is required')),
    address: z
      .url(t('Enter a valid Epay gateway URL'))
      .refine(
        (value) => value.startsWith('http://') || value.startsWith('https://'),
        t('Enter a valid Epay gateway URL')
      ),
    merchant_id: z.string().trim().min(1, t('Merchant ID is required')),
    key: z.string().trim().min(1, t('Secret key is required')),
    enabled: z.boolean(),
    pay_methods: z.string(),
  })
  type FormValues = z.infer<typeof schema>

  const form = useForm<FormValues>({
    resolver: zodResolver(schema),
    defaultValues: {
      id: '',
      name: '',
      address: '',
      merchant_id: '',
      key: '',
      enabled: true,
      pay_methods: '[]',
    },
  })

  useEffect(() => {
    const gateway = props.editData
    form.reset(
      gateway
        ? {
            ...gateway,
            pay_methods: JSON.stringify(gateway.pay_methods, null, 2),
          }
        : {
            id: '',
            name: '',
            address: '',
            merchant_id: '',
            key: '',
            enabled: true,
            pay_methods: '[]',
          }
    )
  }, [form, props.editData, props.open])

  const handleSubmit = (values: FormValues) => {
    const payMethods = JSON.parse(
      values.pay_methods
    ) as EpayGatewayData['pay_methods']
    props.onSave({
      id: values.id.trim(),
      name: values.name.trim(),
      address: values.address.trim().replace(/\/+$/, ''),
      merchant_id: values.merchant_id.trim(),
      key: values.key.trim(),
      enabled: values.enabled,
      pay_methods: payMethods,
    })
    props.onOpenChange(false)
  }

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={props.editData ? t('Edit Epay gateway') : t('Add Epay gateway')}
      description={t(
        'Configure the provider credentials and payment methods offered through this gateway.'
      )}
      contentClassName='sm:max-w-3xl'
      contentHeight='min(72vh, 42rem)'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' form={EPAY_GATEWAY_FORM_ID}>
            {props.editData ? t('Update') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={EPAY_GATEWAY_FORM_ID}
          className='space-y-5'
          onSubmit={form.handleSubmit(handleSubmit)}
        >
          <FormField
            control={form.control}
            name='enabled'
            render={({ field }) => (
              <FormItem className='flex items-center justify-between gap-4 rounded-lg border p-3'>
                <div className='space-y-1'>
                  <FormLabel>{t('Enable gateway')}</FormLabel>
                  <FormDescription>
                    {t('Disabled gateways are hidden from users.')}
                  </FormDescription>
                </div>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Gateway ID')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder='primary'
                      autoComplete='off'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Stable identifier stored with payment orders.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Gateway name')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('Primary Epay')} {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='address'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Epay endpoint')}</FormLabel>
                <FormControl>
                  <Input placeholder='https://pay.example.com' {...field} />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='grid gap-4 sm:grid-cols-2'>
            <FormField
              control={form.control}
              name='merchant_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Epay merchant ID')}</FormLabel>
                  <FormControl>
                    <Input placeholder='10001' autoComplete='off' {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='key'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Epay secret key')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={t('Enter the gateway secret key')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {props.editData?.key === '***'
                      ? t('The saved secret remains unchanged while masked.')
                      : t('Stored securely and never returned after saving.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='pay_methods'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Gateway payment methods')}</FormLabel>
                <FormDescription>
                  {t(
                    'Configure the methods, minimum top-up, and payer fees for this gateway.'
                  )}
                </FormDescription>
                <FormControl>
                  <PaymentMethodsVisualEditor
                    value={field.value}
                    onChange={field.onChange}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}
