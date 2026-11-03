import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { api, apiUrl, fmtDate, keys } from '../../../shared/api'
import type { Page, UserRow, Visit, VisitStatus } from '../../../shared/api'
import { EmptyState, ErrorState, PageHeader, Pagination, SelectField, Spinner, StatusBadge, TextField, statusLabel } from '../../../shared/ui'

const STATUSES: VisitStatus[] = ['SCHEDULED', 'PENDING_APPROVAL', 'APPROVED', 'CHECKED_IN', 'MEETING_OVER', 'CHECKED_OUT', 'REJECTED', 'EXPIRED', 'CANCELLED', 'NO_SHOW', 'DENIED']

export function AdminVisitsPage() {
  const [sp, setSp] = useSearchParams()
  const f = { status: sp.get('status') ?? '', type: sp.get('type') ?? '', hostId: sp.get('hostId') ?? '', from: sp.get('from') ?? '', to: sp.get('to') ?? '', q: sp.get('q') ?? '', page: Number(sp.get('page') ?? 1) || 1 }
  const update = (patch: Record<string, string>) => {
    const next = new URLSearchParams(sp)
    for (const [k, v] of Object.entries(patch)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    if (!('page' in patch)) next.delete('page')
    setSp(next, { replace: true })
  }
  const filters = { status: f.status || undefined, type: f.type || undefined, hostId: f.hostId || undefined, from: f.from || undefined, to: f.to || undefined, q: f.q || undefined }
  const params = { ...filters, page: f.page, pageSize: 20 }
  const q = useQuery({ queryKey: keys.visits(params), queryFn: () => api.get<Page<Visit>>('/visits', params), placeholderData: (p) => p })
  // the host filter lists the people who can host (employees and admins); the search endpoint is security-only
  const hosts = useQuery({ queryKey: ['users', 'hosts'], staleTime: 60_000, queryFn: () => api.get<Page<UserRow>>('/users', { isActive: true, pageSize: 100 }) })
  const hostList = (hosts.data?.items ?? []).filter((u) => u.role !== 'SECURITY')

  return (
    <>
      <PageHeader title="All visits">
        <a href={apiUrl('/reports/visits.csv', filters)} download className="inline-flex min-h-11 items-center rounded-md border border-slate-300 bg-white px-4 text-sm font-medium text-slate-800 hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-blue-700">
          Export CSV
        </a>
      </PageHeader>
      <form role="search" className="mb-4 grid gap-3 rounded-lg border border-slate-200 bg-white p-4 sm:grid-cols-2 lg:grid-cols-3" onSubmit={(e) => e.preventDefault()}>
        <TextField label="Search" placeholder="Visitor, mobile or visit ID" value={f.q} onChange={(e) => update({ q: e.target.value })} />
        <SelectField label="Status" value={f.status} onChange={(e) => update({ status: e.target.value })}>
          <option value="">All statuses</option>
          {STATUSES.map((s) => <option key={s} value={s}>{statusLabel(s)}</option>)}
        </SelectField>
        <SelectField label="Type" value={f.type} onChange={(e) => update({ type: e.target.value })}>
          <option value="">Planned and walk-in</option><option value="PLANNED">Planned</option><option value="UNPLANNED">Walk-in</option>
        </SelectField>
        <SelectField label="Host" value={f.hostId} onChange={(e) => update({ hostId: e.target.value })}>
          <option value="">Any host</option>
          {hostList.map((u) => <option key={u.id} value={u.id}>{u.name}</option>)}
        </SelectField>
        <TextField label="From date" type="date" value={f.from} onChange={(e) => update({ from: e.target.value })} />
        <TextField label="To date" type="date" value={f.to} onChange={(e) => update({ to: e.target.value })} />
      </form>
      {q.isPending && <Spinner />}
      {q.isError && <ErrorState error={q.error} onRetry={() => void q.refetch()} />}
      {q.data && (q.data.items.length === 0 ? <EmptyState title="No visits match these filters" /> : (
        <>
          <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white">
            <table className="w-full min-w-[44rem] text-left text-sm">
              <caption className="sr-only">Visits</caption>
              <thead><tr className="border-b border-slate-200 bg-slate-50 text-slate-700">
                {['Visit ID', 'Visitor', 'Host', 'Type', 'Date', 'Status'].map((h) => <th key={h} scope="col" className="px-3 py-2 font-medium">{h}</th>)}
              </tr></thead>
              <tbody>
                {q.data.items.map((v) => (
                  <tr key={v.id} className="border-b border-slate-100 hover:bg-slate-50">
                    <td className="px-3 py-2 font-mono"><Link className="text-blue-800 underline" to={`/visits/${v.id}`}>{v.code}</Link></td>
                    <td className="px-3 py-2">{v.visitor.name}<span className="block text-xs text-slate-600">{v.visitor.company ?? ''}</span></td>
                    <td className="px-3 py-2">{v.host.name}</td>
                    <td className="px-3 py-2">{v.type === 'PLANNED' ? 'Planned' : 'Walk-in'}</td>
                    <td className="px-3 py-2 whitespace-nowrap">{fmtDate(v.visitDate)}</td>
                    <td className="px-3 py-2"><StatusBadge status={v.status} />{v.isOverstay && <span className="ml-2 text-xs font-semibold text-red-700">Overstay</span>}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination page={q.data.page} pageSize={q.data.pageSize} total={q.data.total} onPage={(p) => update({ page: String(p) })} />
        </>
      ))}
    </>
  )
}
