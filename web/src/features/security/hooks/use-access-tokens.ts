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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useCallback, useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  useSecureVerification,
  type RequestVerificationOptions,
} from '@/features/auth/secure-verification'
import { handleServerError } from '@/lib/handle-server-error'
import { AuthOperationError } from '@/lib/secure-verification'
import { useAuthStore } from '@/stores/auth-store'

import {
  createAccessToken,
  getAccessTokenCatalog,
  listAccessTokens,
  renameAccessToken,
  revokeAccessToken,
  revokeLegacyAccessToken,
  type AccessTokenInput,
} from '../api'

export function useAccessTokens() {
  const { t } = useTranslation()
  const client = useQueryClient()
  const userId = useAuthStore((state) => state.auth.user?.id)
  const sessionId = useAuthStore((state) => state.auth.session?.sid)
  const [createdToken, setCreatedToken] = useState<{
    value: string
    userId: number | undefined
    sessionId: string | undefined
  } | null>(null)
  const [pending, setPending] = useState(false)
  const currentOperation = useRef<AbortController | null>(null)
  const verification = useSecureVerification()
  const requestVerification = verification.requestVerification
  const cancelVerification = verification.cancel
  const list = useQuery({
    queryKey: ['security', 'access-tokens', 'list', userId],
    queryFn: listAccessTokens,
    retry: false,
  })
  const catalog = useQuery({
    queryKey: ['security', 'access-tokens', 'catalog', userId],
    queryFn: getAccessTokenCatalog,
    retry: false,
  })
  useEffect(
    () => () => {
      const current = currentOperation.current
      currentOperation.current = null
      current?.abort()
      cancelVerification()
      setPending(false)
      setCreatedToken(null)
    },
    [cancelVerification, userId, sessionId]
  )

  const invalidateList = useCallback(
    () =>
      client.invalidateQueries({
        queryKey: ['security', 'access-tokens', 'list', userId],
      }),
    [client, userId]
  )

  // Runs one verified mutation at a time. Proofs and plaintext stay local to
  // the action, outside React Query caches.
  const runVerified = useCallback(
    async <T>(
      operation: RequestVerificationOptions,
      action: (proofToken: string, signal: AbortSignal) => Promise<T>
    ): Promise<T | undefined> => {
      if (currentOperation.current) return undefined
      const controller = new AbortController()
      currentOperation.current = controller
      setPending(true)
      try {
        const proof = await requestVerification(operation)
        if (currentOperation.current !== controller || !proof) return undefined
        const result = await action(proof.proof_token, controller.signal)
        if (currentOperation.current !== controller) return undefined
        void invalidateList()
        return result
      } catch (error) {
        if (currentOperation.current !== controller) return undefined
        const failure = AuthOperationError.from(error)
        if (failure.code !== 'AUTH_CANCELLED') handleServerError(failure)
        void invalidateList()
        return undefined
      } finally {
        if (currentOperation.current === controller) {
          currentOperation.current = null
          setPending(false)
        }
      }
    },
    [requestVerification, invalidateList]
  )

  const create = useCallback(
    async (input: AccessTokenInput) => {
      const created = await runVerified(
        {
          scope: 'access_token.generate',
          context: { scopes: input.scopes, expires_at: input.expires_at },
        },
        (proofToken, signal) => createAccessToken(input, proofToken, signal)
      )
      if (!created) return false
      setCreatedToken({ value: created.token, userId, sessionId })
      return true
    },
    [runVerified, userId, sessionId]
  )

  const revoke = useCallback(
    async (id: number) => {
      const revoked = await runVerified(
        { scope: 'access_token.revoke', context: { token_id: id } },
        async (proofToken, signal) => {
          await revokeAccessToken(id, proofToken, signal)
          return true
        }
      )
      if (revoked) toast.success(t('Access token revoked'))
      return revoked === true
    },
    [runVerified, t]
  )

  const revokeLegacy = useCallback(async () => {
    const revoked = await runVerified(
      { scope: 'access_token.revoke', context: { legacy: true } },
      async (proofToken, signal) => {
        await revokeLegacyAccessToken(proofToken, signal)
        return true
      }
    )
    if (revoked) toast.success(t('Access token revoked'))
    return revoked === true
  }, [runVerified, t])

  const rename = useMutation({
    mutationFn: (input: { id: number; name: string }) =>
      renameAccessToken(input.id, input.name),
    onSuccess: () => {
      toast.success(t('Renamed successfully'))
      void invalidateList()
    },
    onError: (error) => handleServerError(error),
  })

  const verificationPhase = verification.dialogProps.state.phase
  return {
    list,
    catalog,
    createdToken:
      createdToken &&
      createdToken.userId === userId &&
      createdToken.sessionId === sessionId
        ? createdToken.value
        : '',
    clearCreatedToken: () => {
      setCreatedToken(null)
    },
    pending,
    create,
    revoke,
    revokeLegacy,
    rename,
    showVerification:
      verificationPhase !== 'idle' && verificationPhase !== 'loading',
    verificationDialogProps: verification.dialogProps,
  }
}
