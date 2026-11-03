import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { api, keys } from '../../../shared/api'
import type { Page, Visit, VisitStatus } from '../../../shared/api'
import { EmptyState, ErrorState, PageHeader, Pagination, SelectField, Spinner, TextField } from '../../../shared/ui'
import { statusLabel } from '../../../shared/ui'
import { VisitCard } from '../components/VisitCard'
import { FILTER_STATUSES } from '../labels'

export function MyVisitsPage() {
  const [sp, setSp] = useSearchParams()
  const f = { status: sp.get('status') ?? '', from: sp.get('from') ?? '', to: sp.get('to') ?? '', q: sp.get('q') ?? '', page: Number(sp.get('page') ?? 1) || 1 }
  const update = (patch: Record<string, string>) => {
    const next = new URLSearchParams(sp)
    for (const [k, v] of Object.entries(patch)) {
      if (v) next.set(k, v)
      else next.delete(k)
    }
    if (!('page' in patch)) next.delete('page')
    setSp(next, { replace: true })
  }
  const params = { mine: true, status: f.status || undefined, from: f.from || undefined, to: f.to || undefined, q: f.q || undefined, page: f.page, pageSize: 10 }
  const q = useQuery({ queryKey: keys.visits(params), queryFn: () => api.get<Page<Visit>>('/visits', params), placeholderData: (p) => p })

  return (
    <>
      <PageHeader title="My visits">
        <Link to="/employee/visits/new" className="inline-flex min-h-11 items-center rounded-md bg-blue-700 px-4 text-sm font-medium text-white hover:bg-blue-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700">New planned visit</Link>
      </PageHeader>
      <form role="search" className="mb-4 grid gap-3 rounded-lg border border-slate-200 bg-white p-4 sm:grid-cols-2 lg:grid-cols-4" onSubmit={(e) => e.preventDefault()}>
        <TextField label="Search" placeholder="Visitor, mobile or visit ID" value={f.q} onChange={(e) => update({ q: e.target.value })} />
        <SelectField label="Status" value={f.status} onChange={(e) => update({ status: e.target.value })}>
          <option value="">All statuses</option>
          {FILTER_STATUSES.map((s: VisitStatus) => <option key={s} value={s}>{statusLabel(s)}</option>)}
        </SelectField>
        <TextField label="From date" type="date" value={f.from} onChange={(e) => update({ from: e.target.value })} />
        <TextField label="To date" type="date" value={f.to} onChange={(e) => update({ to: e.target.value })} />
      </form>
      {q.isPending && <Spinner />}
      {q.isError && <ErrorState error={q.error} onRetry={() => void q.refetch()} />}
      {q.data && (q.data.items.length === 0 ? <EmptyState title="No visits found">Try different filters, or plan a new visit.</EmptyState> : (
        <>
          <div className="grid gap-3 lg:grid-cols-2">{q.data.items.map((v) => <VisitCard key={v.id} visit={v} showHost={false} showVisitDate />)}</div>
          <Pagination page={q.data.page} pageSize={q.data.pageSize} total={q.data.total} onPage={(p) => update({ page: String(p) })} />
        </>
      ))}
    </>
  )
}
