import { useEffect, useState } from 'react'
import type { FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { api, errorMessage, keys } from '../../../shared/api'
import type { Settings } from '../../../shared/api'
import { Button, Card, ErrorState, PageHeader, Spinner, TextField, fieldErrors, useToast } from '../../../shared/ui'
import { settingsSchema, validateForm } from '../../../shared/validation'

type Draft = Record<keyof Settings, string>

const FIELDS: { key: keyof Settings; label: string; hint: string }[] = [
  { key: 'approvalWindowMin', label: 'Walk-in approval window (minutes)', hint: '10–120. Applies to walk-ins registered after the change.' },
  { key: 'approvalReminderMin', label: 'Reminder before the window ends (minutes)', hint: '1 to one less than the approval window.' },
  { key: 'maxAdvanceDays', label: 'How far ahead a visit can be planned (days)', hint: '1–90.' },
  { key: 'photoRetentionDays', label: 'Keep visitor and ID photos for (days)', hint: '7–365, counted from when the visit ends.' },
  { key: 'recordRetentionDays', label: 'Keep visit records for (days)', hint: 'At least 90.' },
]

export function SettingsPage() {
  const qc = useQueryClient()
  const toast = useToast()
  const q = useQuery({ queryKey: keys.settings, queryFn: () => api.get<Settings>('/settings') })
  const [draft, setDraft] = useState<Draft | null>(null)
  const [errors, setErrors] = useState<Record<string, string>>({})
  useEffect(() => {
    if (q.data) setDraft(Object.fromEntries(Object.entries(q.data).map(([k, v]) => [k, String(v)])) as Draft)
  }, [q.data])

  const save = useMutation({
    mutationFn: (body: Settings) => api.patch<Settings>('/settings', body),
    onSuccess: (s) => { qc.setQueryData(keys.settings, s); toast.success('Settings saved') },
    onError: (e) => { const f = fieldErrors(e); if (f) setErrors(f); else toast.error(errorMessage(e)) },
  })

  if (q.isPending || !draft) return <Spinner />
  if (q.isError) return <ErrorState error={q.error} onRetry={() => void q.refetch()} />

  const parsed = Object.fromEntries(Object.entries(draft).map(([k, v]) => [k, v.trim() === '' ? undefined : Number(v)]))
  const live = validateForm(settingsSchema, parsed)
  const shown = { ...(live.errors ?? {}), ...errors }
  const submit = (e: FormEvent) => { e.preventDefault(); if (live.data) { setErrors({}); save.mutate(live.data) } }

  return (
    <>
      <PageHeader title="Settings" />
      <Card>
        <form onSubmit={submit} noValidate className="max-w-xl space-y-4">
          {FIELDS.map((f) => (
            <TextField key={f.key} label={f.label} hint={f.hint} type="number" inputMode="numeric" value={draft[f.key]} error={shown[f.key]}
              onChange={(e) => { setDraft({ ...draft, [f.key]: e.target.value }); setErrors((p) => { const n = { ...p }; delete n[f.key]; return n }) }} />
          ))}
          <Button type="submit" loading={save.isPending} disabled={!live.data}>Save settings</Button>
        </form>
      </Card>
    </>
  )
}
