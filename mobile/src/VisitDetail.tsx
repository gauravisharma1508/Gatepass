import { useState } from 'react'
import { Text, View } from 'react-native'
import { useQuery } from '@tanstack/react-query'
import { useRouter } from 'expo-router'
import { api, fmtDate, fmtDateTime, fmtDuration } from './api'
import type { Visit, VisitAction } from './api'
import { keys, POLL_MS, useVisitAction } from './hooks'
import { Button, Card, ConfirmModal, Countdown, ErrorBox, H1, H2, Muted, Screen, StatusBadge } from './ui/components'
import { useSecondsLeft } from './ui/countdown'

const ACTION_LABEL: Record<string, string> = {
  VISIT_CREATED: 'Visit created', VISIT_UPDATED: 'Visit details edited', PHOTO_CAPTURED: 'Photo captured', APPROVED: 'Approved by host',
  REJECTED: 'Rejected by host', EXPIRED: 'Request expired (no answer in time)', CANCELLED: 'Cancelled', NO_SHOW: 'Marked as no-show',
  DENIED: 'Entry denied by security', CHECKED_IN: 'Checked in', MEETING_OVER: 'Meeting marked over by host', CHECKED_OUT: 'Checked out',
  FORCE_CLOSED: 'Force-closed by admin',
}
export const actionLabel = (a: string) => ACTION_LABEL[a] ?? a.toLowerCase().replace(/_/g, ' ').replace(/^./, (x) => x.toUpperCase())

type Dialog = 'reject' | 'cancel' | 'meeting-over'
const DIALOGS: Record<Dialog, { title: string; confirm: string; danger: boolean; body?: string; reason?: { label: string; required: boolean } }> = {
  reject: { title: 'Reject this visitor?', confirm: 'Reject', danger: true, reason: { label: 'Reason (optional)', required: false } },
  cancel: { title: 'Cancel this visit?', confirm: 'Cancel visit', danger: true, reason: { label: 'Reason (optional)', required: false } },
  'meeting-over': { title: 'Mark the meeting as over?', confirm: 'Meeting over', danger: false, body: 'Security will be told the visitor is ready to check out.' },
}

/** Buttons come only from the server's allowedActions; the app never re-implements the permission rules. */
export function VisitActions({ visit }: { visit: Visit }) {
  const router = useRouter()
  const m = useVisitAction()
  const [open, setOpen] = useState<Dialog | null>(null)
  const left = useSecondsLeft(visit.status === 'PENDING_APPROVAL' ? visit.approvalDeadline : null)
  const closed = left === 0 // the window is over: stop offering answers even before the next refetch
  const has = (a: VisitAction) => visit.allowedActions.includes(a)
  const run = (path: string, body?: unknown) => m.mutate({ id: visit.id, path, body }, { onSettled: () => setOpen(null) })
  const def = open ? DIALOGS[open] : null
  return (
    <View style={{ gap: 8 }}>
      {has('APPROVE') && <Button title="Accept" disabled={closed} loading={m.isPending && m.variables?.path === 'approve'} onPress={() => run('approve')} />}
      {has('REJECT') && <Button title="Reject" variant="secondary" disabled={closed} onPress={() => setOpen('reject')} />}
      {has('MEETING_OVER') && <Button title="Meeting over" onPress={() => setOpen('meeting-over')} />}
      {has('EDIT') && <Button title="Edit" variant="secondary" onPress={() => router.push({ pathname: '/visit/form', params: { id: visit.id } })} />}
      {has('CANCEL') && <Button title="Cancel visit" variant="secondary" onPress={() => setOpen('cancel')} />}
      <ConfirmModal open={!!def} title={def?.title ?? ''} body={def?.body ?? `${visit.visitor.name} · ${visit.code}`} confirmLabel={def?.confirm ?? ''}
        danger={def?.danger} reason={def?.reason} busy={m.isPending}
        onCancel={() => setOpen(null)} onConfirm={(reason) => open && run(open, DIALOGS[open].reason ? { reason } : undefined)} />
    </View>
  )
}

export function VisitDetail({ id }: { id: string }) {
  const q = useQuery({ queryKey: keys.visit(id), queryFn: () => api.get<Visit>(`/visits/${id}`), refetchInterval: POLL_MS, retry: false })
  if (q.isError) return <Screen><ErrorBox message={(q.error as Error).message} onRetry={() => void q.refetch()} /></Screen>
  if (!q.data) return <Screen><Muted>Loading…</Muted></Screen>
  const v = q.data
  const pending = v.status === 'PENDING_APPROVAL'
  return (
    <Screen onRefresh={() => void q.refetch()} refreshing={q.isRefetching}>
      <H1>{v.visitor.name}</H1>
      <View style={{ flexDirection: 'row', gap: 8, alignItems: 'center', marginBottom: 12 }}><StatusBadge status={v.status} /><Muted>{v.code}</Muted></View>
      {pending && (
        <Card>
          <H2>Walk-in waiting for you</H2>
          <Muted>Answer before the time runs out, or the request expires.</Muted>
          <Countdown deadline={v.approvalDeadline} />
        </Card>
      )}
      <VisitActions visit={v} />
      <H2>{' '}</H2>
      <Card>
        <Row k="Purpose" v={v.purpose} />
        <Row k="Mobile" v={v.visitor.mobile} />
        {v.visitor.email && <Row k="Email" v={v.visitor.email} />}
        {v.visitor.company && <Row k="Company" v={v.visitor.company} />}
        <Row k="Date" v={fmtDate(v.visitDate)} />
        {v.expectedArrival && <Row k="Expected" v={v.expectedArrival} />}
        <Row k="Duration" v={fmtDuration(v.expectedDurationMin)} />
        {v.checkedInAt && <Row k="Checked in" v={fmtDateTime(v.checkedInAt)} />}
        {v.meetingOverAt && <Row k="Meeting over" v={fmtDateTime(v.meetingOverAt)} />}
        {v.checkedOutAt && <Row k="Checked out" v={fmtDateTime(v.checkedOutAt)} />}
        {v.decisionReason && <Row k="Reason" v={v.decisionReason} />}
        {v.isOverstay && <Text style={{ color: '#b45309', fontWeight: '700' }}>Overstay</Text>}
      </Card>
      <H2>Timeline</H2>
      <Card>
        {(v.timeline ?? []).length === 0 && <Muted>No events yet.</Muted>}
        {(v.timeline ?? []).map((t, i) => (
          <View key={i} style={{ marginBottom: 6 }}>
            <Text style={{ fontWeight: '600' }}>{actionLabel(t.action)}</Text>
            <Muted>{fmtDateTime(t.createdAt)} · {t.actorRole.toLowerCase()}{t.reason ? ` · ${t.reason}` : ''}</Muted>
          </View>
        ))}
      </Card>
    </Screen>
  )
}

const Row = ({ k, v }: { k: string; v: string }) => (
  <View style={{ flexDirection: 'row', gap: 8, marginBottom: 2 }}><Text style={{ width: 96, color: '#475569' }}>{k}</Text><Text style={{ flex: 1 }}>{v}</Text></View>
)

