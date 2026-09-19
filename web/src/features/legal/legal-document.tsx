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
import { FileWarning } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { useSystemConfig } from '@/hooks/use-system-config'
import { isHttpUrl, isLikelyHtml } from '@/lib/content-format'
import { cn } from '@/lib/utils'

import type { LegalDocumentResponse } from './types'

type LegalDocumentProps = {
  title: string
  queryKey: string
  fetchDocument: () => Promise<LegalDocumentResponse>
  emptyMessage: string
}

export function LegalDocument({
  title,
  queryKey,
  fetchDocument,
  emptyMessage,
}: LegalDocumentProps) {
  const { t } = useTranslation()
  const { appearance } = useSystemConfig()
  const { data, isLoading } = useQuery({
    queryKey: [queryKey],
    queryFn: fetchDocument,
    staleTime: 10 * 60 * 1000,
  })

  const rawContent = data?.data?.trim() ?? ''
  const hasContent = rawContent.length > 0
  const isUrl = hasContent && isHttpUrl(rawContent)
  const contentIsHtml = hasContent && isLikelyHtml(rawContent)
  const success = data?.success ?? false
  const legalBackgroundEnabled = appearance.legalBackgroundEnabled
  const backgroundMode = legalBackgroundEnabled ? 'hero' : undefined
  const contentSpacingClassName = legalBackgroundEnabled
    ? 'pt-20 pb-12 md:pt-28'
    : 'py-12'

  if (isLoading) {
    return (
      <PublicLayout backgroundMode={backgroundMode}>
        <article
          className={cn(
            'mx-auto flex max-w-4xl flex-col gap-4',
            contentSpacingClassName
          )}
        >
          <Skeleton className='h-8 w-[45%]' />
          <Skeleton className='h-4 w-full' />
          <Skeleton className='h-4 w-[90%]' />
          <Skeleton className='h-4 w-[80%]' />
        </article>
      </PublicLayout>
    )
  }

  if (!success || !hasContent) {
    return (
      <PublicLayout backgroundMode={backgroundMode}>
        <article className={cn('mx-auto max-w-2xl', contentSpacingClassName)}>
          <Card className='border-dashed'>
            <CardHeader className='flex flex-row items-center gap-4'>
              <div className='bg-muted rounded-lg p-2'>
                <FileWarning className='text-muted-foreground h-5 w-5' />
              </div>
              <div className='space-y-1'>
                <CardTitle className='text-lg font-semibold'>{title}</CardTitle>
                <p className='text-muted-foreground text-sm'>
                  {data?.message || emptyMessage}
                </p>
              </div>
            </CardHeader>
          </Card>
        </article>
      </PublicLayout>
    )
  }

  if (isUrl) {
    return (
      <PublicLayout backgroundMode={backgroundMode}>
        <article className={cn('mx-auto max-w-2xl', contentSpacingClassName)}>
          <Card>
            <CardHeader>
              <CardTitle>{title}</CardTitle>
            </CardHeader>
            <CardContent className='space-y-4'>
              <p className='text-muted-foreground text-sm'>
                {t(
                  'The administrator configured an external link for this document.'
                )}
              </p>
              <Button
                render={
                  <a
                    href={rawContent}
                    target='_blank'
                    rel='noopener noreferrer'
                  />
                }
              >
                {t('View document')}
              </Button>
            </CardContent>
          </Card>
        </article>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout
      backgroundMode={backgroundMode}
      showMainContainer={!contentIsHtml}
    >
      {contentIsHtml ? (
        <article className={contentSpacingClassName}>
          <RichContent
            mode='html'
            htmlVariant='isolated'
            content={rawContent}
          />
        </article>
      ) : (
        <article
          className={cn('mx-auto max-w-4xl space-y-6', contentSpacingClassName)}
        >
          <div className='space-y-2'>
            <h1 className='text-3xl font-semibold tracking-tight'>{title}</h1>
          </div>

          <RichContent
            mode='markdown'
            content={rawContent}
            className='prose-neutral dark:prose-invert max-w-none'
          />
        </article>
      )}
    </PublicLayout>
  )
}
