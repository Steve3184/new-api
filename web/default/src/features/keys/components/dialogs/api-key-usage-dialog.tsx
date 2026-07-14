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
import { useQuery } from '@tanstack/react-query'
import { Bot, Braces, Terminal } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { CopyButton } from '@/components/copy-button'
import { Dialog } from '@/components/dialog'
import { Badge } from '@/components/ui/badge'
import { ComboboxInput } from '@/components/ui/combobox-input'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useStatus } from '@/hooks/use-status'
import { getUserGroups, getUserModels } from '@/lib/api'

const FALLBACK_DEFAULT_MODEL = 'gpt-5.6-sol'
const FALLBACK_REVIEW_MODEL = 'codex-auto-review'

type ApiKeyUsageDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  tokenKey: string
  group: string
}

function getServerAddress(status: ReturnType<typeof useStatus>['status']) {
  const address =
    status?.server_address ??
    status?.data?.server_address ??
    (typeof window !== 'undefined' ? window.location.origin : '')
  return String(address || '').replace(/\/+$/, '')
}

function buildCodexConfig(
  baseUrl: string,
  defaultModel: string,
  reviewModel: string
) {
  return `model_provider = "OpenAI"
model = "${defaultModel}"
review_model = "${reviewModel}"
model_reasoning_effort = "high"
disable_response_storage = true
windows_wsl_setup_acknowledged = true

[model_providers.OpenAI]
name = "OpenAI"
base_url = "${baseUrl}/v1"
wire_api = "responses"
requires_openai_auth = true

[sandbox_workspace_write]
network_access = true

[features]
goals = true`
}

type OpenCodeModelConfig = {
  name: string
  options: { store: boolean }
  variants: Record<string, Record<string, never>>
}

function buildOpenCodeConfig(
  baseUrl: string,
  tokenKey: string,
  selectedModel: string,
  availableModels: string[]
) {
  const modelIds = [...new Set([...availableModels, selectedModel])].filter(
    Boolean
  )
  const models: Record<string, OpenCodeModelConfig> = Object.fromEntries(
    modelIds.sort().map((model) => [
      model,
      {
        name: model,
        options: { store: false },
        variants: { low: {}, medium: {}, high: {}, xhigh: {}, max: {} },
      },
    ])
  )

  return JSON.stringify(
    {
      model: `openai/${selectedModel}`,
      provider: {
        openai: {
          options: {
            baseURL: `${baseUrl}/v1`,
            apiKey: tokenKey,
          },
          models,
        },
      },
      agent: {
        build: { options: { store: false } },
        plan: { options: { store: false } },
      },
      $schema: 'https://opencode.ai/config.json',
    },
    null,
    2
  )
}

function ConfigPath(props: { label: string; path: string }) {
  const { t } = useTranslation()
  return (
    <div className='bg-muted/40 flex min-w-0 items-center gap-2 rounded-md border px-3 py-2'>
      <span className='text-muted-foreground shrink-0 text-xs'>
        {props.label}
      </span>
      <code className='min-w-0 flex-1 truncate font-mono text-xs'>
        {props.path}
      </code>
      <CopyButton
        value={props.path}
        className='size-7'
        iconClassName='size-3.5'
        tooltip={t('Copy path')}
      />
    </div>
  )
}

function ConfigBlock(props: {
  code: string
  language: 'json' | 'toml'
  title: string
  collapsed?: boolean
}) {
  return (
    <CodeBlock
      className='mx-1 my-0 w-[calc(100%-0.5rem)] [&_.cm-content]:!px-4'
      code={props.code}
      language={props.language}
      title={props.title}
      showToolbar
      enableCollapse={props.collapsed}
      defaultCollapsed={props.collapsed}
      collapsedLines={20}
      maxExpandedLines={42}
    >
      <CodeBlockCopyButton />
    </CodeBlock>
  )
}

export function ApiKeyUsageDialog({
  open,
  onOpenChange,
  tokenKey,
  group,
}: ApiKeyUsageDialogProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [selectedModel, setSelectedModel] = useState('')
  const normalizedGroup = group || 'default'
  const requestedGroup =
    normalizedGroup === 'auto' ? undefined : normalizedGroup
  const { data: groupsResponse } = useQuery({
    queryKey: ['user-groups'],
    queryFn: getUserGroups,
    staleTime: 5 * 60 * 1000,
    enabled: open,
  })
  const { data: modelsResponse, isLoading: modelsLoading } = useQuery({
    queryKey: ['user-models', requestedGroup ?? 'all'],
    queryFn: () => getUserModels(requestedGroup),
    staleTime: 5 * 60 * 1000,
    enabled: open,
  })
  const serverAddress = getServerAddress(status)
  const groupInfo = groupsResponse?.data?.[normalizedGroup]
  const defaultModel = groupInfo?.default_model || FALLBACK_DEFAULT_MODEL
  const effectiveModel = selectedModel || defaultModel
  const reviewModel = effectiveModel.includes('gpt-')
    ? FALLBACK_REVIEW_MODEL
    : effectiveModel
  const modelOptions = useMemo(
    () =>
      [...(modelsResponse?.data ?? [])]
        .sort()
        .map((model) => ({ label: model, value: model })),
    [modelsResponse?.data]
  )
  const availableModels = useMemo(
    () => modelsResponse?.data ?? [],
    [modelsResponse?.data]
  )

  const codexConfig = useMemo(
    () => buildCodexConfig(serverAddress, effectiveModel, reviewModel),
    [effectiveModel, reviewModel, serverAddress]
  )
  const codexAuth = useMemo(
    () => JSON.stringify({ OPENAI_API_KEY: tokenKey }, null, 2),
    [tokenKey]
  )
  const openCodeConfig = useMemo(
    () =>
      buildOpenCodeConfig(
        serverAddress,
        tokenKey,
        effectiveModel,
        availableModels
      ),
    [availableModels, effectiveModel, serverAddress, tokenKey]
  )
  const claudeConfig = useMemo(
    () =>
      JSON.stringify(
        {
          env: {
            CLAUDE_CODE_ATTRIBUTION_HEADER: '0',
            CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC: '1',
            ANTHROPIC_AUTH_TOKEN: tokenKey,
            ANTHROPIC_BASE_URL: `${serverAddress}/`,
          },
          model: effectiveModel,
        },
        null,
        2
      ),
    [effectiveModel, serverAddress, tokenKey]
  )

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Use API Key')}
      description={t(
        'Configure this API key for Codex, OpenCode, or Claude Code.'
      )}
      contentClassName='sm:max-w-4xl'
      contentHeight='min(72vh, 46rem)'
      initialFocus={false}
      showCloseButton
    >
      <div className='mb-4 grid gap-2 sm:grid-cols-[8rem_minmax(0,1fr)] sm:items-center'>
        <div className='flex items-center gap-2 text-sm font-medium'>
          <Badge variant='outline'>{normalizedGroup}</Badge>
          {t('Model')}
        </div>
        <ComboboxInput
          options={modelOptions}
          value={effectiveModel}
          placeholder={
            modelsLoading ? t('Loading models...') : t('Search models...')
          }
          emptyText='No model found.'
          allowCustomValue
          onValueChange={setSelectedModel}
        />
      </div>

      <Tabs defaultValue='codex' className='gap-4'>
        <TabsList className='grid h-auto w-full grid-cols-3'>
          <TabsTrigger value='codex' className='min-h-8'>
            <Terminal />
            Codex
          </TabsTrigger>
          <TabsTrigger value='opencode' className='min-h-8'>
            <Braces />
            OpenCode
          </TabsTrigger>
          <TabsTrigger value='claude-code' className='min-h-8'>
            <Bot />
            Claude Code
          </TabsTrigger>
        </TabsList>

        <TabsContent value='codex' className='space-y-4'>
          <div className='flex flex-wrap items-center gap-2 text-xs'>
            <span className='text-muted-foreground'>
              {t('Default model')}: {effectiveModel}
            </span>
            <span className='text-muted-foreground'>
              {t('Review model')}: {reviewModel}
            </span>
          </div>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Place this content at the beginning of config.toml. Merge it carefully if the file already contains these sections.'
            )}
          </p>
          <div className='grid gap-2 sm:grid-cols-2'>
            <ConfigPath label='macOS / Linux' path='~/.codex/config.toml' />
            <ConfigPath
              label='Windows'
              path='%USERPROFILE%\.codex\config.toml'
            />
          </div>
          <ConfigBlock code={codexConfig} language='toml' title='config.toml' />
          <p className='text-muted-foreground text-sm'>
            {t('Create auth.json if it does not exist and keep it private.')}
          </p>
          <div className='grid gap-2 sm:grid-cols-2'>
            <ConfigPath label='macOS / Linux' path='~/.codex/auth.json' />
            <ConfigPath label='Windows' path='%USERPROFILE%\.codex\auth.json' />
          </div>
          <ConfigBlock code={codexAuth} language='json' title='auth.json' />
        </TabsContent>

        <TabsContent value='opencode' className='space-y-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Create the config file if needed. You may store the API key here or configure credentials with the /connect command.'
            )}
          </p>
          <ConfigPath
            label={t('Config path')}
            path='~/.config/opencode/opencode.json'
          />
          <ConfigBlock
            code={openCodeConfig}
            language='json'
            title='opencode.json'
            collapsed
          />
        </TabsContent>

        <TabsContent value='claude-code' className='space-y-4'>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Merge this object into settings.json. The Anthropic-format base URL must end with / and must not include /v1.'
            )}
          </p>
          <div className='grid gap-2 sm:grid-cols-2'>
            <ConfigPath label='macOS / Linux' path='~/.claude/settings.json' />
            <ConfigPath
              label='Windows'
              path='%USERPROFILE%\.claude\settings.json'
            />
          </div>
          <ConfigBlock
            code={claudeConfig}
            language='json'
            title='settings.json'
          />
        </TabsContent>
      </Tabs>
    </Dialog>
  )
}
