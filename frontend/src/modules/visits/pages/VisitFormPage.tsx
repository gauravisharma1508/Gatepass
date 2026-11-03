import { useState } from 'react'
import type { FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, addDays, errorMessage, keys, serverNow, todayIST } from '../../../shared/api'
import type { Visit } from '../../../shared/api'
import { Button, Card, ErrorState, PageHeader, Spinner, TextField, fieldErrors, useForm, useToast } from '../../../shared/ui'
import { plannedVisitSchema, validateForm } from '../../../shared/validation'
import { VisitorFields } from '../components/VisitorFields'
import type { VisitorValues } from '../components/VisitorFields'

interface Values extends VisitorValues { visitDate: string; expectedArrival: string }
const MAX_ADVANCE_DAYS = 30 // server default; the server enforces the configured value

function PlannedVisitForm({ initial, visitId }: { initial: Values; visitId?: string }) {
  const nav = useNavigate()
  const qc = useQueryClient()
  const toast = useToast()
  const form = useForm<Values>(initial)
  const today = todayIST(serverNow())
  const [busy, setBusy] = useState(false)

  const save = useMutation({
    mutationFn: (body: Record<string, unknown>) =>
      visitId ? api.patch<Visit>(`/visits/${visitId}`, body) : api.post<Visit>('/visits/planned', body),
  })

  async function submit(e: FormEvent) {
    e.preventDefault()
    const r = validateForm(plannedVisitSchema(today, MAX_ADVANCE_DAYS), form.values)
    if (r.errors) return form.setErrors(r.errors)
    setBusy(true)
    try {
      await save.mutateAsync(r.data)
      await qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'me' })
      toast.success(visitId ? 'Visit updated' : 'Planned visit created')
      nav('/employee/visits')
    } catch (err) {
      const f = fieldErrors(err)
      if (f) form.setErrors(f)
      else toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <Card>
      <form onSubmit={submit} noValidate className="space-y-4">
        <VisitorFields values={form.values} errors={form.errors} set={form.set} />
        <div className="grid gap-4 sm:grid-cols-2">
          <TextField label="Visit date" type="date" min={today} max={addDays(today, MAX_ADVANCE_DAYS)} value={form.values.visitDate}
            error={form.errors.visitDate} onChange={(e) => form.set('visitDate', e.target.value)} />
          <TextField label="Expected arrival time (optional)" type="time" value={form.values.expectedArrival}
            error={form.errors.expectedArrival} onChange={(e) => form.set('expectedArrival', e.target.value)} />
        </div>
        <div className="flex flex-wrap justify-end gap-2 pt-2">
          <Link to="/employee/visits" className="inline-flex min-h-11 items-center rounded-md border border-slate-300 px-4 text-sm font-medium text-slate-800 hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-blue-700">Cancel</Link>
          <Button type="submit" loading={busy}>{visitId ? 'Save changes' : 'Create visit'}</Button>
        </div>
      </form>
    </Card>
  )
}

export function VisitFormPage() {
  const { id } = useParams()
  const q = useQuery({ queryKey: keys.visit(id ?? ''), enabled: !!id, queryFn: () => api.get<Visit>(`/visits/${id}`) })
  const today = todayIST(serverNow())

  if (!id)
    return (
      <>
        <PageHeader title="New planned visit" />
        <PlannedVisitForm initial={{ visitorName: '', visitorMobile: '', visitorEmail: '', visitorCompany: '', purpose: '', expectedDurationMin: 60, visitDate: today, expectedArrival: '' }} />
      </>
    )
  if (q.isPending) return <Spinner />
  if (q.isError) return <ErrorState error={q.error} onRetry={() => void q.refetch()} />
  const v = q.data
  if (!v.allowedActions.includes('EDIT'))
    return (
      <>
        <PageHeader title="Edit visit" />
        <div role="alert" className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900">This visit can no longer be edited.{' '}
          <Link className="underline" to={`/visits/${v.id}`}>Back to the visit</Link></div>
      </>
    )
  return (
    <>
      <PageHeader title={`Edit visit ${v.code}`} />
      <PlannedVisitForm visitId={v.id} initial={{
        visitorName: v.visitor.name, visitorMobile: v.visitor.mobile, visitorEmail: v.visitor.email ?? '', visitorCompany: v.visitor.company ?? '',
        purpose: v.purpose, expectedDurationMin: v.expectedDurationMin, visitDate: v.visitDate, expectedArrival: v.expectedArrival ?? '',
      }} />
    </>
  )
}
