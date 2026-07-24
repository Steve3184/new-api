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
import { useEffect, useRef } from 'react'

type HCaptchaAPI = {
  render: (element: HTMLElement, options: Record<string, unknown>) => string
  remove: (widgetId: string) => void
}

declare global {
  interface Window {
    hcaptcha?: HCaptchaAPI
  }
}

type HCaptchaProps = {
  siteKey: string
  onVerify: (token: string) => void
  onExpire?: () => void
  onError?: () => void
  className?: string
}

const HCAPTCHA_SCRIPT_ID = 'hcaptcha-script'
const HCAPTCHA_SCRIPT_URL = 'https://js.hcaptcha.com/1/api.js?render=explicit'

let hCaptchaScriptPromise: Promise<void> | null = null

function loadHCaptchaScript(): Promise<void> {
  if (window.hcaptcha) return Promise.resolve()
  if (hCaptchaScriptPromise) return hCaptchaScriptPromise

  const scriptPromise = new Promise<void>((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(
      `#${HCAPTCHA_SCRIPT_ID}`
    )
    const script = existing ?? document.createElement('script')

    script.addEventListener('load', () => resolve(), { once: true })
    script.addEventListener(
      'error',
      () => reject(new Error('hCaptcha load failed')),
      {
        once: true,
      }
    )

    if (!existing) {
      script.id = HCAPTCHA_SCRIPT_ID
      script.src = HCAPTCHA_SCRIPT_URL
      script.async = true
      script.defer = true
      document.head.appendChild(script)
    }
  })
  hCaptchaScriptPromise = scriptPromise.catch((error) => {
    hCaptchaScriptPromise = null
    throw error
  })

  return scriptPromise
}

export function HCaptcha(props: HCaptchaProps) {
  const containerRef = useRef<HTMLDivElement | null>(null)
  const onVerifyRef = useRef(props.onVerify)
  const onExpireRef = useRef(props.onExpire)
  const onErrorRef = useRef(props.onError)

  useEffect(() => {
    onVerifyRef.current = props.onVerify
    onExpireRef.current = props.onExpire
    onErrorRef.current = props.onError
  }, [props.onError, props.onExpire, props.onVerify])

  useEffect(() => {
    let cancelled = false
    let widgetId: string | null = null

    void loadHCaptchaScript()
      .then(() => {
        if (cancelled || !containerRef.current || !window.hcaptcha) return
        widgetId = window.hcaptcha.render(containerRef.current, {
          sitekey: props.siteKey,
          callback: (token: string) => onVerifyRef.current(token),
          'expired-callback': () => onExpireRef.current?.(),
          'error-callback': () => onErrorRef.current?.(),
        })
      })
      .catch(() => onErrorRef.current?.())

    return () => {
      cancelled = true
      if (widgetId && window.hcaptcha) {
        window.hcaptcha.remove(widgetId)
      }
    }
  }, [props.siteKey])

  return <div ref={containerRef} className={props.className} />
}
