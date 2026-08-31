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
*/
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { JsonCodeEditor } from '@/components/json-code-editor'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import { SettingsPageFormActions } from '../components/settings-page-context'
import { useUpdateOption } from '../hooks/use-update-option'
import { normalizeJsonString, validateJsonString } from '../models/utils'

const DEFAULT_RULES = `[
  {
    "group": "free",
    "logic": "or",
    "rules": [
      { "logic": "and", "conditions": [{ "type": "oauth", "providers": ["linuxdo"] }] },
      { "logic": "and", "conditions": [{ "type": "oauth", "providers": ["github"] }, { "type": "github_registration_days", "days": 90 }] }
    ]
  }
]`

type GroupAccessRulesSectionProps = { defaultValue: string }
type AccessCondition = {
  type: string
  providers?: string[]
  days?: number
  min_quota?: number
  min_spend?: number
  editorId?: string
}
type AccessRule = {
  group?: string
  logic?: string
  conditions?: AccessCondition[]
  rules?: AccessRule[]
  editorId?: string
}

const LOGIC_OPTIONS = [
  { value: 'and', label: 'AND' },
  { value: 'or', label: 'OR' },
]

function cloneAccessRule(rule: AccessRule): AccessRule {
  return {
    ...rule,
    conditions: rule.conditions?.map((condition) => ({ ...condition })),
    rules: rule.rules?.map(cloneAccessRule),
  }
}

function setEditorId<T extends { editorId?: string }>(item: T, id: string): T {
  Object.defineProperty(item, 'editorId', {
    configurable: true,
    enumerable: false,
    value: id,
    writable: true,
  })
  return item
}

function conditionForType(
  type: string,
  current: AccessCondition
): AccessCondition {
  if (type === 'oauth') {
    return { type, providers: current.providers || [] }
  }
  if (type === 'github_registration_days') {
    return { type, days: current.days || 0 }
  }
  if (type === 'balance') {
    return { type, min_quota: current.min_quota || 0 }
  }
  return { type: 'spend', min_spend: current.min_spend || 0 }
}

function conditionAmount(condition: AccessCondition): number {
  if (condition.type === 'github_registration_days') {
    return condition.days || 0
  }
  if (condition.type === 'balance') {
    return condition.min_quota || 0
  }
  return condition.min_spend || 0
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
  const editorIdMap = useRef(new Map<string, string>())
  const nextEditorId = useRef(0)

  useEffect(() => setValue(initialValue), [initialValue])

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

  const parsedRules = useMemo<AccessRule[]>(() => {
    try {
      const parsed = JSON.parse(value)
      if (!Array.isArray(parsed)) return []
      const getEditorId = (path: string) => {
        const existing = editorIdMap.current.get(path)
        if (existing) return existing
        const id = `editor-${nextEditorId.current++}`
        editorIdMap.current.set(path, id)
        return id
      }
      const hydrateRules = (rules: AccessRule[], path: string) => {
        rules.forEach((rule, ruleIndex) => {
          setEditorId(rule, getEditorId(`${path}/rule/${ruleIndex}`))
          rule.conditions?.forEach((condition, conditionIndex) => {
            setEditorId(
              condition,
              getEditorId(
                `${path}/rule/${ruleIndex}/condition/${conditionIndex}`
              )
            )
          })
          if (rule.rules) {
            hydrateRules(rule.rules, `${path}/rule/${ruleIndex}/child`)
          }
        })
      }
      const rules = parsed as AccessRule[]
      hydrateRules(rules, 'root')
      return rules
    } catch {
      return []
    }
  }, [value])

  const conditionOptions = useMemo(
    () => [
      { value: 'oauth', label: t('OAuth') },
      {
        value: 'github_registration_days',
        label: `${t('GitHub')} ${t('days')}`,
      },
      { value: 'balance', label: t('Minimum balance') },
      { value: 'spend', label: t('User spend') },
    ],
    [t]
  )

  const updateRules = (mutate: (rules: AccessRule[]) => void) => {
    const next = parsedRules.map(cloneAccessRule)
    mutate(next)
    setValue(JSON.stringify(next, null, 2))
  }

  const save = async () => {
    if (!validation.valid) return
    await updateOption.mutateAsync({
      key: 'console_setting.group_access_rules',
      value: normalizeJsonString(value),
    })
  }

  const renderConditionRows = (
    conditions: AccessCondition[],
    groupIndex: number,
    ruleIndex: number | null
  ) =>
    conditions.map((condition, conditionIndex) => (
      <div
        key={condition.editorId}
        className='grid gap-2 sm:grid-cols-[180px_1fr_auto] sm:items-center'
      >
        <Select
          items={conditionOptions}
          value={condition.type}
          onValueChange={(type) =>
            type &&
            updateRules((rules) => {
              const target =
                ruleIndex === null
                  ? rules[groupIndex]?.conditions
                  : rules[groupIndex]?.rules?.[ruleIndex]?.conditions
              const item = target?.[conditionIndex]
              if (item && target) {
                target[conditionIndex] = conditionForType(type, item)
              }
            })
          }
        >
          <SelectTrigger className='w-full'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {conditionOptions.map((option) => (
                <SelectItem key={option.value} value={option.value}>
                  {option.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
        {condition.type === 'oauth' ? (
          <Input
            value={(condition.providers || []).join(', ')}
            placeholder='github, linuxdo'
            onChange={(event) =>
              updateRules((rules) => {
                const target =
                  ruleIndex === null
                    ? rules[groupIndex]?.conditions
                    : rules[groupIndex]?.rules?.[ruleIndex]?.conditions
                const item = target?.[conditionIndex]
                if (item) {
                  item.providers = event.target.value
                    .split(',')
                    .map((entry) => entry.trim())
                    .filter(Boolean)
                }
              })
            }
          />
        ) : (
          <Input
            type='number'
            min={0}
            value={conditionAmount(condition)}
            onChange={(event) =>
              updateRules((rules) => {
                const amount = Math.max(
                  0,
                  Number.parseInt(event.target.value, 10) || 0
                )
                const target =
                  ruleIndex === null
                    ? rules[groupIndex]?.conditions
                    : rules[groupIndex]?.rules?.[ruleIndex]?.conditions
                const item = target?.[conditionIndex]
                if (!item) return
                if (condition.type === 'github_registration_days') {
                  item.days = amount
                } else if (condition.type === 'balance') {
                  item.min_quota = amount
                } else {
                  item.min_spend = amount
                }
              })
            }
          />
        )}
        <Button
          type='button'
          variant='ghost'
          size='icon'
          aria-label={t('Remove condition')}
          onClick={() =>
            updateRules((rules) => {
              const target =
                ruleIndex === null
                  ? rules[groupIndex]?.conditions
                  : rules[groupIndex]?.rules?.[ruleIndex]?.conditions
              target?.splice(conditionIndex, 1)
            })
          }
        >
          <Trash2 />
        </Button>
      </div>
    ))

  return (
    <>
      <SettingsPageFormActions
        onSave={save}
        onReset={() => setValue(initialValue)}
        isSaving={updateOption.isPending}
        isSaveDisabled={!validation.valid}
        saveLabel='Save access rules'
      />
      <Card>
        <CardHeader>
          <CardTitle>{t('Group Access Thresholds')}</CardTitle>
          <CardDescription>
            {t(
              'Set per-group access rules with nested AND/OR expressions, OAuth providers, GitHub account age, minimum balance, or user spend amount.'
            )}
          </CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-3'>
            <div className='flex items-center justify-between gap-2'>
              <Label>{t('Visual editor')}</Label>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() =>
                  updateRules((rules) =>
                    rules.push({ group: '', logic: 'and', rules: [] })
                  )
                }
              >
                <Plus data-icon='inline-start' />
                {t('Add group')}
              </Button>
            </div>
            {parsedRules.map((groupRule, groupIndex) => (
              <div
                key={groupRule.editorId}
                className='space-y-3 rounded-md border p-3'
              >
                <div className='grid gap-3 sm:grid-cols-[1fr_160px_auto] sm:items-end'>
                  <div className='space-y-1.5'>
                    <Label>{t('Group')}</Label>
                    <Input
                      value={groupRule.group || ''}
                      onChange={(event) =>
                        updateRules((rules) => {
                          rules[groupIndex].group = event.target.value
                        })
                      }
                    />
                  </div>
                  <div className='space-y-1.5'>
                    <Label>{t('Logic')}</Label>
                    <Select
                      items={LOGIC_OPTIONS}
                      value={groupRule.logic || 'and'}
                      onValueChange={(logic) =>
                        logic &&
                        updateRules((rules) => {
                          rules[groupIndex].logic = logic
                        })
                      }
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent alignItemWithTrigger={false}>
                        <SelectGroup>
                          <SelectItem value='and'>AND</SelectItem>
                          <SelectItem value='or'>OR</SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </div>
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    aria-label={t('Remove rule group')}
                    onClick={() =>
                      updateRules((rules) => {
                        rules.splice(groupIndex, 1)
                      })
                    }
                  >
                    <Trash2 />
                  </Button>
                </div>
                <div className='space-y-2 pl-3'>
                  {groupRule.conditions?.length ? (
                    <>
                      <Label>{t('Conditions')}</Label>
                      {renderConditionRows(
                        groupRule.conditions,
                        groupIndex,
                        null
                      )}
                    </>
                  ) : null}
                  <Button
                    type='button'
                    variant='outline'
                    size='sm'
                    onClick={() =>
                      updateRules((rules) => {
                        const item = rules[groupIndex]
                        if (item) {
                          item.conditions = [
                            ...(item.conditions || []),
                            { type: 'spend', min_spend: 0 },
                          ]
                        }
                      })
                    }
                  >
                    <Plus data-icon='inline-start' />
                    {t('Add condition')}
                  </Button>
                </div>
                <div className='space-y-2 pl-3'>
                  {(groupRule.rules || []).map((rule, ruleIndex) => (
                    <div
                      key={rule.editorId}
                      className='space-y-2 rounded-md border border-dashed p-3'
                    >
                      <div className='flex items-center justify-between gap-2'>
                        <Label>
                          {t('Rule')} {ruleIndex + 1}
                        </Label>
                        <div className='flex items-center gap-2'>
                          <Select
                            items={LOGIC_OPTIONS}
                            value={rule.logic || 'and'}
                            onValueChange={(logic) =>
                              logic &&
                              updateRules((rules) => {
                                const item =
                                  rules[groupIndex].rules?.[ruleIndex]
                                if (item) item.logic = logic
                              })
                            }
                          >
                            <SelectTrigger className='w-28'>
                              <SelectValue />
                            </SelectTrigger>
                            <SelectContent alignItemWithTrigger={false}>
                              <SelectGroup>
                                <SelectItem value='and'>AND</SelectItem>
                                <SelectItem value='or'>OR</SelectItem>
                              </SelectGroup>
                            </SelectContent>
                          </Select>
                          <Button
                            type='button'
                            variant='ghost'
                            size='icon'
                            aria-label={t('Remove')}
                            onClick={() =>
                              updateRules((rules) => {
                                rules[groupIndex].rules?.splice(ruleIndex, 1)
                              })
                            }
                          >
                            <Trash2 />
                          </Button>
                        </div>
                      </div>
                      {rule.conditions?.length
                        ? renderConditionRows(
                            rule.conditions,
                            groupIndex,
                            ruleIndex
                          )
                        : null}
                      <Button
                        type='button'
                        variant='outline'
                        size='sm'
                        onClick={() =>
                          updateRules((rules) => {
                            const item = rules[groupIndex].rules?.[ruleIndex]
                            if (item) {
                              item.conditions = [
                                ...(item.conditions || []),
                                { type: 'spend', min_spend: 0 },
                              ]
                            }
                          })
                        }
                      >
                        <Plus data-icon='inline-start' />
                        {t('Add condition')}
                      </Button>
                    </div>
                  ))}
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    onClick={() =>
                      updateRules((rules) => {
                        const item = rules[groupIndex]
                        item.rules = [
                          ...(item.rules || []),
                          { logic: 'and', conditions: [] },
                        ]
                      })
                    }
                  >
                    <Plus data-icon='inline-start' />
                    {t('Add Rule')}
                  </Button>
                </div>
              </div>
            ))}
          </div>
          <JsonCodeEditor
            value={value}
            onChange={setValue}
            name='group-access-rules'
          />
          {!validation.valid && (
            <p className='text-destructive text-sm'>
              {validation.message || t('JSON structure is invalid')}
            </p>
          )}
          <p className='text-muted-foreground text-xs'>
            {t(
              'Visual condition fields: use type "spend" with min_spend in quota units to require user consumption.'
            )}
          </p>
          <p className='text-muted-foreground text-sm'>{t('Example rule')}:</p>
          <pre className='bg-muted/60 overflow-x-auto rounded-md border p-3 text-xs whitespace-pre-wrap'>
            {DEFAULT_RULES}
          </pre>
        </CardContent>
      </Card>
    </>
  )
}
