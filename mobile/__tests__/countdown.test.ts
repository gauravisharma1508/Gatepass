import { formatMMSS, secondsLeft } from '../src/ui/countdown'
import { resetServerTime, setServerTime } from '../src/api/serverTime'

describe('countdown', () => {
  const deadline = '2026-11-03T10:30:00Z'
  afterEach(() => resetServerTime())

  it('counts down from the given time and never goes negative', () => {
    expect(secondsLeft(deadline, Date.parse('2026-11-03T10:00:00Z'))).toBe(1800)
    expect(secondsLeft(deadline, Date.parse('2026-11-03T10:29:59.200Z'))).toBe(1)
    expect(secondsLeft(deadline, Date.parse(deadline))).toBe(0)
    expect(secondsLeft(deadline, Date.parse('2026-11-03T11:00:00Z'))).toBe(0)
  })
  it('is null without a deadline', () => expect(secondsLeft(null)).toBeNull())
  it('formats mm:ss', () => {
    expect(formatMMSS(1800)).toBe('30:00')
    expect(formatMMSS(65)).toBe('01:05')
    expect(formatMMSS(0)).toBe('00:00')
  })
  it('uses the server clock, not a wrong device clock', () => {
    // device clock is 10 minutes fast; the server Date header says it is 10:00:00
    jest.useFakeTimers().setSystemTime(new Date('2026-11-03T10:10:00Z'))
    setServerTime('Tue, 03 Nov 2026 10:00:00 GMT')
    expect(secondsLeft(deadline)).toBe(1800)
    jest.useRealTimers()
  })
})
