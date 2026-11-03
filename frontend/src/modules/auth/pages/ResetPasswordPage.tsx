import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { api, ApiError } from '../../../shared/api'
import { Button, TextField, useForm, useToast } from '../../../shared/ui'
import { changePasswordSchema, validateForm } from '../../../shared/validation'
import { AuthLayout } from './AuthLayout'

export function ResetPasswordPage() {
  const [params] = useSearchParams()
  const token = params.get('token') ?? ''
  const nav = useNavigate()
  const toast = useToast()
  const form = useForm({ newPassword: '', confirm: '' })
  const [tokenError, setTokenError] = useState(!token)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    // reuse the shared password rules; the "current password" field does not exist here
    const r = validateForm(changePasswordSchema, { currentPassword: 'x', ...form.values })
    if (r.errors) {
      const { currentPassword: _ignored, ...rest } = r.errors
      void _ignored
      if (Object.keys(rest).length) return form.setErrors(rest)
    }
    setBusy(true)
    try {
      await api.post('/auth/reset-password', { token, newPassword: form.values.newPassword }, { noAuthRedirect: true })
      toast.success('Password changed. You can sign in now.')
      nav('/login', { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.fields.token) setTokenError(true)
      else if (err instanceof ApiError && err.fields.newPassword) form.setErrors({ newPassword: err.fields.newPassword })
      else toast.error(err instanceof Error ? err.message : 'Could not reset the password')
    } finally {
      setBusy(false)
    }
  }

  if (tokenError)
    return (
      <AuthLayout title="Link expired">
        <div role="alert" className="space-y-4 text-sm text-slate-800">
          <p>This reset link is invalid or has expired.</p>
          <Link className="font-medium text-blue-800 underline" to="/forgot-password">Request a new link</Link>
        </div>
      </AuthLayout>
    )

  return (
    <AuthLayout title="Choose a new password">
      <form onSubmit={submit} noValidate className="space-y-4">
        <TextField label="New password" type="password" autoComplete="new-password" autoFocus hint="8–72 characters with at least one letter and one digit"
          value={form.values.newPassword} error={form.errors.newPassword} onChange={(e) => form.set('newPassword', e.target.value)} />
        <TextField label="Confirm new password" type="password" autoComplete="new-password" value={form.values.confirm}
          error={form.errors.confirm} onChange={(e) => form.set('confirm', e.target.value)} />
        <Button type="submit" className="w-full" loading={busy}>Change password</Button>
      </form>
    </AuthLayout>
  )
}
