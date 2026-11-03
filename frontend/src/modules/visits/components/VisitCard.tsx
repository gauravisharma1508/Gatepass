import { Link } from 'react-router-dom'
import type { Visit } from '../../../shared/api'
import { fmtDate, fmtDuration, fmtTime } from '../../../shared/api'
import { useAuth } from '../../../shared/auth'
import { Countdown, StatusBadge, cx } from '../../../shared/ui'
import { VisitActions } from './VisitActions'

interface Props {
  visit: Visit
  showHost?: boolean
  showVisitDate?: boolean
}

export function VisitCard({ visit: v, showHost = true, showVisitDate = false }: Props) {
  const { user } = useAuth()
  const waitingForHost = user?.role === 'SECURITY' && v.status === 'CHECKED_IN'
  return (
    <article aria-label={`Visit ${v.code}`} className={cx('rounded-lg border bg-white p-4 shadow-sm', v.isOverstay ? 'border-red-500 ring-1 ring-red-300' : 'border-slate-200')}>
      <header className="flex flex-wrap items-start justify-between gap-2">
        <div className="min-w-0">
          <h3 className="truncate text-base font-semibold text-slate-900">{v.visitor.name}</h3>
          <p className="truncate text-sm text-slate-600">{v.visitor.company ?? 'No company'} · {v.visitor.mobile}</p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {v.isOverstay && <span className="rounded-full bg-red-700 px-2.5 py-0.5 text-xs font-semibold text-white">Overstay</span>}
          <StatusBadge status={v.status} />
        </div>
      </header>
      <dl className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1 text-sm">
        <dt className="text-slate-600">Visit ID</dt>
        <dd className="font-mono text-slate-900"><Link className="text-blue-800 underline" to={`/visits/${v.id}`}>{v.code}</Link></dd>
        {showHost && (<><dt className="text-slate-600">Meeting</dt><dd className="text-slate-900">{v.host.name}</dd></>)}
        <dt className="text-slate-600">Purpose</dt><dd className="break-words text-slate-900">{v.purpose}</dd>
        {showVisitDate && (<><dt className="text-slate-600">Date</dt><dd className="text-slate-900">{fmtDate(v.visitDate)}</dd></>)}
        <dt className="text-slate-600">Expected</dt>
        <dd className="text-slate-900">{v.expectedArrival ? `${v.expectedArrival} · ` : ''}{fmtDuration(v.expectedDurationMin)}</dd>
        {v.checkedInAt && (<><dt className="text-slate-600">Checked in</dt><dd className="text-slate-900">{fmtTime(v.checkedInAt)}</dd></>)}
        {v.meetingOverAt && (<><dt className="text-slate-600">Meeting over</dt><dd className="text-slate-900">{fmtTime(v.meetingOverAt)}</dd></>)}
        {v.status === 'PENDING_APPROVAL' && (<><dt className="text-slate-600">Time left to answer</dt><dd><Countdown deadline={v.approvalDeadline} /></dd></>)}
        {v.decisionReason && (<><dt className="text-slate-600">Reason</dt><dd className="text-slate-900">{v.decisionReason}</dd></>)}
      </dl>
      {waitingForHost && <p className="mt-3 rounded bg-slate-100 px-3 py-2 text-sm text-slate-700">Waiting for host to mark Meeting Over</p>}
      <div className="mt-3"><VisitActions visit={v} /></div>
    </article>
  )
}
