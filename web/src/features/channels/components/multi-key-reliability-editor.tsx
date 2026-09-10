/* Copyright (C) 2023-2026 QuantumNous */
import { Plus, Trash2 } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { FormDescription, FormLabel } from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import type { MultiKeyDisableRule } from '../types'

type Props = {
  rules: string
  autoRetry: boolean
  autoRecovery: boolean
  recoveryIntervalMinutes: number
  disabled?: boolean
  onRulesChange: (value: string) => void
  onAutoRetryChange: (value: boolean) => void
  onAutoRecoveryChange: (value: boolean) => void
  onRecoveryIntervalChange: (value: number) => void
}

function parseRules(value: string): MultiKeyDisableRule[] {
  try {
    const parsed: unknown = JSON.parse(value || '[]')
    if (!Array.isArray(parsed)) return []
    return parsed.filter(
      (item): item is MultiKeyDisableRule =>
        Boolean(item) &&
        typeof item === 'object' &&
        (typeof (item as MultiKeyDisableRule).status_code === 'number' ||
          typeof (item as MultiKeyDisableRule).message === 'string')
    )
  } catch {
    return []
  }
}

export function MultiKeyReliabilityEditor({
  rules,
  autoRetry,
  autoRecovery,
  recoveryIntervalMinutes,
  disabled = false,
  onRulesChange,
  onAutoRetryChange,
  onAutoRecoveryChange,
  onRecoveryIntervalChange,
}: Props) {
  const { t } = useTranslation()
  const parsedRules = useMemo(() => parseRules(rules), [rules])

  const emitRules = (nextRules: MultiKeyDisableRule[]) => {
    onRulesChange(JSON.stringify(nextRules, null, 2))
  }

  return (
    <div className='space-y-4 rounded-md border p-4'>
      <div>
        <FormLabel>{t('Multi-key reliability')}</FormLabel>
        <FormDescription>
          {t(
            'Disable a key when its response matches a rule, then optionally retry with another key.'
          )}
        </FormDescription>
      </div>

      <div className='space-y-2'>
        {parsedRules.map((rule, index) => (
          <div
            key={`${rule.status_code ?? 'any'}:${rule.message ?? ''}`}
            className='grid gap-2 sm:grid-cols-[8rem_1fr_auto]'
          >
            <Input
              type='number'
              min={100}
              max={599}
              value={rule.status_code ?? ''}
              placeholder={t('Status code')}
              disabled={disabled}
              onChange={(event) => {
                const value = event.target.value
                const next = [...parsedRules]
                next[index] = {
                  ...rule,
                  status_code: value === '' ? undefined : Number(value),
                }
                emitRules(next)
              }}
            />
            <Input
              value={rule.message ?? ''}
              placeholder={t('Message contains')}
              disabled={disabled}
              onChange={(event) => {
                const next = [...parsedRules]
                next[index] = { ...rule, message: event.target.value }
                emitRules(next)
              }}
            />
            <Button
              type='button'
              variant='ghost'
              size='sm'
              aria-label={t('Delete')}
              disabled={disabled}
              onClick={() =>
                emitRules(parsedRules.filter((_, i) => i !== index))
              }
            >
              <Trash2 className='h-4 w-4' />
            </Button>
          </div>
        ))}
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={disabled}
          onClick={() =>
            emitRules([...parsedRules, { status_code: 429, message: '' }])
          }
        >
          <Plus className='mr-2 h-4 w-4' />
          {t('Add rule')}
        </Button>
      </div>

      <div className='flex items-center justify-between gap-3 border-t pt-3'>
        <div>
          <FormLabel>{t('Automatic key retry')}</FormLabel>
          <FormDescription>
            {t(
              'When a disable rule matches, disable the current key and retry with another enabled key.'
            )}
          </FormDescription>
        </div>
        <Switch
          checked={autoRetry}
          onCheckedChange={onAutoRetryChange}
          disabled={disabled}
        />
      </div>

      <div className='flex items-center justify-between gap-3 border-t pt-3'>
        <div>
          <FormLabel>{t('Automatic key recovery')}</FormLabel>
          <FormDescription>
            {t(
              'Periodically test auto-disabled keys and enable them again after a successful response.'
            )}
          </FormDescription>
        </div>
        <Switch
          checked={autoRecovery}
          onCheckedChange={onAutoRecoveryChange}
          disabled={disabled}
        />
      </div>

      <div className='max-w-xs'>
        <FormLabel>{t('Recovery interval (minutes)')}</FormLabel>
        <Input
          type='number'
          min={1}
          max={1440}
          step={1}
          value={recoveryIntervalMinutes}
          disabled={disabled || !autoRecovery}
          onChange={(event) =>
            onRecoveryIntervalChange(
              Math.max(1, Math.min(1440, Number(event.target.value) || 1))
            )
          }
        />
        <FormDescription>
          {t('Auto-disabled keys are tested at this interval.')}
        </FormDescription>
      </div>
    </div>
  )
}
