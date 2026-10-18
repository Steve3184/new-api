import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { useState } from 'react'
import { FormProvider, useForm } from 'react-hook-form'
import { describe, expect, test } from 'vitest'

import {
  CHANNEL_FORM_DEFAULT_VALUES,
  channelFormSchema,
  transformChannelToFormDefaults,
  transformFormDataToCreatePayload,
  transformFormDataToUpdatePayload,
} from '../../lib/channel-form'
import { channelSchema } from '../../types'
import { MultiKeyReliabilityEditor } from '../multi-key-reliability-editor'

function ReliabilityForm() {
  const form = useForm()
  const [rules, setRules] = useState('[{"status_code":429}]')
  const [autoRetry, setAutoRetry] = useState(false)
  const [autoRecovery, setAutoRecovery] = useState(false)
  const [interval, setInterval] = useState(10)
  const [inventoryEnabled, setInventoryEnabled] = useState(false)
  const [inventoryTarget, setInventoryTarget] = useState(1)
  return (
    <FormProvider {...form}>
      <MultiKeyReliabilityEditor
        rules={rules}
        autoRetry={autoRetry}
        autoRecovery={autoRecovery}
        recoveryIntervalMinutes={interval}
        inventoryEnabled={inventoryEnabled}
        inventoryTarget={inventoryTarget}
        onRulesChange={setRules}
        onAutoRetryChange={setAutoRetry}
        onAutoRecoveryChange={setAutoRecovery}
        onRecoveryIntervalChange={setInterval}
        onInventoryEnabledChange={setInventoryEnabled}
        onInventoryTargetChange={setInventoryTarget}
      />
    </FormProvider>
  )
}

describe('multi-key reliability', () => {
  test('keeps focus while typing status and message, including duplicate rules', async () => {
    const user = userEvent.setup()
    render(<ReliabilityForm />)
    await user.click(screen.getByRole('button', { name: 'Add rule' }))
    const statuses = screen.getAllByRole('spinbutton', { name: 'Status code' })
    await user.clear(statuses[0])
    await user.type(statuses[0], '503')
    expect(statuses[0]).toHaveFocus()
    expect(statuses[0]).toHaveValue(503)
    const messages = screen.getAllByRole('textbox', {
      name: 'Message contains',
    })
    await user.type(messages[1], 'quota exhausted')
    expect(messages[1]).toHaveFocus()
    expect(messages[1]).toHaveValue('quota exhausted')
    await user.click(screen.getAllByRole('button', { name: 'Delete' })[0])
    const remaining = screen.getByRole('textbox', { name: 'Message contains' })
    expect(remaining).toHaveValue('quota exhausted')
    await user.type(remaining, ' today')
    expect(remaining).toHaveFocus()
    expect(remaining).toHaveValue('quota exhausted today')
  })

  test('enables the inventory target only when inventory mode is enabled', async () => {
    const user = userEvent.setup()
    render(<ReliabilityForm />)
    const target = screen.getByRole('spinbutton', { name: 'Active key target' })
    expect(target).toBeDisabled()
    await user.click(screen.getByRole('switch', { name: 'Key inventory mode' }))
    expect(target).toBeEnabled()
    await user.click(target)
    await user.keyboard('{Control>}a{/Control}3')
    expect(target).toHaveValue(3)
    await user.click(screen.getByRole('switch', { name: 'Key inventory mode' }))
    expect(target).toBeDisabled()
    expect(target).toHaveValue(3)
  })

  test('persists inventory settings through create, reload, and update', () => {
    const values = channelFormSchema.parse({
      ...CHANNEL_FORM_DEFAULT_VALUES,
      name: 'inventory',
      type: 1,
      models: 'test-model',
      key: 'one\ntwo\nthree',
      multi_key_mode: 'multi_to_single',
      multi_key_inventory_enabled: true,
      multi_key_inventory_target: 2,
    })
    const created = transformFormDataToCreatePayload(values)
    expect(created.channel.channel_info).toMatchObject({
      multi_key_inventory_enabled: true,
      multi_key_inventory_target: 2,
    })
    const channel = channelSchema.parse({
      ...created.channel,
      other: '',
      remark: '',
      id: 1,
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      used_quota: 0,
    })
    const defaults = transformChannelToFormDefaults(channel)
    expect(defaults.multi_key_inventory_target).toBe(2)
    expect(defaults.multi_key_inventory_enabled).toBe(true)
    const updated = transformFormDataToUpdatePayload(
      { ...values, multi_key_inventory_enabled: false },
      1
    )
    expect(updated.channel_info).toMatchObject({
      multi_key_inventory_enabled: false,
      multi_key_inventory_target: 2,
    })
    expect(
      channelFormSchema.safeParse({ ...values, multi_key_inventory_target: 0 })
        .success
    ).toBe(false)
    expect(
      channelFormSchema.safeParse({
        ...values,
        multi_key_inventory_target: 1.5,
      }).success
    ).toBe(false)
  })
})
