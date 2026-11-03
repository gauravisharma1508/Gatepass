import { useEffect, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Link, useSearchParams } from 'react-router-dom'
import { api, keys } from '../../../shared/api'
import type { Page, SecurityDashboard, Visit } from '../../../shared/api'
import { EmptyState, ErrorState, PageHeader, Spinner, Tabs, TextField } from '../../../shared/ui'
import { VisitCard } from '../../visits'

const TABS = [
  { id: 'expected', label: 'Expected today', key: 'expectedToday' as const, empty: 'Nobody is expected for the rest of today.' },
  { id: 'pending', label: 'Pending approval', key: 'pendingApprovals' as const, empty: 'No walk-ins are waiting for a host to answer.' },
  { id: 'inside', label: 'On premises', key: 'onPremises' as const, empty: 'No visitors are inside right now.' },
  { id: 'checkout', label: 'Ready for check-out', key: 'readyForCheckout' as const, empty: 'No visitor is waiting to be checked out.' },
  { id: 'overstay', label: 'Overstay', key: 'overstays' as const, empty: 'Nobody has stayed past their expected time.' },
]

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

export function SecurityDashboardPage() {
  const [sp, setSp] = useSearchParams()
  const tab = TABS.find((t) => t.id === sp.get('tab'))?.id ?? 'expected'
  const [search, setSearch] = useState('')
  const term = useDebounced(search.trim(), 300)

  const dash = useQuery({ queryKey: keys.securityDashboard, queryFn: () => api.get<SecurityDashboard>('/dashboard/security'), refetchInterval: 60_000 })
  const searching = term.length >= 2
  const params = { q: term, pageSize: 20 }
  const found = useQuery({ queryKey: keys.visits(params), enabled: searching, queryFn: () => api.get<Page<Visit>>('/visits', params) })

  const active = TABS.find((t) => t.id === tab)!
  const list = dash.data?.[active.key] ?? []

  return (
    <>
      <PageHeader title="Gate dashboard">
        <Link to="/security/walk-in" className="inline-flex min-h-11 items-center rounded-md bg-blue-700 px-4 text-sm font-medium text-white hover:bg-blue-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700">Register walk-in</Link>
      </PageHeader>

      <div role="search" className="mb-4 max-w-xl">
        <TextField label="Find a visitor" type="search" placeholder="Name, mobile number or visit ID" value={search} onChange={(e) => setSearch(e.target.value)} />
      </div>

      {searching ? (
        <section aria-label="Search results">
          <h2 className="mb-3 text-base font-semibold text-slate-900">Search results for “{term}”</h2>
          {found.isPending && <Spinner />}
          {found.isError && <ErrorState error={found.error} onRetry={() => void found.refetch()} />}
          {found.data && (found.data.items.length === 0 ? <EmptyState title="No visitor matches that search" /> : (
            <div className="grid gap-3 lg:grid-cols-2">{found.data.items.map((v) => <VisitCard key={v.id} visit={v} showVisitDate />)}</div>
          ))}
        </section>
      ) : (
        <>
          <Tabs label="Visitor lists" active={tab} onChange={(id) => setSp({ tab: id }, { replace: true })}
            tabs={TABS.map((t) => ({ id: t.id, label: t.label, count: dash.data?.counts[t.key], alert: t.id === 'overstay' }))} />
          <div role="tabpanel" id={`panel-${tab}`} aria-labelledby={`tab-${tab}`} className="pt-4">
            {dash.isPending && <Spinner />}
            {dash.isError && <ErrorState error={dash.error} onRetry={() => void dash.refetch()} />}
            {dash.data && (list.length === 0 ? <EmptyState title={active.empty} /> : (
              <div className="grid gap-3 lg:grid-cols-2">{list.map((v) => <VisitCard key={v.id} visit={v} />)}</div>
            ))}
          </div>
        </>
      )}
    </>
  )
}
