import { fireEvent, render, screen, waitFor } from '@testing-library/react-native'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactElement } from 'react'
import { Alert } from 'react-native'
import { setToken } from '../src/api'
import type { Visit } from '../src/api'
import { AuthProvider } from '../src/auth/AuthContext'
import { LoginScreen } from '../src/screens-login'
import { VisitDetail } from '../src/VisitDetail'
import { PlannedVisitForm, emptyValues } from '../src/VisitForm'

jest.mock('expo-router', () => ({ useRouter: () => ({ push: jest.fn(), back: jest.fn(), replace: jest.fn(), canGoBack: () => true }) }))

const calls: { url: string; init?: RequestInit }[] = []
let routes: Record<string, (init?: RequestInit) => { status?: number; body?: unknown }> = {}
beforeEach(() => {
  calls.length = 0
  routes = {}
  setToken(null)
  global.fetch = jest.fn(async (url: string, init?: RequestInit) => {
    calls.push({ url, init })
    const key = `${init?.method ?? 'GET'} ${new URL(url).pathname}`
    const r = routes[key]?.(init) ?? { status: 404, body: { error: { code: 'NOT_FOUND', message: 'Not found' } } }
    const status = r.status ?? 200
    return { ok: status < 400, status, headers: { get: () => null }, json: async () => r.body } as unknown as Response
  }) as unknown as typeof fetch
})

const wrap = (ui: ReactElement) => (
  <QueryClientProvider client={new QueryClient({ defaultOptions: { queries: { retry: false } } })}><AuthProvider>{ui}</AuthProvider></QueryClientProvider>
)

const baseVisit = (over: Partial<Visit>): Visit => ({
  id: 'v1', code: 'GP-1', type: 'UNPLANNED', status: 'PENDING_APPROVAL', visitor: { name: 'Asha Rao', mobile: '+919876543210', email: null, company: null },
  host: { id: 'h', name: 'Emp A', department: null }, purpose: 'Delivery', visitDate: '2026-11-03', expectedArrival: null, expectedDurationMin: 30,
  approvalDeadline: new Date(Date.now() + 25 * 60_000).toISOString(), approvalSecondsLeft: 1500, decidedAt: null, decisionReason: null, checkedInAt: null,
  meetingOverAt: null, checkedOutAt: null, closedReason: null, isOverstay: false, hasVisitorPhoto: true, hasIdPhoto: true, idType: null,
  allowedActions: ['APPROVE', 'REJECT'], version: 1, createdAt: new Date().toISOString(),
  timeline: [{ action: 'VISIT_CREATED', actorRole: 'SECURITY', fromStatus: null, toStatus: 'PENDING_APPROVAL', reason: null, createdAt: new Date().toISOString() }], ...over,
})

describe('login screen', () => {
  it('validates before calling the server', async () => {
    render(wrap(<LoginScreen />))
    fireEvent.press(screen.getByRole('button', { name: 'Sign in' }))
    expect((await screen.findAllByText('Required')).length).toBe(2)
    expect(calls.length).toBe(0)
  })
  it('sends the mobile headers and shows the server error (security user)', async () => {
    routes['POST /api/auth/login'] = () => ({ status: 403, body: { error: { code: 'MOBILE_NOT_ALLOWED', message: 'Security staff use the web app, not the mobile app' } } })
    render(wrap(<LoginScreen />))
    fireEvent.changeText(screen.getByLabelText('Email'), 'guard@gatepass.local')
    fireEvent.changeText(screen.getByLabelText('Password'), 'Gatepass@123')
    fireEvent.press(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByText(/Security staff use the web app/)).toBeTruthy()
    const h = calls[0].init?.headers as Record<string, string>
    expect(h['X-Client']).toBe('mobile')
    expect(h['X-Requested-With']).toBe('gatepass')
    expect(h.Authorization).toBeUndefined()
  })
})

describe('visit detail', () => {
  it('shows the walk-in countdown, the timeline, and accepts with the bearer token', async () => {
    setToken('tok123')
    routes['GET /api/visits/v1'] = () => ({ body: baseVisit({}) })
    routes['POST /api/visits/v1/approve'] = () => ({ body: baseVisit({ status: 'APPROVED', allowedActions: [] }) })
    render(wrap(<VisitDetail id="v1" />))
    expect(await screen.findByText('Asha Rao')).toBeTruthy()
    expect(screen.getByRole('timer').props.children).toMatch(/^2[45]:\d\d$/)
    expect(screen.getByText('Visit created')).toBeTruthy()
    fireEvent.press(screen.getByRole('button', { name: 'Accept' }))
    await waitFor(() => expect(calls.some((c) => c.url.endsWith('/visits/v1/approve'))).toBe(true))
    const post = calls.find((c) => c.url.endsWith('/approve'))!
    expect((post.init?.headers as Record<string, string>).Authorization).toBe('Bearer tok123')
  })
  it('offers only what allowedActions lists (Meeting over, nothing else)', async () => {
    routes['GET /api/visits/v1'] = () => ({ body: baseVisit({ status: 'CHECKED_IN', approvalDeadline: null, allowedActions: ['MEETING_OVER'] }) })
    render(wrap(<VisitDetail id="v1" />))
    await screen.findByText('Asha Rao')
    expect(screen.getByRole('button', { name: 'Meeting over' })).toBeTruthy()
    expect(screen.queryByRole('button', { name: 'Accept' })).toBeNull()
    expect(screen.queryByRole('button', { name: 'Cancel visit' })).toBeNull()
  })
  it('disables Accept and Reject once the approval window is over', async () => {
    routes['GET /api/visits/v1'] = () => ({ body: baseVisit({ approvalDeadline: new Date(Date.now() - 1000).toISOString() }) })
    render(wrap(<VisitDetail id="v1" />))
    await screen.findByText('Asha Rao')
    expect(screen.getByRole('button', { name: 'Accept' }).props.accessibilityState?.disabled).toBe(true)
    expect(screen.getByText('Expired')).toBeTruthy()
  })
})

describe('planned visit form', () => {
  it('blocks a bad form without calling the server', async () => {
    render(wrap(<PlannedVisitForm initial={emptyValues('2026-11-03')} onSaved={jest.fn()} />))
    fireEvent.press(screen.getByRole('button', { name: 'Create visit' }))
    expect(await screen.findAllByText('Required')).not.toHaveLength(0)
    expect(calls.length).toBe(0)
  })
  it('posts a normalised body and calls onSaved', async () => {
    const onSaved = jest.fn()
    routes['POST /api/visits/planned'] = () => ({ status: 201, body: baseVisit({ type: 'PLANNED' }) })
    const today = new Date().toISOString().slice(0, 10)
    render(wrap(<PlannedVisitForm initial={{ ...emptyValues(today), visitDate: today }} onSaved={onSaved} />))
    fireEvent.changeText(screen.getByLabelText('Visitor name'), 'Rahul Verma')
    fireEvent.changeText(screen.getByLabelText('Visitor mobile'), '98765 43210')
    fireEvent.changeText(screen.getByLabelText('Purpose'), 'Project review')
    fireEvent.press(screen.getByRole('button', { name: 'Create visit' }))
    await waitFor(() => expect(onSaved).toHaveBeenCalled())
    const body = JSON.parse(calls.find((c) => c.url.endsWith('/visits/planned'))!.init!.body as string)
    expect(body.visitorMobile).toBe('+919876543210')
    expect(body.expectedDurationMin).toBe(60)
  })
  it('shows server field errors', async () => {
    jest.spyOn(Alert, 'alert').mockImplementation(() => {})
    routes['POST /api/visits/planned'] = () => ({ status: 400, body: { error: { code: 'VALIDATION_ERROR', message: 'bad', details: { fields: { purpose: 'Server says no' } } } } })
    const today = new Date().toISOString().slice(0, 10)
    render(wrap(<PlannedVisitForm initial={{ ...emptyValues(today), visitorName: 'Rahul Verma', visitorMobile: '9876543210', purpose: 'Review' }} onSaved={jest.fn()} />))
    fireEvent.press(screen.getByRole('button', { name: 'Create visit' }))
    expect(await screen.findByText('Server says no')).toBeTruthy()
  })
})
