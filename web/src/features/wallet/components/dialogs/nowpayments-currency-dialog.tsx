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
*/
import { Check, Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

import { getCryptoCurrencyInfo, getCryptoCurrencyLabel } from '../../lib/crypto'
import { CryptoCurrencyIcon } from '../crypto-currency-icon'

interface NowPaymentsCurrencyDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  currencies: string[]
  selectedCurrency: string
  onSelectedCurrencyChange: (currency: string) => void
  onConfirm: () => void | Promise<void>
  loading?: boolean
}

export function NowPaymentsCurrencyDialog(
  props: NowPaymentsCurrencyDialogProps
) {
  const { t } = useTranslation()

  if (props.currencies.length === 0) return null

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Please select a cryptocurrency')}
      contentClassName='max-sm:w-[calc(100vw-1.5rem)] sm:max-w-lg'
      contentHeight='auto'
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={props.loading}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            onClick={() => void props.onConfirm()}
            disabled={!props.selectedCurrency || props.loading}
          >
            {props.loading && <Loader2 className='animate-spin' />}
            {t('Pay')}
          </Button>
        </>
      }
    >
      <div className='grid grid-cols-1 gap-2 sm:grid-cols-2'>
        {props.currencies.map((currency) => {
          const info = getCryptoCurrencyInfo(currency)
          const selected = props.selectedCurrency === currency
          return (
            <Button
              key={currency}
              type='button'
              variant='outline'
              aria-pressed={selected}
              aria-label={getCryptoCurrencyLabel(currency)}
              onClick={() => props.onSelectedCurrencyChange(currency)}
              disabled={props.loading}
              className={cn(
                'h-auto min-h-16 justify-start gap-3 px-3 py-3 text-left',
                selected && 'border-primary bg-primary/5'
              )}
            >
              <CryptoCurrencyIcon currency={currency} className='h-6 w-6' />
              <span className='flex min-w-0 flex-1 flex-col items-start gap-0.5'>
                <span className='font-semibold'>{info.symbol}</span>
                <span className='text-muted-foreground max-w-full truncate text-xs'>
                  {info.network || currency.toUpperCase()}
                </span>
              </span>
              {selected && <Check className='text-primary h-4 w-4' />}
            </Button>
          )
        })}
      </div>
    </Dialog>
  )
}
