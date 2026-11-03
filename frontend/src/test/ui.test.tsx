import { act, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { Countdown } from '../shared/ui'
import { StatusBadge, statusLabel } from '../shared/ui'
import { ALL_STATUSES, setServerTime } from '../shared/api'

describe('Countdown', () => {
  beforeEach(() => vi.useFakeTimers({ now: new Date('2026-11-03T10:00:00Z') }))
  afterEach(() => vi.useRealTimers())

  it('counts down to the deadline and then says Expired', () => {
    render(<Countdown deadline="2026-11-03T10:01:05Z" />)
    expect(screen.getByRole('timer')).toHaveTextContent('01:05')
    act(() => { vi.advanceTimersByTime(5000) })
    expect(screen.getByRole('timer')).toHaveTextContent('01:00')
    act(() => { vi.advanceTimersByTime(60_000) })
    expect(screen.getByRole('timer')).toHaveTextContent('Expired')
  })

  it('uses the server clock, not a device clock that is 10 minutes ahead', () => {
    // the device says 10:10, the server (Date header) says 10:00
    vi.setSystemTime(new Date('2026-11-03T10:10:00Z'))
    setServerTime(new Date('2026-11-03T10:00:00Z').toUTCString())
    render(<Countdown deadline="2026-11-03T10:01:05Z" />)
    expect(screen.getByRole('timer')).toHaveTextContent('01:05')
  })
})

describe('StatusBadge', () => {
  it.each(ALL_STATUSES)('always prints the status as text: %s', (s) => {
    render(<StatusBadge status={s} />)
    expect(screen.getByText(statusLabel(s))).toBeInTheDocument()
  })
})
