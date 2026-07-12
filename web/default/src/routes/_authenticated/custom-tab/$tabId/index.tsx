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
import { createFileRoute } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { Main } from '@/components/layout'
import { parseCustomTabs } from '@/lib/custom-tabs'
import { useStatus } from '@/hooks/use-status'

export const Route = createFileRoute('/_authenticated/custom-tab/$tabId/')({
  component: CustomTabPage,
})

function CustomTabPage() {
  const { tabId } = Route.useParams()
  const { t } = useTranslation()
  const { status } = useStatus()

  const tab = parseCustomTabs(status?.custom_tabs).find((t) => t.id === tabId)

  if (!tab) {
    return (
      <Main className='items-center justify-center'>
        <p className='text-muted-foreground text-sm'>{t('Tab not found')}</p>
      </Main>
    )
  }

  return (
    <Main className='p-0'>
      <iframe
        src={tab.url}
        title={tab.label}
        className='h-full w-full flex-1 border-0'
        sandbox='allow-scripts allow-same-origin allow-forms allow-popups allow-popups-to-escape-sandbox'
      />
    </Main>
  )
}
