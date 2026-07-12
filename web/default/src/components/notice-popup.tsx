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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'
import { useStatus } from '@/hooks/use-status'
import { getNotice } from '@/lib/api'
import { isLikelyHtml } from '@/lib/content-format'
import { useNotificationStore } from '@/stores/notification-store'

type NoticePopupProps = {
  placement: 'home' | 'dashboard'
}

export function NoticePopup(props: NoticePopupProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const markNoticeRead = useNotificationStore((state) => state.markNoticeRead)
  const closedUntilDate = useNotificationStore((state) => state.closedUntilDate)
  const setClosedUntilDate = useNotificationStore(
    (state) => state.setClosedUntilDate
  )
  const [open, setOpen] = useState(false)
  const hasOpenedRef = useRef(false)
  const popupEnabled = Boolean(status?.notice_popup_enabled)
  const placementEnabled =
    props.placement === 'home' || Boolean(status?.notice_popup_on_dashboard)
  const enabled = popupEnabled && placementEnabled
  const isClosedToday = closedUntilDate === new Date().toDateString()
  const { data } = useQuery({
    queryKey: ['notice'],
    queryFn: getNotice,
    enabled,
    staleTime: 5 * 60 * 1000,
  })
  const notice = data?.success ? (data.data || '').trim() : ''

  useEffect(() => {
    if (!enabled || !notice || isClosedToday || hasOpenedRef.current) return

    hasOpenedRef.current = true
    markNoticeRead(notice)
    setOpen(true)
  }, [enabled, isClosedToday, markNoticeRead, notice])

  if (!enabled || !notice || isClosedToday) return null

  const isHtml = isLikelyHtml(notice)

  return (
    <Dialog
      open={open}
      onOpenChange={setOpen}
      title={t('System Notice')}
      contentClassName='sm:max-w-3xl'
      contentHeight='min(65vh, 36rem)'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => {
            setClosedUntilDate(new Date().toDateString())
            setOpen(false)
          }}
        >
          {t('Close Today')}
        </Button>
      }
      showCloseButton
    >
      <RichContent
        content={notice}
        mode={isHtml ? 'html' : 'markdown'}
        htmlVariant={isHtml ? 'isolated' : undefined}
        breaks={!isHtml}
        className='min-w-0'
      />
    </Dialog>
  )
}
