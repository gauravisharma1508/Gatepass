import type { VisitStatus } from '../api/types'
import { cx } from './cx'

const MAP: Record<VisitStatus, { label: string; cls: string }> = {
  SCHEDULED: { label: 'Scheduled', cls: 'bg-blue-100 text-blue-900 border-blue-300' },
  PENDING_APPROVAL: { label: 'Awaiting approval', cls: 'bg-amber-100 text-amber-900 border-amber-400' },
  APPROVED: { label: 'Approved', cls: 'bg-emerald-100 text-emerald-900 border-emerald-400' },
  CHECKED_IN: { label: 'Checked in', cls: 'bg-green-100 text-green-900 border-green-400' },
  MEETING_OVER: { label: 'Meeting over', cls: 'bg-violet-100 text-violet-900 border-violet-400' },
  CHECKED_OUT: { label: 'Checked out', cls: 'bg-slate-100 text-slate-800 border-slate-400' },
  REJECTED: { label: 'Rejected', cls: 'bg-red-100 text-red-900 border-red-400' },
  EXPIRED: { label: 'Expired', cls: 'bg-orange-100 text-orange-900 border-orange-400' },
  CANCELLED: { label: 'Cancelled', cls: 'bg-slate-100 text-slate-700 border-slate-400' },
  NO_SHOW: { label: 'No show', cls: 'bg-slate-100 text-slate-700 border-slate-400' },
  DENIED: { label: 'Denied', cls: 'bg-red-100 text-red-900 border-red-400' },
}

export function statusLabel(s: VisitStatus): string {
  return MAP[s]?.label ?? s
}

/** Colour is never the only signal: the status name is always printed. */
export function StatusBadge({ status, className }: { status: VisitStatus; className?: string }) {
  const m = MAP[status] ?? { label: status, cls: 'bg-slate-100 text-slate-800 border-slate-400' }
  return (
    <span className={cx('inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-semibold whitespace-nowrap', m.cls, className)}>
      {m.label}
    </span>
  )
}
