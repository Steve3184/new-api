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
import { ArrowRight } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { getTokenGroupNames, migrateTokenGroup } from '../../api'
import { useApiKeys } from '../api-keys-provider'

type TokenGroupMigrationDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TokenGroupMigrationDialog({
  open,
  onOpenChange,
}: TokenGroupMigrationDialogProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useApiKeys()
  const [sourceGroup, setSourceGroup] = useState('')
  const [targetGroup, setTargetGroup] = useState('')
  const [confirmOpen, setConfirmOpen] = useState(false)
  const [isMigrating, setIsMigrating] = useState(false)
  const { data, isLoading } = useQuery({
    queryKey: ['token-group-names'],
    queryFn: getTokenGroupNames,
    enabled: open,
    staleTime: 0,
  })

  useEffect(() => {
    if (!open) {
      setSourceGroup('')
      setTargetGroup('')
      setConfirmOpen(false)
    }
  }, [open])

  const sourceGroups = data?.data?.source_groups ?? []
  const targetGroups = data?.data?.target_groups ?? []
  const canMigrate =
    sourceGroup !== '' && targetGroup !== '' && sourceGroup !== targetGroup

  const handleMigrate = async () => {
    if (!canMigrate) return
    setIsMigrating(true)
    try {
      const result = await migrateTokenGroup(sourceGroup, targetGroup)
      if (!result.success) {
        toast.error(result.message || t('Token group migration failed'))
        return
      }
      toast.success(
        t('Migrated {{count}} token(s) from {{source}} to {{target}}', {
          count: result.data?.migrated ?? 0,
          source: sourceGroup,
          target: targetGroup,
        })
      )
      setConfirmOpen(false)
      onOpenChange(false)
      triggerRefresh()
    } catch {
      toast.error(t('Token group migration failed'))
    } finally {
      setIsMigrating(false)
    }
  }

  return (
    <>
      <Dialog
        open={open}
        onOpenChange={onOpenChange}
        title={t('Migrate token groups')}
        description={t(
          'Move every existing token in one group to another group.'
        )}
        contentClassName='sm:max-w-lg'
        footer={
          <>
            <Button variant='outline' onClick={() => onOpenChange(false)}>
              {t('Cancel')}
            </Button>
            <Button
              disabled={!canMigrate || isLoading}
              onClick={() => setConfirmOpen(true)}
            >
              {t('Review migration')}
            </Button>
          </>
        }
      >
        <div className='grid gap-4 sm:grid-cols-[1fr_auto_1fr] sm:items-end'>
          <div className='space-y-2'>
            <Label>{t('Source group')}</Label>
            <Select
              value={sourceGroup || null}
              onValueChange={(value) => setSourceGroup(value ?? '')}
              disabled={isLoading}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Select source group')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {sourceGroups.map((group) => (
                    <SelectItem key={group} value={group}>
                      {group}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
          <ArrowRight className='text-muted-foreground mb-2 hidden size-4 sm:block' />
          <div className='space-y-2'>
            <Label>{t('Target group')}</Label>
            <Select
              value={targetGroup || null}
              onValueChange={(value) => setTargetGroup(value ?? '')}
              disabled={isLoading}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Select target group')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {targetGroups.map((group) => (
                    <SelectItem key={group} value={group}>
                      {group}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>
        </div>
        {sourceGroup !== '' && sourceGroup === targetGroup && (
          <p className='text-destructive mt-3 text-sm'>
            {t('Source and target groups must be different.')}
          </p>
        )}
      </Dialog>

      <ConfirmDialog
        open={confirmOpen}
        onOpenChange={setConfirmOpen}
        title={t('Confirm token group migration')}
        desc={t(
          'All existing tokens in {{source}} will move to {{target}}. This affects every user and cannot be automatically undone.',
          { source: sourceGroup, target: targetGroup }
        )}
        confirmText={t('Migrate')}
        handleConfirm={handleMigrate}
        isLoading={isMigrating}
      />
    </>
  )
}
