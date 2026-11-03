import { screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { http, HttpResponse } from 'msw'
import { describe, expect, it } from 'vitest'
import { VisitActions } from '../modules/visits'
import type { VisitAction } from '../shared/api'
import { makeVisit, renderWithProviders, signedInAs } from './utils'
import { server } from './setup'

const show = (allowed: VisitAction[], over = {}) => {
  signedInAs('SECURITY')
  return renderWithProviders(<VisitActions visit={makeVisit({ allowedActions: allowed, ...over })} />)
}

describe('VisitActions are driven only by allowedActions', () => {
  it('offers nothing when the server allows nothing', () => {
    show([])
    expect(screen.queryByTestId('visit-actions')).not.toBeInTheDocument()
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('offers Accept and Reject for a pending walk-in, and nothing else', () => {
    show(['APPROVE', 'REJECT'], { status: 'PENDING_APPROVAL', approvalDeadline: new Date(Date.now() + 600_000).toISOString() })
    expect(screen.getByRole('button', { name: 'Accept' })).toBeEnabled()
    expect(screen.getByRole('button', { name: 'Reject' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: /check out/i })).not.toBeInTheDocument()
  })

  it('shows Check out only when CHECK_OUT is allowed (never for a checked-in visitor)', () => {
    const { unmount } = show([], { status: 'CHECKED_IN' })
    expect(screen.queryByRole('button', { name: /check out/i })).not.toBeInTheDocument()
    unmount()
    show(['CHECK_OUT'], { status: 'MEETING_OVER' })
    expect(screen.getByRole('button', { name: 'Check out' })).toBeInTheDocument()
  })

  it('one "Check in" link covers photo capture and check-in', () => {
    show(['UPLOAD_PHOTOS', 'CHECK_IN', 'DENY'])
    expect(screen.getAllByRole('link', { name: 'Check in' })).toHaveLength(1)
    expect(screen.getByRole('link', { name: 'Check in' })).toHaveAttribute('href', '/security/visits/v1/check-in')
    expect(screen.getByRole('button', { name: 'Deny entry' })).toBeInTheDocument()
  })

  it('a visitor without photos still shows "Check in" (the screen captures the photos first)', () => {
    show(['UPLOAD_PHOTOS', 'DENY'])
    expect(screen.getByRole('link', { name: 'Check in' })).toHaveAttribute('href', '/security/visits/v1/check-in')
  })

  it('disables Accept and Reject once the approval window has closed', () => {
    show(['APPROVE', 'REJECT'], { status: 'PENDING_APPROVAL', approvalDeadline: new Date(Date.now() - 5000).toISOString() })
    expect(screen.getByRole('button', { name: 'Accept' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Reject' })).toBeDisabled()
  })

  it('asks for a reason (min 5 characters) before denying entry, then calls the API', async () => {
    let body: unknown
    server.use(http.post('*/api/visits/v1/deny', async ({ request }) => { body = await request.json(); return HttpResponse.json(makeVisit()) }))
    const user = userEvent.setup()
    show(['DENY'])
    await user.click(screen.getByRole('button', { name: 'Deny entry' }))
    const dialog = screen.getByRole('dialog')
    const reason = within(dialog).getByLabelText('Reason for denying entry')
    await user.type(reason, 'no')
    await user.click(within(dialog).getByRole('button', { name: 'Deny entry' }))
    expect(await within(dialog).findByText(/at least 5 characters/i)).toBeInTheDocument()
    expect(body).toBeUndefined()
    await user.clear(reason)
    await user.type(reason, 'Name on ID does not match')
    await user.click(within(dialog).getByRole('button', { name: 'Deny entry' }))
    await waitFor(() => expect(body).toEqual({ reason: 'Name on ID does not match' }))
  })

  it('shows the API message when check-out is refused', async () => {
    server.use(http.post('*/api/visits/v1/check-out', () =>
      HttpResponse.json({ error: { code: 'INVALID_STATE', message: 'Waiting for host to mark Meeting Over', details: {} } }, { status: 409 })))
    const user = userEvent.setup()
    show(['CHECK_OUT'], { status: 'MEETING_OVER' })
    await user.click(screen.getByRole('button', { name: 'Check out' }))
    await user.click(within(screen.getByRole('dialog')).getByRole('button', { name: 'Check out' }))
    expect(await screen.findByText('Waiting for host to mark Meeting Over')).toBeInTheDocument()
  })
})
