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
import { useQuery } from '@tanstack/react-query'
import { Layers, Plus, Trash2 } from 'lucide-react'
import { useFieldArray, type UseFormReturn } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import {
  SideDrawerSection,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import {
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { IconBadge } from '@/components/ui/icon-badge'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { requireServerSuccess } from '@/lib/server-error-message'

import { getAdminPlans } from '../api'
import { MAX_USAGE_MULTIPLIER, type PlanFormValues } from '../lib'

type PlanTiersSectionProps = {
  form: UseFormReturn<PlanFormValues>
  /** Id of the plan being edited, excluded from upgrade targets. */
  planId?: number
}

/** Admin settings for usage tiers, upgrade paths and balance auto-renewal. */
export function PlanTiersSection(props: PlanTiersSectionProps) {
  const { t } = useTranslation()
  const tiers = useFieldArray({
    control: props.form.control,
    name: 'usage_tiers',
  })
  const allowBalancePay = props.form.watch('allow_balance_pay')
  const basePrice = Number(props.form.watch('price_amount') || 0)

  const plansQuery = useQuery({
    queryKey: ['admin-subscription-plans', 'upgrade-targets'],
    queryFn: async () => requireServerSuccess(await getAdminPlans()).data || [],
  })
  const planOptions = (plansQuery.data || [])
    .filter((record) => record.plan.id !== props.planId)
    .map((record) => ({
      value: String(record.plan.id),
      label: record.plan.title || `#${record.plan.id}`,
      hint: `#${record.plan.id}`,
    }))

  const nextMultiplier = () => {
    const used = new Set(
      props.form.getValues('usage_tiers').map((tier) => tier.multiplier)
    )
    for (const candidate of [2, 5, 10, 20, 50, 100]) {
      if (!used.has(candidate)) return candidate
    }
    return 2
  }

  return (
    <SideDrawerSection>
      <h3 className='flex items-center gap-2 text-sm font-medium'>
        <IconBadge tone='info' size='xs'>
          <Layers />
        </IconBadge>
        {t('Usage Tiers & Upgrades')}
      </h3>
      <FormDescription>
        {t(
          'Sell larger usage versions of this plan. A tier multiplies the billing-period quota and every usage limit, and has its own price.'
        )}
      </FormDescription>

      <div className='space-y-2'>
        <div className='flex items-center justify-between gap-3'>
          <FormLabel>{t('Usage tiers')}</FormLabel>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={tiers.fields.length >= 10}
            onClick={() =>
              tiers.append({
                multiplier: nextMultiplier(),
                price_amount: basePrice,
                stripe_price_id: '',
                creem_product_id: '',
                waffo_pancake_product_id: '',
              })
            }
          >
            <Plus />
            {t('Add tier')}
          </Button>
        </div>
        <div className='text-muted-foreground flex items-center justify-between rounded-md border border-dashed px-3 py-2 text-xs'>
          <span>{t('1x (base plan)')}</span>
          <span className='tabular-nums'>
            {t('Price')}: {basePrice}
          </span>
        </div>
        {tiers.fields.map((tier, index) => (
          <div key={tier.id} className='space-y-2 rounded-md border p-3'>
            <div className='grid grid-cols-[1fr_1fr_auto] items-start gap-2'>
              <FormField
                control={props.form.control}
                name={`usage_tiers.${index}.multiplier`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className='text-xs'>{t('Multiplier')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min={2}
                        max={MAX_USAGE_MULTIPLIER}
                        step={1}
                        onChange={(event) =>
                          field.onChange(
                            Number.parseInt(event.target.value, 10) || 0
                          )
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={props.form.control}
                name={`usage_tiers.${index}.price_amount`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel className='text-xs'>{t('Tier price')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min={0}
                        step='0.01'
                        onChange={(event) =>
                          field.onChange(
                            Number.parseFloat(event.target.value) || 0
                          )
                        }
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                className='mt-6'
                aria-label={t('Remove tier')}
                onClick={() => tiers.remove(index)}
              >
                <Trash2 className='text-destructive' />
              </Button>
            </div>
            <div className='grid grid-cols-1 gap-2 sm:grid-cols-3'>
              {(
                [
                  ['stripe_price_id', t('Stripe Price ID')],
                  ['creem_product_id', t('Creem Product ID')],
                  ['waffo_pancake_product_id', t('Waffo Pancake Product ID')],
                ] as const
              ).map(([key, label]) => (
                <FormField
                  key={key}
                  control={props.form.control}
                  name={`usage_tiers.${index}.${key}`}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel className='text-muted-foreground text-xs'>
                        {label}
                      </FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          value={field.value || ''}
                          placeholder={t('Optional')}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              ))}
            </div>
          </div>
        ))}
        {props.form.formState.errors.usage_tiers?.root?.message && (
          <p className='text-destructive text-sm'>
            {props.form.formState.errors.usage_tiers.root.message}
          </p>
        )}
        <FormDescription>
          {t(
            'Leave product IDs empty to charge the tier price directly where the gateway supports it.'
          )}
        </FormDescription>
      </div>

      <FormField
        control={props.form.control}
        name='allow_tier_switch'
        render={({ field }) => (
          <FormItem className={sideDrawerSwitchItemClassName()}>
            <div className='space-y-0.5'>
              <FormLabel className='!mt-0'>
                {t('Allow switching usage tiers')}
              </FormLabel>
              <FormDescription>
                {t(
                  'Subscribers can upgrade to a higher tier now, or pick another tier for their next renewal.'
                )}
              </FormDescription>
            </div>
            <FormControl>
              <Switch checked={field.value} onCheckedChange={field.onChange} />
            </FormControl>
          </FormItem>
        )}
      />

      <FormField
        control={props.form.control}
        name='upgrade_plan_ids'
        render={({ field }) => (
          <FormItem>
            <FormLabel>{t('Can upgrade to')}</FormLabel>
            <FormControl>
              <MultiSelect
                options={planOptions}
                selected={field.value}
                onChange={field.onChange}
                placeholder={t('Select plans')}
              />
            </FormControl>
            <FormDescription>
              {t(
                'Subscribers of this plan can move to these plans. The unused part of the current period is credited, and the difference is paid from balance.'
              )}
            </FormDescription>
          </FormItem>
        )}
      />

      <FormField
        control={props.form.control}
        name='allow_auto_renew'
        render={({ field }) => (
          <FormItem className={sideDrawerSwitchItemClassName()}>
            <div className='space-y-0.5'>
              <FormLabel className='!mt-0'>{t('Allow auto-renewal')}</FormLabel>
              <FormDescription>
                {allowBalancePay
                  ? t(
                      'Subscribers can renew automatically from their wallet balance when the plan ends.'
                    )
                  : t('Requires balance payment to be allowed.')}
              </FormDescription>
            </div>
            <FormControl>
              <Switch
                checked={allowBalancePay && field.value}
                disabled={!allowBalancePay}
                onCheckedChange={field.onChange}
              />
            </FormControl>
          </FormItem>
        )}
      />
    </SideDrawerSection>
  )
}
