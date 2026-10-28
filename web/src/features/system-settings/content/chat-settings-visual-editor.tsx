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
import { Plus, Search, Settings2 } from 'lucide-react'
import { useState, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { StaticRowActions } from '@/components/data-table/static/static-row-actions'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { ApiKeysMutateDrawer } from '@/features/keys/components/api-keys-mutate-drawer'
import {
  transformFormDataToKeyTemplate,
  transformKeyTemplateToFormDefaults,
} from '@/features/keys/lib'
import { useStatus } from '@/hooks/use-status'

import { safeJsonParseWithValidation } from '../utils/json-parser'
import { isArray } from '../utils/json-validators'
import { ChatDialog, type ChatEntryData } from './chat-dialog'
import {
  getDefaultChatPresetKeyName,
  moveChatPresetSetting,
  parseChatPresetSettings,
  type ChatPresetSetting,
} from './chat-preset-settings'

type ChatSettingsVisualEditorProps = {
  value: string
  onChange: (value: string) => void
  presetSettings: string
  onPresetSettingsChange: (value: string) => void
}

type ChatEntry = ChatEntryData

export function ChatSettingsVisualEditor({
  value,
  onChange,
  presetSettings,
  onPresetSettingsChange,
}: ChatSettingsVisualEditorProps) {
  const { t } = useTranslation()
  const { status } = useStatus()
  const [searchText, setSearchText] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [editData, setEditData] = useState<ChatEntry | null>(null)
  const [keyDrawerOpen, setKeyDrawerOpen] = useState(false)
  // Kept after closing so the drawer keeps its title while it animates out
  const [keyPresetName, setKeyPresetName] = useState<string | null>(null)

  const settings = useMemo(
    () => parseChatPresetSettings(presetSettings),
    [presetSettings]
  )

  const chats = useMemo(() => {
    const parsed = safeJsonParseWithValidation<unknown[]>(value, {
      fallback: [],
      validator: isArray,
      validatorMessage: 'Chats must be a JSON array',
      context: 'chats',
    })

    return parsed
      .map((item) => {
        if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
          const entries = Object.entries(item)
          if (entries.length === 1) {
            const [name, url] = entries[0]
            return { name, url: String(url) }
          }
        }
        return null
      })
      .filter((item): item is ChatEntry => item !== null)
  }, [value])

  const filteredChats = useMemo(() => {
    if (!searchText) return chats
    const lowerSearch = searchText.toLowerCase()
    return chats.filter(
      (chat) =>
        chat.name.toLowerCase().includes(lowerSearch) ||
        chat.url.toLowerCase().includes(lowerSearch)
    )
  }, [chats, searchText])

  const updatePresetSetting = (name: string, patch: ChatPresetSetting) => {
    const next = { ...settings, [name]: { ...settings[name], ...patch } }
    onPresetSettingsChange(JSON.stringify(next))
  }

  const handleSave = (data: ChatEntryData) => {
    const chatsArray = safeJsonParseWithValidation<unknown[]>(value, {
      fallback: [],
      validator: isArray,
      silent: true,
    })

    let updatedArray = [...chatsArray]

    if (editData) {
      updatedArray = updatedArray.filter((item) => {
        if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
          return !Object.keys(item).includes(editData.name)
        }
        return true
      })
      const movedSettings = moveChatPresetSetting(
        settings,
        editData.name,
        data.name
      )
      if (movedSettings !== settings) {
        onPresetSettingsChange(JSON.stringify(movedSettings))
      }
    }

    updatedArray.push({ [data.name]: data.url })

    onChange(JSON.stringify(updatedArray, null, 2))
  }

  const handleDelete = (name: string) => {
    const chatsArray = safeJsonParseWithValidation<unknown[]>(value, {
      fallback: [],
      validator: isArray,
      silent: true,
    })

    const updatedArray = chatsArray.filter((item) => {
      if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
        return !Object.keys(item).includes(name)
      }
      return true
    })

    onChange(JSON.stringify(updatedArray, null, 2))
    const remainingSettings = moveChatPresetSetting(settings, name, null)
    if (remainingSettings !== settings) {
      onPresetSettingsChange(JSON.stringify(remainingSettings))
    }
  }

  const handleEdit = (chat: ChatEntry) => {
    setEditData(chat)
    setDialogOpen(true)
  }

  const handleAdd = () => {
    setEditData(null)
    setDialogOpen(true)
  }

  const openKeyDrawer = (name: string) => {
    setKeyPresetName(name)
    setKeyDrawerOpen(true)
  }

  const handleAutoKeyToggle = (name: string, enabled: boolean) => {
    const autoKey = settings[name]?.auto_key
    // Turning it on for the first time asks for the key settings; the switch
    // stays off unless they are saved.
    if (enabled && !autoKey) {
      openKeyDrawer(name)
      return
    }
    if (autoKey) {
      updatePresetSetting(name, { auto_key: { ...autoKey, enabled } })
    }
  }

  const keyPresetDefaultName = keyPresetName
    ? getDefaultChatPresetKeyName(keyPresetName)
    : ''

  return (
    <div className='space-y-4'>
      <div className='flex items-center gap-4'>
        <div className='relative flex-1'>
          <Search className='text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4' />
          <Input
            placeholder={t('Search chat presets...')}
            value={searchText}
            onChange={(e) => setSearchText(e.target.value)}
            className='pl-9'
          />
        </div>
        <Button onClick={handleAdd}>
          <Plus className='mr-2 h-4 w-4' />
          {t('Add chat preset')}
        </Button>
      </div>

      <StaticDataTable
        data={filteredChats}
        getRowKey={(chat) => chat.name}
        emptyContent={
          searchText
            ? t('No chat presets match your search')
            : t(
                'No chat presets configured. Click "Add chat preset" to get started.'
              )
        }
        columns={[
          {
            id: 'name',
            header: t('Chat Client Name'),
            cellClassName: 'font-medium',
            cell: (chat) => chat.name,
          },
          {
            id: 'url',
            header: t('URL'),
            cellClassName: 'max-w-md truncate font-mono text-sm',
            cell: (chat) => chat.url,
          },
          {
            id: 'standalone',
            header: t('Standalone menu'),
            cell: (chat) => (
              <Switch
                checked={settings[chat.name]?.standalone_menu === true}
                onCheckedChange={(checked) =>
                  updatePresetSetting(chat.name, { standalone_menu: checked })
                }
                aria-label={t('Show {{name}} as a standalone menu', {
                  name: chat.name,
                })}
              />
            ),
          },
          {
            id: 'auto-key',
            header: t('Auto-created API key'),
            cell: (chat) => {
              const autoKey = settings[chat.name]?.auto_key
              const enabled = autoKey?.enabled === true
              return (
                <div className='flex min-w-0 items-center gap-2'>
                  <Switch
                    checked={enabled}
                    onCheckedChange={(checked) =>
                      handleAutoKeyToggle(chat.name, checked)
                    }
                    aria-label={t('Create an API key for {{name}}', {
                      name: chat.name,
                    })}
                  />
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon-sm'
                    disabled={!enabled}
                    onClick={() => openKeyDrawer(chat.name)}
                    aria-label={t('Configure the API key for {{name}}', {
                      name: chat.name,
                    })}
                  >
                    <Settings2 className='size-4' />
                  </Button>
                  {enabled && (
                    <span className='text-muted-foreground max-w-40 truncate font-mono text-xs'>
                      {autoKey.name || getDefaultChatPresetKeyName(chat.name)}
                    </span>
                  )}
                </div>
              )
            },
          },
          {
            id: 'actions',
            header: t('Actions'),
            className: 'text-right',
            cellClassName: 'text-right',
            cell: (chat) => (
              <StaticRowActions
                editLabel={t('Edit')}
                deleteLabel={t('Delete')}
                menuLabel={t('Open menu')}
                onEdit={() => handleEdit(chat)}
                onDelete={() => handleDelete(chat.name)}
              />
            ),
          },
        ]}
      />

      <ChatDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        onSave={handleSave}
        editData={editData}
      />

      <ApiKeysMutateDrawer
        open={keyDrawerOpen}
        onOpenChange={setKeyDrawerOpen}
        template={
          keyPresetName === null
            ? undefined
            : {
                title: t('API key for {{name}}', { name: keyPresetName }),
                description: t(
                  'Each user gets this key the first time they open the preset. Changes apply only to keys created afterwards.'
                ),
                values: transformKeyTemplateToFormDefaults(
                  settings[keyPresetName]?.auto_key,
                  keyPresetDefaultName,
                  status?.default_use_auto_group === true
                ),
                onSave: (formValues) => {
                  const template = transformFormDataToKeyTemplate(formValues)
                  updatePresetSetting(keyPresetName, {
                    auto_key: {
                      ...template,
                      name:
                        template.name === keyPresetDefaultName
                          ? ''
                          : template.name,
                      enabled: true,
                    },
                  })
                },
              }
        }
      />
    </div>
  )
}
