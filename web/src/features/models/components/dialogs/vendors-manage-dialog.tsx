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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Pencil, Plus, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { getLobeIcon } from '@/lib/lobe-icon'

import { getVendors } from '../../api'
import {
  handleDeleteVendor as deleteVendor,
  vendorsQueryKeys,
} from '../../lib'
import type { Vendor } from '../../types'
import { VendorMutateDialog } from './vendor-mutate-dialog'

type VendorsManageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function VendorsManageDialog({
  open,
  onOpenChange,
}: VendorsManageDialogProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editVendor, setEditVendor] = useState<Vendor | null>(null)
  const [isCreateOpen, setIsCreateOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<Vendor | null>(null)
  const [isDeleting, setIsDeleting] = useState(false)

  const { data: vendors, isLoading } = useQuery({
    queryKey: vendorsQueryKeys.lists(),
    queryFn: () => getVendors(),
    enabled: open,
  })

  const vendorList = vendors?.data?.items || []

  const handleEditVendor = (vendor: Vendor) => {
    setEditVendor(vendor)
  }

  const handleDeleteVendor = (vendor: Vendor) => {
    setDeleteTarget(vendor)
  }

  const confirmDelete = async () => {
    if (!deleteTarget) return
    setIsDeleting(true)
    try {
      await deleteVendor(deleteTarget.id, queryClient)
    } finally {
      setIsDeleting(false)
      setDeleteTarget(null)
    }
  }

  let vendorContent = (
    <div className='space-y-2'>
      {[1, 2, 3].map((item) => (
        <Skeleton key={item} className='h-12 w-full' />
      ))}
    </div>
  )
  if (!isLoading && vendorList.length === 0) {
    vendorContent = (
      <p className='text-muted-foreground py-4 text-center text-sm'>
        {t('No vendors found.')}
      </p>
    )
  } else if (!isLoading) {
    vendorContent = (
      <div className='divide-y rounded-md border'>
        {vendorList.map((vendor) => (
          <div key={vendor.id} className='flex items-center gap-3 px-4 py-3'>
            <div className='flex h-8 w-8 shrink-0 items-center justify-center'>
              {vendor.icon ? (
                (getLobeIcon(vendor.icon, 24) ?? (
                  <div className='bg-muted h-6 w-6 rounded' />
                ))
              ) : (
                <div className='bg-muted h-6 w-6 rounded' />
              )}
            </div>
            <div className='min-w-0 flex-1'>
              <p className='truncate text-sm font-medium'>{vendor.name}</p>
              {vendor.description && (
                <p className='text-muted-foreground truncate text-xs'>
                  {vendor.description}
                </p>
              )}
            </div>
            <div className='flex shrink-0 items-center gap-1'>
              <Button
                size='sm'
                variant='ghost'
                onClick={() => handleEditVendor(vendor)}
                aria-label={t('Edit')}
              >
                <Pencil className='h-4 w-4' />
              </Button>
              <Button
                size='sm'
                variant='ghost'
                onClick={() => handleDeleteVendor(vendor)}
                aria-label={t('Delete')}
                className='text-destructive hover:text-destructive'
              >
                <Trash2 className='h-4 w-4' />
              </Button>
            </div>
          </div>
        ))}
      </div>
    )
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={onOpenChange}
        title={t('Manage Vendors')}
        description={t('View, edit, and delete vendors')}
        contentClassName='max-w-2xl'
        contentHeight='auto'
        bodyClassName='space-y-2'
        footer={
          <>
            <Button variant='outline' onClick={() => onOpenChange(false)}>
              {t('Close')}
            </Button>
            <Button onClick={() => setIsCreateOpen(true)}>
              <Plus className='h-4 w-4' />
              {t('Create Vendor')}
            </Button>
          </>
        }
      >
        {vendorContent}
      </Dialog>

      <VendorMutateDialog
        open={isCreateOpen || editVendor !== null}
        onOpenChange={(nextOpen) => {
          if (!nextOpen) {
            setEditVendor(null)
            setIsCreateOpen(false)
          }
        }}
        currentVendor={editVendor}
      />

      <AlertDialog
        open={deleteTarget !== null}
        onOpenChange={(v) => !v && setDeleteTarget(null)}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Are you sure?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Vendor "{{name}}" will be permanently deleted. This cannot be undone.',
                { name: deleteTarget?.name ?? '' }
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={confirmDelete}
              disabled={isDeleting}
            >
              {isDeleting ? t('Deleting...') : t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}
