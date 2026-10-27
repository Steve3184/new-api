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
import { Crown, RefreshCw } from 'lucide-react'
import {
  useState,
  useEffect,
  useMemo,
  useCallback,
  type ReactNode,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { dotColorMap, textColorMap } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader } from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { TitledCard } from '@/components/ui/titled-card'
import {
  getPublicPlans,
  getSelfSubscriptionFull,
  updateBillingPreference,
} from '@/features/subscriptions/api'
import { SubscriptionPurchaseDialog } from '@/features/subscriptions/components/dialogs/subscription-purchase-dialog'
import { SubscriptionUpgradeDialog } from '@/features/subscriptions/components/dialogs/subscription-upgrade-dialog'
import {
  getUpgradeOptions,
  getUpgradePlanIds,
  isActiveSubscription,
} from '@/features/subscriptions/lib'
import type {
  PlanRecord,
  SubscriptionPlan,
  UserSubscription,
  UserSubscriptionRecord,
} from '@/features/subscriptions/types'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { cn } from '@/lib/utils'

import type { NowPaymentsInvoice, PaymentMethod, TopupInfo } from '../types'
import { NowPaymentsPaymentDialog } from './dialogs/nowpayments-payment-dialog'
import { MySubscriptionItem } from './my-subscription-item'
import { SubscriptionPlanTile } from './subscription-plan-tile'

interface SubscriptionSectionProps {
  topupInfo: TopupInfo | null
  /**
   * Shown beside the user's subscriptions; the plan cards get their own
   * full-width row below. It is shown alone when no plan is sold.
   */
  recharge?: ReactNode
  userQuota?: number
  onPurchaseSuccess?: () => void | Promise<void>
}

function getEpayMethods(payMethods: PaymentMethod[] = []): PaymentMethod[] {
  return payMethods.filter(
    (m) =>
      m?.type &&
      m.type !== 'stripe' &&
      m.type !== 'creem' &&
      m.type !== 'waffo_pancake' &&
      m.type !== 'waffo' &&
      m.type !== 'monero' &&
      m.type !== 'nowpayments'
  )
}

function getBillingPreferenceLabel(
  preference: string,
  t: (key: string) => string
): string {
  switch (preference) {
    case 'subscription_first':
      return t('Subscription First')
    case 'wallet_first':
      return t('Wallet First')
    case 'subscription_only':
      return t('Subscription Only')
    case 'wallet_only':
      return t('Wallet Only')
    default:
      return preference
  }
}

// Plan grid columns by plan count, so a few plans are not stretched thin.
const PLAN_GRID_COLUMNS: Record<number, string> = {
  1: '@2xl:grid-cols-2',
  2: '@2xl:grid-cols-2',
  3: '@2xl:grid-cols-2 @4xl:grid-cols-3',
  4: '@2xl:grid-cols-2 @4xl:grid-cols-3 @6xl:grid-cols-4',
}

const TOP_ROW_CLASS =
  'grid gap-4 xl:grid-cols-[minmax(0,1.05fr)_minmax(360px,0.95fr)] xl:items-start'

/**
 * The wallet's subscription area: a top row with the recharge card and the
 * user's subscriptions, and a full-width row of purchasable plans.
 */
export function SubscriptionSection({
  topupInfo,
  recharge,
  userQuota,
  onPurchaseSuccess,
}: SubscriptionSectionProps) {
  const { t } = useTranslation()

  const [plans, setPlans] = useState<PlanRecord[]>([])
  const [activeSubscriptions, setActiveSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [allSubscriptions, setAllSubscriptions] = useState<
    UserSubscriptionRecord[]
  >([])
  const [billingPreference, setBillingPreference] =
    useState('subscription_first')
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)

  const [purchaseOpen, setPurchaseOpen] = useState(false)
  const [selectedPlan, setSelectedPlan] = useState<PlanRecord | null>(null)
  const [selectedMultiplier, setSelectedMultiplier] = useState(1)
  const [upgradeOpen, setUpgradeOpen] = useState(false)
  const [upgradeSubscription, setUpgradeSubscription] =
    useState<UserSubscription | null>(null)
  const [upgradeTarget, setUpgradeTarget] = useState<string | undefined>()
  const [nowPaymentsInvoice, setNowPaymentsInvoice] =
    useState<NowPaymentsInvoice | null>(null)
  const [nowPaymentsDialogOpen, setNowPaymentsDialogOpen] = useState(false)

  const enableStripe = !!topupInfo?.enable_stripe_topup
  const enableCreem = !!topupInfo?.enable_creem_topup
  const enableWaffoPancake = !!topupInfo?.enable_waffo_pancake_topup
  const enableNowPayments = !!topupInfo?.enable_nowpayments_topup
  const enableOnlineTopUp = !!topupInfo?.enable_online_topup
  const epayMethods = useMemo(
    () => getEpayMethods(topupInfo?.pay_methods),
    [topupInfo?.pay_methods]
  )

  const fetchPlans = useCallback(async () => {
    try {
      const res = requireServerSuccess(await getPublicPlans())
      if (res.success) {
        setPlans(res.data || [])
      }
    } catch (error) {
      handleServerError(error)
      setPlans([])
    }
  }, [])

  const fetchSelfSubscription = useCallback(async () => {
    try {
      const res = requireServerSuccess(await getSelfSubscriptionFull())
      if (res.success && res.data) {
        setBillingPreference(
          res.data.billing_preference || 'subscription_first'
        )
        setActiveSubscriptions(res.data.subscriptions || [])
        setAllSubscriptions(res.data.all_subscriptions || [])
      }
    } catch (error) {
      handleServerError(error)
    }
  }, [])

  useEffect(() => {
    const init = async () => {
      setLoading(true)
      await Promise.all([fetchPlans(), fetchSelfSubscription()])
      setLoading(false)
    }
    init()
  }, [fetchPlans, fetchSelfSubscription])

  const handleRefresh = async () => {
    setRefreshing(true)
    try {
      await fetchSelfSubscription()
    } finally {
      setRefreshing(false)
    }
  }

  const handlePreferenceChange = async (pref: string) => {
    const previous = billingPreference
    setBillingPreference(pref)
    try {
      const res = await updateBillingPreference(pref)
      if (res.success) {
        toast.success(t('Updated successfully'))
        const normalized = res.data?.billing_preference || pref
        setBillingPreference(normalized)
      } else {
        handleServerError(res, t('Update failed'))
        setBillingPreference(previous)
      }
    } catch (error) {
      handleServerError(error, t('Request failed'))
      setBillingPreference(previous)
    }
  }

  const hasActive = activeSubscriptions.length > 0
  const hasAny = allSubscriptions.length > 0
  const disablePref = !hasActive
  const isSubPref =
    billingPreference === 'subscription_first' ||
    billingPreference === 'subscription_only'

  const planPurchaseCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const sub of allSubscriptions) {
      const planId = sub?.subscription?.plan_id
      if (!planId) continue
      map.set(planId, (map.get(planId) || 0) + 1)
    }
    return map
  }, [allSubscriptions])

  const planActiveCountMap = useMemo(() => {
    const map = new Map<number, number>()
    for (const entry of activeSubscriptions) {
      const sub = entry.subscription
      if (
        !sub ||
        sub.status !== 'active' ||
        sub.end_time <= Date.now() / 1000
      ) {
        continue
      }
      map.set(sub.plan_id, (map.get(sub.plan_id) || 0) + 1)
    }
    return map
  }, [activeSubscriptions])

  const publicPlans = useMemo(() => plans.map((record) => record.plan), [plans])

  const activeSubscriptionList = useMemo(() => {
    const now = Date.now() / 1000
    return activeSubscriptions
      .map((entry) => entry.subscription)
      .filter((sub) => sub && isActiveSubscription(sub, now))
  }, [activeSubscriptions])

  const openUpgrade = (
    subscription: UserSubscription,
    target?: { plan: SubscriptionPlan; multiplier: number }
  ) => {
    setUpgradeSubscription(subscription)
    setUpgradeTarget(
      target ? `${target.plan.id}:${target.multiplier}` : undefined
    )
    setUpgradeOpen(true)
  }

  const handleSubscriptionChanged = async () => {
    await fetchSelfSubscription()
    await onPurchaseSuccess?.()
  }

  const showSubscriptions = loading || plans.length > 0 || hasAny

  const preferenceControls = (
    <div className='flex items-center gap-2'>
      <Select
        items={[
          {
            value: 'subscription_first',
            label: (
              <>
                {getBillingPreferenceLabel('subscription_first', t)}
                {disablePref ? ` (${t('No Active')})` : ''}
              </>
            ),
          },
          {
            value: 'wallet_first',
            label: getBillingPreferenceLabel('wallet_first', t),
          },
          {
            value: 'subscription_only',
            label: (
              <>
                {getBillingPreferenceLabel('subscription_only', t)}
                {disablePref ? ` (${t('No Active')})` : ''}
              </>
            ),
          },
          {
            value: 'wallet_only',
            label: getBillingPreferenceLabel('wallet_only', t),
          },
        ]}
        value={billingPreference}
        onValueChange={(v) => v !== null && handlePreferenceChange(v)}
      >
        <SelectTrigger className='h-8 w-[140px] text-xs'>
          <SelectValue>
            {getBillingPreferenceLabel(billingPreference, t)}
          </SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            <SelectItem value='subscription_first' disabled={disablePref}>
              {getBillingPreferenceLabel('subscription_first', t)}
              {disablePref ? ` (${t('No Active')})` : ''}
            </SelectItem>
            <SelectItem value='wallet_first'>
              {getBillingPreferenceLabel('wallet_first', t)}
            </SelectItem>
            <SelectItem value='subscription_only' disabled={disablePref}>
              {getBillingPreferenceLabel('subscription_only', t)}
              {disablePref ? ` (${t('No Active')})` : ''}
            </SelectItem>
            <SelectItem value='wallet_only'>
              {getBillingPreferenceLabel('wallet_only', t)}
            </SelectItem>
          </SelectGroup>
        </SelectContent>
      </Select>
      <Button
        variant='ghost'
        size='icon'
        className='h-8 w-8'
        aria-label={t('Refresh')}
        onClick={handleRefresh}
        disabled={refreshing}
      >
        <RefreshCw
          className={`h-3.5 w-3.5 ${refreshing ? 'animate-spin' : ''}`}
        />
      </Button>
    </div>
  )

  const subscriptionStatus = (
    <span className='flex items-center gap-1.5 text-xs font-medium'>
      <span
        className={cn(
          'size-1.5 shrink-0 rounded-full',
          hasActive ? dotColorMap.success : dotColorMap.neutral
        )}
        aria-hidden='true'
      />
      {hasActive ? (
        <span className={cn(textColorMap.success)}>
          {activeSubscriptions.length} {t('active')}
        </span>
      ) : (
        <span className='text-muted-foreground'>{t('No Active')}</span>
      )}
      {allSubscriptions.length > activeSubscriptions.length && (
        <>
          <span className='text-muted-foreground/30'>·</span>
          <span className='text-muted-foreground'>
            {allSubscriptions.length - activeSubscriptions.length}{' '}
            {t('expired')}
          </span>
        </>
      )}
    </span>
  )

  return (
    <>
      {/* One wrapper for every state keeps the recharge card mounted. */}
      <div className={showSubscriptions ? TOP_ROW_CLASS : 'grid gap-4'}>
        {recharge}
        {loading && (
          <Card data-card-hover='false' className='gap-0 overflow-hidden py-0'>
            <CardHeader className='border-b p-3 !pb-3 sm:p-5 sm:!pb-5'>
              <Skeleton className='h-6 w-32' />
            </CardHeader>
            <CardContent className='space-y-3 p-3 sm:p-5'>
              <Skeleton className='h-24 w-full' />
              <Skeleton className='h-24 w-full' />
            </CardContent>
          </Card>
        )}
        {!loading && showSubscriptions && (
          <TitledCard
            title={t('My Subscriptions')}
            description={subscriptionStatus}
            icon={<Crown className='h-4 w-4' />}
            iconTone='warning'
            action={preferenceControls}
            disableHoverEffect
            contentClassName='space-y-3'
          >
            {disablePref && isSubPref && (
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Preference saved as {{pref}}, but no active subscription. Wallet will be used automatically.',
                  { pref: getBillingPreferenceLabel(billingPreference, t) }
                )}
              </p>
            )}
            {hasAny ? (
              <div className='max-h-[32rem] space-y-2.5 overflow-y-auto pr-1'>
                {allSubscriptions.map((entry) => {
                  const subscription = entry.subscription
                  if (!subscription) return null
                  return (
                    <MySubscriptionItem
                      key={subscription.id}
                      subscription={subscription}
                      plan={publicPlans.find(
                        (plan) => plan.id === subscription.plan_id
                      )}
                      canUpgrade={
                        isActiveSubscription(subscription, Date.now() / 1000) &&
                        getUpgradeOptions(subscription, publicPlans).length > 0
                      }
                      onUpgrade={(sub) => openUpgrade(sub)}
                      onChanged={handleSubscriptionChanged}
                    />
                  )
                })}
              </div>
            ) : (
              <p className='text-muted-foreground text-xs'>
                {t('Subscribe to a plan for model access')}
              </p>
            )}
          </TitledCard>
        )}
      </div>

      {loading && (
        <div className='grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3'>
          {['first', 'second', 'third'].map((key) => (
            <Skeleton key={key} className='h-72 w-full rounded-xl' />
          ))}
        </div>
      )}
      {!loading && plans.length > 0 && (
        <TitledCard
          title={t('Subscription Plans')}
          description={t('Subscribe to a plan for model access')}
          icon={<Crown className='h-4 w-4' />}
          iconTone='warning'
          disableHoverEffect
        >
          <div className='@container'>
            <div
              className={cn(
                'grid grid-cols-1 gap-3 sm:gap-4',
                PLAN_GRID_COLUMNS[Math.min(plans.length, 4)]
              )}
            >
              {plans.map((record, index) => {
                const plan = record.plan
                if (!plan) return null
                const currentSubscription = activeSubscriptionList
                  .filter((sub) => sub.plan_id === plan.id)
                  .sort(
                    (a, b) =>
                      Number(b.multiplier || 1) - Number(a.multiplier || 1)
                  )[0]
                const upgradeSource = activeSubscriptionList.find((sub) => {
                  const source = publicPlans.find((p) => p.id === sub.plan_id)
                  return (
                    !!source &&
                    plan.allow_balance_pay !== false &&
                    getUpgradePlanIds(source).includes(plan.id)
                  )
                })
                return (
                  <SubscriptionPlanTile
                    key={plan.id}
                    plan={plan}
                    recommended={index === 0 && plans.length > 1}
                    purchaseCount={planPurchaseCountMap.get(plan.id) || 0}
                    activeCount={planActiveCountMap.get(plan.id) || 0}
                    currentSubscription={currentSubscription}
                    upgradeSource={upgradeSource}
                    onSubscribe={(tierPlan, multiplier) => {
                      setSelectedPlan({ plan: tierPlan })
                      setSelectedMultiplier(multiplier)
                      setPurchaseOpen(true)
                    }}
                    onUpgrade={(subscription, targetPlan, multiplier) =>
                      openUpgrade(subscription, {
                        plan: targetPlan,
                        multiplier,
                      })
                    }
                  />
                )
              })}
            </div>
          </div>
        </TitledCard>
      )}

      <SubscriptionPurchaseDialog
        open={purchaseOpen}
        onOpenChange={(open) => {
          setPurchaseOpen(open)
          if (!open) {
            fetchSelfSubscription()
          }
        }}
        plan={selectedPlan}
        multiplier={selectedMultiplier}
        activePurchaseLimit={selectedPlan?.plan.max_active_per_user}
        activePurchaseCount={
          selectedPlan ? planActiveCountMap.get(selectedPlan.plan.id) : 0
        }
        enableStripe={enableStripe}
        enableCreem={enableCreem}
        enableWaffoPancake={enableWaffoPancake}
        enableNowPayments={enableNowPayments}
        nowPaymentsCurrencies={topupInfo?.nowpayments_pay_currencies}
        enableOnlineTopUp={enableOnlineTopUp}
        epayMethods={epayMethods}
        paymentMethods={topupInfo?.pay_methods}
        userQuota={userQuota}
        onPurchaseSuccess={onPurchaseSuccess}
        onNowPaymentsInvoice={(invoice) => {
          setNowPaymentsInvoice(invoice)
          setNowPaymentsDialogOpen(true)
        }}
        purchaseLimit={
          selectedPlan?.plan?.max_purchase_per_user
            ? Number(selectedPlan.plan.max_purchase_per_user)
            : undefined
        }
        purchaseCount={
          selectedPlan?.plan?.id
            ? planPurchaseCountMap.get(selectedPlan.plan.id)
            : undefined
        }
      />
      <SubscriptionUpgradeDialog
        open={upgradeOpen}
        onOpenChange={setUpgradeOpen}
        subscription={upgradeSubscription}
        plans={publicPlans}
        initialTarget={upgradeTarget}
        userQuota={userQuota}
        onUpgraded={handleSubscriptionChanged}
      />
      <NowPaymentsPaymentDialog
        open={nowPaymentsDialogOpen}
        onOpenChange={setNowPaymentsDialogOpen}
        invoice={nowPaymentsInvoice}
        onPaymentSuccess={() => {
          void fetchSelfSubscription()
          void onPurchaseSuccess?.()
        }}
      />
    </>
  )
}
