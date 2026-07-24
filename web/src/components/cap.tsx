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

interface CapProps {
  apiEndpoint: string
  onVerify: (token: string) => void
  onError?: (message: string) => void
  onReset?: () => void
}

const CAP_SCRIPT_URL = 'https://cdn.jsdelivr.net/npm/cap-widget@0.1.50'
const CAP_SCRIPT_ID = 'cap-widget-script'

declare module 'react' {
  namespace JSX {
    interface IntrinsicElements {
      'cap-widget': React.DetailedHTMLProps<
        React.HTMLAttributes<HTMLElement> & {
          'data-cap-api-endpoint'?: string
        },
        HTMLElement
      >
    }
  }
}

export function Cap(props: CapProps) {
  const widgetRef = useRef<HTMLElement | null>(null)
  const onVerify = props.onVerify
  const onError = props.onError
  const onReset = props.onReset

  useEffect(() => {
    if (document.querySelector(`#${CAP_SCRIPT_ID}`)) return

    const script = document.createElement('script')
    script.id = CAP_SCRIPT_ID
    script.src = CAP_SCRIPT_URL
    script.defer = true
    document.head.appendChild(script)
  }, [])

  useEffect(() => {
    const widget = widgetRef.current
    if (!widget) return

    const handleSolve = (event: Event) => {
      const detail = (event as CustomEvent<{ token: string }>).detail
      if (detail?.token) onVerify(detail.token)
    }
    const handleError = (event: Event) => {
      const detail = (event as CustomEvent<{ message?: string }>).detail
      onError?.(detail?.message || 'Cap verification failed')
    }
    const handleReset = () => onReset?.()

    widget.addEventListener('solve', handleSolve)
    widget.addEventListener('error', handleError)
    widget.addEventListener('reset', handleReset)

    return () => {
      widget.removeEventListener('solve', handleSolve)
      widget.removeEventListener('error', handleError)
      widget.removeEventListener('reset', handleReset)
    }
  }, [onError, onReset, onVerify])

  return (
    <cap-widget
      ref={(element: HTMLElement | null) => {
        widgetRef.current = element
      }}
      data-cap-api-endpoint={props.apiEndpoint}
    />
  )
}
