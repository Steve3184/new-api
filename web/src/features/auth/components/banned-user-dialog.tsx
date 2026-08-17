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
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { HtmlContent } from '@/components/html-content'
import { Button } from '@/components/ui/button'

type BannedUserDialogProps = {
  open: boolean
  html: string
  onOpenChange: (open: boolean) => void
}

export function BannedUserDialog({
  open,
  html,
  onOpenChange,
}: BannedUserDialogProps) {
  const { t } = useTranslation()
  const hasHtml = html.trim().length > 0

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Account disabled')}
      description={
        hasHtml
          ? undefined
          : t('This account has been disabled. Please contact support.')
      }
      contentClassName='max-w-md'
      contentHeight='auto'
      bodyClassName='text-sm'
      footer={
        <Button type='button' onClick={() => onOpenChange(false)}>
          {t('Close')}
        </Button>
      }
    >
      {hasHtml ? <HtmlContent content={html} /> : null}
    </Dialog>
  )
}
