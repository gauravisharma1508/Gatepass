import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { renderApp, signedInAs } from './utils'
import { server } from './setup'

const emptyDash = () => {
  server.use(
    http.get('*/api/dashboard/security', () => HttpResponse.json({ expectedToday: [], pendingApprovals: [], onPremises: [], readyForCheckout: [], overstays: [], counts: {} })),
    http.get('*/api/dashboard/employee', () => HttpResponse.json({ pendingApprovals: [], todaysVisits: [], upcoming: [], awaitingMeetingOver: [] })),
  )
}
const where = () => screen.getByTestId('location').textContent

describe('route guards and redirects', () => {
  it('sends a signed-out visitor to /login and remembers where they wanted to go', async () => {
    signedInAs(null)
    renderApp('/employee/visits?status=SCHEDULED')
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeInTheDocument()
    expect(where()).toBe('/login?returnTo=%2Femployee%2Fvisits%3Fstatus%3DSCHEDULED')
  })

  it.each([['EMPLOYEE', '/employee'], ['SECURITY', '/security'], ['ADMIN', '/admin']] as const)('redirects "/" for %s to %s', async (role, home) => {
    signedInAs(role)
    emptyDash()
    renderApp('/')
    await waitFor(() => expect(where()).toBe(home))
  })

  it('shows "Access denied" when an employee opens a security page', async () => {
    signedInAs('EMPLOYEE')
    renderApp('/security')
    expect(await screen.findByRole('heading', { name: 'Access denied' })).toBeInTheDocument()
  })

  it('shows "Access denied" when security opens an admin page', async () => {
    signedInAs('SECURITY')
    renderApp('/admin/users')
    expect(await screen.findByRole('heading', { name: 'Access denied' })).toBeInTheDocument()
  })

  it('forces the password change before anything else', async () => {
    signedInAs('EMPLOYEE', { mustChangePassword: true })
    renderApp('/employee')
    expect(await screen.findByRole('heading', { name: 'Change your password' })).toBeInTheDocument()
    expect(where()).toBe('/change-password')
  })

  it('returns to the original page after signing in', async () => {
    signedInAs(null)
    emptyDash()
    const user = userEvent.setup()
    renderApp('/employee')
    await screen.findByRole('heading', { name: 'Sign in' })
    // now the login succeeds and /auth/me knows the user
    server.use(http.post('*/api/auth/login', () => HttpResponse.json({ user: { id: 'u', name: 'Anita Desai', email: 'a@b.co', role: 'EMPLOYEE', employeeCode: 'E', department: null, mustChangePassword: false } })))
    await user.type(screen.getByLabelText('Email'), 'empA@gatepass.local')
    await user.type(screen.getByLabelText('Password'), 'Gatepass@123')
    await user.click(screen.getByRole('button', { name: 'Sign in' }))
    await waitFor(() => expect(where()).toBe('/employee'))
  })

  it('does not follow an off-site returnTo', async () => {
    signedInAs(null)
    emptyDash()
    const user = userEvent.setup()
    renderApp('/login?returnTo=//evil.example')
    await screen.findByRole('heading', { name: 'Sign in' })
    server.use(http.post('*/api/auth/login', () => HttpResponse.json({ user: { id: 'u', name: 'G', email: 'g@b.co', role: 'SECURITY', employeeCode: 'S', department: null, mustChangePassword: false } })))
    await user.type(screen.getByLabelText('Email'), 'guard@gatepass.local')
    await user.type(screen.getByLabelText('Password'), 'Gatepass@123')
    await user.click(screen.getByRole('button', { name: 'Sign in' }))
    await waitFor(() => expect(where()).toBe('/security'))
  })
})
