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
import { Pencil, Plus, Save, Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { StaticDataTable } from '@/components/data-table/static/static-data-table'
import { Dialog } from '@/components/dialog'
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  CUSTOM_TAB_CATEGORIES,
  CUSTOM_TAB_ICONS,
  parseCustomTabs,
  type CustomTab,
  type CustomTabCategory,
} from '@/lib/custom-tabs'

import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

type CustomTabsSectionProps = {
  defaultValues: { CustomTabs: string }
}

const CATEGORY_LABEL_KEYS: Record<CustomTabCategory, string> = {
  chat: 'Chat',
  general: 'General',
  personal: 'Personal',
  admin: 'Admin',
}

const ICON_NAMES = Object.keys(CUSTOM_TAB_ICONS)

const emptyTab = (): Omit<CustomTab, 'id'> => ({
  label: '',
  url: '',
  icon: 'Globe',
  category: 'general',
  external: false,
})

export function CustomTabsSection({ defaultValues }: CustomTabsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [tabs, setTabs] = useState<CustomTab[]>([])
  const [hasChanges, setHasChanges] = useState(false)
  const [showDialog, setShowDialog] = useState(false)
  const [showDeleteDialog, setShowDeleteDialog] = useState(false)
  const [editingTab, setEditingTab] = useState<CustomTab | null>(null)
  const [formValues, setFormValues] =
    useState<Omit<CustomTab, 'id'>>(emptyTab())

  useEffect(() => {
    setTabs(parseCustomTabs(defaultValues.CustomTabs))
  }, [defaultValues.CustomTabs])

  const handleAdd = () => {
    setEditingTab(null)
    setFormValues(emptyTab())
    setShowDialog(true)
  }

  const handleEdit = (tab: CustomTab) => {
    setEditingTab(tab)
    setFormValues({
      label: tab.label,
      url: tab.url,
      icon: tab.icon,
      category: tab.category,
      external: tab.external,
    })
    setShowDialog(true)
  }

  const handleDelete = (tab: CustomTab) => {
    setEditingTab(tab)
    setShowDeleteDialog(true)
  }

  const confirmDelete = () => {
    if (!editingTab) return
    setTabs((prev) => prev.filter((t) => t.id !== editingTab.id))
    setHasChanges(true)
    setShowDeleteDialog(false)
    setEditingTab(null)
    toast.success(t('Tab deleted. Click "Save Settings" to apply.'))
  }

  const handleSubmit = () => {
    if (!formValues.label.trim()) {
      toast.error(t('Label is required'))
      return
    }
    if (!formValues.url.trim()) {
      toast.error(t('URL is required'))
      return
    }
    if (formValues.external) {
      try {
        const parsedURL = new URL(formValues.url)
        if (parsedURL.protocol !== 'http:' && parsedURL.protocol !== 'https:') {
          throw new Error('unsupported protocol')
        }
      } catch {
        toast.error(t('External URLs must start with http:// or https://'))
        return
      }
    } else if (!formValues.url.trim().startsWith('/')) {
      toast.error(t('Internal URLs must start with /'))
      return
    }

    const normalizedValues = {
      ...formValues,
      label: formValues.label.trim(),
      url: formValues.url.trim(),
    }

    if (editingTab) {
      setTabs((prev) =>
        prev.map((t) =>
          t.id === editingTab.id ? { ...t, ...normalizedValues } : t
        )
      )
      toast.success(t('Tab updated. Click "Save Settings" to apply.'))
    } else {
      const newId = `tab-${Date.now()}`
      setTabs((prev) => [...prev, { id: newId, ...normalizedValues }])
      toast.success(t('Tab added. Click "Save Settings" to apply.'))
    }
    setHasChanges(true)
    setShowDialog(false)
  }

  const handleSave = async () => {
    try {
      await updateOption.mutateAsync({
        key: 'CustomTabs',
        value: JSON.stringify(tabs),
      })
      setHasChanges(false)
      toast.success(t('Custom tabs saved successfully'))
    } catch {
      toast.error(t('Failed to save custom tabs'))
    }
  }

  return (
    <SettingsSection title={t('Custom Tabs')}>
      <div className='space-y-4'>
        <div className='flex flex-wrap items-center gap-2'>
          <Button onClick={handleAdd} size='sm'>
            <Plus className='mr-2 h-4 w-4' />
            {t('Add Tab')}
          </Button>
          <Button
            onClick={handleSave}
            size='sm'
            variant='secondary'
            disabled={!hasChanges || updateOption.isPending}
          >
            <Save className='mr-2 h-4 w-4' />
            {updateOption.isPending ? t('Saving...') : t('Save Settings')}
          </Button>
        </div>

        <StaticDataTable
          data={tabs}
          getRowKey={(tab) => tab.id}
          emptyContent={t('No custom tabs yet. Click "Add Tab" to create one.')}
          columns={[
            {
              id: 'label',
              header: t('Label'),
              cell: (tab) => tab.label,
            },
            {
              id: 'url',
              header: t('URL'),
              cellClassName: 'max-w-xs truncate text-muted-foreground text-sm',
              cell: (tab) => tab.url,
            },
            {
              id: 'icon',
              header: t('Icon'),
              cell: (tab) => tab.icon || '-',
            },
            {
              id: 'category',
              header: t('Category'),
              cell: (tab) => t(CATEGORY_LABEL_KEYS[tab.category]),
            },
            {
              id: 'external',
              header: t('External'),
              cell: (tab) => (tab.external ? t('Yes') : t('No')),
            },
            {
              id: 'actions',
              header: t('Actions'),
              cell: (tab) => (
                <div className='flex items-center gap-2'>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => handleEdit(tab)}
                    aria-label={t('Edit')}
                  >
                    <Pencil className='h-4 w-4' />
                  </Button>
                  <Button
                    size='sm'
                    variant='ghost'
                    onClick={() => handleDelete(tab)}
                    aria-label={t('Delete')}
                    className='text-destructive hover:text-destructive'
                  >
                    <Trash2 className='h-4 w-4' />
                  </Button>
                </div>
              ),
            },
          ]}
        />
      </div>

      <Dialog
        open={showDialog}
        onOpenChange={setShowDialog}
        title={editingTab ? t('Edit Tab') : t('Add Tab')}
        description={t('Configure a custom sidebar tab')}
        contentHeight='auto'
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => setShowDialog(false)}
            >
              {t('Cancel')}
            </Button>
            <Button type='button' onClick={handleSubmit}>
              {editingTab ? t('Update') : t('Add')}
            </Button>
          </>
        }
      >
        <div className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='tab-label'>{t('Label *')}</Label>
            <Input
              id='tab-label'
              value={formValues.label}
              onChange={(e) =>
                setFormValues((v) => ({ ...v, label: e.target.value }))
              }
              placeholder={t('My Custom Page')}
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='tab-url'>{t('URL *')}</Label>
            <Input
              id='tab-url'
              value={formValues.url}
              onChange={(e) =>
                setFormValues((v) => ({ ...v, url: e.target.value }))
              }
              placeholder='https://example.com or /internal-page'
            />
          </div>

          <div className='space-y-2'>
            <Label htmlFor='tab-icon'>{t('Icon')}</Label>
            <Select
              items={ICON_NAMES.map((name) => ({ value: name, label: name }))}
              value={formValues.icon}
              onValueChange={(icon) =>
                setFormValues((value) => ({ ...value, icon: icon || 'Globe' }))
              }
            >
              <SelectTrigger id='tab-icon'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {ICON_NAMES.map((name) => (
                    <SelectItem key={name} value={name}>
                      {name}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-2'>
            <Label>{t('Category')}</Label>
            <Select
              items={CUSTOM_TAB_CATEGORIES.map((category) => ({
                value: category,
                label: t(CATEGORY_LABEL_KEYS[category]),
              }))}
              value={formValues.category}
              onValueChange={(val) =>
                setFormValues((v) => ({
                  ...v,
                  category: val as CustomTabCategory,
                }))
              }
            >
              <SelectTrigger>
                <SelectValue placeholder={t('Select category')} />
              </SelectTrigger>
              <SelectContent alignItemWithTrigger={false}>
                <SelectGroup>
                  {CUSTOM_TAB_CATEGORIES.map((category) => (
                    <SelectItem key={category} value={category}>
                      {t(CATEGORY_LABEL_KEYS[category])}
                    </SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
          </div>

          <div className='flex items-center gap-2'>
            <Checkbox
              id='tab-external'
              checked={formValues.external}
              onCheckedChange={(checked) =>
                setFormValues((v) => ({
                  ...v,
                  external: checked === true,
                }))
              }
            />
            <Label htmlFor='tab-external'>{t('Open in new tab')}</Label>
          </div>
        </div>
      </Dialog>

      <AlertDialog open={showDeleteDialog} onOpenChange={setShowDeleteDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Are you sure?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t('This tab will be removed from the list.')}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('Cancel')}</AlertDialogCancel>
            <AlertDialogAction variant='destructive' onClick={confirmDelete}>
              {t('Delete')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
