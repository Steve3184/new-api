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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Switch } from '@/components/ui/switch'

import { resetPlanSubscriptions } from '../../api'
import { useSubscriptions } from '../subscriptions-provider'

export function ResetSubscriptionsDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useSubscriptions()
  const [advanceResetTime, setAdvanceResetTime] = useState(true)
  const [resetTotal, setResetTotal] = useState(true)
  const [resetFiveHour, setResetFiveHour] = useState(true)
  const [resetWeekly, setResetWeekly] = useState(true)
  const [resetting, setResetting] = useState(false)
  const isOpen = open === 'reset-subscriptions'
  const plan = currentRow?.plan
  const planLabel = plan?.title || (plan?.id ? `#${plan.id}` : '-')

  useEffect(() => {
    if (isOpen) {
      setAdvanceResetTime(true)
      setResetTotal(true)
      setResetFiveHour(true)
      setResetWeekly(true)
    }
  }, [isOpen])

  const handleConfirm = async () => {
    if (!plan?.id) return
    setResetting(true)
    try {
      const res = await resetPlanSubscriptions(plan.id, {
        advance_reset_time: advanceResetTime,
        reset_total: resetTotal,
        reset_five_hour: resetFiveHour,
        reset_weekly: resetWeekly,
      })
      if (res.success) {
        toast.success(
          t('Reset {{count}} active subscriptions', {
            count: res.data?.reset_count || 0,
          })
        )
        triggerRefresh()
        setOpen(null)
      }
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setResetting(false)
    }
  }

  return (
    <ConfirmDialog
      open={isOpen}
      onOpenChange={(nextOpen) => !nextOpen && setOpen(null)}
      title={t('Reset subscription quota')}
      desc={t('Reset all active subscriptions under {{plan}}?', {
        plan: planLabel,
      })}
      confirmText={t('Reset quota')}
      handleConfirm={handleConfirm}
      disabled={!plan?.id}
      isLoading={resetting}
    >
      <label className='flex items-center justify-between gap-3 rounded-md border px-3 py-2 text-sm'>
        <span>{t('Advance next reset time')}</span>
        <Switch
          checked={advanceResetTime}
          onCheckedChange={(checked) => setAdvanceResetTime(!!checked)}
          aria-label={t('Advance next reset time')}
        />
      </label>
      <div className='grid gap-2 rounded-md border p-3'>
        <label className='flex items-center justify-between gap-3 text-sm'>
          <span>{t('Reset total quota')}</span>
          <Switch
            checked={resetTotal}
            onCheckedChange={(checked) => setResetTotal(!!checked)}
            aria-label={t('Reset total quota')}
          />
        </label>
        <label className='flex items-center justify-between gap-3 text-sm'>
          <span>{t('Reset 5-hour quota')}</span>
          <Switch
            checked={resetFiveHour}
            onCheckedChange={(checked) => setResetFiveHour(!!checked)}
            aria-label={t('Reset 5-hour quota')}
          />
        </label>
        <label className='flex items-center justify-between gap-3 text-sm'>
          <span>{t('Reset weekly quota')}</span>
          <Switch
            checked={resetWeekly}
            onCheckedChange={(checked) => setResetWeekly(!!checked)}
            aria-label={t('Reset weekly quota')}
          />
        </label>
      </div>
    </ConfirmDialog>
  )
}
