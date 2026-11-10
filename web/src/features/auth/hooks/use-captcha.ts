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

import type { SystemStatus } from '@/features/auth/types'
import { useStatus } from '@/hooks/use-status'

type CaptchaPurpose =
  | 'login'
  | 'register'
  | 'reset_password'
  | 'checkin'
  | 'redemption'

type CaptchaType = 'none' | 'turnstile' | 'hcaptcha' | 'cap'

/** Why a required captcha cannot run, so a caller can name the missing part. */
export type CaptchaIssue = 'provider-disabled' | 'configuration-incomplete'

/** Product names stay untranslated; they read the same in every locale. */
export function captchaProviderName(type: CaptchaType): string {
  if (type === 'hcaptcha') return 'hCaptcha'
  if (type === 'cap') return 'Cap'
  return 'Cloudflare Turnstile'
}

function toCaptchaType(value: string | undefined): CaptchaType {
  if (value === 'cap' || value === 'hcaptcha' || value === 'turnstile') {
    return value
  }
  return 'none'
}

/**
 * Resolve the provider protecting one form. A form-specific option wins; an
 * empty value follows the global provider, which is what an older site
 * returns.
 */
function providerFor(status: SystemStatus | null, purpose: CaptchaPurpose) {
  if (purpose === 'login' && status?.login_captcha_type) {
    return toCaptchaType(status.login_captcha_type)
  }
  if (purpose === 'register' && status?.register_captcha_type) {
    return toCaptchaType(status.register_captcha_type)
  }
  if (purpose === 'reset_password' && status?.reset_password_captcha_type) {
    return toCaptchaType(status.reset_password_captcha_type)
  }
  if (purpose === 'checkin' && status?.checkin_captcha_type) {
    return toCaptchaType(status.checkin_captcha_type)
  }
  if (purpose === 'redemption' && status?.redemption_captcha_type) {
    return toCaptchaType(status.redemption_captcha_type)
  }
  return toCaptchaType(status?.captcha_type)
}

export function useCaptcha(purpose: CaptchaPurpose = 'login') {
  const { status } = useStatus()
  const [captchaToken, setCaptchaToken] = useState('')
  const captchaType = providerFor(status, purpose)
  let isRequired =
    purpose === 'login' ||
    purpose === 'register' ||
    purpose === 'reset_password'
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
  // "none" is a deliberate choice, not a broken setup: the gateway accepts the
  // request without a token, so callers must go ahead instead of standing by.
  const skipsCaptcha = captchaType === 'none'
  const isCaptchaEnabled =
    isTurnstileEnabled || isHCaptchaEnabled || isCapEnabled
  // A required captcha that cannot run blocks the form; report which part of
  // the configuration is missing so the message can name it.
  let providerEnabled = status?.turnstile_check
  if (captchaType === 'hcaptcha') providerEnabled = status?.hcaptcha_check
  if (captchaType === 'cap') providerEnabled = status?.cap_enabled
  let captchaIssue: CaptchaIssue | undefined
  if (isRequired && !skipsCaptcha && !isCaptchaEnabled) {
    captchaIssue = providerEnabled
      ? 'configuration-incomplete'
      : 'provider-disabled'
  }

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
    skipsCaptcha,
    captchaIssue,
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
