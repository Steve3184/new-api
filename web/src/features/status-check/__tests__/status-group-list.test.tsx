/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
*/
import { cleanup, render, screen, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import dayjs from 'dayjs'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import type {
  StatusBeat,
  StatusGroup,
} from '@/features/performance-metrics/types'

import { StatusGroupList } from '../status-group-list'

const getAnimationsDescriptor = Object.getOwnPropertyDescriptor(
  Element.prototype,
  'getAnimations'
)

beforeEach(() => {
  Object.defineProperty(Element.prototype, 'getAnimations', {
    configurable: true,
    value: () => [],
  })
})

afterEach(() => {
  cleanup()
  vi.unstubAllGlobals()
  if (getAnimationsDescriptor) {
    Object.defineProperty(
      Element.prototype,
      'getAnimations',
      getAnimationsDescriptor
    )
  } else {
    Reflect.deleteProperty(Element.prototype, 'getAnimations')
  }
})

const HOUR = 3600
const LAST_HOUR = 1_760_000_400 - (1_760_000_400 % HOUR)

function buildBeats(
  count: number,
  filled: Record<number, Partial<StatusBeat>>
): StatusBeat[] {
  return Array.from({ length: count }, (_, index) => ({
    ts: LAST_HOUR - (count - 1 - index) * HOUR,
    request_count: 0,
    success_count: 0,
    availability: 0,
    avg_ttft_ms: 0,
    probe_filled: false,
    ...filled[index],
  }))
}

/** Makes every observed element report the given content width. */
function mockObservedWidth(width: number) {
  vi.stubGlobal(
    'ResizeObserver',
    class {
      callback: ResizeObserverCallback
      constructor(callback: ResizeObserverCallback) {
        this.callback = callback
      }
      observe(target: Element) {
        this.callback(
          [{ target, contentRect: { width } } as ResizeObserverEntry],
          this as unknown as ResizeObserver
        )
      }
      unobserve() {}
      disconnect() {}
    }
  )
}

function buildGroup(overrides: Partial<StatusGroup>): StatusGroup {
  return {
    group: 'default',
    availability: 99.5,
    avg_latency_ms: 820,
    avg_ttft_ms: 640,
    cache_hit_rate: 37.5,
    cache_sample_count: 10,
    cache_input_tokens: 1000,
    request_count: 10,
    availability_24h: [99.5],
    history_24h: [],
    beats: buildBeats(24, {
      23: { request_count: 10, success_count: 10, availability: 99.5 },
    }),
    ...overrides,
  }
}

describe('StatusGroupList', () => {
  it('renders configured models as compact lines inside their group without detail buttons', () => {
    render(
      <StatusGroupList
        groups={[
          buildGroup({
            group: 'vip',
            degradation: [
              {
                model: 'gpt-5',
                effort: 'high',
                pass_rate: 50,
                judged_count: 2,
                checks: [
                  {
                    ts: 1_760_000_000,
                    status: 'passed',
                    answer: '21',
                    latency_ms: 900,
                  },
                  {
                    ts: 1_760_003_600,
                    status: 'degraded',
                    answer: '29',
                    latency_ms: 400,
                  },
                ],
              },
              {
                model: 'claude-opus',
                effort: '',
                pass_rate: 0,
                judged_count: 0,
                checks: [],
              },
            ],
          }),
          buildGroup({ group: 'default' }),
        ]}
        bucketSeconds={HOUR}
        onSelectGroup={vi.fn()}
      />
    )

    const modelList = screen.getByRole('list', {
      name: 'Model degradation for vip',
    })
    const lines = within(modelList).getAllByRole('listitem')
    expect(lines).toHaveLength(2)
    expect(within(lines[0]).getByText('gpt-5')).toBeVisible()
    expect(within(lines[0]).getByText('(high)')).toBeVisible()
    expect(within(lines[0]).getByText('50.0%')).toBeVisible()
    expect(
      within(lines[0]).getByLabelText(/^Model degraded: /)
    ).toBeInTheDocument()
    expect(
      within(lines[0]).getByLabelText(/^Not degraded: /)
    ).toBeInTheDocument()
    expect(within(lines[1]).getByText('(Model default)')).toBeVisible()
    expect(within(modelList).queryByRole('button')).toBeNull()
    expect(
      screen.getAllByRole('list', { name: /Model degradation/ })
    ).toHaveLength(1)
  })

  it('shows only the newest beats that fit the available width', () => {
    // (70 + 3 gap) / (4 width + 3 gap) fits 10 narrow lines.
    mockObservedWidth(70)
    render(
      <StatusGroupList
        groups={[
          buildGroup({
            beats: buildBeats(240, {
              237: { request_count: 1, availability: 100, probe_filled: true },
              239: { request_count: 10, availability: 90 },
            }),
          }),
        ]}
        bucketSeconds={HOUR}
        onSelectGroup={vi.fn()}
      />
    )

    const beats = screen.getByRole('img', { name: 'Availability' })
    expect(within(beats).getAllByLabelText(/: /)).toHaveLength(10)
    expect(within(beats).getAllByLabelText(/: No data$/)).toHaveLength(8)
    expect(within(beats).getByLabelText(/: 90\.00%$/)).toBeInTheDocument()
    expect(within(beats).getByLabelText(/: 100\.00%$/)).toBeInTheDocument()
  })

  it('labels each beat with the configured bucket range when buckets are 5 minutes', () => {
    mockObservedWidth(70)
    const bucket = 300
    const lastBucket = 1_760_000_400 - (1_760_000_400 % bucket)
    render(
      <StatusGroupList
        groups={[
          buildGroup({
            beats: [0, 1].map((index) => ({
              ts: lastBucket - (1 - index) * bucket,
              request_count: 4,
              success_count: 4,
              availability: 100,
              avg_ttft_ms: 0,
              probe_filled: false,
            })),
          }),
        ]}
        bucketSeconds={bucket}
        onSelectGroup={vi.fn()}
      />
    )

    const beats = screen.getByRole('img', { name: 'Availability' })
    const labels = within(beats)
      .getAllByLabelText(/: 100\.00%$/)
      .map((beat) => beat.getAttribute('aria-label'))
    const end = dayjs.unix(lastBucket + bucket).format('HH:mm')
    const start = dayjs.unix(lastBucket).format('YYYY-MM-DD HH:mm')
    expect(labels).toHaveLength(2)
    expect(labels[1]).toBe(`${start}–${end}: 100.00%`)
  })

  it('lays groups out in two columns on wide screens', () => {
    render(
      <StatusGroupList
        groups={[buildGroup({ group: 'a' }), buildGroup({ group: 'b' })]}
        bucketSeconds={HOUR}
        onSelectGroup={vi.fn()}
      />
    )

    const list = screen.getAllByRole('list')[0]
    expect(list).toHaveClass('grid-cols-1', 'xl:grid-cols-2')
    expect(within(list).getAllByRole('listitem')).toHaveLength(2)
  })

  it('keeps a lone last group at half width when the group count is odd', () => {
    render(
      <StatusGroupList
        groups={[
          buildGroup({ group: 'a' }),
          buildGroup({ group: 'b' }),
          buildGroup({ group: 'c' }),
        ]}
        bucketSeconds={HOUR}
        onSelectGroup={vi.fn()}
      />
    )

    const list = screen.getAllByRole('list')[0]
    const items = within(list).getAllByRole('listitem')
    expect(items).toHaveLength(3)
    expect(items[2].className).not.toMatch(/col-span-2/)
    expect(list.children).toHaveLength(4)
    expect(list.lastElementChild).toHaveAttribute('aria-hidden', 'true')
  })

  it('pads model checks with empty lines when fewer checks than fit', () => {
    mockObservedWidth(70)
    render(
      <StatusGroupList
        groups={[
          buildGroup({
            degradation: [
              {
                model: 'gpt-5',
                effort: 'high',
                pass_rate: 100,
                judged_count: 1,
                checks: [
                  {
                    ts: 1_760_000_000,
                    status: 'passed',
                    answer: '21',
                    latency_ms: 900,
                  },
                ],
              },
            ],
          }),
        ]}
        bucketSeconds={HOUR}
        onSelectGroup={vi.fn()}
      />
    )

    const checks = screen.getByRole('img', {
      name: 'Degradation checks for gpt-5',
    })
    expect(checks.children).toHaveLength(10)
    expect(within(checks).getAllByLabelText(/^Not degraded: /)).toHaveLength(1)
  })

  it('summarizes cache and first token in the group line and opens history on click', async () => {
    const onSelectGroup = vi.fn()
    const group = buildGroup({ group: 'default' })
    render(
      <StatusGroupList
        groups={[group]}
        bucketSeconds={HOUR}
        onSelectGroup={onSelectGroup}
      />
    )

    const header = screen.getByRole('button', {
      name: 'View default status history',
    })
    expect(within(header).getByText('99.5%')).toBeVisible()
    expect(within(header).getByText('Cache 37.5%')).toBeVisible()
    expect(within(header).getByText('First token 640ms')).toBeVisible()
    await userEvent.click(header)

    expect(onSelectGroup).toHaveBeenCalledWith(group)
  })
})
