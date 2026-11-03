import type { ReactNode } from 'react'
import { Button } from './Button'
import { cx } from './cx'
import { ApiError } from '../api/client'

export function Spinner({ label = 'Loading…' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-3 p-8 text-slate-600">
      <span className="size-5 animate-spin rounded-full border-2 border-slate-400 border-t-transparent" aria-hidden />
      <span>{label}</span>
    </div>
  )
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="rounded-lg border border-dashed border-slate-300 bg-white p-8 text-center">
      <p className="font-medium text-slate-800">{title}</p>
      {children && <p className="mt-1 text-sm text-slate-600">{children}</p>}
    </div>
  )
}

export function ErrorState({ error, onRetry }: { error: unknown; onRetry?: () => void }) {
  const msg = error instanceof ApiError || error instanceof Error ? error.message : 'Something went wrong'
  return (
    <div role="alert" className="rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-900">
      <p className="font-medium">Could not load this page</p>
      <p className="mt-1">{msg}</p>
      {onRetry && <Button className="mt-3" size="sm" variant="secondary" onClick={onRetry}>Try again</Button>}
    </div>
  )
}

export function Card({ title, actions, children, className }: { title?: string; actions?: ReactNode; children: ReactNode; className?: string }) {
  return (
    <section className={cx('rounded-lg border border-slate-200 bg-white shadow-sm', className)}>
      {(title || actions) && (
        <div className="flex items-center justify-between gap-3 border-b border-slate-200 px-4 py-3">
          {title && <h2 className="text-base font-semibold text-slate-900">{title}</h2>}
          {actions}
        </div>
      )}
      <div className="p-4">{children}</div>
    </section>
  )
}

export function PageHeader({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
      <h1 className="text-2xl font-semibold text-slate-900">{title}</h1>
      {children && <div className="flex flex-wrap items-center gap-2">{children}</div>}
    </div>
  )
}

export function Pagination({ page, pageSize, total, onPage }: { page: number; pageSize: number; total: number; onPage: (p: number) => void }) {
  const pages = Math.max(1, Math.ceil(total / pageSize))
  const from = total === 0 ? 0 : (page - 1) * pageSize + 1
  const to = Math.min(total, page * pageSize)
  return (
    <nav aria-label="Pagination" className="mt-4 flex items-center justify-between gap-3 text-sm text-slate-700">
      <span>{from}–{to} of {total}</span>
      <div className="flex items-center gap-2">
        <Button size="sm" variant="secondary" disabled={page <= 1} onClick={() => onPage(page - 1)}>Previous</Button>
        <span aria-current="page">Page {page} of {pages}</span>
        <Button size="sm" variant="secondary" disabled={page >= pages} onClick={() => onPage(page + 1)}>Next</Button>
      </div>
    </nav>
  )
}

export interface TabDef { id: string; label: string; count?: number; alert?: boolean }

/** Accessible tab list (arrow keys move between tabs). */
export function Tabs({ tabs, active, onChange, label }: { tabs: TabDef[]; active: string; onChange: (id: string) => void; label: string }) {
  function onKey(e: React.KeyboardEvent, i: number) {
    const d = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0
    if (!d) return
    e.preventDefault()
    const next = tabs[(i + d + tabs.length) % tabs.length]
    if (next) {
      onChange(next.id)
      document.getElementById(`tab-${next.id}`)?.focus()
    }
  }
  return (
    <div role="tablist" aria-label={label} className="flex gap-1 overflow-x-auto border-b border-slate-300">
      {tabs.map((t, i) => (
        <button key={t.id} id={`tab-${t.id}`} role="tab" type="button" aria-selected={active === t.id} aria-controls={`panel-${t.id}`}
          tabIndex={active === t.id ? 0 : -1} onClick={() => onChange(t.id)} onKeyDown={(e) => onKey(e, i)}
          className={cx('-mb-px flex min-h-11 items-center gap-2 whitespace-nowrap border-b-2 px-4 text-sm font-medium focus-visible:outline-2 focus-visible:outline-blue-700',
            active === t.id ? 'border-blue-700 text-blue-800' : 'border-transparent text-slate-700 hover:text-slate-900')}>
          {t.label}
          {t.count !== undefined && (
            <span className={cx('rounded-full px-2 py-0.5 text-xs font-semibold', t.alert && t.count > 0 ? 'bg-red-700 text-white' : 'bg-slate-200 text-slate-800')}>
              {t.count}
            </span>
          )}
        </button>
      ))}
    </div>
  )
}
