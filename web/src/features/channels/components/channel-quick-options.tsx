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
import { useId } from 'react'
import { useFormContext, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { FieldGroup } from '@/components/ui/field'
import { SettingsSwitchField } from '@/features/system-settings/components/settings-form-layout'

import { CHANNEL_TYPE_TASK_PLUGIN, MODEL_FETCHABLE_TYPES } from '../constants'
import type { ChannelFormValues } from '../lib/channel-form'

type ChannelQuickOptionsProps = {
  channelType: number
  sensitiveLocked: boolean
  disabled: boolean
}

export function ChannelQuickOptions(props: ChannelQuickOptionsProps) {
  const { t } = useTranslation()
  const id = useId()
  const form = useFormContext<ChannelFormValues>()
  const [passthrough, autoBan, modelCheck, websocket] = useWatch({
    control: form.control,
    name: [
      'pass_through_body_enabled',
      'auto_ban',
      'upstream_model_update_check_enabled',
      'responses_websocket_enabled',
    ],
  })
  const sensitiveDisabled = props.sensitiveLocked || props.disabled

  return (
    <section
      role='group'
      aria-labelledby={`${id}-title`}
      className='border-border/60 @container border-t px-4 pt-4 pb-2'
    >
      <h3 id={`${id}-title`} className='mb-1 text-sm font-semibold'>
        {t('Quick options')}
      </h3>
      <FieldGroup className='grid gap-x-6 gap-y-0 @sm:grid-cols-2'>
        {props.channelType !== CHANNEL_TYPE_TASK_PLUGIN && (
          <SettingsSwitchField
            controlId={`${id}-passthrough`}
            checked={passthrough === true}
            onCheckedChange={(value) =>
              form.setValue('pass_through_body_enabled', value, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
            label={t('Pass Through Body')}
            description={t('Preserve the original request body')}
            disabled={sensitiveDisabled}
          />
        )}
        <SettingsSwitchField
          controlId={`${id}-auto-ban`}
          checked={(autoBan ?? 1) === 1}
          onCheckedChange={(value) =>
            form.setValue('auto_ban', value ? 1 : 0, {
              shouldDirty: true,
              shouldValidate: true,
            })
          }
          label={t('Auto-disable channel')}
          description={t('Disable channels on repeated failures')}
          disabled={props.disabled}
        />
        {MODEL_FETCHABLE_TYPES.has(props.channelType) && (
          <SettingsSwitchField
            controlId={`${id}-model-check`}
            checked={modelCheck === true}
            onCheckedChange={(value) =>
              form.setValue('upstream_model_update_check_enabled', value, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
            label={t('Detect model updates')}
            description={t('Check for new upstream models')}
            disabled={sensitiveDisabled}
          />
        )}
        {(props.channelType === 1 || props.channelType === 57) && (
          <SettingsSwitchField
            controlId={`${id}-websocket`}
            checked={websocket === true}
            onCheckedChange={(value) =>
              form.setValue('responses_websocket_enabled', value, {
                shouldDirty: true,
                shouldValidate: true,
              })
            }
            label={t('Responses WebSocket')}
            description={t('Requires upstream WebSocket support')}
            disabled={sensitiveDisabled}
          />
        )}
      </FieldGroup>
    </section>
  )
}
