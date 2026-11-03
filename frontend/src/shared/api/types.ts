// Hand-written DTO types matching the Go backend (backend/internal/modules/*).
export type Role = 'EMPLOYEE' | 'SECURITY' | 'ADMIN'

export type VisitStatus =
  | 'SCHEDULED' | 'PENDING_APPROVAL' | 'APPROVED' | 'CHECKED_IN' | 'MEETING_OVER'
  | 'CHECKED_OUT' | 'REJECTED' | 'EXPIRED' | 'CANCELLED' | 'NO_SHOW' | 'DENIED'

export const ALL_STATUSES: VisitStatus[] = [
  'SCHEDULED', 'PENDING_APPROVAL', 'APPROVED', 'CHECKED_IN', 'MEETING_OVER',
  'CHECKED_OUT', 'REJECTED', 'EXPIRED', 'CANCELLED', 'NO_SHOW', 'DENIED',
]

export type VisitType = 'PLANNED' | 'UNPLANNED'

export type VisitAction =
  | 'APPROVE' | 'REJECT' | 'EXPIRE' | 'CANCEL' | 'NO_SHOW' | 'UPLOAD_PHOTOS' | 'CHECK_IN'
  | 'DENY' | 'MEETING_OVER' | 'CHECK_OUT' | 'FORCE_CLOSE' | 'EDIT'

export interface SessionUser {
  id: string
  name: string
  email: string
  role: Role
  employeeCode: string
  department: string | null
  mustChangePassword: boolean
}

export interface TimelineItem {
  action: string
  actorRole: string
  fromStatus: string | null
  toStatus: string | null
  reason: string | null
  createdAt: string
}

export interface Visit {
  id: string
  code: string
  type: VisitType
  status: VisitStatus
  visitor: { name: string; mobile: string; email: string | null; company: string | null }
  host: { id: string; name: string; department: string | null }
  purpose: string
  visitDate: string
  expectedArrival: string | null
  expectedDurationMin: number
  approvalDeadline: string | null
  approvalSecondsLeft: number | null
  decidedAt: string | null
  decisionReason: string | null
  checkedInAt: string | null
  meetingOverAt: string | null
  checkedOutAt: string | null
  closedReason: string | null
  isOverstay: boolean
  hasVisitorPhoto: boolean
  hasIdPhoto: boolean
  idType: string | null
  allowedActions: VisitAction[]
  version: number
  createdAt: string
  timeline?: TimelineItem[]
}

export interface Page<T> {
  items: T[]
  page: number
  pageSize: number
  total: number
}

export interface SecurityDashboard {
  expectedToday: Visit[]
  pendingApprovals: Visit[]
  onPremises: Visit[]
  readyForCheckout: Visit[]
  overstays: Visit[]
  counts: Record<string, number>
}

export interface EmployeeDashboard {
  pendingApprovals: Visit[]
  todaysVisits: Visit[]
  upcoming: Visit[]
  awaitingMeetingOver: Visit[]
}

export interface AppNotification {
  id: string
  visitId: string | null
  type: string
  title: string
  body: string
  readAt: string | null
  createdAt: string
}

export interface NotificationList extends Page<AppNotification> {
  unreadCount: number
}

export interface UserRow {
  id: string
  employeeCode: string
  name: string
  email: string
  phone: string | null
  department: string | null
  role: Role
  isActive: boolean
  mustChangePassword: boolean
  createdAt: string
}

export interface HostSummary {
  id: string
  name: string
  employeeCode: string
  department: string | null
}

export interface AuditEntry {
  id: string
  action: string
  actorId: string | null
  actorRole: string
  fromStatus: string | null
  toStatus: string | null
  reason: string | null
  metadata: Record<string, unknown> | null
  createdAt: string
}

export interface Settings {
  approvalWindowMin: number
  approvalReminderMin: number
  maxAdvanceDays: number
  photoRetentionDays: number
  recordRetentionDays: number
}

export interface ImportResult {
  row: number
  email: string
  status: 'created' | 'failed'
  error?: string
  tempPassword?: string
}
