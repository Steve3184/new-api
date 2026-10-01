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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { getCoreRowModel, useReactTable } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DataTablePagination,
  StaticDataTable,
  type StaticDataTableColumn,
} from '@/components/data-table'
import { EmptyState } from '@/components/empty-state'
import { ErrorState } from '@/components/error-state'
import { LoadingState } from '@/components/loading-state'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'

import {
  getReferralRewards,
  reverseReferralReward,
  type ReferralReward,
} from './api'

export function ReferralRewardsPanel(props: { admin?: boolean }) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  // The ledger amount is fixed CNY, independent of the configured wallet currency.
  const paidCurrency = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency: 'CNY',
  })
  const userId = useAuthStore((s) => s.auth.user?.id)
  const queryClient = useQueryClient()
  const [pagination, setPagination] = useState({ pageIndex: 0, pageSize: 10 })
  const [status, setStatus] = useState('all')
  const [search, setSearch] = useState({
    inviter_id: '',
    invitee_id: '',
    trade_no: '',
  })
  const [filters, setFilters] = useState(search)
  const [selected, setSelected] = useState<ReferralReward | null>(null)
  const [reason, setReason] = useState('')
  const query = useQuery({
    queryKey: [
      'referral-rewards',
      userId,
      Boolean(props.admin),
      pagination,
      status,
      filters,
    ],
    queryFn: () =>
      getReferralRewards(Boolean(props.admin), {
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        status: status === 'all' ? '' : status,
        ...filters,
      }),
    refetchInterval: 60000,
  })
  const reversal = useMutation({
    mutationFn: (reward: ReferralReward) =>
      reverseReferralReward(reward.id, reason.trim()),
    onSuccess: () => {
      setSelected(null)
      setReason('')
      void queryClient.invalidateQueries({ queryKey: ['referral-rewards'] })
    },
  })
  const table = useReactTable({
    data: query.data?.items ?? [],
    columns: [],
    getCoreRowModel: getCoreRowModel(),
    manualPagination: true,
    rowCount: query.data?.total ?? 0,
    state: { pagination },
    onPaginationChange: setPagination,
  })
  const labels = {
    pending: t('Pending'),
    credited: t('Credited'),
    reversed: t('Reversed'),
  }
  const columns: StaticDataTableColumn<ReferralReward>[] = [
    {
      id: 'invitee',
      header: t('Invitee ID'),
      cell: (row) => formatNumber(row.invitee_id, locale),
    },
    {
      id: 'paid',
      header: t('Paid amount (CNY)'),
      cell: (row) => paidCurrency.format(row.paid_cny),
    },
    {
      id: 'reward',
      header: t('Reward'),
      cell: (row) => formatQuota(row.quota),
    },
    {
      id: 'status',
      header: t('Status'),
      cell: (row) => <Badge variant='outline'>{labels[row.status]}</Badge>,
    },
    {
      id: 'due',
      header: t('Scheduled credit time'),
      cell: (row) => formatTimestamp(row.available_at),
    },
    {
      id: 'credited',
      header: t('Credited at'),
      cell: (row) => (row.credited_at ? formatTimestamp(row.credited_at) : '—'),
    },
  ]
  if (props.admin) {
    columns.unshift({
      id: 'inviter',
      header: t('Inviter ID'),
      cell: (row) => formatNumber(row.inviter_id, locale),
    })
    columns.push(
      {
        id: 'order',
        header: t('Order number'),
        cell: (row) => <span className='break-all'>{row.trade_no}</span>,
      },
      {
        id: 'reason',
        header: t('Reason'),
        cell: (row) => <span className='break-words'>{row.reason || '—'}</span>,
      },
      {
        id: 'actions',
        header: t('Actions'),
        cell: (row) => (
          <Button
            variant='outline'
            size='sm'
            disabled={row.status === 'reversed'}
            onClick={() => {
              setSelected(row)
              setReason('')
            }}
          >
            {t('Reverse reward')}
          </Button>
        ),
      }
    )
  }
  const rules = query.data?.rules

  return (
    <section className='min-w-0 space-y-4' aria-label={t('Referral rewards')}>
      {!props.admin && query.data && (
        <p className='text-sm font-medium'>
          {t('Invites')}: {formatNumber(query.data.invited_users ?? 0, locale)}
        </p>
      )}
      {rules && (
        <div className='text-muted-foreground space-y-1 text-sm'>
          <p>
            {t(
              'Earn {{percent}}% from each invitee’s first {{count}} fiat top-ups strictly above CNY {{amount}}. Rewards reach your balance after {{hours}} hours.',
              {
                percent: formatNumber(rules.percent, locale),
                count: formatNumber(rules.max_orders, locale),
                amount: formatNumber(rules.min_paid_cny, locale),
                hours: formatNumber(rules.delay_hours, locale),
              }
            )}
          </p>
          <p>
            {t(
              'Rewards are for platform usage only and cannot be withdrawn. Gift credits, redemption codes, subscriptions, crypto payments and manual credits are excluded.'
            )}
          </p>
          {!rules.enabled && (
            <p>
              {t(
                'New referral rewards are currently disabled. Existing pending rewards keep their scheduled credit time.'
              )}
            </p>
          )}
        </div>
      )}
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
        {(['pending', 'credited', 'reversed'] as const).map((key) => (
          <div key={key} className='rounded-lg border p-3'>
            <div className='text-muted-foreground text-xs'>{labels[key]}</div>
            <div className='mt-1 text-lg font-semibold'>
              {formatQuota(
                query.data?.summary.find((entry) => entry.status === key)
                  ?.quota ?? 0
              )}
            </div>
          </div>
        ))}
      </div>
      <div className='flex flex-wrap gap-2'>
        <Select
          value={status}
          onValueChange={(value) => {
            setStatus(value ?? 'all')
            setPagination((p) => ({ ...p, pageIndex: 0 }))
          }}
        >
          <SelectTrigger className='w-40' aria-label={t('Reward status')}>
            <SelectValue>
              {status === 'all'
                ? t('All')
                : labels[status as keyof typeof labels]}
            </SelectValue>
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='all'>{t('All')}</SelectItem>
            {(['pending', 'credited', 'reversed'] as const).map((key) => (
              <SelectItem key={key} value={key}>
                {labels[key]}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        {props.admin && (
          <form
            className='flex min-w-0 flex-1 flex-wrap gap-2'
            onSubmit={(event) => {
              event.preventDefault()
              setFilters(search)
              setPagination((p) => ({ ...p, pageIndex: 0 }))
            }}
          >
            <Input
              className='w-36'
              type='number'
              min={1}
              aria-label={t('Inviter ID')}
              placeholder={t('Inviter ID')}
              value={search.inviter_id}
              onChange={(e) =>
                setSearch({ ...search, inviter_id: e.target.value })
              }
            />
            <Input
              className='w-36'
              type='number'
              min={1}
              aria-label={t('Invitee ID')}
              placeholder={t('Invitee ID')}
              value={search.invitee_id}
              onChange={(e) =>
                setSearch({ ...search, invitee_id: e.target.value })
              }
            />
            <Input
              className='w-56'
              aria-label={t('Order number')}
              placeholder={t('Order number')}
              value={search.trade_no}
              onChange={(e) =>
                setSearch({ ...search, trade_no: e.target.value })
              }
            />
            <Button type='submit' variant='outline'>
              {t('Search')}
            </Button>
          </form>
        )}
      </div>
      {query.isPending && <LoadingState />}
      {query.isError && <ErrorState onRetry={() => void query.refetch()} />}
      {!query.isPending && !query.isError && (
        <>
          <StaticDataTable
            columns={columns}
            data={query.data?.items ?? []}
            getRowKey={(row) => row.id}
            emptyContent={
              <EmptyState
                title={t('No referral rewards yet')}
                className='min-h-32'
              />
            }
          />
          <DataTablePagination table={table} compact />
        </>
      )}
      <ConfirmDialog
        open={selected !== null}
        onOpenChange={(open) => {
          if (!open) setSelected(null)
        }}
        title={t('Reverse reward')}
        desc={t(
          'Pending rewards will be cancelled. Credited rewards will be deducted from the inviter’s balance, which may become negative. The order still counts toward the limit.'
        )}
        destructive
        confirmText={t('Reverse reward')}
        handleConfirm={() => {
          if (selected) reversal.mutate(selected)
        }}
        isLoading={reversal.isPending}
        disabled={!reason.trim()}
      >
        <div className='space-y-2'>
          <Label htmlFor='referral-reversal-reason'>{t('Reason')}</Label>
          <Input
            id='referral-reversal-reason'
            maxLength={150}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </div>
      </ConfirmDialog>
    </section>
  )
}
