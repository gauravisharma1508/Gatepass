import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api, keys } from '../../../shared/api'
import type { SecurityDashboard } from '../../../shared/api'
import { Card, ErrorState, PageHeader, Spinner } from '../../../shared/ui'

const TILES: { key: string; label: string; to: string }[] = [
  { key: 'expectedToday', label: 'Expected today', to: '/admin/visits?status=SCHEDULED&status=APPROVED' },
  { key: 'pendingApprovals', label: 'Waiting for host approval', to: '/admin/visits?status=PENDING_APPROVAL' },
  { key: 'onPremises', label: 'On premises now', to: '/admin/visits?status=CHECKED_IN&status=MEETING_OVER' },
  { key: 'readyForCheckout', label: 'Ready for check-out', to: '/admin/visits?status=MEETING_OVER' },
  { key: 'overstays', label: 'Overstaying', to: '/admin/visits?status=CHECKED_IN' },
]

export function AdminHomePage() {
  const q = useQuery({ queryKey: keys.securityDashboard, queryFn: () => api.get<SecurityDashboard>('/dashboard/security') })
  return (
    <>
      <PageHeader title="Overview" />
      {q.isPending && <Spinner />}
      {q.isError && <ErrorState error={q.error} onRetry={() => void q.refetch()} />}
      {q.data && (
        <ul className="mb-6 grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
          {TILES.map((t) => (
            <li key={t.key}>
              <Link to={t.to} className="block rounded-lg border border-slate-200 bg-white p-4 shadow-sm hover:border-blue-400 focus-visible:outline-2 focus-visible:outline-blue-700">
                <span className="block text-3xl font-semibold tabular-nums text-slate-900" data-testid={`count-${t.key}`}>{q.data.counts[t.key] ?? 0}</span>
                <span className="text-sm text-slate-700">{t.label}</span>
              </Link>
            </li>
          ))}
        </ul>
      )}
      <Card title="Manage">
        <ul className="grid gap-2 text-sm sm:grid-cols-3">
          <li><Link className="text-blue-800 underline" to="/admin/visits">All visits and CSV export</Link></li>
          <li><Link className="text-blue-800 underline" to="/admin/users">Users and roles</Link></li>
          <li><Link className="text-blue-800 underline" to="/admin/settings">Settings</Link></li>
          <li><Link className="text-blue-800 underline" to="/employee">Visitors I host</Link></li>
        </ul>
      </Card>
    </>
  )
}
