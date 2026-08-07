/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Coins, Hash, Users } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { formatQuota } from '@/lib/format'

import { formatTokens } from '../lib/format'
import type { RankingPeriod, UserRanking } from '../types'

type UserLeaderboardProps = {
  byQuota: UserRanking[]
  byTokens: UserRanking[]
  period: RankingPeriod
}

const PERIOD_LABEL_KEYS: Record<RankingPeriod, string> = {
  today: 'Today',
  week: 'Week',
  month: 'Month',
  year: 'Year',
}

export function UserLeaderboard(props: UserLeaderboardProps) {
  const { t } = useTranslation()

  return (
    <section className='bg-card overflow-hidden rounded-lg border'>
      <header className='border-b px-5 py-4'>
        <h2 className='text-foreground inline-flex items-center gap-2 text-base font-semibold'>
          <Users className='text-primary size-4' />
          {t('User Leaderboard')}
        </h2>
        <p className='text-muted-foreground mt-1 text-sm'>
          {t(
            'Top users by usage and quota for the selected period: {{period}}',
            {
              period: t(PERIOD_LABEL_KEYS[props.period]),
            }
          )}
        </p>
      </header>

      <div className='grid gap-x-8 md:grid-cols-2'>
        <UserRankingColumn
          icon={<Hash className='size-4 text-sky-500' />}
          title={t('Token Usage')}
          valueLabel={t('tokens')}
          rows={props.byTokens}
          formatValue={(row) => formatTokens(row.total_tokens)}
        />
        <UserRankingColumn
          icon={<Coins className='size-4 text-amber-500' />}
          title={t('Quota Usage')}
          valueLabel={t('quota')}
          rows={props.byQuota}
          formatValue={(row) => formatQuota(row.total_quota)}
        />
      </div>
    </section>
  )
}

type UserRankingColumnProps = {
  icon: ReactNode
  title: string
  valueLabel: string
  rows: UserRanking[]
  formatValue: (row: UserRanking) => string
}

function UserRankingColumn(props: UserRankingColumnProps) {
  const { t } = useTranslation()

  return (
    <div className='min-w-0 px-5 py-3 first:border-b md:first:border-r md:first:border-b-0'>
      <header className='flex items-center gap-2 py-2'>
        {props.icon}
        <h3 className='text-foreground text-sm font-semibold'>{props.title}</h3>
      </header>

      {props.rows.length === 0 ? (
        <p className='text-muted-foreground/80 py-8 text-center text-sm'>
          {t('No user usage data available')}
        </p>
      ) : (
        <ol>
          {props.rows.map((row) => (
            <li
              key={`${row.rank}-${row.username}`}
              className='flex min-w-0 items-center gap-3 border-t py-2.5'
            >
              <span className='text-muted-foreground/80 w-6 shrink-0 text-right font-mono text-xs tabular-nums'>
                {row.rank}.
              </span>
              <div className='min-w-0 flex-1'>
                <p className='text-foreground truncate text-sm font-medium'>
                  {row.display_name || row.username || t('Unknown user')}
                </p>
                <p className='text-muted-foreground/80 truncate text-xs'>
                  {row.display_name && row.username ? `${row.username} · ` : ''}
                  {t('Top group')}: {row.top_group || t('Unknown')}
                </p>
              </div>
              <div className='shrink-0 text-right'>
                <div className='text-foreground font-mono text-sm font-semibold tabular-nums'>
                  {props.formatValue(row)}
                </div>
                <div className='text-muted-foreground/80 text-[10px] font-medium tracking-widest uppercase'>
                  {props.valueLabel}
                </div>
              </div>
            </li>
          ))}
        </ol>
      )}
    </div>
  )
}
