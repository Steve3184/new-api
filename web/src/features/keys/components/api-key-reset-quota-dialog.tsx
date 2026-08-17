import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'

import { resetApiKeyUsedQuota } from '../api'
import { ERROR_MESSAGES } from '../constants'
import { useApiKeys } from './api-keys-provider'

export function ApiKeyResetQuotaDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useApiKeys()
  const [isResetting, setIsResetting] = useState(false)

  const handleReset = async () => {
    if (!currentRow) return
    setIsResetting(true)
    try {
      const result = await resetApiKeyUsedQuota(currentRow.id)
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.UNEXPECTED))
        return
      }
      toast.success(t('Used quota reset'))
      setOpen(null)
      triggerRefresh()
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsResetting(false)
    }
  }

  return (
    <AlertDialog
      open={open === 'reset-used-quota'}
      onOpenChange={(value) => !value && setOpen(null)}
    >
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>{t('Reset used quota?')}</AlertDialogTitle>
          <AlertDialogDescription>
            {t('This restores finite quota already used by this API key.')}{' '}
            <span className='font-semibold'>{currentRow?.name}</span>
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={isResetting}>
            {t('Cancel')}
          </AlertDialogCancel>
          <AlertDialogAction onClick={handleReset} disabled={isResetting}>
            {isResetting ? t('Resetting...') : t('Reset used quota')}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  )
}
