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
            {t(
              'Set per-group access rules. Rules support nested AND/OR expressions, OAuth provider lists, GitHub account age in days, and minimum balance in quota units.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <JsonCodeEditor
            value={value}
            onChange={setValue}
            name='group-access-rules'
          />
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
