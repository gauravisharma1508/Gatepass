import { screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { WalkInPage } from '../modules/walkins/pages/WalkInPage'
import { makeVisit, renderWithProviders, signedInAs } from './utils'
import { server } from './setup'

const anita = { id: 'h1', name: 'Anita Desai', employeeCode: 'EMP002', department: 'Engineering' }

function setup() {
  signedInAs('SECURITY')
  const calls: string[] = []
  server.use(
    http.get('*/api/employees/search', ({ request }) => { calls.push(new URL(request.url).searchParams.get('q') ?? ''); return HttpResponse.json({ items: [anita] }) }),
    http.get('*/api/visitors/lookup', () => HttpResponse.json({ error: { code: 'NOT_FOUND', message: 'x', details: {} } }, { status: 404 })),
  )
  return { calls, user: userEvent.setup() }
}

describe('"Whom to meet" typeahead', () => {
  it('does not search for 1 character, and sends one debounced request for the rest', async () => {
    const { calls, user } = setup()
    renderWithProviders(<WalkInPage />)
    const box = await screen.findByRole('combobox', { name: 'Whom to meet' })
    await user.type(box, 'a')
    await new Promise((r) => setTimeout(r, 450))
    expect(calls).toEqual([])
    await user.type(box, 'ni')
    expect(await screen.findByRole('option', { name: /Anita Desai/ })).toBeInTheDocument()
    expect(calls).toEqual(['ani']) // typed quickly -> a single request, not one per keystroke
  })

  it('refuses free text: a walk-in cannot be sent until an employee is chosen', async () => {
    const { user } = setup()
    let posted = false
    server.use(http.post('*/api/visits/walk-in', () => { posted = true; return HttpResponse.json(makeVisit(), { status: 201 }) }))
    renderWithProviders(<WalkInPage />)
    await user.type(await screen.findByRole('combobox', { name: 'Whom to meet' }), 'Anita Desai')
    await user.type(screen.getByLabelText('Visitor mobile number'), '9876543210')
    await user.type(screen.getByLabelText('Visitor full name'), 'Ravi Kumar')
    await user.type(screen.getByLabelText('Purpose of visit'), 'Vendor discussion')
    await user.click(screen.getByRole('button', { name: 'Ask the host to approve' }))
    expect(await screen.findByText('Choose who to meet')).toBeInTheDocument()
    expect(posted).toBe(false)
  })

  it('sends the chosen employee id and then shows the live request', async () => {
    const { user } = setup()
    let body: Record<string, unknown> | undefined
    const pending = makeVisit({ id: 'w1', type: 'UNPLANNED', status: 'PENDING_APPROVAL', approvalDeadline: new Date(Date.now() + 1_800_000).toISOString() })
    server.use(
      http.post('*/api/visits/walk-in', async ({ request }) => { body = (await request.json()) as Record<string, unknown>; return HttpResponse.json(pending, { status: 201 }) }),
      http.get('*/api/visits/w1', () => HttpResponse.json(pending)),
    )
    renderWithProviders(<WalkInPage />)
    await user.type(await screen.findByRole('combobox', { name: 'Whom to meet' }), 'ani')
    await user.click(await screen.findByRole('option', { name: /Anita Desai/ }))
    expect(screen.getByTestId('selected-host')).toHaveTextContent('Anita Desai')
    await user.type(screen.getByLabelText('Visitor mobile number'), '9876543210')
    await user.type(screen.getByLabelText('Visitor full name'), 'Ravi Kumar')
    await user.type(screen.getByLabelText('Purpose of visit'), 'Vendor discussion')
    await user.click(screen.getByRole('button', { name: 'Ask the host to approve' }))
    await waitFor(() => expect(body).toMatchObject({ hostId: 'h1', visitorMobile: '+919876543210', expectedDurationMin: 60 }))
    expect(await screen.findByRole('timer')).toHaveTextContent(/^\d\d:\d\d$/)
    expect(screen.getByText(/Request sent/)).toBeInTheDocument()
  })
})
