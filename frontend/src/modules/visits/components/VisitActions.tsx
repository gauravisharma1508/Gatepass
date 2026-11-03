import { useState } from 'react'
import { Link } from 'react-router-dom'
import type { Visit, VisitAction } from '../../../shared/api'
import { Button, ConfirmDialog, useSecondsLeft } from '../../../shared/ui'
import { useVisitMutation } from '../hooks/useVisitMutation'

type DialogKey = 'reject' | 'cancel' | 'deny' | 'meeting-over' | 'check-out' | 'force-close'

interface DialogDef {
  title: string
  confirm: string
  danger: boolean
  reason?: { label: string; required: boolean }
  body?: string
  success: string
}

const DIALOGS: Record<DialogKey, DialogDef> = {
  reject: { title: 'Reject this visitor?', confirm: 'Reject', danger: true, reason: { label: 'Reason (optional)', required: false }, success: 'Visit rejected' },
  cancel: { title: 'Cancel this visit?', confirm: 'Cancel visit', danger: true, reason: { label: 'Reason (optional)', required: false }, success: 'Visit cancelled' },
  deny: { title: 'Deny entry?', confirm: 'Deny entry', danger: true, reason: { label: 'Reason for denying entry', required: true }, success: 'Entry denied' },
  'meeting-over': { title: 'Mark the meeting as over?', confirm: 'Meeting over', danger: false, body: 'Security will be told the visitor is ready to check out.', success: 'Meeting marked over' },
  'check-out': { title: 'Check this visitor out?', confirm: 'Check out', danger: false, body: 'The visitor is leaving the premises.', success: 'Visitor checked out' },
  'force-close': { title: 'Force-close this visit?', confirm: 'Force-close', danger: true, reason: { label: 'Reason (recorded in the audit log)', required: true }, body: 'Use this only when the host cannot mark Meeting Over.', success: 'Visit force-closed' },
}

const DIALOG_FOR: Partial<Record<VisitAction, DialogKey>> = {
  REJECT: 'reject', CANCEL: 'cancel', DENY: 'deny', MEETING_OVER: 'meeting-over', CHECK_OUT: 'check-out', FORCE_CLOSE: 'force-close',
}

const LABEL: Partial<Record<VisitAction, string>> = {
  APPROVE: 'Accept', REJECT: 'Reject', CANCEL: 'Cancel visit', DENY: 'Deny entry', MEETING_OVER: 'Meeting over',
  CHECK_OUT: 'Check out', FORCE_CLOSE: 'Force-close', EDIT: 'Edit',
}

const ORDER: VisitAction[] = ['APPROVE', 'CHECK_IN', 'UPLOAD_PHOTOS', 'MEETING_OVER', 'CHECK_OUT', 'EDIT', 'REJECT', 'DENY', 'CANCEL', 'FORCE_CLOSE']

interface Props {
  visit: Visit
  size?: 'sm' | 'md'
  /** actions to leave out (the check-in screen renders its own check-in / photo controls) */
  exclude?: VisitAction[]
}

/**
 * Buttons come ONLY from the server's `allowedActions` — the screens never repeat the permission
 * rules (spec §8.3). Anything not listed there is simply not offered.
 */
export function VisitActions({ visit, size = 'sm', exclude = [] }: Props) {
  const m = useVisitMutation()
  // once the approval window is over the answer buttons stop working, even before the server refetch
  const left = useSecondsLeft(visit.status === 'PENDING_APPROVAL' ? visit.approvalDeadline : null)
  const windowClosed = left === 0
  const [open, setOpen] = useState<DialogKey | null>(null)
  const allowed = ORDER.filter((a) => visit.allowedActions.includes(a) && !exclude.includes(a))
  // check-in and photo capture share one screen: offer a single entry
  const entries = allowed.filter((a) => !(a === 'UPLOAD_PHOTOS' && allowed.includes('CHECK_IN')))
  if (entries.length === 0) return null

  const run = (path: string, success: string, body?: unknown) =>
    m.mutate({ id: visit.id, path, body, success }, { onSettled: () => setOpen(null) })
  const def = open ? DIALOGS[open] : null

  return (
    <>
      <div className="flex flex-wrap gap-2" data-testid="visit-actions">
        {entries.map((a) => {
          if (a === 'EDIT')
            return <Link key={a} to={`/employee/visits/${visit.id}/edit`} className={linkCls(size)}>Edit</Link>
          if (a === 'CHECK_IN' || a === 'UPLOAD_PHOTOS')
            // both open the check-in screen; without photos it first captures them, so the guard always sees "Check in"
            return <Link key={a} to={`/security/visits/${visit.id}/check-in`} className={linkCls(size, true)}>Check in</Link>
          if (a === 'APPROVE')
            return <Button key={a} size={size} disabled={windowClosed} loading={m.isPending && m.variables?.path === 'approve'} onClick={() => run('approve', 'Visitor accepted')}>Accept</Button>
          const key = DIALOG_FOR[a]
          return (
            <Button key={a} size={size} disabled={windowClosed && a === 'REJECT'} variant={DIALOGS[key!].danger ? 'secondary' : 'primary'} onClick={() => setOpen(key!)}>
              {LABEL[a]}
            </Button>
          )
        })}
      </div>
      <ConfirmDialog open={!!def} title={def?.title ?? ''} confirmLabel={def?.confirm} danger={def?.danger} reason={def?.reason}
        busy={m.isPending} onCancel={() => setOpen(null)} onConfirm={(reason) => open && run(open, DIALOGS[open].success, DIALOGS[open].reason ? { reason } : undefined)}>
        {def?.body ?? `${visit.visitor.name} · ${visit.code}`}
      </ConfirmDialog>
    </>
  )
}

function linkCls(size: 'sm' | 'md', primary = false) {
  return [
    'inline-flex items-center justify-center rounded-md font-medium focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-700',
    size === 'sm' ? 'min-h-9 px-3 text-sm' : 'min-h-11 px-4 text-sm',
    primary ? 'bg-blue-700 text-white hover:bg-blue-800' : 'border border-slate-300 bg-white text-slate-800 hover:bg-slate-50',
  ].join(' ')
}
