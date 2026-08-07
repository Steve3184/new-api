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
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { ErrorRewriteTable } from './error-rewrite-table'
import {
  canonicalErrorRewriteRules,
  createErrorRewriteRuleId,
  parseErrorRewriteRules,
  serializeErrorRewriteRules,
  validateErrorRewriteRules,
  type ErrorRewriteRuleDraft,
  type ErrorRewriteRuleErrorCode,
} from './error-rewrite-utils'

type ErrorRewriteSettings = {
  enabled: boolean
  rules: string
}

type ErrorRewriteSectionProps = {
  defaultValues: ErrorRewriteSettings
}

type NormalizedErrorRewriteSettings = {
  enabled: boolean
  rules: string
}

export function ErrorRewriteSection(props: ErrorRewriteSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const defaultEnabled = props.defaultValues.enabled
  const defaultRules = props.defaultValues.rules
  const defaultRulesJson = useMemo(
    () => canonicalErrorRewriteRules(defaultRules),
    [defaultRules]
  )
  const baselineRef = useRef<NormalizedErrorRewriteSettings>({
    enabled: defaultEnabled,
    rules: defaultRulesJson,
  })
  const pendingSyncRef = useRef<NormalizedErrorRewriteSettings | null>(null)
  const [enabled, setEnabled] = useState(defaultEnabled)
  const [rules, setRules] = useState<ErrorRewriteRuleDraft[]>(() =>
    parseErrorRewriteRules(defaultRules)
  )
  const [saving, setSaving] = useState(false)

  // Keep external option updates from replacing local edits while two option
  // values are being persisted independently by the generic option endpoint.
  useEffect(() => {
    const pending = pendingSyncRef.current
    if (pending) {
      if (
        pending.enabled !== defaultEnabled ||
        pending.rules !== defaultRulesJson
      ) {
        return
      }
      pendingSyncRef.current = null
    }

    setEnabled(defaultEnabled)
    setRules(parseErrorRewriteRules(defaultRules))
    baselineRef.current = {
      enabled: defaultEnabled,
      rules: defaultRulesJson,
    }
  }, [defaultEnabled, defaultRules, defaultRulesJson])

  const validationErrors = useMemo(
    () => validateErrorRewriteRules(rules),
    [rules]
  )
  const hasValidationErrors = Object.keys(validationErrors).length > 0

  const errorText = (code: ErrorRewriteRuleErrorCode) => {
    switch (code) {
      case 'invalid-status-code':
        return t('Status code must be an integer between 100 and 599.')
      case 'duplicate-status-code':
        return t('Duplicate status codes are not allowed.')
      case 'empty-message':
        return t('Error message is required.')
    }
  }

  const handleAddRow = () => {
    setRules((current) => [
      ...current,
      {
        id: createErrorRewriteRuleId(),
        statusCode: '',
        message: '',
      },
    ])
  }

  const handleDeleteRow = (id: string) => {
    setRules((current) => current.filter((rule) => rule.id !== id))
  }

  const handleSave = async () => {
    if (hasValidationErrors) {
      toast.error(t('Please fix the error rewrite rules before saving.'))
      return
    }

    const normalized: NormalizedErrorRewriteSettings = {
      enabled,
      rules: serializeErrorRewriteRules(rules),
    }
    const updates: Array<{ key: string; value: string | boolean }> = []

    if (normalized.rules !== baselineRef.current.rules) {
      updates.push({ key: 'error_rewrite.rules', value: normalized.rules })
    }
    if (normalized.enabled !== baselineRef.current.enabled) {
      updates.push({ key: 'error_rewrite.enabled', value: normalized.enabled })
    }

    if (updates.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    pendingSyncRef.current = normalized
    setSaving(true)
    try {
      for (const update of updates) {
        await updateOption.mutateAsync(update)
      }
      baselineRef.current = normalized
      toast.success(t('Saved successfully'))
    } catch {
      pendingSyncRef.current = null
    } finally {
      setSaving(false)
    }
  }

  const handleReset = () => {
    pendingSyncRef.current = null
    setEnabled(defaultEnabled)
    setRules(parseErrorRewriteRules(defaultRules))
  }

  return (
    <SettingsSection title={t('Global Error Rewrite')}>
      <SettingsPageFormActions
        onSave={handleSave}
        onReset={handleReset}
        isSaving={saving}
        isSaveDisabled={hasValidationErrors}
      />

      <div className='space-y-4'>
        <SettingsSwitchField
          checked={enabled}
          onCheckedChange={setEnabled}
          disabled={saving}
          label={t('Enable global error rewrite')}
          description={t(
            'Replace client-facing error messages for matching upstream HTTP status codes.'
          )}
        />

        <Alert>
          <AlertDescription className='text-xs'>
            {t(
              'Use {model}, {status_code}, and {upstream_status_code} in a message. The upstream status code and response format are preserved.'
            )}
          </AlertDescription>
        </Alert>

        <ErrorRewriteTable
          rules={rules}
          validationErrors={validationErrors}
          disabled={saving}
          onAddRow={handleAddRow}
          onDeleteRow={handleDeleteRow}
          onChangeRule={(id, field, value) => {
            setRules((current) =>
              current.map((rule) =>
                rule.id === id ? { ...rule, [field]: value } : rule
              )
            )
          }}
          getErrorText={errorText}
        />
      </div>
    </SettingsSection>
  )
}
