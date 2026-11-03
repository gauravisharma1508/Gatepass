import { useState } from 'react'
import type { FormEvent } from 'react'
import { Link } from 'react-router-dom'
import { api, ApiError } from '../../../shared/api'
import { Button, TextField } from '../../../shared/ui'
import { emailSchema } from '../../../shared/validation'
import { AuthLayout } from './AuthLayout'

export function ForgotPasswordPage() {
  const [email, setEmail] = useState('')
  const [error, setError] = useState<string>()
  const [done, setDone] = useState(false)
  const [busy, setBusy] = useState(false)

  async function submit(e: FormEvent) {
    e.preventDefault()
    const r = emailSchema.safeParse(email)
    if (!r.success) return setError(r.error.issues[0]?.message)
    setBusy(true)
    try {
      await api.post('/auth/forgot-password', { email: r.data }, { noAuthRedirect: true })
    } catch (err) {
      // a format error from the server is shown; anything else looks the same as success
      if (err instanceof ApiError && err.fields.email) {
        setError(err.fields.email)
        setBusy(false)
        return
      }
    }
    setBusy(false)
    setDone(true)
  }

  return (
    <AuthLayout title="Reset your password">
      {done ? (
        <div className="space-y-4 text-sm text-slate-800" role="status">
          <p>If an account exists for that email, we have sent a link to reset the password. The link is valid for 30 minutes.</p>
          <Link className="text-blue-800 underline" to="/login">Back to sign in</Link>
        </div>
      ) : (
        <form onSubmit={submit} noValidate className="space-y-4">
          <TextField label="Email" type="email" autoComplete="username" autoFocus value={email} error={error}
            onChange={(e) => { setEmail(e.target.value); setError(undefined) }} />
          <Button type="submit" className="w-full" loading={busy}>Send reset link</Button>
          <p className="text-center text-sm"><Link className="text-blue-800 underline" to="/login">Back to sign in</Link></p>
        </form>
      )}
    </AuthLayout>
  )
}
