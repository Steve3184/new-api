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
import { Power, PowerOff, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTableBulkActions as BulkActionsToolbar } from '@/components/data-table'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { deleteUser, manageUser } from '../api'
import type { User } from '../types'
import { useUsers } from './users-provider'

interface DataTableBulkActionsProps {
  table: Table<User>
}

export function DataTableBulkActions({ table }: DataTableBulkActionsProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useUsers()
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const selectedRows = table.getFilteredSelectedRowModel().rows
  const selectedIds = selectedRows
    .map((row) => row.original.id)
    .filter((id): id is number => typeof id === 'number')

  const clearSelection = () => table.resetRowSelection()

  const runManageAction = async (action: 'enable' | 'disable') => {
    try {
      const results = await Promise.all(
        selectedIds.map((id) => manageUser(id, action))
      )
      const failed = results.find((result) => !result.success)
      if (failed) {
        toast.error(
          failed.message || t('Failed to {{action}} user', { action })
        )
      } else {
        toast.success(
          action === 'enable'
            ? t('User enabled successfully')
            : t('User disabled successfully')
        )
      }
      triggerRefresh()
      clearSelection()
    } catch {
      toast.error(t('An unexpected error occurred'))
    }
  }

  const runDeleteAction = async () => {
    try {
      const results = await Promise.all(selectedIds.map((id) => deleteUser(id)))
      const failed = results.find((result) => !result.success)
      if (failed) {
        toast.error(failed.message || t('Failed to delete user'))
      } else {
        toast.success(t('Users deleted successfully'))
      }
      setShowDeleteConfirm(false)
      triggerRefresh()
      clearSelection()
    } catch {
      toast.error(t('An unexpected error occurred'))
    }
  }

  return (
    <>
      <BulkActionsToolbar table={table} entityName='user'>
        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                className='size-8'
                onClick={() => runManageAction('enable')}
                aria-label={t('Enable selected users')}
                title={t('Enable selected users')}
              />
            }
          >
            <Power />
            <span className='sr-only'>{t('Enable selected users')}</span>
          </TooltipTrigger>
          <TooltipContent>{t('Enable selected users')}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='outline'
                size='icon'
                className='size-8'
                onClick={() => runManageAction('disable')}
                aria-label={t('Disable selected users')}
                title={t('Disable selected users')}
              />
            }
          >
            <PowerOff />
            <span className='sr-only'>{t('Disable selected users')}</span>
          </TooltipTrigger>
          <TooltipContent>{t('Disable selected users')}</TooltipContent>
        </Tooltip>

        <Tooltip>
          <TooltipTrigger
            render={
              <Button
                variant='destructive'
                size='icon'
                className='size-8'
                onClick={() => setShowDeleteConfirm(true)}
                aria-label={t('Delete selected users')}
                title={t('Delete selected users')}
              />
            }
          >
            <Trash2 />
            <span className='sr-only'>{t('Delete selected users')}</span>
          </TooltipTrigger>
          <TooltipContent>{t('Delete selected users')}</TooltipContent>
        </Tooltip>
      </BulkActionsToolbar>

      <Dialog
        open={showDeleteConfirm}
        onOpenChange={setShowDeleteConfirm}
        title={t('Delete Users?')}
        description={t(
          'Are you sure you want to delete {{count}} user(s)? This action cannot be undone.',
          { count: selectedIds.length }
        )}
        contentHeight='auto'
        footer={
          <>
            <Button
              variant='outline'
              onClick={() => setShowDeleteConfirm(false)}
            >
              {t('Cancel')}
            </Button>
            <Button variant='destructive' onClick={runDeleteAction}>
              {t('Delete')}
            </Button>
          </>
        }
      >
        {' '}
      </Dialog>
    </>
  )
}
