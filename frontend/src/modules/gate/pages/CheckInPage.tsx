import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { api, ApiError, errorMessage, fmtDate, fmtDuration, keys } from '../../../shared/api'
import type { Visit } from '../../../shared/api'
import { Button, Card, ErrorState, PageHeader, SelectField, Spinner, StatusBadge, TextField, useToast } from '../../../shared/ui'
import { idLast4Schema } from '../../../shared/validation'
import { VisitActions } from '../../visits'
import { PhotoSlot } from '../components/PhotoSlot'

export const ID_TYPES = [
  ['AADHAAR', 'Aadhaar'], ['PAN', 'PAN card'], ['PASSPORT', 'Passport'], ['DRIVING_LICENCE', 'Driving licence'], ['VOTER_ID', 'Voter ID'], ['OTHER', 'Other government ID'],
] as const

export function CheckInPage() {
  const { id = '' } = useParams()
  const nav = useNavigate()
  const qc = useQueryClient()
  const toast = useToast()
  const q = useQuery({ queryKey: keys.visit(id), queryFn: () => api.get<Visit>(`/visits/${id}`), retry: false })
  const [idType, setIdType] = useState('')
  const [idLast4, setIdLast4] = useState('')
  const [last4Error, setLast4Error] = useState<string>()

  const effectiveIdType = idType || q.data?.idType || ''
  const upload = useMutation({
    mutationFn: ({ kind, photo }: { kind: 'VISITOR' | 'ID_DOCUMENT'; photo: Blob }) => {
      const fd = new FormData()
      fd.append('file', photo, kind === 'VISITOR' ? 'visitor.jpg' : 'id.jpg')
      if (kind === 'ID_DOCUMENT') {
        fd.append('idType', effectiveIdType)
        if (idLast4) fd.append('idLast4', idLast4)
      }
      return api.put(`/visits/${id}/photos/${kind}`, fd)
    },
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.visit(id) }),
  })
  const checkIn = useMutation({
    mutationFn: () => api.post<Visit>(`/visits/${id}/check-in`),
    onSuccess: async (v) => {
      toast.success(`${v.visitor.name} checked in`)
      await qc.invalidateQueries({ predicate: (x) => x.queryKey[0] !== 'me' })
      nav('/security')
    },
    onError: (e) => toast.error(errorMessage(e)),
  })

  if (q.isPending) return <Spinner />
  if (q.isError) return <ErrorState error={q.error} onRetry={() => void q.refetch()} />
  const v = q.data
  const canCapture = v.allowedActions.includes('UPLOAD_PHOTOS') || v.allowedActions.includes('CHECK_IN')
  const canCheckIn = v.allowedActions.includes('CHECK_IN')

  const save = (kind: 'VISITOR' | 'ID_DOCUMENT') => async (photo: Blob) => {
    try {
      await upload.mutateAsync({ kind, photo })
      toast.success(kind === 'VISITOR' ? 'Visitor photo saved' : 'ID photo saved')
    } catch (e) {
      throw new Error(e instanceof ApiError ? e.fields.idType ?? e.fields.file ?? e.message : errorMessage(e))
    }
  }

  return (
    <>
      <PageHeader title={`Check in ${v.visitor.name}`}><StatusBadge status={v.status} /></PageHeader>
      <div className="grid gap-5 lg:grid-cols-3">
        <Card title="Visit" className="lg:col-span-3">
          <dl className="grid gap-x-6 gap-y-1 text-sm sm:grid-cols-2 lg:grid-cols-4">
            <div><dt className="text-slate-600">Visit ID</dt><dd className="font-mono"><Link className="text-blue-800 underline" to={`/visits/${v.id}`}>{v.code}</Link></dd></div>
            <div><dt className="text-slate-600">Visitor</dt><dd>{v.visitor.name} · {v.visitor.mobile}</dd></div>
            <div><dt className="text-slate-600">Meeting</dt><dd>{v.host.name}{v.host.department ? ` · ${v.host.department}` : ''}</dd></div>
            <div><dt className="text-slate-600">Planned</dt><dd>{fmtDate(v.visitDate)} · {fmtDuration(v.expectedDurationMin)}</dd></div>
          </dl>
        </Card>

        {!canCapture ? (
          <div role="alert" className="rounded-lg border border-amber-300 bg-amber-50 p-4 text-sm text-amber-900 lg:col-span-3">
            This visit cannot be checked in right now (status: {v.status.toLowerCase().replace(/_/g, ' ')}).
            {v.status === 'SCHEDULED' && ' Planned visits can only be checked in on their visit date.'}
            <div className="mt-2"><Link className="font-medium underline" to="/security">Back to the dashboard</Link></div>
          </div>
        ) : (
          <>
            <PhotoSlot title="Visitor photo" kind="VISITOR" visitId={v.id} saved={v.hasVisitorPhoto} version={v.version} facingMode="user" onSave={save('VISITOR')} />
            <PhotoSlot title="ID photo" kind="ID_DOCUMENT" visitId={v.id} saved={v.hasIdPhoto} version={v.version} facingMode="environment" onSave={save('ID_DOCUMENT')}
              blockedReason={!effectiveIdType ? 'Choose the type of ID first.' : last4Error ? 'Fix the ID number digits first.' : undefined}>
              <SelectField label="Type of ID" value={effectiveIdType} onChange={(e) => setIdType(e.target.value)}>
                <option value="">Choose…</option>
                {ID_TYPES.map(([k, l]) => <option key={k} value={k}>{l}</option>)}
              </SelectField>
              <TextField label="Last 4 characters of the ID number (optional)" maxLength={4} autoComplete="off" value={idLast4} error={last4Error}
                onChange={(e) => { setIdLast4(e.target.value); const r = idLast4Schema.safeParse(e.target.value); setLast4Error(r.success ? undefined : r.error.issues[0]?.message) }} />
            </PhotoSlot>
            <Card title="Check in">
              <div className="space-y-3">
                <p className="text-sm text-slate-700">
                  {canCheckIn ? 'Both photos are saved. You can check the visitor in.' : 'Capture and save both the visitor photo and the ID photo to enable check-in.'}
                </p>
                <Button className="w-full" disabled={!canCheckIn} loading={checkIn.isPending} onClick={() => checkIn.mutate()}>Check in</Button>
                <div className="border-t border-slate-200 pt-3">
                  <p className="mb-2 text-sm text-slate-700">Details do not match, or the host does not know this visitor?</p>
                  <VisitActions visit={v} size="md" exclude={['CHECK_IN', 'UPLOAD_PHOTOS']} />
                </div>
              </div>
            </Card>
          </>
        )}
        <p className="rounded-lg bg-slate-100 p-3 text-xs text-slate-700 lg:col-span-3">
          <strong>Privacy notice:</strong> the visitor&apos;s photo and ID photo are captured for security and visitor-management purposes only. They are visible to security and admin staff,
          and deleted automatically after the retention period. Please tell the visitor before taking the photos.
        </p>
      </div>
    </>
  )
}
