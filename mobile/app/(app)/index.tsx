import { useQuery } from '@tanstack/react-query'
import { useRouter } from 'expo-router'
import { api } from '../../src/api'
import type { EmployeeDashboard, Visit } from '../../src/api'
import { useAuth } from '../../src/auth/AuthContext'
import { keys, POLL_MS } from '../../src/hooks'
import { Button, Empty, ErrorBox, H1, H2, Screen, VisitCard } from '../../src/ui/components'

export default function Home() {
  const { user, signOut } = useAuth()
  const router = useRouter()
  const q = useQuery({ queryKey: keys.dashboard, queryFn: () => api.get<EmployeeDashboard>('/dashboard/employee'), refetchInterval: POLL_MS })
  const open = (v: Visit) => router.push(`/visit/${v.id}`)
  const section = (title: string, items: Visit[] | undefined, empty: string) => (
    <>
      <H2>{title}</H2>
      {items && items.length > 0 ? items.map((v) => <VisitCard key={v.id} visit={v} onPress={() => open(v)} />) : <Empty text={empty} />}
    </>
  )
  return (
    <Screen onRefresh={() => void q.refetch()} refreshing={q.isRefetching}>
      <H1>Hello, {user?.name.split(' ')[0]}</H1>
      <Button title="New planned visit" onPress={() => router.push('/visit/form')} />
      <H2>{' '}</H2>
      {q.isError && <ErrorBox message={(q.error as Error).message} onRetry={() => void q.refetch()} />}
      {q.data && (
        <>
          {section('Waiting for your answer', q.data.pendingApprovals, 'No walk-ins are waiting.')}
          {section('Awaiting Meeting Over', q.data.awaitingMeetingOver, 'No visitors are inside.')}
          {section("Today's visits", q.data.todaysVisits, 'No visits today.')}
          {section('Upcoming', q.data.upcoming, 'Nothing planned.')}
        </>
      )}
      <H2>{' '}</H2>
      <Button title="Sign out" variant="secondary" onPress={() => void signOut()} />
    </Screen>
  )
}
