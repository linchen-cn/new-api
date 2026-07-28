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
import { useEffect, useState } from 'react'
import type { z } from 'zod'
import { useForm } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { Loader2, LogIn } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { sendSMSCode, smsLogin } from '@/features/auth/api'
import { smsLoginFormSchema } from '@/features/auth/constants'
import { useAuthRedirect } from '@/features/auth/hooks/use-auth-redirect'
import type { AuthFormProps } from '@/features/auth/types'

const SMS_COUNTDOWN = 60
const PHONE_REGEX = /^1[3-9]\d{9}$/

export function SMSAuthForm({
  className,
  redirectTo,
  ...props
}: AuthFormProps) {
  const { t } = useTranslation()
  const [isLoading, setIsLoading] = useState(false)
  const [isSendingCode, setIsSendingCode] = useState(false)
  const [countdown, setCountdown] = useState(0)
  const { handleLoginSuccess } = useAuthRedirect()

  const form = useForm<z.infer<typeof smsLoginFormSchema>>({
    resolver: zodResolver(smsLoginFormSchema),
    defaultValues: {
      phone: '',
      code: '',
    },
  })

  const phoneValue = form.watch('phone')
  const isPhoneValid = PHONE_REGEX.test(phoneValue)
  const canSendCode = isPhoneValid && countdown === 0 && !isSendingCode

  useEffect(() => {
    if (countdown <= 0) return
    const timer = setInterval(() => {
      setCountdown((prev) => prev - 1)
    }, 1000)
    return () => clearInterval(timer)
  }, [countdown])

  async function handleSendCode() {
    if (!isPhoneValid) {
      toast.error(t('Please enter a valid phone number'))
      return
    }

    setIsSendingCode(true)
    try {
      const res = await sendSMSCode(phoneValue, 'login')
      if (res.success) {
        toast.success(t('Verification code sent!'))
        setCountdown(SMS_COUNTDOWN)
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsSendingCode(false)
    }
  }

  async function onSubmit(data: z.infer<typeof smsLoginFormSchema>) {
    setIsLoading(true)
    try {
      const res = await smsLogin({
        phone: data.phone,
        code: data.code,
      })

      if (res.success) {
        await handleLoginSuccess(res.data as { id?: number } | null, redirectTo)
        toast.success(t('Welcome back!'))
      }
    } catch (_error) {
      // Errors are handled by global interceptor
    } finally {
      setIsLoading(false)
    }
  }

  return (
    <Form {...form}>
      <form
        onSubmit={form.handleSubmit(onSubmit)}
        className={cn('grid gap-4', className)}
        {...props}
      >
        <FormField
          control={form.control}
          name='phone'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Phone Number')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('Enter your phone number')}
                  inputMode='tel'
                  maxLength={11}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='code'
          render={({ field }) => (
            <FormItem>
              <FormLabel>{t('Verification Code')}</FormLabel>
              <div className='flex gap-2'>
                <FormControl>
                  <Input
                    placeholder={t('Enter code')}
                    inputMode='numeric'
                    maxLength={6}
                    className='flex-1'
                    {...field}
                  />
                </FormControl>
                <Button
                  type='button'
                  variant='outline'
                  disabled={!canSendCode}
                  onClick={handleSendCode}
                  className='shrink-0'
                >
                  {isSendingCode ? (
                    <Loader2 className='h-4 w-4 animate-spin' />
                  ) : countdown > 0 ? (
                    t('Resend in {{seconds}}s', { seconds: countdown })
                  ) : (
                    t('Send Code')
                  )}
                </Button>
              </div>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button
          type='submit'
          className='mt-2 w-full justify-center gap-2'
          disabled={isLoading}
        >
          {isLoading ? <Loader2 className='animate-spin' /> : <LogIn />}
          {t('Sign in')}
        </Button>
      </form>
    </Form>
  )
}
