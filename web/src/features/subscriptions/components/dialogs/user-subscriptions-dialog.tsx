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
import { Ban, Plus, RotateCcw, Trash2 } from 'lucide-react'
import { useCallback, useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import {
  DataTableRowActionMenu,
  StaticDataTable,
} from '@/components/data-table'
import {
  sideDrawerContentClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetTitle,
  SheetDescription,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { formatBillingCurrencyFromUSD } from '@/lib/currency'
import { formatQuota } from '@/lib/format'
import {
  getServerMessage,
  requireServerSuccess,
} from '@/lib/server-error-message'

import {
  getAdminPlans,
  getUserSubscriptions,
  createUserSubscription,
  invalidateUserSubscription,
  deleteUserSubscription,
  resetUserSubscriptionsByPlan,
  setUserSubscriptionTier,
} from '../../api'
import {
  formatMultiplier,
  formatTimestamp,
  getUsageTierOptions,
} from '../../lib'
import type { PlanRecord, UserSubscriptionRecord } from '../../types'

interface Props {
  open: boolean
  onOpenChange: (open: boolean) => void
  user: { id: number; username?: string } | null
  onSuccess?: () => void
}

function SubscriptionStatusBadge(props: {
  sub: UserSubscriptionRecord['subscription']
  t: (key: string) => string
}) {
  // eslint-disable-next-line react-hooks/purity
  const now = Date.now() / 1000
  const isExpired = (props.sub.end_time || 0) > 0 && props.sub.end_time < now
  const isActive = props.sub.status === 'active' && !isExpired
  if (isActive) {
    return (
      <StatusBadge
        label={props.t('Active')}
        variant='success'
        copyable={false}
      />
    )
  }
  if (props.sub.status === 'cancelled') {
    return (
      <StatusBadge
        label={props.t('Invalidated')}
        variant='neutral'
        copyable={false}
      />
    )
  }
  return (
    <StatusBadge
      label={props.t('Expired')}
      variant='neutral'
      copyable={false}
    />
  )
}

export function UserSubscriptionsDialog(props: Props) {
  const { t } = useTranslation()
  const [loading, setLoading] = useState(false)
  const [creating, setCreating] = useState(false)
  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [subs, setSubs] = useState<UserSubscriptionRecord[]>([])
  const [selectedPlanId, setSelectedPlanId] = useState<string>('')
  const [multiplier, setMultiplier] = useState(1)
  const [autoRenew, setAutoRenew] = useState(true)
  const [tierEdit, setTierEdit] = useState<
    UserSubscriptionRecord['subscription'] | null
  >(null)
  const [editMultiplier, setEditMultiplier] = useState(1)
  const [savingTier, setSavingTier] = useState(false)
  const selectedPlan = plans.find(
    ({ plan }) => String(plan.id) === selectedPlanId
  )?.plan
  const canAutoRenew =
    selectedPlan?.enabled &&
    selectedPlan.allow_auto_renew &&
    selectedPlan.allow_balance_pay !== false
  const grantTiers = selectedPlan ? getUsageTierOptions(selectedPlan) : []
  const editPlan = plans.find(({ plan }) => plan.id === tierEdit?.plan_id)?.plan
  const editTiers = editPlan ? getUsageTierOptions(editPlan) : []
  const [resetting, setResetting] = useState(false)
  const [advanceResetTime, setAdvanceResetTime] = useState(true)
  const [resetTotal, setResetTotal] = useState(true)
  const [resetFiveHour, setResetFiveHour] = useState(true)
  const [resetWeekly, setResetWeekly] = useState(true)
  const [resetAction, setResetAction] = useState<{
    planId: number
    planTitle: string
  } | null>(null)
  const [confirmAction, setConfirmAction] = useState<{
    type: 'invalidate' | 'delete'
    subId: number
  } | null>(null)

  const planTitleMap = useMemo(() => {
    const map = new Map<number, string>()
    plans.forEach((p) => {
      if (p.plan.id) map.set(p.plan.id, p.plan.title || `#${p.plan.id}`)
    })
    return map
  }, [plans])

  const loadData = useCallback(async () => {
    if (!props.user?.id) return
    setLoading(true)
    try {
      const [plansRes, subsRes] = await Promise.all([
        getAdminPlans(),
        getUserSubscriptions(props.user.id),
      ])
      if (plansRes.success) setPlans(plansRes.data || [])
      if (subsRes.success) setSubs(subsRes.data || [])
    } catch {
      toast.error(t('Loading failed'))
    } finally {
      setLoading(false)
    }
  }, [props.user?.id, t])

  useEffect(() => {
    if (props.open && props.user?.id) {
      setSelectedPlanId('')
      setTierEdit(null)
      loadData()
    }
  }, [props.open, props.user?.id, loadData])

  const handleCreate = async () => {
    if (!props.user?.id || !selectedPlanId) {
      toast.error(t('Please select a subscription plan'))
      return
    }
    setCreating(true)
    try {
      const res = await createUserSubscription(props.user.id, {
        plan_id: Number(selectedPlanId),
        multiplier,
        auto_renew: Boolean(canAutoRenew && autoRenew),
      })
      requireServerSuccess(res)
      if (res.success) {
        toast.success(getServerMessage(res.data) || t('Added successfully'))
        setSelectedPlanId('')
        await loadData()
        props.onSuccess?.()
      }
    } catch {
      toast.error(t('Request failed'))
    } finally {
      setCreating(false)
    }
  }

  const handleTierChange = async () => {
    if (
      !props.user?.id ||
      !tierEdit ||
      !editTiers.some((tier) => tier.multiplier === editMultiplier)
    ) {
      return
    }
    setSavingTier(true)
    try {
      requireServerSuccess(
        await setUserSubscriptionTier(
          props.user.id,
          tierEdit.id,
          editMultiplier
        )
      )
      setTierEdit(null)
      toast.success(t('Usage tier updated'))
      await loadData()
      props.onSuccess?.()
    } catch (error) {
      toast.error(getServerMessage(error) || t('Operation failed'))
    } finally {
      setSavingTier(false)
    }
  }

  const renderTierPicker = (
    tiers: typeof grantTiers,
    value: number,
    setValue: (value: number) => void
  ) => (
    <Select
      items={tiers.map(({ multiplier }) => ({
        value: String(multiplier),
        label: formatMultiplier(multiplier),
      }))}
      value={String(value)}
      onValueChange={(value) => value !== null && setValue(Number(value))}
    >
      <SelectTrigger aria-label={t('Usage tier')}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {tiers.map(({ multiplier }) => (
          <SelectItem key={multiplier} value={String(multiplier)}>
            {formatMultiplier(multiplier)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )

  const handleConfirmAction = async () => {
    if (!confirmAction) return
    try {
      if (confirmAction.type === 'invalidate') {
        const res = await invalidateUserSubscription(confirmAction.subId)
        if (res.success) {
          toast.success(getServerMessage(res.data) || t('Has been invalidated'))
          await loadData()
          props.onSuccess?.()
        }
      } else {
        const res = await deleteUserSubscription(confirmAction.subId)
        if (res.success) {
          toast.success(t('Deleted'))
          await loadData()
          props.onSuccess?.()
        }
      }
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setConfirmAction(null)
    }
  }

  const handleResetConfirm = async () => {
    if (!props.user?.id || !resetAction) return
    setResetting(true)
    try {
      const res = await resetUserSubscriptionsByPlan(props.user.id, {
        plan_id: resetAction.planId,
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
        await loadData()
        props.onSuccess?.()
      }
    } catch {
      toast.error(t('Operation failed'))
    } finally {
      setResetting(false)
      setResetAction(null)
    }
  }

  return (
    <>
      <Sheet open={props.open} onOpenChange={props.onOpenChange}>
        <SheetContent className={sideDrawerContentClassName('sm:max-w-2xl')}>
          <SheetHeader className={sideDrawerHeaderClassName()}>
            <SheetTitle>{t('User Subscription Management')}</SheetTitle>
            <SheetDescription>
              {props.user?.username || '-'} (ID: {props.user?.id || '-'})
            </SheetDescription>
          </SheetHeader>

          <div className={sideDrawerFormClassName()}>
            <div className='flex gap-2'>
              <Select
                items={plans.map((p) => ({
                  value: String(p.plan.id),
                  label: (
                    <>
                      {p.plan.title} (
                      {formatBillingCurrencyFromUSD(
                        Number(p.plan.price_amount || 0),
                        { digitsLarge: 2, digitsSmall: 2, abbreviate: false }
                      )}
                      )
                    </>
                  ),
                }))}
                value={selectedPlanId}
                onValueChange={(v) => {
                  if (v === null) return
                  setSelectedPlanId(v)
                  setMultiplier(1)
                  setAutoRenew(true)
                }}
              >
                <SelectTrigger className='flex-1'>
                  <SelectValue placeholder={t('Select subscription plan')} />
                </SelectTrigger>
                <SelectContent alignItemWithTrigger={false}>
                  <SelectGroup>
                    {plans.map((p) => (
                      <SelectItem key={p.plan.id} value={String(p.plan.id)}>
                        {p.plan.title} (
                        {formatBillingCurrencyFromUSD(
                          Number(p.plan.price_amount || 0),
                          { digitsLarge: 2, digitsSmall: 2, abbreviate: false }
                        )}
                        )
                      </SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              {grantTiers.length > 1 &&
                renderTierPicker(grantTiers, multiplier, setMultiplier)}
              <Button
                onClick={handleCreate}
                disabled={creating || !selectedPlanId}
              >
                <Plus className='mr-1 h-4 w-4' />
                {t('Add subscription')}
              </Button>
            </div>

            {canAutoRenew && (
              <label className='flex items-center gap-2 text-sm'>
                <Switch
                  checked={autoRenew}
                  onCheckedChange={setAutoRenew}
                  disabled={creating}
                />
                {t('Auto-renew from balance')}
              </label>
            )}
            <StaticDataTable
              data={loading ? [] : subs}
              getRowKey={(record) => record.subscription.id}
              emptyClassName={loading ? 'py-8' : 'text-muted-foreground py-8'}
              emptyContent={
                loading ? t('Loading...') : t('No subscription records')
              }
              columns={[
                {
                  id: 'id',
                  header: t('ID'),
                  cell: (record) => <TableId value={record.subscription.id} />,
                },
                {
                  id: 'plan',
                  header: t('Plan'),
                  cell: (record) => {
                    const sub = record.subscription

                    return (
                      <div>
                        <div className='font-medium'>
                          {planTitleMap.get(sub.plan_id) || `#${sub.plan_id}`} ·{' '}
                          {formatMultiplier(sub.multiplier)}
                        </div>
                        <div className='text-muted-foreground text-sm'>
                          {t('Source')}: {sub.source || '-'}
                        </div>
                      </div>
                    )
                  },
                },
                {
                  id: 'status',
                  header: t('Status'),
                  cell: (record) => (
                    <SubscriptionStatusBadge sub={record.subscription} t={t} />
                  ),
                },
                {
                  id: 'validity',
                  header: t('Validity'),
                  cell: (record) => {
                    const sub = record.subscription

                    return (
                      <div className='text-sm'>
                        <div>
                          {t('Start')}: {formatTimestamp(sub.start_time)}
                        </div>
                        <div>
                          {t('End')}: {formatTimestamp(sub.end_time)}
                        </div>
                      </div>
                    )
                  },
                },
                {
                  id: 'quota',
                  header: t('Quota per Billing Period'),
                  cell: (record) => {
                    const sub = record.subscription
                    const total = Number(sub.amount_total || 0)
                    const used = Number(sub.amount_used || 0)
                    return total > 0
                      ? `${formatQuota(used)}/${formatQuota(total)}`
                      : t('Unlimited')
                  },
                },
                {
                  id: 'actions',
                  header: t('Actions'),
                  className: 'text-right',
                  cellClassName: 'text-right',
                  cell: (record) => {
                    const sub = record.subscription
                    const now = Date.now() / 1000
                    const isExpired =
                      (sub.end_time || 0) > 0 && sub.end_time < now
                    const isActive = sub.status === 'active' && !isExpired

                    return (
                      <DataTableRowActionMenu ariaLabel={t('Actions')}>
                        {isActive &&
                          plans.some(
                            ({ plan }) =>
                              plan.id === sub.plan_id &&
                              getUsageTierOptions(plan).some(
                                (tier) =>
                                  tier.multiplier !== (sub.multiplier || 1)
                              )
                          ) && (
                            <DropdownMenuItem
                              onClick={() => {
                                setEditMultiplier(sub.multiplier || 1)
                                setTierEdit(sub)
                              }}
                            >
                              {t('Change usage tier')}
                            </DropdownMenuItem>
                          )}
                        <DropdownMenuItem
                          disabled={!isActive}
                          onClick={() => {
                            setAdvanceResetTime(true)
                            setResetTotal(true)
                            setResetFiveHour(true)
                            setResetWeekly(true)
                            setResetAction({
                              planId: sub.plan_id,
                              planTitle:
                                planTitleMap.get(sub.plan_id) ||
                                `#${sub.plan_id}`,
                            })
                          }}
                        >
                          {t('Reset quota')}
                          <DropdownMenuShortcut>
                            <RotateCcw size={16} />
                          </DropdownMenuShortcut>
                        </DropdownMenuItem>
                        <DropdownMenuItem
                          disabled={!isActive}
                          onClick={() =>
                            setConfirmAction({
                              type: 'invalidate',
                              subId: sub.id,
                            })
                          }
                        >
                          {t('Invalidate')}
                          <DropdownMenuShortcut>
                            <Ban size={16} />
                          </DropdownMenuShortcut>
                        </DropdownMenuItem>
                        <DropdownMenuSeparator />
                        <DropdownMenuItem
                          variant='destructive'
                          onClick={() =>
                            setConfirmAction({
                              type: 'delete',
                              subId: sub.id,
                            })
                          }
                        >
                          {t('Delete')}
                          <DropdownMenuShortcut>
                            <Trash2 size={16} />
                          </DropdownMenuShortcut>
                        </DropdownMenuItem>
                      </DataTableRowActionMenu>
                    )
                  },
                },
              ]}
            />
          </div>
        </SheetContent>
      </Sheet>

      {tierEdit && (
        <ConfirmDialog
          open
          onOpenChange={(open) => !open && !savingTier && setTierEdit(null)}
          title={t('Change usage tier')}
          desc={t(
            'Changes quota limits without resetting usage, changing expiry, or charging the wallet.'
          )}
          handleConfirm={handleTierChange}
          isLoading={savingTier}
          disabled={
            editMultiplier === (tierEdit.multiplier || 1) ||
            !editTiers.some((tier) => tier.multiplier === editMultiplier)
          }
        >
          {renderTierPicker(editTiers, editMultiplier, setEditMultiplier)}
        </ConfirmDialog>
      )}

      {confirmAction && (
        <ConfirmDialog
          open
          onOpenChange={(v) => !v && setConfirmAction(null)}
          title={
            confirmAction.type === 'invalidate'
              ? t('Confirm invalidate')
              : t('Confirm delete')
          }
          desc={
            confirmAction.type === 'invalidate'
              ? t(
                  'After invalidating, this subscription will be immediately deactivated. Historical records are not affected. Continue?'
                )
              : t(
                  'Deleting will permanently remove this subscription record (including benefit details). Continue?'
                )
          }
          handleConfirm={handleConfirmAction}
          destructive={confirmAction.type === 'delete'}
        />
      )}

      {resetAction && (
        <ConfirmDialog
          open
          onOpenChange={(v) => !v && setResetAction(null)}
          title={t('Reset subscription quota')}
          desc={t('Reset active {{plan}} subscriptions for this user?', {
            plan: resetAction.planTitle,
          })}
          confirmText={t('Reset quota')}
          handleConfirm={handleResetConfirm}
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
      )}
    </>
  )
}
