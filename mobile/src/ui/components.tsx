import { useState } from 'react'
import type { ReactNode } from 'react'
import {
  ActivityIndicator, Modal, Pressable, RefreshControl, ScrollView, StyleSheet, Text, TextInput, View,
} from 'react-native'
import type { TextInputProps } from 'react-native'
import type { Visit, VisitStatus } from '../api'
import { fmtDate, fmtTime } from '../api'
import { c } from './theme'
import { formatMMSS, useSecondsLeft } from './countdown'

export function Button({ title, onPress, variant = 'primary', disabled, loading }: {
  title: string; onPress: () => void; variant?: 'primary' | 'secondary' | 'danger'; disabled?: boolean; loading?: boolean
}) {
  const off = disabled || loading
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={title} onPress={off ? undefined : onPress} disabled={off}
      style={[s.btn, variant === 'primary' && { backgroundColor: c.primary }, variant === 'danger' && { backgroundColor: c.danger },
        variant === 'secondary' && { backgroundColor: '#fff', borderWidth: 1, borderColor: c.border }, off && { opacity: 0.5 }]}>
      {loading ? <ActivityIndicator color={variant === 'secondary' ? c.text : '#fff'} /> :
        <Text style={[s.btnText, variant === 'secondary' && { color: c.text }]}>{title}</Text>}
    </Pressable>
  )
}

export function Field({ label, error, ...rest }: { label: string; error?: string } & TextInputProps) {
  return (
    <View style={{ marginBottom: 12 }}>
      <Text style={s.label}>{label}</Text>
      <TextInput accessibilityLabel={label} placeholderTextColor="#94a3b8" style={[s.input, !!error && { borderColor: c.danger }]} {...rest} />
      {!!error && <Text accessibilityRole="alert" style={s.error}>{error}</Text>}
    </View>
  )
}

export function Screen({ children, onRefresh, refreshing = false }: { children: ReactNode; onRefresh?: () => void; refreshing?: boolean }) {
  return (
    <ScrollView style={{ flex: 1, backgroundColor: c.bg }} contentContainerStyle={{ padding: 16, paddingBottom: 40 }}
      keyboardShouldPersistTaps="handled"
      refreshControl={onRefresh ? <RefreshControl refreshing={refreshing} onRefresh={onRefresh} /> : undefined}>
      {children}
    </ScrollView>
  )
}

export const H1 = ({ children }: { children: ReactNode }) => <Text style={s.h1}>{children}</Text>
export const H2 = ({ children }: { children: ReactNode }) => <Text style={s.h2}>{children}</Text>
export const Muted = ({ children }: { children: ReactNode }) => <Text style={{ color: c.muted, fontSize: 14 }}>{children}</Text>
export const Card = ({ children }: { children: ReactNode }) => <View style={s.card}>{children}</View>
export const ErrorBox = ({ message, onRetry }: { message: string; onRetry?: () => void }) => (
  <View accessibilityRole="alert" style={[s.card, { borderColor: c.danger }]}>
    <Text style={{ color: c.danger, marginBottom: 8 }}>{message}</Text>
    {onRetry && <Button title="Try again" variant="secondary" onPress={onRetry} />}
  </View>
)
export const Empty = ({ text }: { text: string }) => <Card><Muted>{text}</Muted></Card>

const BADGE: Record<VisitStatus, [string, string]> = {
  SCHEDULED: ['Scheduled', '#1e40af'], PENDING_APPROVAL: ['Awaiting approval', '#b45309'], APPROVED: ['Approved', '#047857'],
  CHECKED_IN: ['Checked in', '#166534'], MEETING_OVER: ['Meeting over', '#6d28d9'], CHECKED_OUT: ['Checked out', '#334155'],
  REJECTED: ['Rejected', '#b91c1c'], EXPIRED: ['Expired', '#c2410c'], CANCELLED: ['Cancelled', '#475569'],
  NO_SHOW: ['No show', '#475569'], DENIED: ['Denied', '#b91c1c'],
}
export const statusLabel = (st: VisitStatus) => BADGE[st]?.[0] ?? st
export function StatusBadge({ status }: { status: VisitStatus }) {
  const [label, color] = BADGE[status] ?? [status, '#334155']
  return <Text style={[s.badge, { color, borderColor: color }]}>{label}</Text>
}

export function Countdown({ deadline }: { deadline: string | null }) {
  const left = useSecondsLeft(deadline)
  if (left === null) return null
  return (
    <Text accessibilityRole="timer" style={{ fontVariant: ['tabular-nums'], fontWeight: '700', fontSize: 18, color: left === 0 ? c.warn : left <= 300 ? c.danger : c.text }}>
      {left === 0 ? 'Expired' : formatMMSS(left)}
    </Text>
  )
}

export function VisitCard({ visit, onPress }: { visit: Visit; onPress: () => void }) {
  const when = visit.type === 'UNPLANNED' ? 'Walk-in' : `${fmtDate(visit.visitDate)}${visit.expectedArrival ? ' · ' + visit.expectedArrival : ''}`
  return (
    <Pressable accessibilityRole="button" accessibilityLabel={`Visit ${visit.code} ${visit.visitor.name}`} onPress={onPress} style={s.card}>
      <View style={{ flexDirection: 'row', justifyContent: 'space-between', alignItems: 'center' }}>
        <Text style={s.h2}>{visit.visitor.name}</Text>
        <StatusBadge status={visit.status} />
      </View>
      <Muted>{visit.code} · {when}</Muted>
      <Muted>{visit.purpose}</Muted>
      {visit.status === 'PENDING_APPROVAL' && (
        <View style={{ flexDirection: 'row', gap: 8, marginTop: 6, alignItems: 'center' }}><Muted>Answer within</Muted><Countdown deadline={visit.approvalDeadline} /></View>
      )}
      {visit.checkedInAt && <Muted>In at {fmtTime(visit.checkedInAt)}</Muted>}
    </Pressable>
  )
}

/** Confirmation sheet with an optional reason box. */
export function ConfirmModal({ open, title, body, confirmLabel, danger, reason, busy, onCancel, onConfirm }: {
  open: boolean; title: string; body?: string; confirmLabel: string; danger?: boolean
  reason?: { label: string; required: boolean }; busy?: boolean; onCancel: () => void; onConfirm: (reason: string) => void
}) {
  const [text, setText] = useState('')
  const [err, setErr] = useState('')
  const go = () => {
    const t = text.trim()
    if (reason?.required && Array.from(t).length < 5) return setErr('Please give at least 5 characters')
    setErr('')
    onConfirm(t)
  }
  return (
    <Modal visible={open} transparent animationType="fade" onRequestClose={onCancel}>
      <View style={s.scrim}>
        <View style={s.sheet}>
          <Text style={s.h2}>{title}</Text>
          {!!body && <Muted>{body}</Muted>}
          {reason && <Field label={reason.label} value={text} onChangeText={setText} error={err} multiline />}
          <View style={{ flexDirection: 'row', gap: 8, marginTop: 12 }}>
            <View style={{ flex: 1 }}><Button title="Back" variant="secondary" onPress={onCancel} /></View>
            <View style={{ flex: 1 }}><Button title={confirmLabel} variant={danger ? 'danger' : 'primary'} loading={busy} onPress={go} /></View>
          </View>
        </View>
      </View>
    </Modal>
  )
}

const s = StyleSheet.create({
  btn: { minHeight: 48, borderRadius: 8, alignItems: 'center', justifyContent: 'center', paddingHorizontal: 16 },
  btnText: { color: '#fff', fontWeight: '600', fontSize: 16 },
  label: { fontSize: 14, fontWeight: '600', color: c.text, marginBottom: 4 },
  input: { minHeight: 48, borderWidth: 1, borderColor: c.border, borderRadius: 8, paddingHorizontal: 12, fontSize: 16, backgroundColor: '#fff', color: c.text },
  error: { color: c.danger, fontSize: 13, marginTop: 4 },
  h1: { fontSize: 24, fontWeight: '700', color: c.text, marginBottom: 12 },
  h2: { fontSize: 17, fontWeight: '700', color: c.text, marginBottom: 4 },
  card: { backgroundColor: c.card, borderWidth: 1, borderColor: c.border, borderRadius: 10, padding: 14, marginBottom: 10, gap: 2 },
  badge: { fontSize: 12, fontWeight: '700', borderWidth: 1, borderRadius: 99, paddingHorizontal: 8, paddingVertical: 2, overflow: 'hidden' },
  scrim: { flex: 1, backgroundColor: 'rgba(15,23,42,0.5)', justifyContent: 'center', padding: 20 },
  sheet: { backgroundColor: '#fff', borderRadius: 12, padding: 16 },
})
