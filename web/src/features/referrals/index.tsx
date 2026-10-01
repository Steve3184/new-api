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
import { useTranslation } from 'react-i18next'

import { Main } from '@/components/layout'

import { ReferralRewardsPanel } from './rewards-panel'

export function Referrals() {
  const { t } = useTranslation()
  return (
    <Main>
      <div className='min-h-0 flex-1 space-y-5 overflow-auto p-4 sm:p-6'>
        <h1 className='text-2xl font-semibold'>{t('Referral management')}</h1>
        <ReferralRewardsPanel admin />
      </div>
    </Main>
  )
}
