import { Tabs } from 'expo-router'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../src/api'
import type { NotificationList } from '../../src/api'
import { keys, POLL_MS } from '../../src/hooks'

export default function AppTabs() {
  const n = useQuery({ queryKey: keys.notifications, queryFn: () => api.get<NotificationList>('/notifications', { page: 1, pageSize: 30 }), refetchInterval: POLL_MS })
  const unread = n.data?.unreadCount ?? 0
  return (
    <Tabs>
      <Tabs.Screen name="index" options={{ title: 'Home' }} />
      <Tabs.Screen name="visits" options={{ title: 'My visits' }} />
      <Tabs.Screen name="notifications" options={{ title: 'Notifications', tabBarBadge: unread > 0 ? unread : undefined }} />
    </Tabs>
  )
}
