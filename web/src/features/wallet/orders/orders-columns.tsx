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
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { toIntlLocale } from '@/i18n/languages'
import { formatCurrencyFromUSD } from '@/lib/currency'
import { formatNumber } from '@/lib/format'

import {
  formatTimestamp,
  getPaymentMethodName,
  getStatusConfig,
} from '../lib/billing'
import type { TopupRecord } from '../types'

const CRYPTO_PROVIDERS = new Set(['nowpayments', 'monero'])

interface OrdersColumnsOptions {
  isAdmin: boolean
  currentUserId?: number
  resumingTradeNo: string | null
  onContinue: (record: TopupRecord) => void
  onComplete: (record: TopupRecord) => void
}

export function useOrdersColumns(
  options: OrdersColumnsOptions
): ColumnDef<TopupRecord>[] {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)

  const columns: ColumnDef<TopupRecord>[] = [
    {
      accessorKey: 'trade_no',
      header: t('Order Number'),
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <div className='flex min-w-0 items-center gap-1'>
          <code className='truncate font-mono text-xs'>
            {row.original.trade_no}
          </code>
          <CopyButton
            value={row.original.trade_no}
            className='size-6'
            iconClassName='size-3'
          />
        </div>
      ),
      size: 280,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const config = getStatusConfig(row.original.status)
        return (
          <StatusBadge
            label={t(config.label)}
            variant={config.variant}
            showDot
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
      size: 110,
    },
    {
      accessorKey: 'payment_method',
      header: t('Payment Method'),
      cell: ({ row }) => getPaymentMethodName(row.original.payment_method, t),
      size: 130,
    },
    {
      accessorKey: 'amount',
      header: t('Amount'),
      cell: ({ row }) => (
        <span className='font-semibold'>
          {formatCurrencyFromUSD(row.original.amount, {
            digitsLarge: 2,
            digitsSmall: 2,
            abbreviate: false,
          })}
        </span>
      ),
      size: 110,
    },
    {
      accessorKey: 'money',
      header: t('Payment'),
      cell: ({ row }) => {
        const crypto = row.original.crypto_payment
        if (!crypto) {
          return (
            <span className='font-semibold'>
              {formatNumber(row.original.money, locale)}
            </span>
          )
        }
        return (
          <div className='space-y-0.5'>
            <div className='font-semibold'>
              {crypto.actually_paid || '0'} / {crypto.pay_amount}{' '}
              {crypto.pay_currency.toUpperCase()}
            </div>
            <div className='text-muted-foreground text-xs'>
              {formatNumber(row.original.money, locale)} USD
            </div>
          </div>
        )
      },
      size: 200,
    },
    {
      accessorKey: 'create_time',
      header: t('Created At'),
      cell: ({ row }) => (
        <span className='text-muted-foreground text-xs'>
          {formatTimestamp(row.original.create_time)}
        </span>
      ),
      size: 170,
    },
    {
      id: 'actions',
      header: '',
      cell: ({ row }) => {
        const record = row.original
        const isOwner = !options.isAdmin || record.user_id === options.currentUserId
        const canContinue = Boolean(record.can_continue_payment) && isOwner
        const isCrypto = CRYPTO_PROVIDERS.has(record.payment_provider ?? '')
        const canComplete =
          options.isAdmin &&
          (record.status === 'pending' ||
            (isCrypto && record.status !== 'success'))
        return (
          <div className='flex items-center gap-2'>
            {canContinue && (
              <Button
                size='sm'
                disabled={options.resumingTradeNo === record.trade_no}
                onClick={() => options.onContinue(record)}
              >
                {t('Continue Payment')}
              </Button>
            )}
            {canComplete && (
              <Button
                size='sm'
                variant='outline'
                onClick={() => options.onComplete(record)}
              >
                {t('Complete Order')}
              </Button>
            )}
          </div>
        )
      },
      enableSorting: false,
      size: 220,
    },
  ]

  if (options.isAdmin) {
    columns.splice(1, 0, {
      accessorKey: 'user_id',
      header: t('User ID'),
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <StatusBadge
          label={String(row.original.user_id)}
          variant='neutral'
          size='sm'
          copyText={String(row.original.user_id)}
        />
      ),
      size: 100,
    })
  }

  return columns
}
