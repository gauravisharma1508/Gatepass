import { render } from '@testing-library/react'
import { QueryClient } from '@tanstack/react-query'
import { MemoryRouter, useLocation } from 'react-router-dom'
import type { ReactElement } from 'react'
import { http, HttpResponse } from 'msw'
import { Providers } from '../app/Providers'
import { AppRoutes } from '../app/AppRoutes'
import type { Role, SessionUser, Visit } from '../shared/api'
import { server } from './setup'

export function LocationProbe() {
  const l = useLocation()
  return <output data-testid="location">{l.pathname + l.search}</output>
}

export function makeUser(role: Role, over: Partial<SessionUser> = {}): SessionUser {
  return { id: `u-${role}`, name: role === 'SECURITY' ? 'Suresh Guard' : 'Anita Desai', email: `${role.toLowerCase()}@gatepass.local`, role,
    employeeCode: 'EMP002', department: 'Engineering', mustChangePassword: false, ...over }
}

export function signedInAs(role: Role | null, over: Partial<SessionUser> = {}) {
  server.use(
    http.get('*/api/auth/me', () => role
      ? HttpResponse.json({ user: makeUser(role, over) })
      : HttpResponse.json({ error: { code: 'UNAUTHENTICATED', message: 'Please sign in', details: {} } }, { status: 401 })),
    http.get('*/api/notifications', () => HttpResponse.json({ items: [], page: 1, pageSize: 10, total: 0, unreadCount: 0 })),
  )
}

export function renderWithProviders(ui: ReactElement, { route = '/', client }: { route?: string; client?: QueryClient } = {}) {
  const qc = client ?? new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <Providers client={qc}>
      <MemoryRouter initialEntries={[route]}>
        {ui}
        <LocationProbe />
      </MemoryRouter>
    </Providers>,
  )
}

export function renderApp(route: string) {
  return renderWithProviders(<AppRoutes />, { route })
}

export function makeVisit(over: Partial<Visit> = {}): Visit {
  return {
    id: 'v1', code: 'GP-20261103-0001', type: 'PLANNED', status: 'SCHEDULED',
    visitor: { name: 'Ravi Kumar', mobile: '+919876543210', email: null, company: 'Acme' },
    host: { id: 'h1', name: 'Anita Desai', department: 'Engineering' }, purpose: 'Vendor discussion', visitDate: '2026-11-03',
    expectedArrival: null, expectedDurationMin: 60, approvalDeadline: null, approvalSecondsLeft: null, decidedAt: null, decisionReason: null,
    checkedInAt: null, meetingOverAt: null, checkedOutAt: null, closedReason: null, isOverstay: false, hasVisitorPhoto: false, hasIdPhoto: false,
    idType: null, allowedActions: [], version: 1, createdAt: '2026-11-03T04:30:00Z', ...over,
  }
}
