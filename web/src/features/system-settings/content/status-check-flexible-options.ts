/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/

export type FlexibleGroupConfig = {
  enabled: boolean
  idle_minutes: number
  /** 0 selects continuous monitoring: no limit while the group stays idle. */
  max_consecutive_probes: number
  /** Empty tests each channel with its own test model. */
  model: string
}

export type FlexibleModeConfig = {
  groups: Record<string, FlexibleGroupConfig>
}

// Mirrors the bounds in pkg/perf_metrics/status_check_flexible.go.
export const FLEXIBLE_IDLE_MIN = 1
export const FLEXIBLE_IDLE_MAX = 1440
export const FLEXIBLE_PROBES_MIN = 1
export const FLEXIBLE_PROBES_MAX = 1000
export const FLEXIBLE_CONTINUOUS_PROBES = 0

export const defaultFlexibleGroup: FlexibleGroupConfig = {
  enabled: false,
  idle_minutes: 15,
  max_consecutive_probes: 40,
  model: '',
}

function isWholeNumberInRange(value: unknown, min: number, max: number) {
  return (
    typeof value === 'number' &&
    Number.isInteger(value) &&
    value >= min &&
    value <= max
  )
}

export function isValidProbeLimit(value: unknown): value is number {
  return (
    value === FLEXIBLE_CONTINUOUS_PROBES ||
    isWholeNumberInRange(value, FLEXIBLE_PROBES_MIN, FLEXIBLE_PROBES_MAX)
  )
}

export function isValidIdleMinutes(value: unknown): value is number {
  return isWholeNumberInRange(value, FLEXIBLE_IDLE_MIN, FLEXIBLE_IDLE_MAX)
}

export function parseFlexibleMode(value: string): FlexibleModeConfig {
  try {
    const parsed = JSON.parse(value) as { groups?: unknown }
    if (
      !parsed.groups ||
      typeof parsed.groups !== 'object' ||
      Array.isArray(parsed.groups)
    ) {
      return { groups: {} }
    }
    const groups: Record<string, FlexibleGroupConfig> = {}
    for (const [group, config] of Object.entries(parsed.groups)) {
      if (!config || typeof config !== 'object' || Array.isArray(config)) {
        continue
      }
      const candidate = config as Partial<FlexibleGroupConfig>
      if (
        typeof candidate.enabled !== 'boolean' ||
        !isValidIdleMinutes(candidate.idle_minutes) ||
        !isValidProbeLimit(candidate.max_consecutive_probes)
      ) {
        continue
      }
      groups[group] = {
        enabled: candidate.enabled,
        idle_minutes: candidate.idle_minutes,
        max_consecutive_probes: candidate.max_consecutive_probes,
        model: typeof candidate.model === 'string' ? candidate.model : '',
      }
    }
    return { groups }
  } catch {
    return { groups: {} }
  }
}
