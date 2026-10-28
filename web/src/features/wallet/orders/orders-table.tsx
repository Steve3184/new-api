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
import { getRouteApi } from '@tanstack/react-router'
import { useCallback, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { useAuthStore } from '@/stores/auth-store'

import { getAllBillingHistory, getUserBillingHistory, isApiSuccess } from '../api'
import { MoneroPaymentDialog } from '../components/dialogs/monero-payment-dialog'
import { NowPaymentsPaymentDialog } from '../components/dialogs/nowpayments-payment-dialog'
import { useOrderActions } from '../hooks/use-order-actions'
import type { TopupRecord } from '../types'
import { CompleteOrderDialog } from './complete-order-dialog'
import { useOrdersColumns } from './orders-columns'

const route = getRouteApi('/_authenticated/wallet/orders')

export function OrdersTable() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const currentUserId = useAuthStore((s) => s.auth.user?.id)
  const [completeTarget, setCompleteTarget] = useState<TopupRecord | null>(null)

  const { globalFilter, onGlobalFilterChange, pagination, onPaginationChange, ensurePageInRange } =
    useTableUrlState({
      search: route.useSearch(),
      navigate: route.useNavigate(),
      pagination: { defaultPage: 1, defaultPageSize: 10 },
      globalFilter: { enabled: true, key: 'filter' },
    })

  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: [
      'wallet-orders',
      isAdmin,
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
    ],
    queryFn: async () => {
      const keyword = globalFilter?.trim() || undefined
      const page = pagination.pageIndex + 1
      const response = isAdmin
        ? await getAllBillingHistory(page, pagination.pageSize, keyword)
        : await getUserBillingHistory(page, pagination.pageSize, keyword)
      if (!isApiSuccess(response) || !response.data) {
        throw new Error(response.message || t('Failed to load billing history'))
      }
      return {
        items: response.data.items || [],
        total: response.data.total || 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const onChanged = useCallback(() => {
    void refetch()
  }, [refetch])
  const actions = useOrderActions(onChanged)

  const columns = useOrdersColumns({
    isAdmin,
    currentUserId,
    resumingTradeNo: actions.resumingTradeNo,
    onContinue: (record) => void actions.resume(record),
    onComplete: setCompleteTarget,
  })

  const { table } = useDataTable({
    data: data?.items ?? [],
    columns,
    pagination,
    globalFilter,
    onPaginationChange,
    onGlobalFilterChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    ensurePageInRange,
  })

  return (
    <>
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={isLoading}
        isFetching={isFetching}
        emptyTitle={t('No billing records found')}
        emptyDescription={t('Your transaction history will appear here')}
        skeletonKeyPrefix='wallet-orders-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by order number...'),
          searchDebounceMs: 500,
        }}
      />
      <CompleteOrderDialog
        record={completeTarget}
        completing={actions.completing}
        onClose={() => setCompleteTarget(null)}
        onConfirm={actions.complete}
      />
      <NowPaymentsPaymentDialog
        open={actions.nowPaymentsInvoice !== null}
        onOpenChange={(open) => !open && actions.closeNowPaymentsInvoice()}
        invoice={actions.nowPaymentsInvoice}
        onPaymentSuccess={onChanged}
      />
      <MoneroPaymentDialog
        open={actions.moneroInvoice !== null}
        onOpenChange={(open) => !open && actions.closeMoneroInvoice()}
        invoice={actions.moneroInvoice}
        onPaymentSuccess={onChanged}
      />
    </>
  )
}
