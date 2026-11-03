import { Pressable, Text } from 'react-native'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useRouter } from 'expo-router'
import { api, fmtDateTime } from '../../src/api'
import type { NotificationList } from '../../src/api'
import { keys, POLL_MS } from '../../src/hooks'
import { Button, Card, Empty, ErrorBox, Muted, Screen } from '../../src/ui/components'

export default function Notifications() {
  const router = useRouter()
  const qc = useQueryClient()
  const q = useQuery({ queryKey: keys.notifications, queryFn: () => api.get<NotificationList>('/notifications', { page: 1, pageSize: 30 }), refetchInterval: POLL_MS })
  const read = useMutation({ mutationFn: (id: string) => api.post(`/notifications/${id}/read`), onSuccess: () => qc.invalidateQueries({ queryKey: keys.notifications }) })
  const readAll = useMutation({ mutationFn: () => api.post('/notifications/read-all'), onSuccess: () => qc.invalidateQueries({ queryKey: keys.notifications }) })
  return (
    <Screen onRefresh={() => void q.refetch()} refreshing={q.isRefetching}>
      {q.isError && <ErrorBox message={(q.error as Error).message} onRetry={() => void q.refetch()} />}
      {q.data && q.data.unreadCount > 0 && <Button title={`Mark all read (${q.data.unreadCount})`} variant="secondary" onPress={() => readAll.mutate()} />}
      {q.data && (q.data.items.length === 0 ? <Empty text="No notifications." /> : q.data.items.map((n) => (
        <Pressable key={n.id} accessibilityRole="button" accessibilityLabel={n.title} onPress={() => { if (!n.readAt) read.mutate(n.id); if (n.visitId) router.push(`/visit/${n.visitId}`) }}>
          <Card>
            <Text style={{ fontWeight: n.readAt ? '400' : '700', fontSize: 16 }}>{n.title}</Text>
            <Muted>{n.body}</Muted>
            <Muted>{fmtDateTime(n.createdAt)}</Muted>
          </Card>
        </Pressable>
      )))}
    </Screen>
  )
}
