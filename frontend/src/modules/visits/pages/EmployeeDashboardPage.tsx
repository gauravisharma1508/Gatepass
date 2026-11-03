import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router-dom'
import { api, keys } from '../../../shared/api'
import type { EmployeeDashboard, Visit } from '../../../shared/api'
import { useUser } from '../../../shared/auth'
import { Card, EmptyState, ErrorState, PageHeader, Spinner } from '../../../shared/ui'
import { VisitCard } from '../components/VisitCard'

function Section({ title, visits, empty, showDate }: { title: string; visits: Visit[]; empty: string; showDate?: boolean }) {
  return (
    <Card title={`${title} (${visits.length})`}>
      {visits.length === 0 ? <EmptyState title={empty} /> : (
        <div className="grid gap-3 lg:grid-cols-2">{visits.map((v) => <VisitCard key={v.id} visit={v} showHost={false} showVisitDate={showDate} />)}</div>
      )}
    </Card>
  )
}

export function EmployeeDashboardPage() {
  const user = useUser()
  const q = useQuery({ queryKey: keys.employeeDashboard, queryFn: () => api.get<EmployeeDashboard>('/dashboard/employee') })
  return (
    <>
      <PageHeader title={`Hello, ${user.name.split(' ')[0]}`}>
        <Link to="/employee/visits/new" className="inline-flex min-h-11 items-center rounded-md bg-blue-700 px-4 text-sm font-medium text-white hover:bg-blue-800 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700">
          New planned visit
        </Link>
      </PageHeader>
      {q.isPending && <Spinner />}
      {q.isError && <ErrorState error={q.error} onRetry={() => void q.refetch()} />}
      {q.data && (
        <div className="space-y-5">
          <Section title="Waiting for your answer" visits={q.data.pendingApprovals} empty="No walk-in requests right now." />
          {/* a visitor who is inside is listed once, under "Waiting for Meeting Over" */}
          <Section title="Visitors today" visits={q.data.todaysVisits.filter((v) => !q.data.awaitingMeetingOver.some((a) => a.id === v.id))} empty="No other visitors today." />
          <Section title="Waiting for Meeting Over" visits={q.data.awaitingMeetingOver} empty="No visitors are inside waiting for you." />
          <Section title="Upcoming" visits={q.data.upcoming} empty="No upcoming visits." showDate />
        </div>
      )}
    </>
  )
}
