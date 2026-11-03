import type { VisitStatus } from '../../shared/api'

const ACTIONS: Record<string, string> = {
  VISIT_CREATED: 'Visit created',
  VISIT_UPDATED: 'Visit details edited',
  PHOTO_CAPTURED: 'Photo captured',
  APPROVED: 'Approved by host',
  REJECTED: 'Rejected by host',
  EXPIRED: 'Request expired (no answer in time)',
  CANCELLED: 'Cancelled',
  NO_SHOW: 'Marked as no-show',
  DENIED: 'Entry denied by security',
  CHECKED_IN: 'Checked in',
  MEETING_OVER: 'Meeting marked over by host',
  CHECKED_OUT: 'Checked out',
  FORCE_CLOSED: 'Force-closed by admin',
}

export function actionLabel(a: string): string {
  return ACTIONS[a] ?? a.toLowerCase().replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase())
}

export function roleLabel(r: string): string {
  return r === 'SYSTEM' ? 'System' : r.charAt(0) + r.slice(1).toLowerCase()
}

export const FILTER_STATUSES: VisitStatus[] = [
  'SCHEDULED', 'PENDING_APPROVAL', 'APPROVED', 'CHECKED_IN', 'MEETING_OVER', 'CHECKED_OUT', 'REJECTED', 'EXPIRED', 'CANCELLED', 'NO_SHOW', 'DENIED',
]
