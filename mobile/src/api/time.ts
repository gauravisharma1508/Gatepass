export const OFFICE_TZ = 'Asia/Kolkata'

const dtf = new Intl.DateTimeFormat('en-IN', {
  timeZone: OFFICE_TZ, day: '2-digit', month: 'short', year: 'numeric', hour: '2-digit', minute: '2-digit', hour12: true,
})
const tf = new Intl.DateTimeFormat('en-IN', { timeZone: OFFICE_TZ, hour: '2-digit', minute: '2-digit', hour12: true })
const df = new Intl.DateTimeFormat('en-IN', { timeZone: OFFICE_TZ, day: '2-digit', month: 'short', year: 'numeric' })
const ymd = new Intl.DateTimeFormat('en-CA', { timeZone: OFFICE_TZ, year: 'numeric', month: '2-digit', day: '2-digit' })

export function fmtDateTime(iso: string | null | undefined): string {
  return iso ? dtf.format(new Date(iso)) : '—'
}
export function fmtTime(iso: string | null | undefined): string {
  return iso ? tf.format(new Date(iso)) : '—'
}
/** "2026-11-03" → "03 Nov 2026" */
export function fmtDate(date: string | null | undefined): string {
  if (!date) return '—'
  const t = Date.parse(`${date}T00:00:00Z`)
  return Number.isNaN(t) ? date : df.format(new Date(t + 12 * 3600 * 1000))
}

/** today's calendar date in the office time zone, "YYYY-MM-DD" */
export function todayIST(nowMs: number = Date.now()): string {
  return ymd.format(new Date(nowMs))
}

export function addDays(date: string, n: number): string {
  const t = new Date(`${date}T00:00:00Z`)
  t.setUTCDate(t.getUTCDate() + n)
  return t.toISOString().slice(0, 10)
}

export function fmtDuration(min: number): string {
  if (min < 60) return `${min} min`
  const h = Math.floor(min / 60)
  const m = min % 60
  return m ? `${h} h ${m} min` : `${h} h`
}
