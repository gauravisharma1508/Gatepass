import { useState } from 'react'
import { Alert, Text, View } from 'react-native'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useRouter } from 'expo-router'
import { addDays, api, errorMessage, fieldErrors, serverNow, todayIST } from './api'
import type { Visit } from './api'
import { keys } from './hooks'
import { Button, ErrorBox, Field, Muted, Screen } from './ui/components'
import { plannedVisitSchema, validateForm } from './validation'

export interface FormValues {
  visitorName: string; visitorMobile: string; visitorEmail: string; visitorCompany: string
  purpose: string; visitDate: string; expectedArrival: string; expectedDurationMin: number
}
const MAX_ADVANCE_DAYS = 30 // server default; the server enforces the configured value

export function emptyValues(today: string): FormValues {
  return { visitorName: '', visitorMobile: '', visitorEmail: '', visitorCompany: '', purpose: '', visitDate: today, expectedArrival: '', expectedDurationMin: 60 }
}

export function PlannedVisitForm({ initial, visitId, onSaved }: { initial: FormValues; visitId?: string; onSaved: () => void }) {
  const qc = useQueryClient()
  const today = todayIST(serverNow())
  const [v, setV] = useState(initial)
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [busy, setBusy] = useState(false)
  const set = <K extends keyof FormValues>(k: K, val: FormValues[K]) => setV((p) => ({ ...p, [k]: val }))
  const step = (d: number) => set('expectedDurationMin', Math.min(480, Math.max(30, v.expectedDurationMin + d)))

  async function submit() {
    const r = validateForm(plannedVisitSchema(today, MAX_ADVANCE_DAYS), v)
    if (r.errors) return setErrors(r.errors)
    setErrors({}); setBusy(true)
    try {
      if (visitId) await api.patch<Visit>(`/visits/${visitId}`, r.data)
      else await api.post<Visit>('/visits/planned', r.data)
      await qc.invalidateQueries()
      onSaved()
    } catch (e) {
      const f = fieldErrors(e)
      if (f) setErrors(f); else Alert.alert('Could not save', errorMessage(e))
    } finally { setBusy(false) }
  }

  return (
    <Screen>
      <Field label="Visitor name" value={v.visitorName} onChangeText={(x) => set('visitorName', x)} error={errors.visitorName} />
      <Field label="Visitor mobile" value={v.visitorMobile} onChangeText={(x) => set('visitorMobile', x)} error={errors.visitorMobile} keyboardType="phone-pad" />
      <Field label="Visitor email (optional)" value={v.visitorEmail} onChangeText={(x) => set('visitorEmail', x)} error={errors.visitorEmail} autoCapitalize="none" keyboardType="email-address" />
      <Field label="Company (optional)" value={v.visitorCompany} onChangeText={(x) => set('visitorCompany', x)} error={errors.visitorCompany} />
      <Field label="Purpose" value={v.purpose} onChangeText={(x) => set('purpose', x)} error={errors.purpose} />
      <Field label="Visit date (YYYY-MM-DD)" value={v.visitDate} onChangeText={(x) => set('visitDate', x)} error={errors.visitDate} placeholder={today}
        autoCapitalize="none" />
      <Muted>Today is {today}; up to {addDays(today, MAX_ADVANCE_DAYS)}.</Muted>
      <View style={{ height: 12 }} />
      <Field label="Expected arrival, 24-hour HH:MM (optional)" value={v.expectedArrival} onChangeText={(x) => set('expectedArrival', x)} error={errors.expectedArrival} placeholder="14:30" />
      <View style={{ flexDirection: 'row', alignItems: 'center', gap: 8, marginBottom: 4 }}>
        <View style={{ flex: 1 }}><Button title="− 15 min" variant="secondary" onPress={() => step(-15)} /></View>
        <Text accessibilityLabel="Duration" style={{ fontSize: 16, fontWeight: '600' }}>{v.expectedDurationMin} min</Text>
        <View style={{ flex: 1 }}><Button title="+ 15 min" variant="secondary" onPress={() => step(15)} /></View>
      </View>
      {!!errors.expectedDurationMin && <Text accessibilityRole="alert" style={{ color: '#b91c1c' }}>{errors.expectedDurationMin}</Text>}
      <View style={{ height: 16 }} />
      <Button title={visitId ? 'Save changes' : 'Create visit'} onPress={() => void submit()} loading={busy} />
    </Screen>
  )
}

export function VisitFormScreen({ id }: { id?: string }) {
  const router = useRouter()
  const q = useQuery({ queryKey: keys.visit(id ?? ''), enabled: !!id, queryFn: () => api.get<Visit>(`/visits/${id}`) })
  const today = todayIST(serverNow())
  const done = () => (router.canGoBack() ? router.back() : router.replace('/'))
  if (!id) return <PlannedVisitForm initial={emptyValues(today)} onSaved={done} />
  if (q.isError) return <Screen><ErrorBox message={(q.error as Error).message} onRetry={() => void q.refetch()} /></Screen>
  if (!q.data) return <Screen><Muted>Loading…</Muted></Screen>
  const v = q.data
  if (!v.allowedActions.includes('EDIT')) return <Screen><ErrorBox message="This visit can no longer be edited." /></Screen>
  return (
    <PlannedVisitForm visitId={v.id} onSaved={done} initial={{
      visitorName: v.visitor.name, visitorMobile: v.visitor.mobile, visitorEmail: v.visitor.email ?? '', visitorCompany: v.visitor.company ?? '',
      purpose: v.purpose, visitDate: v.visitDate, expectedArrival: v.expectedArrival ?? '', expectedDurationMin: v.expectedDurationMin,
    }} />
  )
}
