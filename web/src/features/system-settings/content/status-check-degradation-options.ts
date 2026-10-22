/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

export type DegradationModelConfig = {
  model: string
  effort: string
}

export type DegradationGroupConfig = {
  enabled: boolean
  interval_minutes: number
  models: DegradationModelConfig[]
}

export const defaultDegradationGroup: DegradationGroupConfig = {
  enabled: false,
  interval_minutes: 60,
  models: [],
}

export const DEGRADATION_INTERVAL_MIN = 5
export const DEGRADATION_INTERVAL_MAX = 10080
export const DEGRADATION_MAX_MODELS = 20
// Mirrors perfmetrics.StatusCheckDegradationEfforts; '' keeps the model default.
export const DEGRADATION_EFFORTS = [
  '',
  'none',
  'minimal',
  'low',
  'medium',
  'high',
  'xhigh',
  'max',
] as const
