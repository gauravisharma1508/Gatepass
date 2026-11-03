import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link, Navigate, useNavigate, useSearchParams } from 'react-router-dom'
import { ApiError } from '../../../shared/api'
import { homeFor, useAuth } from '../../../shared/auth'
import { Button, TextField, useForm } from '../../../shared/ui'
import { loginSchema, validateForm } from '../../../shared/validation'
import { AuthLayout } from './AuthLayout'

/** Only same-site paths are accepted as a return target (no open redirect). */
export function safeReturnTo(v: string | null): string | null {
  return v && v.startsWith('/') && !v.startsWith('//') && !v.startsWith('/login') ? v : null
}

export function LoginPage() {
  const { user, login } = useAuth()
  const nav = useNavigate()
  const [params] = useSearchParams()
  const form = useForm({ email: '', password: '' })
  const [formError, setFormError] = useState<string>()
  const [busy, setBusy] = useState(false)

  if (user) return <Navigate to={safeReturnTo(params.get('returnTo')) ?? homeFor(user.role)} replace />

  async function submit(e: FormEvent) {
    e.preventDefault()
    setFormError(undefined)
    const r = validateForm(loginSchema, form.values)
    if (r.errors) return form.setErrors(r.errors)
    setBusy(true)
    try {
      const u = await login(form.values.email.trim(), form.values.password)
      nav(safeReturnTo(params.get('returnTo')) ?? homeFor(u.role), { replace: true })
    } catch (err) {
      if (err instanceof ApiError && err.status === 423) setFormError('This account is temporarily locked after too many failed attempts. Try again in 15 minutes.')
      else if (err instanceof ApiError && err.status === 429) setFormError('Too many attempts. Please wait a minute and try again.')
      else if (err instanceof ApiError && err.status === 401) setFormError('Invalid email or password.')
      else setFormError(err instanceof Error ? err.message : 'Could not sign in.')
    } finally {
      setBusy(false)
    }
  }

  return (
    <AuthLayout title="Sign in">
      <form onSubmit={submit} noValidate className="space-y-4">
        {formError && <p role="alert" className="rounded-md bg-red-50 p-3 text-sm text-red-900">{formError}</p>}
        <TextField label="Email" type="email" autoComplete="username" autoFocus value={form.values.email}
          error={form.errors.email} onChange={(e) => form.set('email', e.target.value)} />
        <TextField label="Password" type="password" autoComplete="current-password" value={form.values.password}
          error={form.errors.password} onChange={(e) => form.set('password', e.target.value)} />
        <Button type="submit" className="w-full" loading={busy}>Sign in</Button>
        <p className="text-center text-sm"><Link className="text-blue-800 underline" to="/forgot-password">Forgot your password?</Link></p>
      </form>
    </AuthLayout>
  )
}
