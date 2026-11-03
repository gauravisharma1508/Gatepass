import { useEffect, useState } from 'react'
import { serverNow } from '../api/serverTime'
import { cx } from './cx'

/** Seconds left until `deadline` (ISO), measured with the server clock, ticking every second. */
export function useSecondsLeft(deadline: string | null | undefined): number | null {
  const calc = () => (deadline ? Math.max(0, Math.ceil((Date.parse(deadline) - serverNow()) / 1000)) : null)
  const [left, setLeft] = useState<number | null>(calc)
  useEffect(() => {
    setLeft(calc())
    if (!deadline) return
    const t = setInterval(() => setLeft(calc()), 1000)
    return () => clearInterval(t)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [deadline])
  return left
}

export function formatMMSS(s: number): string {
  const m = Math.floor(s / 60)
  return `${String(m).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

export function Countdown({ deadline, className }: { deadline: string | null; className?: string }) {
  const left = useSecondsLeft(deadline)
  if (left === null) return null
  const expired = left === 0
  return (
    <span role="timer" aria-live="off" className={cx('font-mono text-sm font-semibold tabular-nums',
      expired ? 'text-orange-800' : left <= 300 ? 'text-red-700' : 'text-slate-800', className)}>
      {expired ? 'Expired' : formatMMSS(left)}
    </span>
  )
}
