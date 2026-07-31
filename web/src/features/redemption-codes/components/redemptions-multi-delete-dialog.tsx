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
import type { Table } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'

import { batchDeleteRedemptions } from '../api'
import type { Redemption } from '../types'
import { useRedemptions } from './redemptions-provider'

type RedemptionsMultiDeleteDialogProps<TData> = {
  open: boolean
  onOpenChange: (open: boolean) => void
  table: Table<TData>
}

export function RedemptionsMultiDeleteDialog<TData>(
  props: RedemptionsMultiDeleteDialogProps<TData>
) {
  const { t } = useTranslation()
  const { triggerRefresh } = useRedemptions()
  const [isDeleting, setIsDeleting] = useState(false)
  const selectedRows = props.table.getFilteredSelectedRowModel().rows

  const handleConfirm = async () => {
    const ids = selectedRows.map((row) => (row.original as Redemption).id)
    if (ids.length === 0) {
      props.onOpenChange(false)
      return
    }

    setIsDeleting(true)
    try {
      const result = await batchDeleteRedemptions(ids)
      if (!result.success) {
        toast.error(result.message || t('Batch delete failed'))
        return
      }

      const count = result.data ?? ids.length
      toast.success(
        t('Successfully deleted {{count}} redemption code(s)', { count })
      )
      props.table.resetRowSelection()
      triggerRefresh()
      props.onOpenChange(false)
    } catch {
      toast.error(t('Unexpected error'))
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <ConfirmDialog
      destructive
      open={props.open}
      onOpenChange={props.onOpenChange}
      handleConfirm={handleConfirm}
      isLoading={isDeleting}
      className='max-w-md'
      title={t('Delete {{count}} redemption code(s)?', {
        count: selectedRows.length,
      })}
      desc={
        <>
          {t('You are about to delete {{count}} redemption code(s).', {
            count: selectedRows.length,
          })}{' '}
          <br />
          {t('This action cannot be undone.')}
        </>
      }
      confirmText={t('Delete')}
    />
  )
}
