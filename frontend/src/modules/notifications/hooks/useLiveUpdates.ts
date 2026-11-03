import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import type { QueryClient } from '@tanstack/react-query'
import type { AppNotification, NotificationList } from '../../../shared/api'
import { keys } from '../../../shared/api'
import { useToast } from '../../../shared/ui'

const refetchAll = (qc: QueryClient) => qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'me' })

/**
 * Opens the server-sent event stream while signed in. `visit-updated` is only a signal: every
 * query is refetched, the payload is never trusted as data. After a reconnect everything is
 * refetched too, because events may have been missed.
 */
export function useLiveUpdates(enabled: boolean) {
  const qc = useQueryClient()
  const toast = useToast()

  useEffect(() => {
    if (!enabled || typeof EventSource === 'undefined') return
    const es = new EventSource('/api/events')
    let opened = false

    es.onopen = () => {
      if (opened) void refetchAll(qc) // reconnected
      opened = true
    }
    es.addEventListener('visit-updated', () => void refetchAll(qc))
    es.addEventListener('notification', (ev) => {
      try {
        const data = JSON.parse((ev as MessageEvent<string>).data) as { notification: AppNotification; unreadCount: number }
        qc.setQueryData<NotificationList>(keys.notifications, (old) =>
          old
            ? { ...old, items: [data.notification, ...old.items.filter((n) => n.id !== data.notification.id)].slice(0, old.pageSize), total: old.total + 1, unreadCount: data.unreadCount }
            : old,
        )
        void qc.invalidateQueries({ queryKey: keys.notifications })
        toast.info(data.notification.title)
      } catch {
        void qc.invalidateQueries({ queryKey: keys.notifications })
      }
    })
    return () => es.close()
  }, [enabled, qc, toast])
}
