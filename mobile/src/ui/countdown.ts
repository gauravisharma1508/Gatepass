import { useEffect, useState } from 'react'
import { serverNow } from '../api/serverTime'

/** Whole seconds left until `deadline` (ISO), measured on the server clock. Never negative. */
export function secondsLeft(deadline: string | null | undefined, nowMs: number = serverNow()): number | null {
  return deadline ? Math.max(0, Math.ceil((Date.parse(deadline) - nowMs) / 1000)) : null
}

export function formatMMSS(s: number): string {
  return `${String(Math.floor(s / 60)).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}`
}

export function useSecondsLeft(deadline: string | null | undefined): number | null {
  const [, tick] = useState(0)
  useEffect(() => {
    if (!deadline) return
    const t = setInterval(() => tick((n) => n + 1), 1000)
    return () => clearInterval(t)
  }, [deadline])
  return secondsLeft(deadline) // computed on every render, so a new deadline is never stale
}
