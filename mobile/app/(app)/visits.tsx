import { useState } from 'react'
import { Pressable, Text, View } from 'react-native'
import { useQuery } from '@tanstack/react-query'
import { useRouter } from 'expo-router'
import { ALL_STATUSES, api } from '../../src/api'
import type { Page, Visit, VisitStatus } from '../../src/api'
import { keys, POLL_MS } from '../../src/hooks'
import { Button, Empty, ErrorBox, Field, Screen, statusLabel, VisitCard } from '../../src/ui/components'
import { c } from '../../src/ui/theme'

export default function MyVisits() {
  const router = useRouter()
  const [status, setStatus] = useState<VisitStatus | ''>('')
  const [text, setText] = useState('')
  const [from, setFrom] = useState('')
  const [to, setTo] = useState('')
  const [page, setPage] = useState(1)
  const params = { mine: true, status: status || undefined, q: text.trim() || undefined, from: from || undefined, to: to || undefined, page, pageSize: 10 }
  const q = useQuery({ queryKey: keys.visits(params), queryFn: () => api.get<Page<Visit>>('/visits', params), placeholderData: (p) => p, refetchInterval: POLL_MS })
  const reset = <T,>(set: (v: T) => void) => (v: T) => { set(v); setPage(1) }
  const pages = q.data ? Math.max(1, Math.ceil(q.data.total / q.data.pageSize)) : 1

  return (
    <Screen onRefresh={() => void q.refetch()} refreshing={q.isRefetching}>
      <Field label="Search" placeholder="Visitor, mobile or visit ID" value={text} onChangeText={reset(setText)} autoCapitalize="none" />
      <View style={{ flexDirection: 'row', gap: 8 }}>
        <View style={{ flex: 1 }}><Field label="From (YYYY-MM-DD)" value={from} onChangeText={reset(setFrom)} placeholder="2026-11-01" /></View>
        <View style={{ flex: 1 }}><Field label="To (YYYY-MM-DD)" value={to} onChangeText={reset(setTo)} placeholder="2026-11-30" /></View>
      </View>
      <View style={{ flexDirection: 'row', flexWrap: 'wrap', gap: 6, marginBottom: 12 }}>
        {(['', ...ALL_STATUSES] as (VisitStatus | '')[]).map((s) => (
          <Pressable key={s || 'all'} accessibilityRole="button" accessibilityState={{ selected: status === s }} onPress={() => { setStatus(s); setPage(1) }}
            style={{ paddingHorizontal: 10, paddingVertical: 6, borderRadius: 99, borderWidth: 1, borderColor: c.border, backgroundColor: status === s ? c.primary : '#fff' }}>
            <Text style={{ color: status === s ? '#fff' : c.text, fontSize: 13 }}>{s ? statusLabel(s) : 'All'}</Text>
          </Pressable>
        ))}
      </View>
      {q.isError && <ErrorBox message={(q.error as Error).message} onRetry={() => void q.refetch()} />}
      {q.data && (q.data.items.length === 0 ? <Empty text="No visits found." /> : q.data.items.map((v) => <VisitCard key={v.id} visit={v} onPress={() => router.push(`/visit/${v.id}`)} />))}
      {q.data && pages > 1 && (
        <View style={{ flexDirection: 'row', gap: 8, alignItems: 'center' }}>
          <View style={{ flex: 1 }}><Button title="Previous" variant="secondary" disabled={page <= 1} onPress={() => setPage(page - 1)} /></View>
          <Text>{page} / {pages}</Text>
          <View style={{ flex: 1 }}><Button title="Next" variant="secondary" disabled={page >= pages} onPress={() => setPage(page + 1)} /></View>
        </View>
      )}
    </Screen>
  )
}
