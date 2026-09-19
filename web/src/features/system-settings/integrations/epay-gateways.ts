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
import { safeJsonParseWithValidation } from '../utils/json-parser'
import { isArray } from '../utils/json-validators'
import type { PaymentMethodData } from './payment-method-dialog'

export type EpayGatewayData = {
  id: string
  name: string
  address: string
  merchant_id: string
  key: string
  enabled: boolean
  pay_methods: PaymentMethodData[]
}

function isPaymentMethod(value: unknown): value is PaymentMethodData {
  if (!value || typeof value !== 'object') return false
  const method = value as Record<string, unknown>
  return typeof method.name === 'string' && typeof method.type === 'string'
}

function isEpayGateway(value: unknown): value is EpayGatewayData {
  if (!value || typeof value !== 'object') return false
  const gateway = value as Record<string, unknown>
  return (
    typeof gateway.id === 'string' &&
    typeof gateway.name === 'string' &&
    typeof gateway.address === 'string' &&
    typeof gateway.merchant_id === 'string' &&
    typeof gateway.key === 'string' &&
    typeof gateway.enabled === 'boolean' &&
    Array.isArray(gateway.pay_methods) &&
    gateway.pay_methods.every(isPaymentMethod)
  )
}

export function parseEpayGateways(value: string): EpayGatewayData[] {
  const parsed = safeJsonParseWithValidation<unknown[]>(value, {
    fallback: [],
    validator: isArray,
    validatorMessage: 'Epay gateways must be a JSON array',
    context: 'Epay gateways',
  })
  return parsed.filter(isEpayGateway)
}

export function serializeEpayGateways(gateways: EpayGatewayData[]): string {
  return JSON.stringify(gateways, null, 2)
}
