/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { toIntlLocale } from '@/i18n/languages'
import { formatNumber } from '@/lib/format'

import { SettingsSwitchField } from '../components/settings-form-layout'
import { StatusCheckDegradationConfig } from './status-check-degradation-config'
import {
  DEGRADATION_INTERVAL_MAX,
  DEGRADATION_INTERVAL_MIN,
  type DegradationGroupConfig,
} from './status-check-degradation-options'
import {
  FLEXIBLE_CONTINUOUS_PROBES,
  FLEXIBLE_IDLE_MAX,
  FLEXIBLE_IDLE_MIN,
  FLEXIBLE_PROBES_MAX,
  FLEXIBLE_PROBES_MIN,
  type FlexibleGroupConfig,
  defaultFlexibleGroup,
  isValidIdleMinutes,
  isValidProbeLimit,
} from './status-check-flexible-options'
import { StatusCheckProbeModelField } from './status-check-probe-model-field'

const MINUTES_PER_DAY = 24 * 60

export type StatusCheckGroupSheetApply = {
  /** A batch edit leaves model out, so each group keeps its own probe model. */
  flexible: Omit<FlexibleGroupConfig, 'model'> & { model?: string }
  /** Absent when several groups are edited: detection models are per group. */
  degradation?: DegradationGroupConfig
}

/**
 * Edits the probe settings of one group, or of several groups at once. A batch
 * edit replaces the availability probe settings of every selected group and
 * leaves probe models and degradation detection untouched, because both are
 * chosen from each group's own models. Applying only updates the page form;
 * the page's save action still persists the change.
 */
export function StatusCheckGroupSheet(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  groups: string[]
  flexible: FlexibleGroupConfig
  degradation: DegradationGroupConfig
  onApply: (update: StatusCheckGroupSheetApply) => void
}) {
  const { t, i18n } = useTranslation()
  const locale = toIntlLocale(i18n.resolvedLanguage || i18n.language)
  const batch = props.groups.length > 1
  const group = props.groups[0] ?? ''
  const [enabled, setEnabled] = useState(props.flexible.enabled)
  const [idleMinutes, setIdleMinutes] = useState(props.flexible.idle_minutes)
  const [continuous, setContinuous] = useState(
    props.flexible.max_consecutive_probes === FLEXIBLE_CONTINUOUS_PROBES
  )
  // Remembered separately so turning continuous mode off restores the limit.
  const [probeLimit, setProbeLimit] = useState(
    props.flexible.max_consecutive_probes === FLEXIBLE_CONTINUOUS_PROBES
      ? defaultFlexibleGroup.max_consecutive_probes
      : props.flexible.max_consecutive_probes
  )
  const [model, setModel] = useState(props.flexible.model)
  const [degradation, setDegradation] = useState(props.degradation)
  const [showErrors, setShowErrors] = useState(false)

  // Degradation checks already fill idle hours, so the backend skips flexible
  // probes for groups that run them.
  const coveredByDegradation = !batch && degradation.enabled
  const idleInvalid = !isValidIdleMinutes(idleMinutes)
  const limitInvalid = !continuous && !isValidProbeLimit(probeLimit)
  const intervalInvalid =
    !batch &&
    !(
      Number.isInteger(degradation.interval_minutes) &&
      degradation.interval_minutes >= DEGRADATION_INTERVAL_MIN &&
      degradation.interval_minutes <= DEGRADATION_INTERVAL_MAX
    )
  const probeControlsDisabled = !enabled || coveredByDegradation
  const rangeMessage = (min: number, max: number) =>
    t('Enter a whole number from {{min}} to {{max}}', { min, max })

  const apply = () => {
    if (idleInvalid || limitInvalid || intervalInvalid) {
      setShowErrors(true)
      return
    }
    const flexible: StatusCheckGroupSheetApply['flexible'] = {
      enabled,
      idle_minutes: idleMinutes,
      max_consecutive_probes: continuous
        ? FLEXIBLE_CONTINUOUS_PROBES
        : probeLimit,
    }
    if (batch) {
      props.onApply({ flexible })
    } else {
      props.onApply({
        flexible: { ...flexible, model: model.trim() },
        degradation,
      })
    }
    props.onOpenChange(false)
  }

  const controlId = batch
    ? 'status-check-batch'
    : `status-check-group-${group}`

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {batch
              ? t('Edit {{count}} groups', { count: props.groups.length })
              : group}
          </SheetTitle>
          <SheetDescription>
            {batch
              ? t(
                  'These availability probe settings replace the current ones of every selected group. Probe models and degradation detection stay per group.'
                )
              : t(
                  'Applying updates the group list; save the page to keep the change.'
                )}
          </SheetDescription>
        </SheetHeader>
        <div className={sideDrawerFormClassName()}>
          <SideDrawerSection>
            <SideDrawerSectionHeader title={t('Availability probe')} />
            <SettingsSwitchField
              controlId={`${controlId}-enabled`}
              checked={enabled && !coveredByDegradation}
              disabled={coveredByDegradation}
              onCheckedChange={setEnabled}
              label={t('Enable availability probe')}
              description={
                coveredByDegradation
                  ? t(
                      'Not needed while model degradation detection is on: its checks fill hours without requests.'
                    )
                  : t(
                      'Probe this group only when it has no real relay requests.'
                    )
              }
            />
            <div className='space-y-2'>
              <Label htmlFor={`${controlId}-idle`}>
                {t('Idle period (minutes)')}
              </Label>
              <Input
                id={`${controlId}-idle`}
                type='number'
                min={FLEXIBLE_IDLE_MIN}
                max={FLEXIBLE_IDLE_MAX}
                step={1}
                disabled={probeControlsDisabled}
                aria-invalid={showErrors && idleInvalid}
                value={Number.isNaN(idleMinutes) ? '' : idleMinutes}
                onChange={(event) =>
                  setIdleMinutes(event.target.valueAsNumber)
                }
              />
              <p className='text-muted-foreground text-xs'>
                {t('Minimum time without a relay request before probing.')}
              </p>
              {showErrors && idleInvalid && (
                <p className='text-destructive text-xs' role='alert'>
                  {rangeMessage(FLEXIBLE_IDLE_MIN, FLEXIBLE_IDLE_MAX)}
                </p>
              )}
            </div>
            <SettingsSwitchField
              controlId={`${controlId}-continuous`}
              checked={continuous}
              disabled={probeControlsDisabled}
              onCheckedChange={setContinuous}
              label={t('Continuous monitoring')}
              description={t(
                'Keep probing once per idle period for as long as the group has no real traffic. Real requests replace probes, so busy hours cost nothing.'
              )}
            />
            {continuous ? (
              !idleInvalid && (
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'At most {{count}} probes a day, only while the group has no traffic.',
                    {
                      count: formatNumber(
                        Math.floor(MINUTES_PER_DAY / idleMinutes),
                        locale
                      ),
                    }
                  )}
                </p>
              )
            ) : (
              <div className='space-y-2'>
                <Label htmlFor={`${controlId}-limit`}>
                  {t('Maximum consecutive probes')}
                </Label>
                <Input
                  id={`${controlId}-limit`}
                  type='number'
                  min={FLEXIBLE_PROBES_MIN}
                  max={FLEXIBLE_PROBES_MAX}
                  step={1}
                  disabled={probeControlsDisabled}
                  aria-invalid={showErrors && limitInvalid}
                  value={Number.isNaN(probeLimit) ? '' : probeLimit}
                  onChange={(event) =>
                    setProbeLimit(event.target.valueAsNumber)
                  }
                />
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'After this many automatic probes without a real relay request, probing pauses until traffic resumes.'
                  )}
                </p>
                {showErrors && limitInvalid && (
                  <p className='text-destructive text-xs' role='alert'>
                    {rangeMessage(FLEXIBLE_PROBES_MIN, FLEXIBLE_PROBES_MAX)}
                  </p>
                )}
              </div>
            )}
            {!batch && (
              <StatusCheckProbeModelField
                group={group}
                value={model}
                disabled={probeControlsDisabled}
                onChange={setModel}
              />
            )}
          </SideDrawerSection>
          {!batch && (
            <SideDrawerSection>
              <StatusCheckDegradationConfig
                className='border-t-0 pt-0'
                group={group}
                value={degradation}
                intervalError={
                  showErrors && intervalInvalid
                    ? rangeMessage(
                        DEGRADATION_INTERVAL_MIN,
                        DEGRADATION_INTERVAL_MAX
                      )
                    : undefined
                }
                onChange={(update) =>
                  setDegradation((current) => ({ ...current, ...update }))
                }
              />
            </SideDrawerSection>
          )}
        </div>
        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button type='button' variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button type='button' onClick={apply}>
            {t('Apply')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
