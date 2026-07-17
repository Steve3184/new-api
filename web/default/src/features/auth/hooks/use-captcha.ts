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
import i18next from 'i18next'
import { useState } from 'react'
import { toast } from 'sonner'

import { useStatus } from '@/hooks/use-status'

type CaptchaPurpose = 'auth' | 'checkin' | 'redemption'

export function useCaptcha(purpose: CaptchaPurpose = 'auth') {
  const { status } = useStatus()
  const [captchaToken, setCaptchaToken] = useState('')
  let captchaType: 'turnstile' | 'hcaptcha' | 'cap' = 'turnstile'
  if (status?.captcha_type === 'cap') {
    captchaType = 'cap'
  } else if (status?.captcha_type === 'hcaptcha') {
    captchaType = 'hcaptcha'
  }
  let isRequired = purpose === 'auth'
  if (purpose === 'checkin') {
    isRequired = Boolean(status?.force_checkin_captcha)
  } else if (purpose === 'redemption') {
    isRequired = Boolean(status?.force_redemption_captcha)
  }

  const isTurnstileEnabled = Boolean(
    isRequired &&
    captchaType === 'turnstile' &&
    status?.turnstile_check &&
    status?.turnstile_site_key
  )

  const isHCaptchaEnabled = Boolean(
    isRequired &&
    captchaType === 'hcaptcha' &&
    status?.hcaptcha_check &&
    status?.hcaptcha_site_key
  )

  const capApiEndpoint =
    purpose === 'checkin'
      ? status?.cap_checkin_api_endpoint || ''
      : status?.cap_api_endpoint || ''
  const isCapEnabled = Boolean(
    isRequired && captchaType === 'cap' && status?.cap_enabled && capApiEndpoint
  )
  const isCaptchaEnabled =
    isTurnstileEnabled || isHCaptchaEnabled || isCapEnabled

  let tokenQueryParam = 'turnstile'
  if (isCapEnabled) {
    tokenQueryParam = 'cap_token'
  } else if (isHCaptchaEnabled) {
    tokenQueryParam = 'hcaptcha'
  }

  const validateCaptcha = (): boolean => {
    if (isCaptchaEnabled && !captchaToken) {
      toast.info(
        i18next.t('Please wait a moment, human check is initializing...')
      )
      return false
    }
    return true
  }

  return {
    captchaType,
    isRequired,
    isCaptchaEnabled,
    isTurnstileEnabled,
    isHCaptchaEnabled,
    isCapEnabled,
    turnstileSiteKey: status?.turnstile_site_key || '',
    hCaptchaSiteKey: status?.hcaptcha_site_key || '',
    capApiEndpoint,
    captchaToken,
    setCaptchaToken,
    validateCaptcha,
    tokenQueryParam,
  }
}
