import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonCodeEditor } from '@/components/json-code-editor'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { useUpdateOption } from '../hooks/use-update-option'
import { normalizeJsonString, validateJsonString } from '../models/utils'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'

const DEFAULT_RULES = `[
  {
    "group": "free",
    "logic": "or",
    "rules": [
      {
        "logic": "and",
        "conditions": [{ "type": "oauth", "providers": ["linuxdo"] }]
      },
      {
        "logic": "and",
        "conditions": [
          { "type": "oauth", "providers": ["github"] },
          { "type": "github_registration_days", "days": 90 }
        ]
      }
    ]
  }
]`

type GroupAccessRulesSectionProps = {
  defaultValue: string
}

export function GroupAccessRulesSection({
  defaultValue,
}: GroupAccessRulesSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const initialValue = useMemo(
    () => normalizeJsonString(defaultValue || '[]'),
    [defaultValue]
  )
  const [value, setValue] = useState(initialValue)
  const [conditionType, setConditionType] = useState('spend')
  const [conditionAmount, setConditionAmount] = useState('')
  useEffect(() => {
    setValue(initialValue)
  }, [initialValue])
  const validation = validateJsonString(value, {
    predicate: (parsed) =>
      Array.isArray(parsed) &&
      parsed.every(
        (rule) =>
          typeof rule === 'object' &&
          rule !== null &&
          typeof (rule as { group?: unknown }).group === 'string'
      ),
    predicateMessage: t('JSON structure is invalid'),
  })

  const save = async () => {
    if (!validation.valid) return
    await updateOption.mutateAsync({
      key: 'console_setting.group_access_rules',
      value: normalizeJsonString(value),
    })
  }

  const reset = () => setValue(initialValue)

  const addVisualCondition = () => {
    const amount = Number.parseInt(conditionAmount, 10)
    if (!Number.isFinite(amount) || amount < 0) return
    try {
      const rules = JSON.parse(value) as Array<Record<string, unknown>>
      const first = rules[0]
      if (!first || typeof first !== 'object') return
      const conditions = Array.isArray(first.conditions) ? first.conditions : []
      conditions.push(
        conditionType === 'spend'
          ? { type: 'spend', min_spend: amount }
          : { type: 'balance', min_quota: amount }
      )
      first.conditions = conditions
      setValue(JSON.stringify(rules, null, 2))
      setConditionAmount('')
    } catch {
      // Keep the JSON editor as the source of truth when it is incomplete.
    }
  }

  return (
    <>
      <SettingsPageFormActions
        onSave={save}
        onReset={reset}
        isSaving={updateOption.isPending}
        isSaveDisabled={!validation.valid}
        saveLabel='Save access rules'
      />
      <Card>
        <CardHeader>
          <CardTitle>{t('Group Access Thresholds')}</CardTitle>
          <CardDescription>
            {t('Set per-group access rules with nested AND/OR expressions, OAuth providers, GitHub account age, minimum balance, or user spend amount.')}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <JsonCodeEditor
            value={value}
            onChange={setValue}
            name='group-access-rules'
          />
          <div className='bg-muted/30 grid gap-3 rounded-md border p-3 sm:grid-cols-[1fr_1fr_auto] sm:items-end'>
            <div className='space-y-1.5'>
              <Label>{t('Condition type')}</Label>
              <Select value={conditionType} onValueChange={(next) => next && setConditionType(next)}>
                <SelectTrigger><SelectValue /></SelectTrigger>
                <SelectContent><SelectItem value='spend'>{t('User spend')}</SelectItem><SelectItem value='balance'>{t('Minimum balance')}</SelectItem></SelectContent>
              </Select>
            </div>
            <div className='space-y-1.5'>
              <Label>{t('Threshold (quota units)')}</Label>
              <Input type='number' min={0} value={conditionAmount} onChange={(event) => setConditionAmount(event.target.value)} />
            </div>
            <Button type='button' variant='outline' onClick={addVisualCondition}>{t('Add condition')}</Button>
          </div>
          <p className='text-muted-foreground text-xs'>
            {t('Visual condition fields: use type "spend" with min_spend in quota units to require user consumption.')}
          </p>
          {!validation.valid ? (
            <p className='text-destructive text-sm'>
              {validation.message || t('JSON structure is invalid')}
            </p>
          ) : null}
          <p className='text-muted-foreground text-sm'>{t('Example rule')}:</p>
          <pre className='bg-muted/60 overflow-x-auto rounded-md border p-3 text-xs whitespace-pre-wrap'>
            {DEFAULT_RULES}
          </pre>
        </CardContent>
      </Card>
    </>
  )
}
