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
import { Plus, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import type {
  ErrorRewriteRuleDraft,
  ErrorRewriteRuleErrorCode,
} from './error-rewrite-utils'

type ErrorRewriteTableProps = {
  rules: ErrorRewriteRuleDraft[]
  validationErrors: Record<string, ErrorRewriteRuleErrorCode[]>
  disabled: boolean
  onAddRow: () => void
  onDeleteRow: (id: string) => void
  onChangeRule: (
    id: string,
    field: 'statusCode' | 'message',
    value: string
  ) => void
  getErrorText: (code: ErrorRewriteRuleErrorCode) => string
}

export function ErrorRewriteTable(props: ErrorRewriteTableProps) {
  const { t } = useTranslation()

  return (
    <div className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <div className='min-w-0 space-y-1'>
          <h4 className='text-sm font-medium'>{t('Rewrite rules')}</h4>
          <p className='text-muted-foreground text-xs'>
            {t('Rules match the upstream HTTP status code exactly.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={props.onAddRow}
          disabled={props.disabled}
        >
          <Plus data-icon='inline-start' />
          <span>{t('Add Row')}</span>
        </Button>
      </div>

      <div className='overflow-x-auto rounded-md border'>
        <Table className='min-w-[42rem]'>
          <TableHeader>
            <TableRow>
              <TableHead className='w-40'>{t('Status Code')}</TableHead>
              <TableHead>{t('Error Message')}</TableHead>
              <TableHead className='w-16 text-right'>{t('Actions')}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {props.rules.length === 0 ? (
              <TableRow>
                <TableCell
                  colSpan={3}
                  className='text-muted-foreground h-24 text-center text-sm'
                >
                  {t(
                    'No global error rewrite rules configured. Click "Add Row" to get started.'
                  )}
                </TableCell>
              </TableRow>
            ) : (
              props.rules.map((rule, index) => (
                <ErrorRewriteTableRow
                  key={rule.id}
                  rule={rule}
                  index={index}
                  errors={props.validationErrors[rule.id] ?? []}
                  disabled={props.disabled}
                  getErrorText={props.getErrorText}
                  onDelete={props.onDeleteRow}
                  onChange={props.onChangeRule}
                />
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

type ErrorRewriteTableRowProps = {
  rule: ErrorRewriteRuleDraft
  index: number
  errors: ErrorRewriteRuleErrorCode[]
  disabled: boolean
  getErrorText: (code: ErrorRewriteRuleErrorCode) => string
  onDelete: (id: string) => void
  onChange: (id: string, field: 'statusCode' | 'message', value: string) => void
}

function ErrorRewriteTableRow(props: ErrorRewriteTableRowProps) {
  const { t } = useTranslation()
  const statusError = props.errors.find(
    (code) => code === 'invalid-status-code'
  )
  const messageError = props.errors.find((code) => code === 'empty-message')
  const duplicateError = props.errors.find(
    (code) => code === 'duplicate-status-code'
  )
  const statusErrorCode = statusError ?? duplicateError
  const statusDescriptionId = `error-rewrite-status-${props.rule.id}`
  const messageDescriptionId = `error-rewrite-message-${props.rule.id}`

  return (
    <TableRow>
      <TableCell className='align-top'>
        <Input
          type='number'
          min={100}
          max={599}
          step={1}
          inputMode='numeric'
          value={props.rule.statusCode}
          aria-label={`${t('Status Code')} ${props.index + 1}`}
          aria-invalid={statusErrorCode ? 'true' : 'false'}
          aria-describedby={statusErrorCode ? statusDescriptionId : undefined}
          onChange={(event) =>
            props.onChange(props.rule.id, 'statusCode', event.target.value)
          }
          disabled={props.disabled}
        />
        {statusErrorCode && (
          <p id={statusDescriptionId} className='text-destructive mt-1 text-xs'>
            {props.getErrorText(statusErrorCode)}
          </p>
        )}
      </TableCell>
      <TableCell className='align-top'>
        <Input
          value={props.rule.message}
          placeholder={t('e.g. Model {model} is unavailable')}
          aria-label={`${t('Error Message')} ${props.index + 1}`}
          aria-invalid={messageError ? 'true' : 'false'}
          aria-describedby={messageError ? messageDescriptionId : undefined}
          onChange={(event) =>
            props.onChange(props.rule.id, 'message', event.target.value)
          }
          disabled={props.disabled}
        />
        {messageError && (
          <p
            id={messageDescriptionId}
            className='text-destructive mt-1 text-xs'
          >
            {props.getErrorText(messageError)}
          </p>
        )}
      </TableCell>
      <TableCell className='text-right align-top'>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          aria-label={`${t('Delete')} ${props.index + 1}`}
          title={t('Delete')}
          onClick={() => props.onDelete(props.rule.id)}
          disabled={props.disabled}
        >
          <Trash2 aria-hidden='true' />
        </Button>
      </TableCell>
    </TableRow>
  )
}
