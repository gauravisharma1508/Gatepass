import { useQuery } from '@tanstack/react-query'
import { Link, useParams } from 'react-router-dom'
import { api, apiUrl, ApiError, fmtDate, fmtDateTime, fmtDuration, keys } from '../../../shared/api'
import type { Visit } from '../../../shared/api'
import { useUser } from '../../../shared/auth'
import { Card, Countdown, EmptyState, ErrorState, PageHeader, Spinner, StatusBadge } from '../../../shared/ui'
import { AuditTrail } from '../../reports'
import { VisitActions } from '../components/VisitActions'
import { actionLabel, roleLabel } from '../labels'

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (<><dt className="text-sm text-slate-600">{label}</dt><dd className="text-sm text-slate-900 break-words">{children}</dd></>)
}

export function VisitDetailPage() {
  const { id = '' } = useParams()
  const user = useUser()
  const q = useQuery({ queryKey: keys.visit(id), queryFn: () => api.get<Visit>(`/visits/${id}`), retry: false })

  if (q.isPending) return <Spinner />
  if (q.isError) {
    const e = q.error
    if (e instanceof ApiError && e.status === 403)
      return <div role="alert" className="mx-auto mt-10 max-w-md rounded-lg border border-slate-200 bg-white p-8 text-center"><h1 className="text-xl font-semibold">Access denied</h1><p className="mt-2 text-sm text-slate-700">You can only open visits that you host.</p></div>
    if (e instanceof ApiError && e.status === 404)
      return <EmptyState title="Visit not found">It may have been deleted or the link is wrong.</EmptyState>
    return <ErrorState error={e} onRetry={() => void q.refetch()} />
  }
  const v = q.data
  const photoUrl = (k: string) => `${apiUrl(`/visits/${v.id}/photos/${k}`)}?t=${q.dataUpdatedAt}`
  const canSeeId = user.role === 'SECURITY' || user.role === 'ADMIN'

  return (
    <>
      <PageHeader title={`Visit ${v.code}`}>
        <StatusBadge status={v.status} />
        {v.isOverstay && <span className="rounded-full bg-red-700 px-2.5 py-0.5 text-xs font-semibold text-white">Overstay</span>}
      </PageHeader>
      <div className="grid gap-5 lg:grid-cols-3">
        <div className="space-y-5 lg:col-span-2">
          <Card title="Details">
            <dl className="grid grid-cols-[8rem_1fr] gap-x-4 gap-y-2 sm:grid-cols-[10rem_1fr]">
              <Row label="Visitor">{v.visitor.name}</Row>
              <Row label="Mobile">{v.visitor.mobile}</Row>
              {v.visitor.email && <Row label="Email">{v.visitor.email}</Row>}
              <Row label="Company">{v.visitor.company ?? '—'}</Row>
              <Row label="Meeting">{v.host.name}{v.host.department ? ` · ${v.host.department}` : ''}</Row>
              <Row label="Purpose">{v.purpose}</Row>
              <Row label="Type">{v.type === 'PLANNED' ? 'Planned visit' : 'Walk-in'}</Row>
              <Row label="Visit date">{fmtDate(v.visitDate)}{v.expectedArrival ? ` at ${v.expectedArrival}` : ''}</Row>
              <Row label="Expected duration">{fmtDuration(v.expectedDurationMin)}</Row>
              {v.status === 'PENDING_APPROVAL' && <Row label="Time left to answer"><Countdown deadline={v.approvalDeadline} /></Row>}
              {v.decidedAt && <Row label="Host answered">{fmtDateTime(v.decidedAt)}</Row>}
              {v.decisionReason && <Row label="Reason">{v.decisionReason}</Row>}
              {v.checkedInAt && <Row label="Checked in">{fmtDateTime(v.checkedInAt)}</Row>}
              {v.meetingOverAt && <Row label="Meeting over">{fmtDateTime(v.meetingOverAt)}</Row>}
              {v.checkedOutAt && <Row label="Checked out">{fmtDateTime(v.checkedOutAt)}</Row>}
              {v.closedReason && <Row label="Closing reason">{v.closedReason}</Row>}
              {v.idType && canSeeId && <Row label="ID type">{v.idType.replace(/_/g, ' ').toLowerCase()}</Row>}
            </dl>
            <div className="mt-4"><VisitActions visit={v} size="md" /></div>
            {user.role === 'SECURITY' && v.status === 'CHECKED_IN' && <p className="mt-3 text-sm text-slate-700">Waiting for host to mark Meeting Over</p>}
          </Card>

          <Card title="Timeline">
            {!v.timeline || v.timeline.length === 0 ? <EmptyState title="No history yet" /> : (
              <ol className="space-y-3 border-l-2 border-slate-200 pl-4">
                {v.timeline.map((t, i) => (
                  <li key={i} className="relative">
                    <span className="absolute -left-[1.4rem] top-1.5 size-2.5 rounded-full bg-blue-700" aria-hidden />
                    <p className="text-sm font-medium text-slate-900">{actionLabel(t.action)}</p>
                    <p className="text-xs text-slate-600">{fmtDateTime(t.createdAt)} · {roleLabel(t.actorRole)}{t.reason ? ` · “${t.reason}”` : ''}</p>
                  </li>
                ))}
              </ol>
            )}
          </Card>
          {user.role === 'ADMIN' && <AuditTrail visitId={v.id} />}
        </div>

        <div className="space-y-5">
          <Card title="Visitor photo">
            {v.hasVisitorPhoto ? <img src={photoUrl('VISITOR')} alt={`Photo of ${v.visitor.name}`} className="w-full rounded-md border border-slate-200" /> : <p className="text-sm text-slate-600">No photo captured yet.</p>}
          </Card>
          {canSeeId && (
            <Card title="ID document">
              {v.hasIdPhoto ? <img src={photoUrl('ID_DOCUMENT')} alt="Photo of the visitor's ID document" className="w-full rounded-md border border-slate-200" /> : <p className="text-sm text-slate-600">No ID photo captured yet.</p>}
            </Card>
          )}
          <Link to={user.role === 'SECURITY' ? '/security' : user.role === 'ADMIN' ? '/admin/visits' : '/employee/visits'} className="inline-block text-sm text-blue-800 underline">← Back to the list</Link>
        </div>
      </div>
    </>
  )
}
