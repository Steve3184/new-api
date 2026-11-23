/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useTranslation } from 'react-i18next'

import {
  StaticDataTable,
  StaticRowActions,
  type StaticDataTableColumn,
  staticDataTableClassNames,
} from '@/components/data-table'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Switch } from '@/components/ui/switch'

import type { DegradationGroupConfig } from './status-check-degradation-options'
import {
  FLEXIBLE_CONTINUOUS_PROBES,
  type FlexibleGroupConfig,
} from './status-check-flexible-options'

export type StatusCheckGroupRow = {
  group: string
  flexible: FlexibleGroupConfig
  degradation: DegradationGroupConfig
  /** The row holds a value the page form rejects, so saving is blocked. */
  invalid: boolean
}

/**
 * One row per visible group, so many groups stay compact and comparable.
 * Detailed settings open in the group sheet; the switch toggles the probe
 * in place because it is the most frequent change.
 */
export function StatusCheckGroupTable(props: {
  rows: StatusCheckGroupRow[]
  selected: string[]
  onSelectedChange: (groups: string[]) => void
  onProbeToggle: (group: string, enabled: boolean) => void
  onEdit: (groups: string[]) => void
}) {
  const { t } = useTranslation()
  const allSelected =
    props.rows.length > 0 && props.selected.length === props.rows.length
  const hasSelection = props.selected.length > 0
  const someSelected = hasSelection && !allSelected

  const toggleRow = (group: string, checked: boolean) => {
    props.onSelectedChange(
      checked
        ? [...props.selected, group]
        : props.selected.filter((candidate) => candidate !== group)
    )
  }

  const probeSchedule = (row: StatusCheckGroupRow) => {
    if (row.degradation.enabled) {
      return t('Covered by degradation detection')
    }
    if (!row.flexible.enabled) {
      return t('Off')
    }
    if (row.flexible.max_consecutive_probes === FLEXIBLE_CONTINUOUS_PROBES) {
      return t('Every {{minutes}} min while idle · continuous', {
        minutes: row.flexible.idle_minutes,
      })
    }
    return t('After {{minutes}} min idle · up to {{count}} probes', {
      minutes: row.flexible.idle_minutes,
      count: row.flexible.max_consecutive_probes,
    })
  }

  const columns: StaticDataTableColumn<StatusCheckGroupRow>[] = [
    {
      id: 'select',
      className: 'w-10',
      header: (
        <Checkbox
          checked={allSelected}
          indeterminate={someSelected}
          onCheckedChange={(checked) =>
            props.onSelectedChange(
              checked ? props.rows.map((row) => row.group) : []
            )
          }
          aria-label={t('Select all')}
        />
      ),
      cell: (row) => (
        <Checkbox
          checked={props.selected.includes(row.group)}
          onCheckedChange={(checked) => toggleRow(row.group, checked)}
          aria-label={t('Select group {{group}}', { group: row.group })}
        />
      ),
    },
    {
      id: 'group',
      header: t('Group'),
      cell: (row) => (
        <div className='flex min-w-0 items-center gap-2'>
          <span className='truncate font-medium'>{row.group}</span>
          {row.invalid && (
            <Badge variant='destructive'>{t('Needs fixing')}</Badge>
          )}
        </div>
      ),
    },
    {
      id: 'probe',
      className: 'w-20',
      header: t('Probe'),
      cell: (row) => (
        <Switch
          checked={row.flexible.enabled && !row.degradation.enabled}
          disabled={row.degradation.enabled}
          onCheckedChange={(enabled) => props.onProbeToggle(row.group, enabled)}
          aria-label={t('Availability probe for {{group}}', {
            group: row.group,
          })}
        />
      ),
    },
    {
      id: 'schedule',
      header: t('Schedule'),
      cellClassName: (row) =>
        row.flexible.enabled && !row.degradation.enabled
          ? undefined
          : staticDataTableClassNames.mutedCell,
      cell: (row) => <span>{probeSchedule(row)}</span>,
    },
    {
      id: 'model',
      header: t('Probe model'),
      cellClassName: (row) =>
        row.flexible.model
          ? staticDataTableClassNames.codeCell
          : staticDataTableClassNames.mutedCell,
      cell: (row) => (
        <span>{row.flexible.model || t('Channel test model')}</span>
      ),
    },
    {
      id: 'degradation',
      header: t('Degradation detection'),
      cellClassName: (row) =>
        row.degradation.enabled
          ? undefined
          : staticDataTableClassNames.mutedCell,
      cell: (row) => (
        <span>
          {row.degradation.enabled
            ? t('Every {{minutes}} min · {{count}} models', {
                minutes: row.degradation.interval_minutes,
                count: row.degradation.models.length,
              })
            : t('Off')}
        </span>
      ),
    },
    {
      id: 'actions',
      className: staticDataTableClassNames.actionHeaderCell,
      cellClassName: staticDataTableClassNames.actionCell,
      header: <span className='sr-only'>{t('Actions')}</span>,
      cell: (row) => (
        <StaticRowActions
          editLabel={t('Edit {{group}}', { group: row.group })}
          deleteLabel={t('Delete')}
          menuLabel={t('Open menu')}
          onEdit={() => props.onEdit([row.group])}
        />
      ),
    },
  ]

  return (
    <div className='space-y-2'>
      {/* Always rendered with rows so selecting one never shifts the rows below. */}
      {props.rows.length > 0 && (
      <div className='bg-muted/40 flex flex-wrap items-center gap-2 rounded-md border px-3 py-2'>
        <span
          className={
            hasSelection ? 'text-sm' : 'text-muted-foreground text-sm'
          }
        >
          {hasSelection
            ? t('{{count}} groups selected', { count: props.selected.length })
            : t('Select groups to edit them together')}
        </span>
        <div className='ms-auto flex gap-2'>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            disabled={!hasSelection}
            onClick={() => props.onSelectedChange([])}
          >
            {t('Clear selection')}
          </Button>
          <Button
            type='button'
            size='sm'
            disabled={!hasSelection}
            onClick={() => props.onEdit(props.selected)}
          >
            {t('Edit selected')}
          </Button>
        </div>
      </div>
      )}
      <StaticDataTable
        tableClassName={staticDataTableClassNames.compactTable}
        columns={columns}
        data={props.rows}
        getRowKey={(row) => row.group}
        emptyContent={t(
          'Select one or more visible groups to configure probes.'
        )}
      />
    </div>
  )
}
