import { useQuery } from '@tanstack/react-query'
import { api, fmtDateTime, keys } from '../../../shared/api'
import type { AuditEntry } from '../../../shared/api'
import { Card, EmptyState, ErrorState, Spinner } from '../../../shared/ui'

function summarize(m: Record<string, unknown> | null): string {
  if (!m) return ''
  return Object.entries(m).map(([k, v]) => `${k}: ${typeof v === 'object' ? JSON.stringify(v) : String(v)}`).join(' · ')
}

/** Admin-only full audit log of one visit (GET /visits/:id/audit). */
export function AuditTrail({ visitId }: { visitId: string }) {
  const q = useQuery({ queryKey: keys.audit(visitId), queryFn: () => api.get<{ items: AuditEntry[] }>(`/visits/${visitId}/audit`) })
  return (
    <Card title="Audit trail">
      {q.isPending && <Spinner />}
      {q.isError && <ErrorState error={q.error} onRetry={() => void q.refetch()} />}
      {q.data && (q.data.items.length === 0 ? <EmptyState title="No audit entries" /> : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[34rem] text-left text-sm">
            <caption className="sr-only">Audit trail</caption>
            <thead><tr className="border-b border-slate-200 text-slate-600">
              <th scope="col" className="py-2 pr-3 font-medium">When (IST)</th><th scope="col" className="py-2 pr-3 font-medium">Who</th>
              <th scope="col" className="py-2 pr-3 font-medium">Action</th><th scope="col" className="py-2 pr-3 font-medium">Status change</th><th scope="col" className="py-2 font-medium">Details</th>
            </tr></thead>
            <tbody>
              {q.data.items.map((e) => (
                <tr key={e.id} className="border-b border-slate-100 align-top">
                  <td className="py-2 pr-3 whitespace-nowrap">{fmtDateTime(e.createdAt)}</td>
                  <td className="py-2 pr-3">{e.actorRole.toLowerCase()}</td>
                  <td className="py-2 pr-3 font-mono text-xs">{e.action}</td>
                  <td className="py-2 pr-3">{e.fromStatus || e.toStatus ? `${e.fromStatus ?? '—'} → ${e.toStatus ?? '—'}` : '—'}</td>
                  <td className="py-2 break-words">{[e.reason && `“${e.reason}”`, summarize(e.metadata)].filter(Boolean).join(' ')}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}
    </Card>
  )
}
