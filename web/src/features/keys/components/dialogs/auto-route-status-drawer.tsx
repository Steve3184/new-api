import { useQuery } from '@tanstack/react-query'
import {
  Ban,
  CheckCircle2,
  CircleDashed,
  TriangleAlert,
} from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  sideDrawerContentClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { cn } from '@/lib/utils'

import { getTokenAutoRouteStatus } from '../../api'
import type { ApiKey, TokenAutoRouteModelStatus } from '../../types'

type AutoRouteStatusDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  token: ApiKey | null
}

function statusCopy(
  t: (key: string) => string,
  state: string
): string {
  switch (state) {
    case 'available':
      return t('Available')
    case 'degraded':
      return t('Degraded')
    case 'auto_disabled':
      return t('Auto-disabled')
    case 'disabled':
      return t('Disabled')
    default:
      return t('Unavailable')
  }
}

function stateClass(state: string): string {
  switch (state) {
    case 'available':
      return 'border-emerald-500/40 bg-emerald-500/10 text-emerald-700 dark:text-emerald-300'
    case 'degraded':
      return 'border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-300'
    case 'auto_disabled':
    case 'disabled':
      return 'border-rose-500/40 bg-rose-500/10 text-rose-700 dark:text-rose-300'
    default:
      return 'border-muted-foreground/30 bg-muted text-muted-foreground'
  }
}

function StateIcon({ state }: { state: string }) {
  if (state === 'available') return <CheckCircle2 className='size-4' />
  if (state === 'degraded') return <TriangleAlert className='size-4' />
  if (state === 'auto_disabled' || state === 'disabled') {
    return <Ban className='size-4' />
  }
  return <CircleDashed className='size-4' />
}

function RouteNode({
  status,
  t,
}: {
  status: TokenAutoRouteModelStatus
  t: (key: string) => string
}) {
  return (
    <div
      className={cn(
        'min-w-36 flex-1 rounded-lg border px-3 py-2.5',
        stateClass(status.state)
      )}
      title={status.last_reason || undefined}
    >
      <div className='flex items-center gap-2'>
        <StateIcon state={status.state} />
        <span className='min-w-0 truncate text-sm font-semibold'>
          {status.model}
        </span>
      </div>
      <div className='mt-2 flex items-center justify-between gap-2 text-[11px]'>
        <span>{statusCopy(t, status.state)}</span>
        <span className='tabular-nums'>
          {status.enabled_channels}/{status.total_channels}
        </span>
      </div>
      {status.last_reason && (
        <p className='mt-1 truncate text-[10px]'>{status.last_reason}</p>
      )}
    </div>
  )
}

export function AutoRouteStatusDrawer({
  open,
  onOpenChange,
  token,
}: AutoRouteStatusDrawerProps) {
  const { t } = useTranslation()
  const tokenId = token?.id
  const { data, isLoading, isFetching, refetch } = useQuery({
    queryKey: ['token-auto-route-status', tokenId],
    queryFn: async () => {
      const result = await getTokenAutoRouteStatus(tokenId || 0)
      if (!result.success) throw new Error(result.message || 'request failed')
      return result.data
    },
    enabled: open && tokenId !== undefined,
    refetchInterval: 5_000,
    refetchIntervalInBackground: false,
    staleTime: 0,
  })
  const routeCount = data?.routes?.length || 0
  const autoGroups = useMemo(() => data?.auto_groups || [], [data])

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent
        className={sideDrawerContentClassName('max-w-none sm:!max-w-[720px]')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <div className='flex items-center justify-between gap-3 pr-8'>
            <div className='min-w-0'>
              <SheetTitle className='truncate'>
                {t('Virtual model routes')}
              </SheetTitle>
              <SheetDescription className='truncate'>
                {token?.name || t('API key')}
              </SheetDescription>
            </div>
            <StatusBadge
              label={isFetching ? t('Updating') : t('Live')}
              variant={isFetching ? 'warning' : 'success'}
              copyable={false}
            />
          </div>
        </SheetHeader>
        <div className='min-h-0 flex-1 space-y-5 overflow-y-auto px-4 pb-6'>
          <div className='flex flex-wrap items-center gap-2 text-xs'>
            <span className='text-muted-foreground'>
              {t('{{count}} virtual models', { count: routeCount })}
            </span>
            {autoGroups.map((group) => (
              <span
                key={group}
                className='bg-muted rounded px-2 py-1 font-medium'
              >
                {group}
              </span>
            ))}
            {data?.updated_at ? (
              <span className='text-muted-foreground ml-auto tabular-nums'>
                {t('Updated {{time}}', {
                  time: new Date(data.updated_at * 1000).toLocaleTimeString(),
                })}
              </span>
            ) : null}
          </div>

          {isLoading ? (
            <div className='text-muted-foreground rounded-lg border border-dashed p-8 text-center text-sm'>
              {t('Loading route status...')}
            </div>
          ) : routeCount === 0 ? (
            <div className='text-muted-foreground rounded-lg border border-dashed p-8 text-center text-sm'>
              {t('No virtual models configured')}
            </div>
          ) : (
            data?.routes.map((route) => {
              const statusByModel = new Map(
                route.models.map((status) => [status.model, status])
              )
              return (
                <section key={route.virtual_model} className='space-y-3'>
                  <div className='flex items-center justify-between gap-3'>
                    <h3 className='text-sm font-semibold'>
                      {route.virtual_model}
                    </h3>
                    <span className='text-muted-foreground text-xs'>
                      {t('{{count}} models', { count: route.chain.length })}
                    </span>
                  </div>
                  <div className='flex items-stretch gap-2 overflow-x-auto pb-1'>
                    {route.chain.map((modelName, index) => {
                      const status =
                        statusByModel.get(modelName) ||
                        ({
                          model: modelName,
                          state: 'unavailable',
                          total_channels: 0,
                          enabled_channels: 0,
                          auto_disabled: 0,
                          manual_disabled: 0,
                          groups: [],
                        } satisfies TokenAutoRouteModelStatus)
                      return (
                        <div
                          key={`${route.virtual_model}-${modelName}`}
                          className='flex min-w-0 flex-1 items-center gap-2'
                        >
                          <RouteNode status={status} t={t} />
                          {index < route.chain.length - 1 && (
                            <div className='text-muted-foreground shrink-0 text-lg'>
                              &rarr;
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>
                  <div className='flex flex-wrap gap-2'>
                    {route.models.map((status) => (
                      <span
                        key={`${route.virtual_model}-${status.model}-groups`}
                        className='text-muted-foreground text-[11px]'
                      >
                        {status.model}: {status.groups.map((group) => `${group.group} ${group.enabled_channels}/${group.total_channels}`).join(' · ')}
                      </span>
                    ))}
                  </div>
                </section>
              )
            })
          )}
          <Button
            type='button'
            variant='outline'
            size='sm'
            onClick={() => void refetch()}
            disabled={isFetching}
          >
            {t('Refresh now')}
          </Button>
        </div>
      </SheetContent>
    </Sheet>
  )
}
