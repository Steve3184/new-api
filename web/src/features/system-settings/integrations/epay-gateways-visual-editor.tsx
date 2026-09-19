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
import { Plus, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'

import { EpayGatewayDialog } from './epay-gateway-dialog'
import {
  parseEpayGateways,
  serializeEpayGateways,
  type EpayGatewayData,
} from './epay-gateways'
import type { PaymentMethodData } from './payment-method-dialog'

type EpayGatewaysVisualEditorProps = {
  value: string
  onChange: (value: string) => void
}

function paymentMethodSummary(method: PaymentMethodData): string {
  const feeParts: string[] = []
  if (method.fee && Number(method.fee) > 0) feeParts.push(method.fee)
  if (method.fee_rate && Number(method.fee_rate) > 0) {
    feeParts.push(`${method.fee_rate}%`)
  }
  return feeParts.length > 0
    ? `${method.name} (${feeParts.join(' + ')})`
    : method.name
}

export function EpayGatewaysVisualEditor(props: EpayGatewaysVisualEditorProps) {
  const { t } = useTranslation()
  const [searchText, setSearchText] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editData, setEditData] = useState<EpayGatewayData | null>(null)
  const gateways = useMemo(() => parseEpayGateways(props.value), [props.value])
  const filteredGateways = useMemo(() => {
    const query = searchText.trim().toLowerCase()
    if (!query) return gateways
    return gateways.filter((gateway) =>
      [gateway.id, gateway.name, gateway.address, gateway.merchant_id]
        .join(' ')
        .toLowerCase()
        .includes(query)
    )
  }, [gateways, searchText])

  const updateGateways = (nextGateways: EpayGatewayData[]) => {
    props.onChange(serializeEpayGateways(nextGateways))
  }

  const handleSave = (gateway: EpayGatewayData) => {
    if (!editData) {
      updateGateways([...gateways, gateway])
      return
    }
    updateGateways(
      gateways.map((item) => (item.id === editData.id ? gateway : item))
    )
  }

  const handleEdit = (gateway: EpayGatewayData) => {
    setEditData(gateway)
    setDialogOpen(true)
  }

  const handleDelete = (gateway: EpayGatewayData) => {
    updateGateways(gateways.filter((item) => item.id !== gateway.id))
  }

  const handleEnabledChange = (gateway: EpayGatewayData, enabled: boolean) => {
    updateGateways(
      gateways.map((item) =>
        item.id === gateway.id ? { ...item, enabled } : item
      )
    )
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-col gap-3 sm:flex-row sm:items-center'>
        <div className='relative flex-1'>
          <Search className='text-muted-foreground absolute top-2.5 left-2.5 size-4' />
          <Input
            className='pl-9'
            placeholder={t('Search Epay gateways...')}
            value={searchText}
            onChange={(event) => setSearchText(event.target.value)}
          />
        </div>
        <Button
          type='button'
          onClick={() => {
            setEditData(null)
            setDialogOpen(true)
          }}
        >
          <Plus className='size-4 sm:mr-2' />
          {t('Add gateway')}
        </Button>
      </div>

      {filteredGateways.length === 0 ? (
        <div className='text-muted-foreground rounded-lg border border-dashed p-8 text-center text-sm'>
          {searchText
            ? t('No Epay gateways match your search')
            : t('No Epay gateways configured. Add a gateway to get started.')}
        </div>
      ) : (
        <div className='rounded-md border'>
          <StaticDataTable
            className='hidden rounded-none border-0 lg:block'
            data={filteredGateways}
            getRowKey={(gateway) => gateway.id}
            columns={[
              {
                id: 'enabled',
                header: t('Enabled'),
                cell: (gateway) => (
                  <Switch
                    aria-label={t('Enable {{name}}', { name: gateway.name })}
                    checked={gateway.enabled}
                    onCheckedChange={(enabled) =>
                      handleEnabledChange(gateway, enabled)
                    }
                  />
                ),
              },
              {
                id: 'name',
                header: t('Gateway'),
                cell: (gateway) => (
                  <div className='min-w-0'>
                    <div className='truncate font-medium'>{gateway.name}</div>
                    <code className='text-muted-foreground text-xs'>
                      {gateway.id}
                    </code>
                  </div>
                ),
              },
              {
                id: 'address',
                header: t('Epay endpoint'),
                cell: (gateway) => gateway.address,
              },
              {
                id: 'merchant',
                header: t('Epay merchant ID'),
                cell: (gateway) => gateway.merchant_id,
              },
              {
                id: 'methods',
                header: t('Payment methods and fees'),
                cell: (gateway) => (
                  <div className='flex max-w-80 flex-wrap gap-1'>
                    {gateway.pay_methods.length > 0 ? (
                      gateway.pay_methods.map((method) => (
                        <Badge key={method.type} variant='secondary'>
                          {paymentMethodSummary(method)}
                        </Badge>
                      ))
                    ) : (
                      <span className='text-muted-foreground text-sm'>—</span>
                    )}
                  </div>
                ),
              },
              {
                id: 'actions',
                header: t('Actions'),
                className: 'text-right',
                cellClassName: 'text-right',
                cell: (gateway) => (
                  <StaticRowActions
                    editLabel={t('Edit')}
                    deleteLabel={t('Delete')}
                    menuLabel={t('Open menu')}
                    onEdit={() => handleEdit(gateway)}
                    onDelete={() => handleDelete(gateway)}
                  />
                ),
              },
            ]}
          />

          <div className='divide-y lg:hidden'>
            {filteredGateways.map((gateway) => (
              <div key={gateway.id} className='space-y-3 p-4'>
                <div className='flex items-start justify-between gap-3'>
                  <div className='min-w-0'>
                    <div className='truncate font-medium'>{gateway.name}</div>
                    <code className='text-muted-foreground text-xs'>
                      {gateway.id}
                    </code>
                  </div>
                  <div className='flex items-center gap-2'>
                    <Switch
                      aria-label={t('Enable {{name}}', { name: gateway.name })}
                      checked={gateway.enabled}
                      onCheckedChange={(enabled) =>
                        handleEnabledChange(gateway, enabled)
                      }
                    />
                    <StaticRowActions
                      editLabel={t('Edit')}
                      deleteLabel={t('Delete')}
                      menuLabel={t('Open menu')}
                      onEdit={() => handleEdit(gateway)}
                      onDelete={() => handleDelete(gateway)}
                    />
                  </div>
                </div>
                <div className='text-muted-foreground space-y-1 text-xs'>
                  <div className='truncate'>{gateway.address}</div>
                  <div>{gateway.merchant_id}</div>
                </div>
                <div className='flex flex-wrap gap-1'>
                  {gateway.pay_methods.map((method) => (
                    <Badge key={method.type} variant='secondary'>
                      {paymentMethodSummary(method)}
                    </Badge>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <EpayGatewayDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleSave}
        editData={editData}
        existingIds={gateways.map((gateway) => gateway.id)}
      />
    </div>
  )
}
