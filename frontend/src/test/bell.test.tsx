import { QueryClient } from '@tanstack/react-query'
import { screen, waitFor, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { NotificationBell } from '../modules/notifications'
import { renderWithProviders, signedInAs } from './utils'
import { server } from './setup'

class FakeEventSource {
  static last: FakeEventSource
  static all: FakeEventSource[] = []
  listeners: Record<string, ((e: MessageEvent) => void)[]> = {}
  onopen: (() => void) | null = null
  close = vi.fn()
  constructor(public url: string) { FakeEventSource.last = this; FakeEventSource.all.push(this) }
  addEventListener(type: string, fn: (e: MessageEvent) => void) { (this.listeners[type] ??= []).push(fn) }
  emit(type: string, data: unknown) { for (const fn of this.listeners[type] ?? []) fn({ data: JSON.stringify(data) } as MessageEvent) }
}

const note = (id: string, readAt: string | null = null) => ({ id, visitId: 'v1', type: 'APPROVAL_REQUESTED', title: 'Visitor waiting for you', body: 'Ravi Kumar is at reception.', readAt, createdAt: '2026-11-03T04:30:00Z' })

let state: { items: ReturnType<typeof note>[]; unread: number }

beforeEach(() => {
  FakeEventSource.all = []
  vi.stubGlobal('EventSource', FakeEventSource)
  state = { items: [note('n1')], unread: 1 }
  signedInAs('EMPLOYEE')
  server.use(
    http.get('*/api/notifications', () => HttpResponse.json({ items: state.items, page: 1, pageSize: 10, total: state.items.length, unreadCount: state.unread })),
    http.post('*/api/notifications/read-all', () => { state = { items: state.items.map((n) => ({ ...n, readAt: 'x' })), unread: 0 }; return new HttpResponse(null, { status: 204 }) }),
  )
})
afterEach(() => vi.unstubAllGlobals())

describe('notification bell', () => {
  it('shows the unread count and opens the event stream with the session', async () => {
    renderWithProviders(<NotificationBell />)
    expect(await screen.findByTestId('unread-badge')).toHaveTextContent('1')
    expect(FakeEventSource.last.url).toBe('/api/events')
  })

  it('raises the badge as soon as a "notification" event arrives', async () => {
    renderWithProviders(<NotificationBell />)
    await screen.findByTestId('unread-badge')
    state = { items: [note('n2'), note('n1')], unread: 2 }
    act(() => FakeEventSource.last.emit('notification', { notification: note('n2'), unreadCount: 2 }))
    await waitFor(() => expect(screen.getByTestId('unread-badge')).toHaveTextContent('2'))
  })

  it('refetches everything on "visit-updated" and after a reconnect, never trusting the payload', async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    const spy = vi.spyOn(client, 'invalidateQueries')
    renderWithProviders(<NotificationBell />, { client })
    await screen.findByTestId('unread-badge')
    spy.mockClear()
    act(() => FakeEventSource.last.emit('visit-updated', { id: 'v1', status: 'APPROVED', version: 2 }))
    expect(spy).toHaveBeenCalledTimes(1)
    spy.mockClear()
    act(() => FakeEventSource.last.onopen?.()) // first open: nothing to catch up on
    expect(spy).not.toHaveBeenCalled()
    act(() => FakeEventSource.last.onopen?.()) // reconnect: events may have been missed
    expect(spy).toHaveBeenCalledTimes(1)
  })

  it('marks everything read', async () => {
    const user = userEvent.setup()
    renderWithProviders(<NotificationBell />)
    await user.click(await screen.findByRole('button', { name: /Notifications, 1 unread/ }))
    await user.click(screen.getByRole('button', { name: 'Mark all read' }))
    await waitFor(() => expect(screen.queryByTestId('unread-badge')).not.toBeInTheDocument())
  })

  it('closes the stream when the bell is removed (sign-out)', async () => {
    const { unmount } = renderWithProviders(<NotificationBell />)
    await screen.findByTestId('unread-badge')
    const es = FakeEventSource.last
    unmount()
    expect(es.close).toHaveBeenCalled()
  })
})
