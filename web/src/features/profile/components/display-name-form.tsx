/* Copyright (C) 2023-2026 QuantumNous */
import { zodResolver } from '@hookform/resolvers/zod'
import { useMutation } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { z } from 'zod'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { handleServerError } from '@/lib/handle-server-error'
import { requireServerSuccess } from '@/lib/server-error-message'
import { useAuthStore } from '@/stores/auth-store'

import { updateUserProfile } from '../api'
import type { UserProfile } from '../types'

const displayNameSchema = z.object({
  display_name: z
    .string()
    .trim()
    .refine(
      (value) => value.length > 0 && [...value].length <= 20,
      'Display name must contain 1 to 20 characters.'
    ),
})

type DisplayNameFormProps = {
  profile: UserProfile
  onProfileUpdate: () => void
}

export function DisplayNameForm(props: DisplayNameFormProps) {
  const { t } = useTranslation()
  const savedName = props.profile.display_name || props.profile.username
  const form = useForm<z.infer<typeof displayNameSchema>>({
    resolver: zodResolver(displayNameSchema),
    defaultValues: { display_name: savedName },
  })
  useEffect(() => {
    form.reset({ display_name: savedName })
  }, [form, savedName, props.profile.id])

  const mutation = useMutation({
    mutationFn: async (values: z.infer<typeof displayNameSchema>) =>
      requireServerSuccess(await updateUserProfile(values)),
    onSuccess: (_data, values) => {
      const { auth } = useAuthStore.getState()
      if (auth.user?.id === props.profile.id) {
        auth.setUser({ ...auth.user, display_name: values.display_name })
      }
      form.reset(values)
      props.onProfileUpdate()
      toast.success(t('Profile updated successfully'))
    },
    onError: (error) => handleServerError(error, t('Failed to update profile')),
  })

  const unchanged = form.watch('display_name').trim() === savedName

  return (
    <Form {...form}>
      <form
        className='mb-6 grid gap-3 border-b pb-6'
        onSubmit={form.handleSubmit((values) => {
          if (!mutation.isPending) mutation.mutate(values)
        })}
      >
        <FormField
          control={form.control}
          name='display_name'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Display Name')}</FormLabel>
              <div className='flex flex-col gap-3 sm:flex-row sm:items-start'>
                <FormControl>
                  <Input
                    {...field}
                    autoComplete='nickname'
                    placeholder={t('Enter display name')}
                    disabled={mutation.isPending}
                    className='min-w-0 sm:max-w-sm'
                  />
                </FormControl>
                <Button
                  type='submit'
                  disabled={
                    mutation.isPending || !form.formState.isDirty || unchanged
                  }
                  className='w-full shrink-0 sm:w-auto'
                >
                  {mutation.isPending && (
                    <Loader2
                      aria-hidden='true'
                      className='size-4 animate-spin'
                    />
                  )}
                  {t('Save display name')}
                </Button>
              </div>
              <FormDescription>
                {t('Use 1–20 characters. Your login username stays unchanged.')}
              </FormDescription>
              <FormMessage />
            </FormItem>
          )}
        />
      </form>
    </Form>
  )
}
