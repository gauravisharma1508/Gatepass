import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from 'react-router-dom'
import { api, keys, fmtDateTime } from '../../../shared/api'
import type { NotificationList } from '../../../shared/api'
import { Button, cx } from '../../../shared/ui'
import { useLiveUpdates } from '../hooks/useLiveUpdates'

export function NotificationBell() {
  useLiveUpdates(true)
  const qc = useQueryClient()
  const nav = useNavigate()
  const [open, setOpen] = useState(false)
  const root = useRef<HTMLDivElement>(null)

  const list = useQuery({
    queryKey: keys.notifications,
    queryFn: () => api.get<NotificationList>('/notifications', { page: 1, pageSize: 10 }),
  })
  const unread = list.data?.unreadCount ?? 0

  const markRead = useMutation({
    mutationFn: (id: string) => api.post(`/notifications/${id}/read`),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.notifications }),
  })
  const markAll = useMutation({
    mutationFn: () => api.post('/notifications/read-all'),
    onSuccess: () => qc.invalidateQueries({ queryKey: keys.notifications }),
  })

  useEffect(() => {
    if (!open) return
    const onDoc = (e: MouseEvent) => {
      if (root.current && !root.current.contains(e.target as Node)) setOpen(false)
    }
    const onKey = (e: KeyboardEvent) => e.key === 'Escape' && setOpen(false)
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey)
    }
  }, [open])

  return (
    <div ref={root} className="relative">
      <button type="button" aria-haspopup="true" aria-expanded={open} onClick={() => setOpen((o) => !o)}
        aria-label={unread > 0 ? `Notifications, ${unread} unread` : 'Notifications'}
        className="relative inline-flex size-11 items-center justify-center rounded-md text-slate-700 hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-blue-700">
        <svg viewBox="0 0 24 24" className="size-6" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
          <path d="M18 8a6 6 0 10-12 0c0 7-3 9-3 9h18s-3-2-3-9M13.7 21a2 2 0 01-3.4 0" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
        {unread > 0 && (
          <span data-testid="unread-badge" className="absolute right-1 top-1 min-w-5 rounded-full bg-red-700 px-1 text-center text-xs font-bold leading-5 text-white">
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>
      {open && (
        <div role="region" aria-label="Notifications" className="absolute right-0 z-30 mt-2 w-[min(22rem,calc(100vw-2rem))] rounded-lg border border-slate-200 bg-white shadow-xl">
          <div className="flex items-center justify-between border-b border-slate-200 px-3 py-2">
            <h2 className="text-sm font-semibold text-slate-900">Notifications</h2>
            <Button size="sm" variant="ghost" disabled={unread === 0} onClick={() => markAll.mutate()}>Mark all read</Button>
          </div>
          <ul className="max-h-96 overflow-y-auto">
            {list.data?.items.length === 0 && <li className="p-4 text-sm text-slate-600">No notifications yet.</li>}
            {list.data?.items.map((n) => (
              <li key={n.id}>
                <button type="button" onClick={() => {
                  if (!n.readAt) markRead.mutate(n.id)
                  setOpen(false)
                  if (n.visitId) nav(`/visits/${n.visitId}`)
                }} className={cx('block w-full border-b border-slate-100 px-3 py-2 text-left hover:bg-slate-50 focus-visible:outline-2 focus-visible:outline-blue-700', !n.readAt && 'bg-blue-50')}>
                  <span className="flex items-center gap-2 text-sm font-medium text-slate-900">
                    {!n.readAt && <span className="size-2 rounded-full bg-blue-700" aria-label="Unread" />}
                    {n.title}
                  </span>
                  <span className="block text-sm text-slate-700">{n.body}</span>
                  <span className="block text-xs text-slate-500">{fmtDateTime(n.createdAt)}</span>
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
