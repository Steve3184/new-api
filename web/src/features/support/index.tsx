/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  CheckCircle2,
  CircleDollarSign,
  FileText,
  Gift,
  ImagePlus,
  Loader2,
  MessageCircle,
  Paperclip,
  RefreshCw,
  Search,
  Send,
  TicketCheck,
  X,
} from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog as ImagePreviewDialog } from '@/components/dialog'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { ScrollArea } from '@/components/ui/scroll-area'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'
import { useIsAdmin } from '@/hooks/use-admin'
import { useDebounce } from '@/hooks/use-debounce'
import { useStatus } from '@/hooks/use-status'
import { formatLocalCurrencyAmount, getCurrencyLabel } from '@/lib/currency'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { useAuthStore } from '@/stores/auth-store'
import { useSystemConfigStore } from '@/stores/system-config-store'

import {
  completeSupportOrder,
  getAdminSupportConversation,
  getAdminSupportConversations,
  getSupportConversation,
  getSupportOrders,
  grantSupportQuota,
  grantSupportSubscription,
  sendAdminSupportMessage,
  sendSupportMessage,
} from './api'
import type {
  SupportConversation,
  SupportMessage,
  SupportOrderQuote,
} from './types'

function formatMessageTime(timestamp: number) {
  return new Date(timestamp * 1000).toLocaleString()
}

function orderLabel(order: SupportOrderQuote, t: (key: string) => string) {
  const title =
    order.order_type === 'topup'
      ? t('Top-up order')
      : order.plan_title || t('Subscription order')
  return `${title} #${order.order_id} · ${formatLocalCurrencyAmount(order.money)} · ${order.status}`
}

function MessageBody({
  message,
  isAdmin,
  onComplete,
  onImageClick,
}: {
  message: SupportMessage
  isAdmin: boolean
  onComplete: (message: SupportMessage) => void
  onImageClick: (imageData: string) => void
}) {
  const { t } = useTranslation()
  if (message.kind === 'image' && message.image_data) {
    return (
      <div className='space-y-2'>
        <button
          type='button'
          className='focus-visible:ring-ring block max-w-full cursor-zoom-in rounded-md focus-visible:ring-2 focus-visible:outline-none'
          aria-label={t('Support attachment')}
          onClick={() => onImageClick(message.image_data || '')}
        >
          <img
            src={message.image_data}
            alt={t('Support attachment')}
            className='max-h-64 max-w-full rounded-md object-contain'
          />
        </button>
        {message.content && (
          <p className='whitespace-pre-wrap'>{message.content}</p>
        )}
      </div>
    )
  }
  if (message.kind === 'order_quote') {
    const canComplete =
      isAdmin &&
      message.order_type === 'topup' &&
      message.order_status === 'pending' &&
      message.order_provider !== 'monero'
    return (
      <div className='max-w-full min-w-0 space-y-2 rounded-md border border-current/15 p-3'>
        <div className='flex items-center gap-2 font-medium'>
          <FileText className='size-4' />
          <span>
            {message.order_type === 'topup'
              ? t('Top-up order')
              : message.order_plan_title || t('Subscription order')}
          </span>
        </div>
        <div className='grid grid-cols-2 gap-x-3 gap-y-1 text-xs opacity-80'>
          <span>{t('Order ID')}</span>
          <span className='text-right tabular-nums'>{message.order_id}</span>
          <span>{t('Status')}</span>
          <span className='text-right'>{message.order_status}</span>
          <span>{t('Amount')}</span>
          <span className='text-right tabular-nums'>
            {formatLocalCurrencyAmount(message.order_money)}
          </span>
          {message.order_type === 'topup' && message.order_amount ? (
            <>
              <span>{t('Quota')}</span>
              <span className='text-right tabular-nums'>
                {message.order_amount}
              </span>
            </>
          ) : null}
          <span>{t('Trade number')}</span>
          <span className='truncate text-right'>{message.order_trade_no}</span>
        </div>
        {canComplete && (
          <Button
            type='button'
            size='sm'
            variant='secondary'
            onClick={() => onComplete(message)}
          >
            <CheckCircle2 className='size-4' />
            {t('Complete order')}
          </Button>
        )}
        {message.content && (
          <p className='text-sm whitespace-pre-wrap'>{message.content}</p>
        )}
      </div>
    )
  }
  if (message.kind === 'quota_grant') {
    return (
      <div className='flex items-center gap-2'>
        <CircleDollarSign className='size-4 shrink-0' />
        <span>{message.content || t('Quota granted')}</span>
        {message.grant_quota ? (
          <Badge variant='secondary' className='tabular-nums'>
            +{formatQuota(message.grant_quota)}
          </Badge>
        ) : null}
      </div>
    )
  }
  if (message.kind === 'subscription_grant') {
    return (
      <div className='flex items-center gap-2'>
        <Gift className='size-4 shrink-0' />
        <span>{message.content || t('Subscription granted')}</span>
        {message.grant_plan_title ? (
          <Badge variant='secondary'>{message.grant_plan_title}</Badge>
        ) : null}
      </div>
    )
  }
  return <p className='whitespace-pre-wrap'>{message.content}</p>
}

function MessageList({
  messages,
  isAdmin,
  onComplete,
  onImageClick,
}: {
  messages: SupportMessage[]
  isAdmin: boolean
  onComplete: (message: SupportMessage) => void
  onImageClick: (imageData: string) => void
}) {
  const bottomRef = useRef<HTMLDivElement | null>(null)
  const { t } = useTranslation()
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ block: 'end' })
  }, [messages.length])
  return (
    <ScrollArea className='min-h-0 flex-1 px-4 py-5'>
      <div className='mx-auto flex w-full max-w-3xl flex-col gap-4'>
        {messages.length === 0 ? (
          <div className='text-muted-foreground flex min-h-48 flex-col items-center justify-center gap-2 text-sm'>
            <MessageCircle className='size-8 opacity-50' />
            <span>{t('No messages yet')}</span>
          </div>
        ) : (
          messages.map((message) => {
            const own = isAdmin
              ? message.sender_role >= 10
              : message.sender_role < 10
            return (
              <div
                key={message.id}
                className={`flex ${own ? 'justify-end' : 'justify-start'}`}
              >
                <div className='w-full max-w-[min(85%,42rem)] min-w-0 space-y-1'>
                  <div
                    className={`w-fit max-w-full min-w-0 overflow-hidden rounded-lg px-3 py-2 text-sm shadow-xs ${
                      own
                        ? 'bg-primary text-primary-foreground'
                        : 'bg-muted text-foreground'
                    }`}
                  >
                    <MessageBody
                      message={message}
                      isAdmin={isAdmin}
                      onComplete={onComplete}
                      onImageClick={onImageClick}
                    />
                  </div>
                  <div
                    className={`text-muted-foreground max-w-full text-[10px] break-all whitespace-normal ${own ? 'text-right' : 'text-left'}`}
                  >
                    {formatMessageTime(message.created_at)}
                  </div>
                </div>
              </div>
            )
          })
        )}
        <div ref={bottomRef} />
      </div>
    </ScrollArea>
  )
}

function ConversationList({
  conversations,
  selectedId,
  onSelect,
  isAdmin,
  searchKeyword,
  onSearchKeywordChange,
}: {
  conversations: SupportConversation[]
  selectedId: number | null
  onSelect: (id: number) => void
  isAdmin: boolean
  searchKeyword: string
  onSearchKeywordChange: (value: string) => void
}) {
  const { t } = useTranslation()
  return (
    <aside className='bg-muted/10 flex w-full shrink-0 flex-col border-b md:w-72 md:border-r md:border-b-0'>
      <div className='flex items-center justify-between border-b px-4 py-3'>
        <div className='flex items-center gap-2 font-semibold'>
          <MessageCircle className='size-4' />
          {t('Conversations')}
        </div>
        {isAdmin && <Badge variant='outline'>{conversations.length}</Badge>}
      </div>
      {isAdmin && (
        <div className='border-b px-3 py-2'>
          <div className='relative'>
            <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2' />
            <Input
              value={searchKeyword}
              onChange={(event) => onSearchKeywordChange(event.target.value)}
              placeholder={t('Search')}
              aria-label={t('Search')}
              className='h-8 pl-8'
            />
          </div>
        </div>
      )}
      <ScrollArea className='max-h-44 md:max-h-none md:min-h-0 md:flex-1'>
        <div className='p-2'>
          {conversations.map((conversation) => {
            const unread = isAdmin
              ? conversation.unread_admin
              : conversation.unread_user
            return (
              <button
                type='button'
                key={conversation.id}
                onClick={() => onSelect(conversation.id)}
                className={`w-full rounded-md px-3 py-3 text-left transition-colors ${selectedId === conversation.id ? 'bg-accent' : 'hover:bg-accent/60'}`}
              >
                <div className='flex items-center justify-between gap-2'>
                  <span className='truncate text-sm font-medium'>
                    {isAdmin
                      ? conversation.display_name ||
                        conversation.username ||
                        `#${conversation.user_id}`
                      : conversation.title}
                  </span>
                  {unread > 0 && (
                    <Badge
                      variant='destructive'
                      className='shrink-0 tabular-nums'
                    >
                      {unread}
                    </Badge>
                  )}
                </div>
                <span className='text-muted-foreground mt-1 block text-xs'>
                  {conversation.last_message_at
                    ? formatMessageTime(conversation.last_message_at)
                    : t('No messages yet')}
                </span>
              </button>
            )
          })}
        </div>
      </ScrollArea>
    </aside>
  )
}

export function Support() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const { status } = useStatus()
  const supportEnabled = status !== null && status.support_enabled !== false
  const currencyConfig = useSystemConfigStore((state) => state.config.currency)
  const queryClient = useQueryClient()
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [content, setContent] = useState('')
  const [image, setImage] = useState<File | null>(null)
  const [quoteMode, setQuoteMode] = useState(false)
  const [selectedOrder, setSelectedOrder] = useState<SupportOrderQuote | null>(
    null
  )
  const [quota, setQuota] = useState('')
  const [planId, setPlanId] = useState('')
  const [adminNote, setAdminNote] = useState('')
  const [searchKeyword, setSearchKeyword] = useState('')
  const [previewImage, setPreviewImage] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const debouncedKeyword = useDebounce(searchKeyword, 300)
  const quotaInputValue = Number(quota)
  const quotaUnits = parseQuotaFromDollars(quotaInputValue)
  const canGrantQuota =
    quota.trim() !== '' &&
    Number.isFinite(quotaInputValue) &&
    quotaInputValue > 0 &&
    quotaUnits > 0
  const quotaPlaceholder =
    currencyConfig.quotaDisplayType === 'TOKENS'
      ? t('Enter quota in tokens')
      : t('Enter quota in {{currency}}', { currency: getCurrencyLabel() })

  const userConversationQuery = useQuery({
    queryKey: ['support-conversation', userId],
    queryFn: getSupportConversation,
    enabled: supportEnabled && !isAdmin,
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
    staleTime: 0,
    retry: false,
  })
  const adminConversationsQuery = useQuery({
    queryKey: ['support-admin-conversations', debouncedKeyword],
    queryFn: () => getAdminSupportConversations(debouncedKeyword),
    enabled: supportEnabled && isAdmin,
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
    staleTime: 0,
    retry: false,
  })
  const conversations = useMemo(() => {
    if (isAdmin) return adminConversationsQuery.data?.data?.items || []
    const conversation = userConversationQuery.data?.data?.conversation
    return conversation ? [conversation] : []
  }, [adminConversationsQuery.data, isAdmin, userConversationQuery.data])
  useEffect(() => {
    const conversationId = conversations[0]?.id
    if (!conversationId) return
    if (isAdmin && selectedId !== null) return
    if (!isAdmin && selectedId === conversationId) return
    // The conversation arrives asynchronously; sync the selection once it exists.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSelectedId(conversationId)
  }, [conversations, isAdmin, selectedId])

  const selectedConversation = conversations.find(
    (item) => item.id === selectedId
  )
  const conversationQuery = useQuery({
    queryKey: ['support-conversation-detail', selectedId, isAdmin],
    queryFn: () =>
      isAdmin
        ? getAdminSupportConversation(selectedId || 0)
        : getSupportConversation(),
    enabled: supportEnabled && selectedId !== null,
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
    staleTime: 0,
    retry: false,
  })
  const ordersQuery = useQuery({
    queryKey: ['support-orders', userId],
    queryFn: getSupportOrders,
    enabled: supportEnabled && !isAdmin && quoteMode,
    staleTime: 30_000,
  })
  const plansQuery = useQuery({
    queryKey: ['support-admin-plans'],
    queryFn: async () => {
      const response = await import('@/features/subscriptions/api').then(
        (module) => module.getAdminPlans()
      )
      return (response.data || []).map((record) => record.plan)
    },
    enabled: supportEnabled && isAdmin,
    staleTime: 60_000,
  })
  const orderItems = useMemo(
    () =>
      (ordersQuery.data?.data || []).map((order) => ({
        value: `${order.order_type}:${order.order_id}`,
        label: orderLabel(order, t),
      })),
    [ordersQuery.data?.data, t]
  )
  const planItems = useMemo(
    () =>
      (plansQuery.data || []).map((plan) => ({
        value: String(plan.id),
        label: plan.title,
      })),
    [plansQuery.data]
  )

  const invalidateSupport = () => {
    if (!supportEnabled) return
    void queryClient.invalidateQueries({ queryKey: ['support-conversation'] })
    void queryClient.invalidateQueries({
      queryKey: ['support-conversation-detail'],
    })
    void queryClient.invalidateQueries({
      queryKey: ['support-admin-conversations'],
    })
    void queryClient.invalidateQueries({ queryKey: ['support-unread'] })
  }
  const sendMutation = useMutation({
    mutationFn: async () => {
      if (!selectedId) throw new Error(t('Select a conversation'))
      if (isAdmin) {
        return sendAdminSupportMessage(selectedId, { content, image })
      }
      let kind = 'text'
      if (selectedOrder) {
        kind = 'order_quote'
      } else if (image) {
        kind = 'image'
      }
      return sendSupportMessage({
        content,
        image,
        kind,
        order_type: selectedOrder?.order_type,
        order_id: selectedOrder?.order_id,
      })
    },
    onSuccess: (response) => {
      if (!response.success) {
        toast.error(response.message || t('Unable to send message'))
        return
      }
      setContent('')
      setImage(null)
      if (fileInputRef.current) fileInputRef.current.value = ''
      setSelectedOrder(null)
      setQuoteMode(false)
      invalidateSupport()
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Unable to send message')
      ),
  })
  const quotaMutation = useMutation({
    mutationFn: () => grantSupportQuota(selectedId || 0, quotaUnits, adminNote),
    onSuccess: (response) => {
      if (!response.success) {
        return toast.error(response.message || t('Unable to grant quota'))
      }
      setQuota('')
      setAdminNote('')
      invalidateSupport()
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Unable to grant quota')
      ),
  })
  const subscriptionMutation = useMutation({
    mutationFn: () =>
      grantSupportSubscription(selectedId || 0, Number(planId), adminNote),
    onSuccess: (response) => {
      if (!response.success) {
        return toast.error(
          response.message || t('Unable to grant subscription')
        )
      }
      setPlanId('')
      setAdminNote('')
      invalidateSupport()
    },
    onError: (error) =>
      toast.error(
        error instanceof Error
          ? error.message
          : t('Unable to grant subscription')
      ),
  })
  const completeMutation = useMutation({
    mutationFn: (message: SupportMessage) => completeSupportOrder(message.id),
    onSuccess: (response) => {
      if (!response.success) {
        return toast.error(response.message || t('Unable to complete order'))
      }
      invalidateSupport()
    },
    onError: (error) =>
      toast.error(
        error instanceof Error ? error.message : t('Unable to complete order')
      ),
  })

  const messages = conversationQuery.data?.data?.messages || []
  const busy = sendMutation.isPending
  const handleSend = () => {
    if (!content.trim() && !image && !selectedOrder) return
    sendMutation.mutate()
  }

  if (!supportEnabled) return null

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>{t('Support')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={invalidateSupport}
        >
          <RefreshCw className='size-4' />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='bg-card flex h-full min-h-0 min-w-0 flex-col overflow-hidden rounded-lg border md:flex-row'>
          <ConversationList
            conversations={conversations}
            selectedId={selectedId}
            onSelect={setSelectedId}
            isAdmin={isAdmin}
            searchKeyword={searchKeyword}
            onSearchKeywordChange={setSearchKeyword}
          />
          <section className='flex min-h-0 min-w-0 flex-1 flex-col'>
            <header className='flex shrink-0 items-center justify-between border-b px-4 py-3'>
              <div className='min-w-0'>
                <h3 className='truncate text-sm font-semibold'>
                  {isAdmin
                    ? selectedConversation?.display_name ||
                      selectedConversation?.username ||
                      t('Select a conversation')
                    : t('Service support')}
                </h3>
                {selectedConversation && isAdmin && (
                  <p className='text-muted-foreground mt-0.5 text-xs'>
                    {t('User ID')}: {selectedConversation.user_id}
                  </p>
                )}
              </div>
              {conversationQuery.isFetching && (
                <Loader2 className='text-muted-foreground size-4 animate-spin' />
              )}
            </header>
            {selectedId === null ? (
              <div className='text-muted-foreground flex flex-1 items-center justify-center text-sm'>
                {t('Select a conversation')}
              </div>
            ) : (
              <>
                <MessageList
                  messages={messages}
                  isAdmin={isAdmin}
                  onComplete={(message) => completeMutation.mutate(message)}
                  onImageClick={setPreviewImage}
                />
                <div className='shrink-0 border-t p-3'>
                  {!isAdmin && quoteMode && (
                    <div className='mb-3 flex items-center gap-2'>
                      <Select
                        items={orderItems}
                        value={
                          selectedOrder
                            ? `${selectedOrder.order_type}:${selectedOrder.order_id}`
                            : null
                        }
                        onValueChange={(value) => {
                          const order = (ordersQuery.data?.data || []).find(
                            (item) =>
                              `${item.order_type}:${item.order_id}` === value
                          )
                          if (order) setImage(null)
                          setSelectedOrder(order || null)
                        }}
                      >
                        <SelectTrigger className='min-w-0 flex-1'>
                          <SelectValue
                            placeholder={t('Select an order to quote')}
                          />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {(ordersQuery.data?.data || []).map((order) => (
                              <SelectItem
                                key={`${order.order_type}:${order.order_id}`}
                                value={`${order.order_type}:${order.order_id}`}
                              >
                                {orderLabel(order, t)}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        aria-label={t('Cancel')}
                        onClick={() => {
                          setQuoteMode(false)
                          setSelectedOrder(null)
                        }}
                      >
                        <X className='size-4' />
                      </Button>
                    </div>
                  )}
                  {isAdmin && (
                    <div className='bg-muted/20 mb-3 grid gap-2 rounded-md border p-2 sm:grid-cols-[1fr_1fr_auto]'>
                      <div className='flex items-center gap-2'>
                        <Input
                          value={quota}
                          onChange={(event) => setQuota(event.target.value)}
                          type='number'
                          min='0'
                          step='any'
                          placeholder={quotaPlaceholder}
                          aria-label={t('Quota amount')}
                        />
                        <Button
                          type='button'
                          variant='secondary'
                          size='sm'
                          disabled={!canGrantQuota || quotaMutation.isPending}
                          onClick={() => quotaMutation.mutate()}
                        >
                          <Gift className='size-4' />
                          {t('Grant quota')}
                        </Button>
                      </div>
                      <Select
                        items={planItems}
                        value={planId || null}
                        onValueChange={(value) => setPlanId(value || '')}
                      >
                        <SelectTrigger className='min-w-0'>
                          <SelectValue placeholder={t('Select subscription')} />
                        </SelectTrigger>
                        <SelectContent alignItemWithTrigger={false}>
                          <SelectGroup>
                            {(plansQuery.data || []).map((plan) => (
                              <SelectItem key={plan.id} value={String(plan.id)}>
                                {plan.title}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <Button
                        type='button'
                        variant='secondary'
                        size='sm'
                        disabled={!planId || subscriptionMutation.isPending}
                        onClick={() => subscriptionMutation.mutate()}
                      >
                        <TicketCheck className='size-4' />
                        {t('Grant subscription')}
                      </Button>
                    </div>
                  )}
                  {isAdmin && (quota || planId) && (
                    <Input
                      value={adminNote}
                      onChange={(event) => setAdminNote(event.target.value)}
                      placeholder={t('Optional grant note')}
                      className='mb-2'
                    />
                  )}
                  <div className='flex items-end gap-2'>
                    <input
                      ref={fileInputRef}
                      type='file'
                      accept='image/*'
                      className='hidden'
                      onChange={(event) =>
                        setImage(event.target.files?.[0] || null)
                      }
                    />
                    <Button
                      type='button'
                      variant='outline'
                      size='icon'
                      aria-label={t('Attach image')}
                      disabled={busy || Boolean(selectedOrder)}
                      onClick={() => fileInputRef.current?.click()}
                    >
                      <Paperclip className='size-4' />
                    </Button>
                    {!isAdmin && (
                      <Button
                        type='button'
                        variant={quoteMode ? 'secondary' : 'outline'}
                        size='icon'
                        aria-label={t('Quote order')}
                        onClick={() => setQuoteMode((value) => !value)}
                      >
                        <FileText className='size-4' />
                      </Button>
                    )}
                    <Textarea
                      value={content}
                      onChange={(event) => setContent(event.target.value)}
                      onKeyDown={(event) => {
                        if (event.key === 'Enter' && !event.shiftKey) {
                          event.preventDefault()
                          handleSend()
                        }
                      }}
                      placeholder={t('Write a message...')}
                      className='max-h-28 min-h-10 resize-none'
                    />
                    <Button
                      type='button'
                      size='icon'
                      aria-label={t('Send message')}
                      disabled={
                        busy || (!content.trim() && !image && !selectedOrder)
                      }
                      onClick={handleSend}
                    >
                      {busy ? (
                        <Loader2 className='size-4 animate-spin' />
                      ) : (
                        <Send className='size-4' />
                      )}
                    </Button>
                  </div>
                  {(image || selectedOrder) && (
                    <div className='text-muted-foreground mt-2 flex items-center gap-2 text-xs'>
                      {image ? (
                        <>
                          <ImagePlus className='size-3.5' />
                          {image.name}
                        </>
                      ) : null}
                      {selectedOrder ? (
                        <>
                          <FileText className='size-3.5' />
                          {orderLabel(selectedOrder, t)}
                        </>
                      ) : null}
                    </div>
                  )}
                </div>
              </>
            )}
          </section>
        </div>
      </SectionPageLayout.Content>
      <ImagePreviewDialog
        open={Boolean(previewImage)}
        onOpenChange={(open) => {
          if (!open) setPreviewImage(null)
        }}
        title={t('Image Preview')}
        description={t('View the generated image')}
        contentClassName='sm:max-w-4xl'
        bodyClassName='flex min-h-0 items-center justify-center'
      >
        {previewImage && (
          <img
            src={previewImage}
            alt={t('Support attachment')}
            className='max-h-[calc(100vh-12rem)] max-w-full object-contain'
          />
        )}
      </ImagePreviewDialog>
    </SectionPageLayout>
  )
}
