import { useState } from 'react'
import type { FormEvent } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../../../shared/api'
import { homeFor, useAuth } from '../../../shared/auth'
import { Button, TextField, fieldErrors, useForm, useToast } from '../../../shared/ui'
import { changePasswordSchema, validateForm } from '../../../shared/validation'
import { AuthLayout } from './AuthLayout'

export function ChangePasswordPage() {
  const { user, refresh, logout } = useAuth()
  const nav = useNavigate()
  const toast = useToast()
  const form = useForm({ currentPassword: '', newPassword: '', confirm: '' })
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    const r = validateForm(changePasswordSchema, form.values)
    if (r.errors) return form.setErrors(r.errors)
    setBusy(true)
    try {
      await api.post('/auth/change-password', { currentPassword: form.values.currentPassword, newPassword: form.values.newPassword })
      await refresh()
      toast.success('Password changed')
      nav(user ? homeFor(user.role) : '/', { replace: true })
    } catch (err) {
      const f = fieldErrors(err)
      if (f) form.setErrors(f)
      else toast.error(err instanceof Error ? err.message : 'Could not change the password')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthLayout title="Change your password">
      <form onSubmit={submit} noValidate className="space-y-4">
        {user?.mustChangePassword && <p role="status" className="rounded-md bg-amber-50 p-3 text-sm text-amber-900">For security, you must choose a new password before continuing.</p>}
        <TextField label="Current password" type="password" autoComplete="current-password" autoFocus value={form.values.currentPassword}
          error={form.errors.currentPassword} onChange={(e) => form.set('currentPassword', e.target.value)} />
        <TextField label="New password" type="password" autoComplete="new-password" hint="8–72 characters with at least one letter and one digit"
          value={form.values.newPassword} error={form.errors.newPassword} onChange={(e) => form.set('newPassword', e.target.value)} />
        <TextField label="Confirm new password" type="password" autoComplete="new-password" value={form.values.confirm}
          error={form.errors.confirm} onChange={(e) => form.set('confirm', e.target.value)} />
        <Button type="submit" className="w-full" loading={busy}>Change password</Button>
        <p className="text-center"><button type="button" className="text-sm text-blue-800 underline" onClick={() => void logout()}>Sign out</button></p>
      </form>
    </AuthLayout>
  )
}
