import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { renderApp, signedInAs, makeVisit } from './utils'
import { server } from './setup'

function setup() {
  signedInAs('EMPLOYEE')
  server.use(
    http.get('*/api/visitors/lookup', () => HttpResponse.json({ error: { code: 'NOT_FOUND', message: 'Not found', details: {} } }, { status: 404 })),
    http.get('*/api/dashboard/employee', () => HttpResponse.json({ pendingApprovals: [], todaysVisits: [], upcoming: [], awaitingMeetingOver: [] })),
    http.get('*/api/visits', () => HttpResponse.json({ items: [], page: 1, pageSize: 10, total: 0 })),
  )
  return userEvent.setup()
}

describe('planned visit form', () => {
  it('shows an error under every invalid field and sends nothing', async () => {
    const user = setup()
    let posted = false
    server.use(http.post('*/api/visits/planned', () => { posted = true; return HttpResponse.json(makeVisit(), { status: 201 }) }))
    renderApp('/employee/visits/new')
    await user.type(await screen.findByLabelText('Visitor mobile number'), '12345')
    await user.type(screen.getByLabelText('Visitor full name'), 'R2')
    await user.type(screen.getByLabelText('Purpose of visit'), 'ab')
    await user.click(screen.getByRole('button', { name: 'Create visit' }))
    expect(await screen.findByText('Enter a valid 10-digit Indian mobile number')).toBeInTheDocument()
    expect(screen.getByText("Only letters, spaces and . ' - are allowed")).toBeInTheDocument()
    expect(screen.getByText('Too short')).toBeInTheDocument()
    expect(posted).toBe(false)
  })

  it('rejects a past visit date', async () => {
    const user = setup()
    renderApp('/employee/visits/new')
    await user.type(await screen.findByLabelText('Visitor mobile number'), '9876543210')
    await user.type(screen.getByLabelText('Visitor full name'), 'Ravi Kumar')
    await user.type(screen.getByLabelText('Purpose of visit'), 'Vendor discussion')
    const date = screen.getByLabelText('Visit date')
    await user.clear(date)
    await user.type(date, '2020-01-01')
    await user.click(screen.getByRole('button', { name: 'Create visit' }))
    expect(await screen.findByText('Visit date cannot be in the past')).toBeInTheDocument()
  })

  it('submits normalised data and goes back to My visits', async () => {
    const user = setup()
    let body: Record<string, unknown> | undefined
    server.use(http.post('*/api/visits/planned', async ({ request }) => { body = (await request.json()) as Record<string, unknown>; return HttpResponse.json(makeVisit(), { status: 201 }) }))
    renderApp('/employee/visits/new')
    await user.type(await screen.findByLabelText('Visitor mobile number'), '98765 43210')
    await user.type(screen.getByLabelText('Visitor full name'), '  Ravi Kumar ')
    await user.type(screen.getByLabelText('Company (optional)'), 'Acme')
    await user.type(screen.getByLabelText('Purpose of visit'), 'Vendor discussion')
    await user.click(screen.getByRole('button', { name: 'Create visit' }))
    await waitFor(() => expect(screen.getByTestId('location')).toHaveTextContent('/employee/visits'))
    expect(body).toMatchObject({ visitorMobile: '+919876543210', visitorName: 'Ravi Kumar', visitorCompany: 'Acme', purpose: 'Vendor discussion', expectedDurationMin: 60, expectedArrival: '' })
  })

  it('shows field errors that come back from the server', async () => {
    const user = setup()
    server.use(http.post('*/api/visits/planned', () => HttpResponse.json({ error: { code: 'VALIDATION_ERROR', message: 'Some fields are invalid', details: { fields: { visitDate: 'Visit date is too far ahead' } } } }, { status: 400 })))
    renderApp('/employee/visits/new')
    await user.type(await screen.findByLabelText('Visitor mobile number'), '9876543210')
    await user.type(screen.getByLabelText('Visitor full name'), 'Ravi Kumar')
    await user.type(screen.getByLabelText('Purpose of visit'), 'Vendor discussion')
    await user.click(screen.getByRole('button', { name: 'Create visit' }))
    expect(await screen.findByText('Visit date is too far ahead')).toBeInTheDocument()
  })

  it('pre-fills the visitor from a known mobile number', async () => {
    const user = setup()
    server.use(http.get('*/api/visitors/lookup', () => HttpResponse.json({ name: 'Ravi Kumar', email: 'ravi@acme.com', company: 'Acme' })))
    renderApp('/employee/visits/new')
    await user.type(await screen.findByLabelText('Visitor mobile number'), '9876543210')
    await user.tab()
    await waitFor(() => expect(screen.getByLabelText('Visitor full name')).toHaveValue('Ravi Kumar'))
    expect(screen.getByLabelText('Company (optional)')).toHaveValue('Acme')
  })
})
