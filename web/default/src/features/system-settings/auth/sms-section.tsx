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
import { useEffect, useMemo, useRef } from 'react'
import * as z from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
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
import { Switch } from '@/components/ui/switch'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const smsSchema = z.object({
  aliyun_sms: z.object({
    enabled: z.boolean(),
    access_key_id: z.string(),
    access_secret: z.string(),
    sign_name: z.string(),
    template_code: z.string(),
    endpoint: z.string(),
  }),
})

type SMSFormInput = z.input<typeof smsSchema>
type SMSFormValues = z.output<typeof smsSchema>

type FlatSMSDefaults = {
  'aliyun_sms.enabled': boolean
  'aliyun_sms.access_key_id': string
  'aliyun_sms.access_secret': string
  'aliyun_sms.sign_name': string
  'aliyun_sms.template_code': string
  'aliyun_sms.endpoint': string
}

const buildFormDefaults = (defaults: FlatSMSDefaults): SMSFormInput => ({
  aliyun_sms: {
    enabled: defaults['aliyun_sms.enabled'],
    access_key_id: defaults['aliyun_sms.access_key_id'] ?? '',
    access_secret: defaults['aliyun_sms.access_secret'] ?? '',
    sign_name: defaults['aliyun_sms.sign_name'] ?? '',
    template_code: defaults['aliyun_sms.template_code'] ?? '',
    endpoint:
      defaults['aliyun_sms.endpoint'] || 'dysmsapi.aliyuncs.com',
  },
})

const normalizeFormValues = (values: SMSFormValues): FlatSMSDefaults => ({
  'aliyun_sms.enabled': values.aliyun_sms.enabled,
  'aliyun_sms.access_key_id': values.aliyun_sms.access_key_id,
  'aliyun_sms.access_secret': values.aliyun_sms.access_secret,
  'aliyun_sms.sign_name': values.aliyun_sms.sign_name,
  'aliyun_sms.template_code': values.aliyun_sms.template_code,
  'aliyun_sms.endpoint': values.aliyun_sms.endpoint,
})

interface SMSSectionProps {
  defaultValues: FlatSMSDefaults
}

export function SMSSection(props: SMSSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo(
    () => buildFormDefaults(props.defaultValues),
    [props.defaultValues]
  )

  const form = useForm<SMSFormInput, unknown, SMSFormValues>({
    resolver: zodResolver(smsSchema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatSMSDefaults>(props.defaultValues)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(props.defaultValues)
  )

  useEffect(() => {
    const serialized = JSON.stringify(props.defaultValues)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = props.defaultValues
    baselineSerializedRef.current = serialized
    form.reset(buildFormDefaults(props.defaultValues))
  }, [props.defaultValues, form])

  const onSubmit = async (values: SMSFormValues) => {
    const normalized = normalizeFormValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatSMSDefaults>
    ).filter((key) => normalized[key] !== baselineRef.current[key])

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of changedKeys) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    baselineRef.current = normalized
    baselineSerializedRef.current = JSON.stringify(normalized)
    form.reset(buildFormDefaults(normalized))
  }

  return (
    <SettingsSection title={t('SMS Authentication')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />
          <FormField
            control={form.control}
            name='aliyun_sms.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable SMS Login')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Allow users to sign in with phone number and SMS verification code'
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
            name='aliyun_sms.access_key_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Access Key ID')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='LTAI5t...'
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('Alibaba Cloud AccessKey ID')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='aliyun_sms.access_secret'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Access Secret')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder='******'
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('Alibaba Cloud AccessKey Secret')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='aliyun_sms.sign_name'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Sign Name')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('e.g. New API')}
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('SMS signature name (must be approved on Alibaba Cloud)')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='aliyun_sms.template_code'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Template Code')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='SMS_123456789'
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'SMS template code. Template variable must be ${code}'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='aliyun_sms.endpoint'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Endpoint')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='dysmsapi.aliyuncs.com'
                    value={field.value ?? ''}
                    onChange={(event) => field.onChange(event.target.value)}
                    name={field.name}
                    onBlur={field.onBlur}
                    ref={field.ref}
                  />
                </FormControl>
                <FormDescription>
                  {t('Alibaba Cloud SMS API endpoint')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
