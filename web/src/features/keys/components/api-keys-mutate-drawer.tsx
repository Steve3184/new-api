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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import {
  ArrowDown,
  ArrowUp,
  ChevronDown,
  Edit,
  KeyRound,
  Plus,
  Settings2,
  Trash2,
  WalletCards,
} from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm, type SubmitErrorHandler } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
} from '@/components/drawer-layout'
import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ModelGroupSelector } from '@/components/model-group-selector'
import { useStatus } from '@/hooks/use-status'
import { getUserModels, getUserGroups } from '@/lib/api'
import { getCurrencyDisplay, getCurrencyLabel } from '@/lib/currency'
import { cn } from '@/lib/utils'

import {
  createApiKey,
  updateApiKey,
  getApiKey,
  getTokenAutoGroups,
} from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  getApiKeyFormSchema,
  type ApiKeyFormValues,
  getApiKeyFormDefaultValues,
  transformFormDataToPayload,
  transformApiKeyToFormDefaults,
} from '../lib'
import type { ApiKey, ApiKeyAutoRoutes } from '../types'
import {
  ApiKeyGroupCombobox,
  type ApiKeyGroupOption,
} from './api-key-group-combobox'
import { useApiKeys } from './api-keys-provider'
import { AutoGroupOrderEditor } from './auto-group-order-editor'

type ApiKeyMutateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  currentRow?: ApiKey
}

export function ApiKeysMutateDrawer({
  open,
  onOpenChange,
  currentRow,
}: ApiKeyMutateDrawerProps) {
  const { t } = useTranslation()
  const isUpdate = !!currentRow
  const currentRowId = currentRow?.id
  const { triggerRefresh } = useApiKeys()
  const { status, loading: statusLoading } = useStatus()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [initializedTarget, setInitializedTarget] = useState<string | null>(
    null
  )
  const [routeEditorOpen, setRouteEditorOpen] = useState(false)
  const [editingVirtualModel, setEditingVirtualModel] = useState<string | null>(
    null
  )
  const [routeName, setRouteName] = useState('')
  const [routeChain, setRouteChain] = useState<string[]>([])
  const [routeModel, setRouteModel] = useState('')
  const [routeGroup, setRouteGroup] = useState('')
  const defaultUseAutoGroup = status?.default_use_auto_group === true

  // Fetch models
  const { data: modelsData } = useQuery({
    queryKey: ['user-models'],
    queryFn: () => getUserModels(),
    enabled: open,
    staleTime: 0,
  })

  const { data: autoModelsData } = useQuery({
    queryKey: ['user-models', 'auto'],
    queryFn: () => getUserModels('auto'),
    enabled: open,
    staleTime: 0,
  })

  // Fetch groups
  const {
    data: groupsData,
    isFetched: groupsFetched,
    isFetching: groupsFetching,
  } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    enabled: open,
    staleTime: 0,
  })

  const {
    data: apiKeyData,
    isFetched: apiKeyFetched,
    isFetching: apiKeyFetching,
  } = useQuery({
    queryKey: ['api-key', currentRowId],
    queryFn: () => getApiKey(currentRowId ?? 0),
    enabled: open && isUpdate && currentRowId !== undefined,
    staleTime: 0,
  })

  const {
    data: autoGroupsData,
    isFetched: autoGroupsFetched,
    isFetching: autoGroupsFetching,
  } = useQuery({
    queryKey: ['token-auto-groups'],
    queryFn: getTokenAutoGroups,
    enabled: open,
    staleTime: 0,
  })

  const models = modelsData?.data || []
  const autoModels = autoModelsData?.data || models
  const groups = useMemo<ApiKeyGroupOption[]>(
    () =>
      Object.entries(groupsData?.data || {}).map(([key, info]) => ({
        value: key,
        label: key,
        desc: info.desc || key,
        ratio: info.ratio,
      })),
    [groupsData]
  )
  const backendHasAuto = groups.some((g) => g.value === 'auto')
  const availableAutoGroupNames = useMemo(
    () => groups.filter((group) => group.value !== 'auto').map((g) => g.value),
    [groups]
  )
  const globalAutoGroups = useMemo(() => {
    const available = new Set(availableAutoGroupNames)
    return (autoGroupsData?.data?.groups || []).filter((group) =>
      available.has(group)
    )
  }, [autoGroupsData, availableAutoGroupNames])
  const globalAutoGroupOptions = useMemo(() => {
    const groupsByValue = new Map(groups.map((group) => [group.value, group]))
    return globalAutoGroups.flatMap((group) => {
      const option = groupsByValue.get(group)
      return option ? [option] : []
    })
  }, [globalAutoGroups, groups])
  const maxAutoGroups =
    Number.isInteger(autoGroupsData?.data?.max_count) &&
    Number(autoGroupsData?.data?.max_count) > 0
      ? Number(autoGroupsData?.data?.max_count)
      : 5
  const schema = useMemo(
    () => getApiKeyFormSchema(t, maxAutoGroups),
    [t, maxAutoGroups]
  )

  const form = useForm<ApiKeyFormValues>({
    resolver: zodResolver(schema),
    defaultValues: getApiKeyFormDefaultValues(defaultUseAutoGroup),
  })

  // Load existing data when updating
  useEffect(() => {
    if (!open) {
      setInitializedTarget(null)
      return
    }
    if (
      !groupsFetched ||
      groupsFetching ||
      !autoGroupsFetched ||
      autoGroupsFetching
    ) {
      return
    }
    if (isUpdate && (!apiKeyFetched || apiKeyFetching)) return
    if (!isUpdate && statusLoading) return

    const target = isUpdate && currentRow ? `update:${currentRow.id}` : 'create'
    if (initializedTarget === target) return
    if (isUpdate && currentRow) {
      if (apiKeyData?.success && apiKeyData.data) {
        form.reset(
          transformApiKeyToFormDefaults(
            apiKeyData.data,
            availableAutoGroupNames,
            maxAutoGroups
          )
        )
        setInitializedTarget(target)
      }
    } else {
      form.reset(
        getApiKeyFormDefaultValues(defaultUseAutoGroup && backendHasAuto)
      )
      setInitializedTarget(target)
    }
  }, [
    open,
    isUpdate,
    currentRow,
    form,
    defaultUseAutoGroup,
    statusLoading,
    backendHasAuto,
    groupsFetched,
    groupsFetching,
    autoGroupsFetched,
    autoGroupsFetching,
    apiKeyData,
    apiKeyFetched,
    apiKeyFetching,
    availableAutoGroupNames,
    maxAutoGroups,
    initializedTarget,
  ])

  const formTarget =
    isUpdate && currentRow ? `update:${currentRow.id}` : 'create'
  const isFormInitialized = initializedTarget === formTarget
  const selectedGroup = form.watch('group')

  // Correct group after groups load: if the form value is not in available groups, fall back
  useEffect(() => {
    if (groups.length === 0) return
    const currentGroup = selectedGroup
    if (currentGroup && !groups.some((g) => g.value === currentGroup)) {
      const fallback =
        groups.find((g) => g.value === 'default')?.value ??
        groups[0]?.value ??
        ''
      form.setValue('group', fallback)
      if (currentGroup === 'auto') {
        form.setValue('auto_groups', [])
        form.setValue('auto_groups_mode', 'inherit')
        form.setValue('cross_group_retry', false)
      }
    }
  }, [groups, form, selectedGroup])

  const onSubmit = async (data: ApiKeyFormValues) => {
    setIsSubmitting(true)
    try {
      const basePayload = transformFormDataToPayload(data)

      if (isUpdate && currentRow) {
        const result = await updateApiKey({
          ...basePayload,
          id: currentRow.id,
        })
        if (result.success) {
          toast.success(t(SUCCESS_MESSAGES.API_KEY_UPDATED))
          onOpenChange(false)
          triggerRefresh()
        } else {
          toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        }
      } else {
        // Create mode - handle batch creation
        const count = data.tokenCount || 1
        let successCount = 0

        for (let i = 0; i < count; i++) {
          const result = await createApiKey({
            ...basePayload,
            name:
              i === 0 && data.name
                ? data.name
                : `${data.name || 'default'}-${Math.random().toString(36).slice(2, 8)}`,
          })
          if (result.success) {
            successCount++
          } else {
            toast.error(result.message || t(ERROR_MESSAGES.CREATE_FAILED))
            break
          }
        }

        if (successCount > 0) {
          toast.success(
            t('Successfully created {{count}} API Key(s)', {
              count: successCount,
            })
          )
          onOpenChange(false)
          triggerRefresh()
        }
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsSubmitting(false)
    }
  }

  const onInvalid: SubmitErrorHandler<ApiKeyFormValues> = () => {
    toast.error(t('Please fix the highlighted fields before saving'))
  }

  const handleSetExpiry = (months: number, days: number, hours: number) => {
    if (months === 0 && days === 0 && hours === 0) {
      form.setValue('expired_time', undefined)
      return
    }

    const now = new Date()
    now.setMonth(now.getMonth() + months)
    now.setDate(now.getDate() + days)
    now.setHours(now.getHours() + hours)

    form.setValue('expired_time', now)
  }

  const { meta: currencyMeta } = getCurrencyDisplay()
  const currencyLabel = getCurrencyLabel()
  const tokensOnly = currencyMeta.kind === 'tokens'
  const quotaLabel = t('Quota ({{currency}})', { currency: currencyLabel })
  const quotaPlaceholder = tokensOnly
    ? t('Enter quota in tokens')
    : t('Enter quota in {{currency}}', { currency: currencyLabel })
  const autoGroupsMode = form.watch('auto_groups_mode')
  const unlimitedQuota = form.watch('unlimited_quota')
  const autoRoutes = form.watch('auto_routes')
  const routeModelOptions = useMemo(
    () => autoModels.map((model) => ({ label: model, value: model })),
    [autoModels]
  )
  const routeGroupOptions = useMemo(
    () =>
      groups
        .filter((group) => group.value !== 'auto')
        .map((group) => ({
          ...group,
          ratio:
            typeof group.ratio === 'number' ? group.ratio : undefined,
        })),
    [groups]
  )

  useEffect(() => {
    if (!routeGroup && routeGroupOptions.length > 0) {
      setRouteGroup(routeGroupOptions[0].value)
    }
  }, [routeGroup, routeGroupOptions])

  const openRouteEditor = (virtualModel?: string) => {
    const routes = autoRoutes || {}
    const name = virtualModel || ''
    setEditingVirtualModel(virtualModel ?? null)
    setRouteName(name)
    setRouteChain(virtualModel ? [...(routes[virtualModel] || [])] : [])
    setRouteModel('')
    setRouteEditorOpen(true)
  }

  const saveRoute = () => {
    const name = routeName.trim()
    if (!name.startsWith('auto/') || name.length <= 'auto/'.length) {
      toast.error(t('Virtual model names must start with auto/'))
      return
    }
    if (routeChain.length === 0) {
      toast.error(t('Each virtual model needs at least one route model'))
      return
    }
    const routes: ApiKeyAutoRoutes = { ...autoRoutes }
    if (editingVirtualModel && editingVirtualModel !== name) {
      delete routes[editingVirtualModel]
    }
    if (!editingVirtualModel && routes[name]) {
      toast.error(t('This virtual model already exists'))
      return
    }
    routes[name] = routeChain
    form.setValue('auto_routes', routes, {
      shouldDirty: true,
      shouldValidate: true,
    })
    setRouteEditorOpen(false)
  }

  const deleteRoute = (virtualModel: string) => {
    const routes: ApiKeyAutoRoutes = { ...autoRoutes }
    delete routes[virtualModel]
    form.setValue('auto_routes', routes, { shouldDirty: true })
  }

  return (
    <>
      <Sheet
      open={open}
      onOpenChange={(v) => {
        onOpenChange(v)
        if (!v) {
          form.reset()
        }
      }}
    >
      <SheetContent
        className={sideDrawerContentClassName('max-w-none sm:!max-w-[620px]')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isUpdate ? t('Update API Key') : t('Create API Key')}
          </SheetTitle>
          <SheetDescription>
            {isUpdate
              ? t('Update the API key by providing necessary info.')
              : t('Add a new API key by providing necessary info.')}
          </SheetDescription>
        </SheetHeader>
        <Form {...form}>
          <form
            id='api-key-form'
            onSubmit={form.handleSubmit(onSubmit, onInvalid)}
            aria-busy={!isFormInitialized}
            inert={!isFormInitialized || isSubmitting ? true : undefined}
            className={sideDrawerFormClassName('gap-5')}
          >
            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Basic Information')}
                description={t('Set API key basic information')}
                icon={<KeyRound className='size-4' />}
                iconTone='info'
              />
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder={t('Enter a name')} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Group')}</FormLabel>
                    <FormControl>
                      <ApiKeyGroupCombobox
                        options={groups}
                        value={field.value}
                        onValueChange={(group) => {
                          field.onChange(group)
                          if (group === 'auto') {
                            form.setValue('cross_group_retry', true, {
                              shouldDirty: true,
                            })
                            return
                          }
                          form.setValue('cross_group_retry', false, {
                            shouldDirty: true,
                          })
                          form.setValue('auto_routes', {}, {
                            shouldDirty: true,
                          })
                        }}
                        placeholder={t('Select a group')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {selectedGroup === 'auto' && (
                <FormField
                  control={form.control}
                  name='auto_groups'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Auto group order')}</FormLabel>
                      <FormDescription>
                        {t(
                          'Choose and order the groups this API key will try.'
                        )}
                      </FormDescription>
                      <FormControl>
                        <AutoGroupOrderEditor
                          value={field.value}
                          mode={autoGroupsMode}
                          options={groups}
                          globalOptions={globalAutoGroupOptions}
                          maxCount={maxAutoGroups}
                          onChange={(value) => {
                            form.setValue('auto_groups_mode', value.mode, {
                              shouldDirty: true,
                              shouldValidate: false,
                            })
                            form.setValue(
                              'auto_groups',
                              value.groups.slice(0, maxAutoGroups),
                              {
                                shouldDirty: true,
                                shouldValidate: true,
                              }
                            )
                          }}
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              {selectedGroup === 'auto' && (
                <FormField
                  control={form.control}
                  name='auto_routes'
                  render={({ field }) => {
                    const routes = field.value || {}
                    const entries = Object.entries(routes)
                    return (
                      <FormItem>
                        <div className='flex items-start justify-between gap-3'>
                          <div>
                            <FormLabel>{t('Virtual model routes')}</FormLabel>
                            <FormDescription>
                              {t(
                                'Create auto/ models with an ordered fallback chain.'
                              )}
                            </FormDescription>
                          </div>
                          <Button
                            type='button'
                            variant='outline'
                            size='sm'
                            onClick={() => openRouteEditor()}
                          >
                            <Plus className='size-4' />
                            {t('Add virtual model')}
                          </Button>
                        </div>
                        <FormControl>
                          <div className='overflow-hidden rounded-lg border'>
                            <table className='w-full text-sm'>
                              <thead className='bg-muted/50 text-left text-xs'>
                                <tr>
                                  <th className='px-3 py-2 font-medium'>
                                    {t('Model name')}
                                  </th>
                                  <th className='px-3 py-2 font-medium'>
                                    {t('Route models')}
                                  </th>
                                  <th className='px-3 py-2 text-right font-medium'>
                                    {t('Options')}
                                  </th>
                                </tr>
                              </thead>
                              <tbody className='divide-y'>
                                {entries.length === 0 ? (
                                  <tr>
                                    <td
                                      colSpan={3}
                                      className='text-muted-foreground px-3 py-5 text-center text-xs'
                                    >
                                      {t('No virtual models configured')}
                                    </td>
                                  </tr>
                                ) : (
                                  entries.map(([virtualModel, chain]) => (
                                    <tr key={virtualModel}>
                                      <td className='px-3 py-2 font-medium'>
                                        {virtualModel}
                                      </td>
                                      <td className='text-muted-foreground px-3 py-2 tabular-nums'>
                                        {chain.length}
                                      </td>
                                      <td className='px-3 py-2'>
                                        <div className='flex justify-end gap-1'>
                                          <Button
                                            type='button'
                                            variant='ghost'
                                            size='icon-sm'
                                            aria-label={t('Edit route')}
                                            onClick={() =>
                                              openRouteEditor(virtualModel)
                                            }
                                          >
                                            <Edit className='size-4' />
                                          </Button>
                                          <Button
                                            type='button'
                                            variant='ghost'
                                            size='icon-sm'
                                            aria-label={t('Delete virtual model')}
                                            className='text-destructive hover:text-destructive'
                                            onClick={() =>
                                              deleteRoute(virtualModel)
                                            }
                                          >
                                            <Trash2 className='size-4' />
                                          </Button>
                                        </div>
                                      </td>
                                    </tr>
                                  ))
                                )}
                              </tbody>
                            </table>
                          </div>
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )
                  }}
                />
              )}

              {selectedGroup === 'auto' && (
                <FormField
                  control={form.control}
                  name='cross_group_retry'
                  render={({ field }) => (
                    <FormItem className={sideDrawerSwitchItemClassName()}>
                      <div className='flex flex-col gap-0.5'>
                        <FormLabel className='text-sm'>
                          {t('Cross-group retry')}
                        </FormLabel>
                        <FormDescription className='line-clamp-2 text-xs sm:line-clamp-none'>
                          {t(
                            'When enabled, if channels in the current group fail, it will try channels in the next group in order.'
                          )}
                        </FormDescription>
                      </div>
                      <FormControl>
                        <Switch
                          checked={!!field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='expired_time'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Expiration Time')}</FormLabel>
                    <div className='grid gap-2 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center'>
                      <FormControl>
                        <DateTimePicker
                          value={field.value}
                          onChange={field.onChange}
                          placeholder={t('Never expires')}
                          className='min-w-0 [&_input[type=time]]:w-24 sm:[&_input[type=time]]:w-32'
                        />
                      </FormControl>
                      <div className='grid grid-cols-4 gap-2 sm:flex'>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 0)}
                        >
                          {t('Never')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(1, 0, 0)}
                        >
                          {t('1 Month')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 1, 0)}
                        >
                          {t('1 Day')}
                        </Button>
                        <Button
                          type='button'
                          variant='outline'
                          size='sm'
                          className='px-2 text-xs sm:px-3 sm:text-sm'
                          onClick={() => handleSetExpiry(0, 0, 1)}
                        >
                          {t('1 Hour')}
                        </Button>
                      </div>
                    </div>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate && (
                <FormField
                  control={form.control}
                  name='tokenCount'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Quantity')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          placeholder={t('Number of keys to create')}
                          onChange={(e) =>
                            field.onChange(
                              Number.parseInt(e.target.value, 10) || 1
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Create multiple API keys at once (random suffix will be added to names)'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
            </SideDrawerSection>

            <SideDrawerSection>
              <SideDrawerSectionHeader
                title={t('Quota Settings')}
                description={t('Set quota amount and limits')}
                icon={<WalletCards className='size-4' />}
                iconTone='success'
              />
              {!unlimitedQuota && (
                <FormField
                  control={form.control}
                  name='remain_quota_dollars'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{quotaLabel}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          step={tokensOnly ? 1 : 0.01}
                          placeholder={quotaPlaceholder}
                          onChange={(e) =>
                            field.onChange(
                              Number.parseFloat(e.target.value) || 0
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {tokensOnly
                          ? t('Enter the quota amount in tokens')
                          : t('Enter the quota amount in {{currency}}', {
                              currency: currencyLabel,
                            })}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}

              <FormField
                control={form.control}
                name='unlimited_quota'
                render={({ field }) => (
                  <FormItem className={sideDrawerSwitchItemClassName()}>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-sm'>
                        {t('Unlimited Quota')}
                      </FormLabel>
                      <FormDescription className='text-xs'>
                        {t('Enable unlimited quota for this API key')}
                      </FormDescription>
                    </div>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                  </FormItem>
                )}
              />
            </SideDrawerSection>

            <Collapsible open={advancedOpen} onOpenChange={setAdvancedOpen}>
              <SideDrawerSection>
                <CollapsibleTrigger
                  render={
                    <button
                      type='button'
                      className='hover:bg-muted/40 flex w-full items-center gap-3 rounded-md py-1.5 text-left transition-colors'
                    />
                  }
                >
                  <SideDrawerSectionHeader
                    className='flex-1'
                    title={t('Advanced Settings')}
                    description={t('Set API key access restrictions')}
                    icon={<Settings2 className='size-4' />}
                  />
                  <ChevronDown
                    className={cn(
                      'text-muted-foreground size-4 shrink-0 transition-transform',
                      advancedOpen && 'rotate-180'
                    )}
                  />
                </CollapsibleTrigger>
                <CollapsibleContent>
                  <div className='flex flex-col gap-4 pt-2'>
                    <FormField
                      control={form.control}
                      name='model_limits'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Model Limits')}</FormLabel>
                          <FormControl>
                            <MultiSelect
                              options={models.map((m) => ({
                                label: m,
                                value: m,
                              }))}
                              selected={field.value}
                              onChange={field.onChange}
                              placeholder={t(
                                'Select models (empty for allow all)'
                              )}
                            />
                          </FormControl>
                          <FormDescription>
                            {t('Limit which models can be used with this key')}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />

                    <FormField
                      control={form.control}
                      name='allow_ips'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>
                            {t('IP Whitelist (supports CIDR)')}
                          </FormLabel>
                          <FormControl>
                            <Textarea
                              {...field}
                              className='min-h-20 resize-none'
                              placeholder={t(
                                'One IP per line (empty for no restriction)'
                              )}
                              rows={3}
                            />
                          </FormControl>
                          <FormDescription>
                            {t(
                              'Do not over-trust this feature. IP may be spoofed. Please use with nginx, CDN and other gateways.'
                            )}
                          </FormDescription>
                          <FormMessage />
                        </FormItem>
                      )}
                    />
                  </div>
                </CollapsibleContent>
              </SideDrawerSection>
            </Collapsible>
          </form>
        </Form>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' className='w-full sm:w-auto' />}
          >
            {t('Close')}
          </SheetClose>
          <Button
            type='button'
            onClick={form.handleSubmit(onSubmit, onInvalid)}
            disabled={!isFormInitialized || isSubmitting}
            className='w-full sm:w-auto'
          >
            {isSubmitting ? t('Saving...') : t('Save changes')}
          </Button>
        </SheetFooter>
      </SheetContent>
      </Sheet>
      <Dialog open={routeEditorOpen} onOpenChange={setRouteEditorOpen}>
        <DialogContent className='max-w-[calc(100%-2rem)] sm:max-w-xl'>
          <DialogHeader>
            <DialogTitle>{t('Edit virtual model route')}</DialogTitle>
            <DialogDescription>
              {t('Choose the ordered models to try for this virtual model.')}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-2'>
              <label htmlFor='virtual-model-name' className='text-sm font-medium'>
                {t('Model name')}
              </label>
              <Input
                id='virtual-model-name'
                value={routeName}
                onChange={(event) => setRouteName(event.target.value)}
                placeholder='auto/free'
                disabled={editingVirtualModel !== null}
              />
            </div>
            <div className='space-y-2'>
              <span className='text-sm font-medium'>{t('Add route model')}</span>
              <div className='flex items-center gap-2'>
                <ModelGroupSelector
                  selectedModel={routeModel}
                  models={routeModelOptions}
                  onModelChange={setRouteModel}
                  selectedGroup={routeGroup}
                  groups={routeGroupOptions}
                  onGroupChange={setRouteGroup}
                  className='min-w-0 flex-1'
                  disabled={routeModelOptions.length === 0}
                />
                <Button
                  type='button'
                  variant='outline'
                  size='icon-sm'
                  aria-label={t('Add route model')}
                  disabled={!routeModel || routeChain.includes(routeModel)}
                  onClick={() => {
                    if (!routeModel || routeChain.includes(routeModel)) return
                    setRouteChain((current) => [...current, routeModel])
                    setRouteModel('')
                  }}
                >
                  <Plus className='size-4' />
                </Button>
              </div>
            </div>
            <div className='space-y-2'>
              <span className='text-sm font-medium'>{t('Call chain')}</span>
              {routeChain.length === 0 ? (
                <div className='text-muted-foreground rounded-lg border border-dashed px-3 py-6 text-center text-sm'>
                  {t('Add at least one route model')}
                </div>
              ) : (
                <ol className='space-y-2'>
                  {routeChain.map((modelName, index) => (
                    <li
                      key={modelName}
                      className='bg-muted/30 flex items-center gap-2 rounded-lg border px-2 py-1.5'
                    >
                      <span className='bg-background text-muted-foreground flex size-6 shrink-0 items-center justify-center rounded-full border text-xs tabular-nums'>
                        {index + 1}
                      </span>
                      <span className='min-w-0 flex-1 truncate text-sm font-medium'>
                        {modelName}
                      </span>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Move route model up')}
                        disabled={index === 0}
                        onClick={() =>
                          setRouteChain((current) => {
                            const next = [...current]
                            ;[next[index - 1], next[index]] = [
                              next[index],
                              next[index - 1],
                            ]
                            return next
                          })
                        }
                      >
                        <ArrowUp className='size-4' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Move route model down')}
                        disabled={index === routeChain.length - 1}
                        onClick={() =>
                          setRouteChain((current) => {
                            const next = [...current]
                            ;[next[index], next[index + 1]] = [
                              next[index + 1],
                              next[index],
                            ]
                            return next
                          })
                        }
                      >
                        <ArrowDown className='size-4' />
                      </Button>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Remove route model')}
                        className='text-destructive hover:text-destructive'
                        onClick={() =>
                          setRouteChain((current) =>
                            current.filter((_, itemIndex) => itemIndex !== index)
                          )
                        }
                      >
                        <Trash2 className='size-4' />
                      </Button>
                    </li>
                  ))}
                </ol>
              )}
            </div>
          </div>
          <DialogFooter>
            <Button
              type='button'
              variant='outline'
              onClick={() => setRouteEditorOpen(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='button' onClick={saveRoute}>
              {t('Save route')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  )
}
