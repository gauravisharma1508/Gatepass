import { useState } from 'react'
import { Text } from 'react-native'
import { api, errorMessage, fieldErrors } from './api'
import { useAuth } from './auth/AuthContext'
import { Button, Field, H1, Screen } from './ui/components'
import { c } from './ui/theme'
import { changePasswordSchema, loginSchema, validateForm } from './validation'


export function LoginScreen() {
  const { signIn } = useAuth()
  const [v, setV] = useState({ email: '', password: '' })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit() {
    const r = validateForm(loginSchema, v)
    if (r.errors) return setErrors(r.errors)
    setErrors({}); setFormError(''); setBusy(true)
    try {
      await signIn(v.email, v.password)
    } catch (e) {
      setFormError(errorMessage(e))
    } finally { setBusy(false) }
  }
  return (
    <Screen>
      <H1>Sign in</H1>
      <Field label="Email" value={v.email} onChangeText={(email) => setV({ ...v, email })} error={errors.email}
        autoCapitalize="none" keyboardType="email-address" autoCorrect={false} />
      <Field label="Password" value={v.password} onChangeText={(password) => setV({ ...v, password })} error={errors.password} secureTextEntry />
      {!!formError && <Text accessibilityRole="alert" style={{ color: c.danger, marginBottom: 12 }}>{formError}</Text>}
      <Button title="Sign in" onPress={() => void submit()} loading={busy} />
    </Screen>
  )
}

export function ChangePasswordScreen() {
  const { refreshUser, signOut } = useAuth()
  const [v, setV] = useState({ currentPassword: '', newPassword: '', confirm: '' })
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [formError, setFormError] = useState('')
  const [busy, setBusy] = useState(false)

  async function submit() {
    const r = validateForm(changePasswordSchema, v)
    if (r.errors) return setErrors(r.errors)
    setErrors({}); setFormError(''); setBusy(true)
    try {
      await api.post('/auth/change-password', { currentPassword: v.currentPassword, newPassword: v.newPassword })
      await refreshUser()
    } catch (e) {
      const f = fieldErrors(e)
      if (f) setErrors(f); else setFormError(errorMessage(e))
    } finally { setBusy(false) }
  }
  return (
    <Screen>
      <H1>Choose a new password</H1>
      <Field label="Current password" value={v.currentPassword} onChangeText={(currentPassword) => setV({ ...v, currentPassword })} error={errors.currentPassword} secureTextEntry />
      <Field label="New password" value={v.newPassword} onChangeText={(newPassword) => setV({ ...v, newPassword })} error={errors.newPassword} secureTextEntry />
      <Field label="Confirm new password" value={v.confirm} onChangeText={(confirm) => setV({ ...v, confirm })} error={errors.confirm} secureTextEntry />
      {!!formError && <Text accessibilityRole="alert" style={{ color: c.danger, marginBottom: 12 }}>{formError}</Text>}
      <Button title="Change password" onPress={() => void submit()} loading={busy} />
      <Text style={{ height: 12 }} />
      <Button title="Sign out" variant="secondary" onPress={() => void signOut()} />
    </Screen>
  )
}
