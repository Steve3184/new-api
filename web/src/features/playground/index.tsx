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
import { Box, ImageIcon, MessageSquare, Volume2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { useStatus } from '@/hooks/use-status'

import { PlaygroundChat } from './components/chat/playground-chat'
import { filterGenerationModels } from './components/generation/generation-utils'
import { ImagePlayground } from './components/generation/image-playground'
import { SpeechPlayground } from './components/generation/speech-playground'
import { ThreeDPlayground } from './components/generation/three-d-playground'
import { PlaygroundInput } from './components/input/playground-input'
import {
  useChatHandler,
  usePlaygroundConversation,
  usePlaygroundOptions,
  usePlaygroundState,
} from './hooks'
import { useGenerationOptions } from './hooks/use-generation-options'
import type { PlaygroundFeature, PlaygroundPublicSettings } from './types'

const DEFAULT_PLAYGROUND_SETTINGS: PlaygroundPublicSettings = {
  enabled_features: ['chat'],
  models: { chat: [], image: [], speech: [], three_d: [] },
  speech_model_types: {},
}

export function Playground() {
  const { t } = useTranslation()
  const { status } = useStatus()
  const playgroundSettings =
    (status?.playground as PlaygroundPublicSettings | undefined) ??
    DEFAULT_PLAYGROUND_SETTINGS
  const enabledFeatures: PlaygroundFeature[] =
    playgroundSettings.enabled_features
  const [activeFeature, setActiveFeature] = useState<PlaygroundFeature>('chat')
  const {
    config,
    parameterEnabled,
    messages,
    isLoadingMessages,
    models,
    groups,
    updateMessages,
    setModels,
    setGroups,
    updateConfig,
    updateParameterEnabled,
    clearMessages,
  } = usePlaygroundState()

  const { sendChat, stopGeneration, isGenerating } = useChatHandler({
    config,
    parameterEnabled,
    onMessageUpdate: updateMessages,
  })

  const {
    editingMessageKey,
    handleSendMessage,
    handleRegenerateMessage,
    handleEditMessage,
    handleEditOpenChange,
    applyEdit,
    handleDeleteMessage,
  } = usePlaygroundConversation({
    messages,
    updateMessages,
    sendChat,
  })

  const handleClearMessages = () => {
    handleEditOpenChange(false)
    clearMessages()
  }

  const { isLoadingModels } = usePlaygroundOptions({
    currentGroup: config.group,
    currentModel: config.model,
    setGroups,
    setModels,
    updateConfig,
  })
  const generationOptions = useGenerationOptions(groups)

  const chatModels = filterGenerationModels(
    models,
    playgroundSettings.models.chat ?? []
  )
  const imageModels = filterGenerationModels(
    generationOptions.models,
    playgroundSettings.models.image ?? []
  )
  const speechModels = filterGenerationModels(
    generationOptions.models,
    playgroundSettings.models.speech ?? []
  )
  const threeDModels = filterGenerationModels(
    generationOptions.models,
    playgroundSettings.models.three_d ?? []
  )

  useEffect(() => {
    if (enabledFeatures.includes(activeFeature)) return
    setActiveFeature(enabledFeatures[0] ?? 'chat')
  }, [activeFeature, enabledFeatures])

  useEffect(() => {
    if (chatModels.some((model) => model.value === config.model)) return
    updateConfig('model', chatModels[0]?.value ?? '')
  }, [chatModels, config.model, updateConfig])

  const tabItems = [
    { feature: 'chat' as const, label: t('Chat'), icon: MessageSquare },
    { feature: 'image' as const, label: t('Image'), icon: ImageIcon },
    { feature: 'speech' as const, label: t('Speech'), icon: Volume2 },
    { feature: 'three_d' as const, label: t('3D'), icon: Box },
  ].filter((item) => enabledFeatures.includes(item.feature))

  return (
    <Tabs
      value={activeFeature}
      onValueChange={(value) =>
        value !== null && setActiveFeature(value as PlaygroundFeature)
      }
      className='relative flex size-full min-h-0 flex-col overflow-hidden'
    >
      {tabItems.length > 1 && (
        <div className='flex shrink-0 justify-center border-b px-4 py-2'>
          <TabsList>
            {tabItems.map((item) => {
              const Icon = item.icon
              return (
                <TabsTrigger key={item.feature} value={item.feature}>
                  <Icon data-icon='inline-start' />
                  {item.label}
                </TabsTrigger>
              )
            })}
          </TabsList>
        </div>
      )}

      <TabsContent
        value='chat'
        className='flex min-h-0 flex-1 flex-col overflow-hidden'
      >
        <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
          <PlaygroundChat
            messages={messages}
            isLoadingMessages={isLoadingMessages}
            onRegenerateMessage={handleRegenerateMessage}
            onEditMessage={handleEditMessage}
            onDeleteMessage={handleDeleteMessage}
            onSelectPrompt={handleSendMessage}
            isGenerating={isGenerating}
            editingKey={editingMessageKey}
            onCancelEdit={handleEditOpenChange}
            onSaveEdit={(newContent) => applyEdit(newContent, false)}
            onSaveEditAndSubmit={(newContent) => applyEdit(newContent, true)}
          />
        </div>
        <div className='mx-auto w-full max-w-4xl'>
          <PlaygroundInput
            config={config}
            disabled={isGenerating}
            groups={groups}
            groupValue={config.group}
            isGenerating={isGenerating}
            isModelLoading={isLoadingModels}
            modelValue={config.model}
            models={chatModels}
            onGroupChange={(value) => updateConfig('group', value)}
            onConfigChange={updateConfig}
            onClearMessages={handleClearMessages}
            onModelChange={(value) => updateConfig('model', value)}
            onParameterEnabledChange={updateParameterEnabled}
            onStop={stopGeneration}
            onSubmit={handleSendMessage}
            parameterEnabled={parameterEnabled}
            hasMessages={messages.length > 0}
          />
        </div>
      </TabsContent>

      <TabsContent value='image' className='min-h-0 flex-1 overflow-hidden'>
        <ImagePlayground
          models={imageModels}
          groups={groups}
          group={config.group}
          onGroupChange={(value) => updateConfig('group', value)}
          groupModels={generationOptions.groupModels}
        />
      </TabsContent>
      <TabsContent value='speech' className='min-h-0 flex-1 overflow-hidden'>
        <SpeechPlayground
          models={speechModels}
          groups={groups}
          group={config.group}
          onGroupChange={(value) => updateConfig('group', value)}
          modelTypes={playgroundSettings.speech_model_types ?? {}}
          groupModels={generationOptions.groupModels}
        />
      </TabsContent>
      <TabsContent value='three_d' className='min-h-0 flex-1 overflow-hidden'>
        <ThreeDPlayground
          models={threeDModels}
          groups={groups}
          group={config.group}
          onGroupChange={(value) => updateConfig('group', value)}
          groupModels={generationOptions.groupModels}
        />
      </TabsContent>
    </Tabs>
  )
}
