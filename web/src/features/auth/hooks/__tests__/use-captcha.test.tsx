/* Copyright (C) 2023-2026 QuantumNous */
import { renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { useCaptcha } from '../use-captcha'

const status = vi.hoisted(() => ({ current: {} as Record<string, unknown> }))

vi.mock('@/hooks/use-status', () => ({
  useStatus: () => ({ status: status.current }),
}))

function captchaFor(purpose: Parameters<typeof useCaptcha>[0]) {
  const { result } = renderHook(() => useCaptcha(purpose))
  return result.current
}

describe('captcha purpose resolution', () => {
  it('falls back to the global provider when no purpose is configured', () => {
    status.current = {
      captcha_type: 'hcaptcha',
      hcaptcha_check: true,
      hcaptcha_site_key: 'site-key',
      // Check-in and redemption only run a captcha while their switch is on.
      force_checkin_captcha: true,
      force_redemption_captcha: true,
    }

    for (const purpose of [
      'login',
      'register',
      'reset_password',
      'checkin',
      'redemption',
    ] as const) {
      const captcha = captchaFor(purpose)
      expect(captcha.captchaType).toBe('hcaptcha')
      expect(captcha.isHCaptchaEnabled).toBe(true)
    }
  })

  it('lets login use no captcha while registration stays protected', () => {
    status.current = {
      captcha_type: 'turnstile',
      turnstile_check: true,
      turnstile_site_key: 'site-key',
      login_captcha_type: 'none',
      register_captcha_type: 'turnstile',
    }

    expect(captchaFor('login').isCaptchaEnabled).toBe(false)
    expect(captchaFor('register').isCaptchaEnabled).toBe(true)
  })

  it('gives each form its own provider', () => {
    status.current = {
      captcha_type: 'turnstile',
      turnstile_check: true,
      turnstile_site_key: 'turnstile-site',
      hcaptcha_check: true,
      hcaptcha_site_key: 'hcaptcha-site',
      login_captcha_type: 'hcaptcha',
      register_captcha_type: 'turnstile',
      reset_password_captcha_type: 'none',
    }

    const login = captchaFor('login')
    expect(login.captchaType).toBe('hcaptcha')
    expect(login.isHCaptchaEnabled).toBe(true)
    expect(login.tokenQueryParam).toBe('hcaptcha')

    const register = captchaFor('register')
    expect(register.captchaType).toBe('turnstile')
    expect(register.isTurnstileEnabled).toBe(true)
    expect(register.tokenQueryParam).toBe('turnstile')

    expect(captchaFor('reset_password').isCaptchaEnabled).toBe(false)
  })

  it('gives check-in and redemption their own providers', () => {
    status.current = {
      captcha_type: 'turnstile',
      turnstile_check: true,
      turnstile_site_key: 'turnstile-site',
      hcaptcha_check: true,
      hcaptcha_site_key: 'hcaptcha-site',
      cap_enabled: true,
      cap_api_endpoint: 'https://cap.example.test/challenge',
      force_checkin_captcha: true,
      force_redemption_captcha: true,
      checkin_captcha_type: 'hcaptcha',
      redemption_captcha_type: 'none',
    }

    const checkin = captchaFor('checkin')
    expect(checkin.captchaType).toBe('hcaptcha')
    expect(checkin.isHCaptchaEnabled).toBe(true)

    // "none" for redemption means no captcha even though the switch is on.
    expect(captchaFor('redemption').isCaptchaEnabled).toBe(false)
  })

  it('keeps the global provider when a purpose option is an empty string', () => {
    status.current = {
      captcha_type: 'turnstile',
      turnstile_check: true,
      turnstile_site_key: 'site-key',
      login_captcha_type: '',
      register_captcha_type: '',
      reset_password_captcha_type: '',
      checkin_captcha_type: '',
      redemption_captcha_type: '',
    }

    expect(captchaFor('login').captchaType).toBe('turnstile')
    expect(captchaFor('register').captchaType).toBe('turnstile')
    expect(captchaFor('reset_password').captchaType).toBe('turnstile')
    expect(captchaFor('checkin').captchaType).toBe('turnstile')
    expect(captchaFor('redemption').captchaType).toBe('turnstile')
  })
})
