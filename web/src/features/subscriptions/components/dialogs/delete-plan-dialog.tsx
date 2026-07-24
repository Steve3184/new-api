/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'

import { deletePlan } from '../../api'
import { useSubscriptions } from '../subscriptions-provider'

export function DeletePlanDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useSubscriptions()
  const [loading, setLoading] = useState(false)
  const isOpen = open === 'delete'
  const plan = currentRow?.plan

  const handleConfirm = async () => {
    if (!plan?.id) return
    setLoading(true)
    try {
      const result = await deletePlan(plan.id)
      if (result.success) {
        toast.success(t('Subscription plan deleted'))
        triggerRefresh()
        setOpen(null)
      } else {
        toast.error(result.message || t('Operation failed'))
      }
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <ConfirmDialog
      open={isOpen}
      onOpenChange={(nextOpen) => !nextOpen && setOpen(null)}
      title={t('Delete subscription plan?')}
      desc={t(
        'This permanently deletes {{plan}} and is allowed only when it has no active user subscriptions.',
        { plan: plan?.title || `#${plan?.id ?? ''}` }
      )}
      confirmText={t('Delete')}
      destructive
      handleConfirm={handleConfirm}
      isLoading={loading}
      disabled={!plan?.id}
    />
  )
}
