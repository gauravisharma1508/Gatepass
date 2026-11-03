import { screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { renderApp, signedInAs } from './utils'
import { server } from './setup'

const err = (status: number, code: string, message: string) =>
  HttpResponse.json({ error: { code, message, details: {} } }, { status })

describe('login form', () => {
  it('asks for both fields when submitted empty', async () => {
    signedInAs(null)
    const user = userEvent.setup()
    renderApp('/login')
    await user.click(await screen.findByRole('button', { name: 'Sign in' }))
    expect(screen.getAllByText('Required')).toHaveLength(2)
  })

  it.each([
    [401, 'UNAUTHENTICATED', 'Invalid email or password', 'Invalid email or password.'],
    [423, 'ACCOUNT_LOCKED', 'locked', /temporarily locked/i],
    [429, 'RATE_LIMITED', 'Too many', /too many attempts/i],
  ] as const)('shows a clear message for HTTP %i', async (status, code, message, expected) => {
    signedInAs(null)
    server.use(http.post('*/api/auth/login', () => err(status, code, message)))
    const user = userEvent.setup()
    renderApp('/login')
    await user.type(await screen.findByLabelText('Email'), 'empA@gatepass.local')
    await user.type(screen.getByLabelText('Password'), 'wrong-password1')
    await user.click(screen.getByRole('button', { name: 'Sign in' }))
    expect(await screen.findByRole('alert')).toHaveTextContent(expected)
  })

  it('forgot password shows the same confirmation whatever the server says', async () => {
    signedInAs(null)
    server.use(http.post('*/api/auth/forgot-password', () => err(500, 'INTERNAL_ERROR', 'boom')))
    const user = userEvent.setup()
    renderApp('/forgot-password')
    await user.type(await screen.findByLabelText('Email'), 'nobody@gatepass.local')
    await user.click(screen.getByRole('button', { name: 'Send reset link' }))
    expect(await screen.findByText(/if an account exists/i)).toBeInTheDocument()
  })

  it('reset page without a valid token offers a new link', async () => {
    signedInAs(null)
    renderApp('/reset-password')
    expect(await screen.findByRole('link', { name: 'Request a new link' })).toBeInTheDocument()
  })
})
