import { useState } from 'react'
import type { FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api, errorMessage, keys } from '../../../shared/api'
import type { HostSummary, Visit } from '../../../shared/api'
import { Button, Card, PageHeader, Spinner, fieldErrors, useForm, useToast } from '../../../shared/ui'
import { validateForm, walkInSchema } from '../../../shared/validation'
import { VisitCard, VisitorFields } from '../../visits'
import type { VisitorValues } from '../../visits'
import { HostPicker } from '../components/HostPicker'

const EMPTY: VisitorValues = { visitorName: '', visitorMobile: '', visitorEmail: '', visitorCompany: '', purpose: '', expectedDurationMin: 60 }

/** The request just sent: a live card with the countdown; updates when the host answers. */
function PendingRequest({ id, onAnother }: { id: string; onAnother: () => void }) {
  const q = useQuery({ queryKey: keys.visit(id), queryFn: () => api.get<Visit>(`/visits/${id}`), refetchInterval: 15_000 })
  return (
    <div className="space-y-4">
      <div role="status" className="rounded-lg border border-green-300 bg-green-50 p-4 text-sm text-green-900">
        Request sent. The host has been notified and has 30 minutes to answer.
      </div>
      {q.isPending ? <Spinner /> : q.data && <VisitCard visit={q.data} />}
      <div className="flex flex-wrap gap-2">
        <Button variant="secondary" onClick={onAnother}>Register another visitor</Button>
        <Link to="/security" className="inline-flex min-h-11 items-center rounded-md border border-slate-300 px-4 text-sm font-medium text-slate-800 hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-blue-700">Back to the dashboard</Link>
      </div>
    </div>
  )
}

export function WalkInPage() {
  const qc = useQueryClient()
  const toast = useToast()
  const form = useForm<VisitorValues>(EMPTY)
  const [host, setHost] = useState<HostSummary | null>(null)
  const [hostError, setHostError] = useState<string>()
  const [createdId, setCreatedId] = useState<string | null>(null)

  const create = useMutation({
    mutationFn: (body: unknown) => api.post<Visit>('/visits/walk-in', body),
    onSuccess: (v) => { setCreatedId(v.id); void qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'me' }) },
    onError: (e) => {
      const f = fieldErrors(e)
      if (f) { const { hostId, ...rest } = f; form.setErrors(rest); if (hostId) setHostError(hostId) }
      else toast.error(errorMessage(e))
    },
  })

  function submit(e: FormEvent) {
    e.preventDefault()
    const r = validateForm(walkInSchema, { ...form.values, hostId: host?.id ?? '' })
    if (r.errors) {
      const { hostId, ...rest } = r.errors
      form.setErrors(rest)
      setHostError(hostId)
      return
    }
    create.mutate(r.data)
  }

  function reset() {
    form.setValues(EMPTY)
    form.setErrors({})
    setHost(null)
    setCreatedId(null)
  }

  return (
    <>
      <PageHeader title="Register a walk-in visitor" />
      {createdId ? <PendingRequest id={createdId} onAnother={reset} /> : (
        <Card>
          <form onSubmit={submit} noValidate className="max-w-2xl space-y-4">
            <HostPicker value={host} onChange={(h) => { setHost(h); setHostError(undefined) }} error={hostError} />
            <VisitorFields values={form.values} errors={form.errors} set={form.set} />
            <div className="flex justify-end gap-2 pt-2">
              <Link to="/security" className="inline-flex min-h-11 items-center rounded-md border border-slate-300 px-4 text-sm font-medium text-slate-800 hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-blue-700">Cancel</Link>
              <Button type="submit" loading={create.isPending}>Ask the host to approve</Button>
            </div>
          </form>
        </Card>
      )}
    </>
  )
}
