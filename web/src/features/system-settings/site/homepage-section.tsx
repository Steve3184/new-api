import { zodResolver } from '@hookform/resolvers/zod'
import type { Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { FormDirtyIndicator } from '../components/form-dirty-indicator'
import { FormNavigationGuard } from '../components/form-navigation-guard'
import {
  SettingsForm,
  SettingsFormGrid,
  SettingsFormGridItem,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useSettingsForm } from '../hooks/use-settings-form'
import { useUpdateOption } from '../hooks/use-update-option'

const homepageSchema = z.object({
  HomePageContent: z.string().optional(),
  HomepageStyle: z.enum(['default', 'custom', 'preset-1']),
  HomepagePresetTitleMode: z.enum(['i18n', 'english']),
  HomepagePresetSLAEnabled: z.boolean(),
  HomepagePresetSLAText: z.string().max(120).optional(),
})

type HomepageFormValues = z.infer<typeof homepageSchema>

type HomepageSectionProps = {
  defaultValues: HomepageFormValues
}

const HOMEPAGE_STYLES = [
  { value: 'default', label: 'Default homepage' },
  { value: 'custom', label: 'Custom homepage' },
  { value: 'preset-1', label: 'Homepage preset 1' },
] as const

function normalizeValue(value: unknown): string {
  if (value === undefined || value === null) return ''
  return typeof value === 'string' ? value : String(value)
}

export function HomepageSection(props: HomepageSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const normalizedDefaults: HomepageFormValues = {
    HomePageContent: normalizeValue(props.defaultValues.HomePageContent),
    HomepageStyle: props.defaultValues.HomepageStyle || 'default',
    HomepagePresetTitleMode:
      props.defaultValues.HomepagePresetTitleMode || 'i18n',
    HomepagePresetSLAEnabled:
      props.defaultValues.HomepagePresetSLAEnabled ?? true,
    HomepagePresetSLAText: normalizeValue(
      props.defaultValues.HomepagePresetSLAText
    ),
  }
  const { form, handleSubmit, handleReset, isDirty, isSubmitting } =
    useSettingsForm<HomepageFormValues>({
      resolver: zodResolver(homepageSchema) as Resolver<HomepageFormValues>,
      defaultValues: normalizedDefaults,
      onSubmit: async (_data, changedFields) => {
        for (const [key, value] of Object.entries(changedFields)) {
          const optionKey: Record<string, string> = {
            HomepageStyle: 'console_setting.homepage_style',
            HomepagePresetTitleMode:
              'console_setting.homepage_preset_title_mode',
            HomepagePresetSLAEnabled:
              'console_setting.homepage_preset_sla_enabled',
            HomepagePresetSLAText: 'console_setting.homepage_preset_sla_text',
          }
          await updateOption.mutateAsync({
            key: optionKey[key] || key,
            value: normalizeValue(value),
          })
        }
      },
    })

  const homepageStyle = form.watch('HomepageStyle')
  const selectedHomepageStyle =
    HOMEPAGE_STYLES.find((item) => item.value === homepageStyle)?.label ||
    'Default homepage'

  return (
    <>
      <FormNavigationGuard when={isDirty} />
      <SettingsSection title={t('Homepage')}>
        <Form {...form}>
          <SettingsForm onSubmit={handleSubmit}>
            <SettingsPageFormActions
              onSave={handleSubmit}
              onReset={handleReset}
              isSaving={isSubmitting || updateOption.isPending}
              isResetDisabled={!isDirty}
            />
            <FormDirtyIndicator isDirty={isDirty} />
            <SettingsFormGrid>
              <SettingsFormGridItem span='full'>
                <FormField
                  control={form.control}
                  name='HomepageStyle'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Homepage style')}</FormLabel>
                      <FormControl>
                        <Select
                          value={field.value}
                          onValueChange={field.onChange}
                        >
                          <SelectTrigger className='w-full'>
                            <SelectValue>
                              {t(selectedHomepageStyle)}
                            </SelectValue>
                          </SelectTrigger>
                          <SelectContent alignItemWithTrigger={false}>
                            {HOMEPAGE_STYLES.map((item) => (
                              <SelectItem key={item.value} value={item.value}>
                                {t(item.label)}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                      </FormControl>
                      <FormDescription>
                        {t(
                          'Choose the public homepage presentation. Custom content supports HTML, Markdown, or an absolute HTTP(S) URL.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </SettingsFormGridItem>

              {homepageStyle === 'custom' && (
                <SettingsFormGridItem span='full'>
                  <FormField
                    control={form.control}
                    name='HomePageContent'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Home Page Content')}</FormLabel>
                        <FormControl>
                          <Textarea
                            placeholder={t('Welcome to our New API...')}
                            rows={12}
                            {...field}
                          />
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Enter HTML, Markdown, or an absolute HTTP(S) URL to show an isolated iframe.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGridItem>
              )}

              {homepageStyle === 'preset-1' && (
                <SettingsFormGridItem span='full'>
                  <FormField
                    control={form.control}
                    name='HomepagePresetTitleMode'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Hero subtitle language')}</FormLabel>
                        <FormControl>
                          <Select
                            value={field.value}
                            onValueChange={field.onChange}
                          >
                            <SelectTrigger className='w-full'>
                              <SelectValue>
                                {t(
                                  field.value === 'english'
                                    ? 'Always English'
                                    : 'Follow site language'
                                )}
                              </SelectValue>
                            </SelectTrigger>
                            <SelectContent alignItemWithTrigger={false}>
                              <SelectItem value='i18n'>
                                {t('Follow site language')}
                              </SelectItem>
                              <SelectItem value='english'>
                                {t('Always English')}
                              </SelectItem>
                            </SelectContent>
                          </Select>
                        </FormControl>
                        <FormDescription>
                          {t(
                            'Choose whether the preset 1 second-line title follows the visitor language or stays in English.'
                          )}
                        </FormDescription>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='HomepagePresetSLAEnabled'
                    render={({ field }) => (
                      <SettingsSwitchItem>
                        <SettingsSwitchContent>
                          <FormLabel>{t('Show SLA badge')}</FormLabel>
                          <FormDescription>
                            {t(
                              'Display the configurable SLA commitment on homepage preset 1.'
                            )}
                          </FormDescription>
                        </SettingsSwitchContent>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                          />
                        </FormControl>
                      </SettingsSwitchItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name='HomepagePresetSLAText'
                    render={({ field }) => (
                      <FormItem className='mt-4'>
                        <FormLabel>{t('SLA badge text')}</FormLabel>
                        <FormControl>
                          <Textarea rows={2} {...field} />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </SettingsFormGridItem>
              )}
            </SettingsFormGrid>
          </SettingsForm>
        </Form>
      </SettingsSection>
    </>
  )
}
