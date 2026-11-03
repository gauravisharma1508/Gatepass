import { useEffect, useId, useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { api } from '../../../shared/api'
import type { HostSummary } from '../../../shared/api'
import { Button } from '../../../shared/ui'

function useDebounced<T>(value: T, ms: number): T {
  const [v, setV] = useState(value)
  useEffect(() => {
    const t = setTimeout(() => setV(value), ms)
    return () => clearTimeout(t)
  }, [value, ms])
  return v
}

interface Props {
  value: HostSummary | null
  onChange: (host: HostSummary | null) => void
  error?: string
}

/**
 * "Whom to meet": search active employees (≥ 2 characters, 300 ms debounce). Free text is not
 * accepted — a result must be chosen, because the server needs the employee's id.
 */
export function HostPicker({ value, onChange, error }: Props) {
  const id = useId()
  const [text, setText] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(0)
  const term = useDebounced(text.trim(), 300)
  const enabled = !value && term.length >= 2

  const q = useQuery({
    queryKey: ['employee-search', term],
    enabled,
    queryFn: () => api.get<{ items: HostSummary[] }>('/employees/search', { q: term }),
  })
  const items = q.data?.items ?? []

  useEffect(() => setActive(0), [term])

  function pick(h: HostSummary) {
    onChange(h)
    setText('')
    setOpen(false)
  }

  if (value)
    return (
      <div>
        <span className="mb-1 block text-sm font-medium text-slate-800">Whom to meet</span>
        <div className="flex items-center justify-between gap-3 rounded-md border border-slate-300 bg-slate-50 px-3 py-2">
          <span className="text-sm text-slate-900" data-testid="selected-host">{value.name}{value.department ? ` · ${value.department}` : ''} <span className="text-slate-600">({value.employeeCode})</span></span>
          <Button size="sm" variant="secondary" onClick={() => onChange(null)}>Change</Button>
        </div>
      </div>
    )

  function onKey(e: React.KeyboardEvent) {
    if (e.key === 'ArrowDown') { e.preventDefault(); setOpen(true); setActive((a) => Math.min(a + 1, items.length - 1)) }
    else if (e.key === 'ArrowUp') { e.preventDefault(); setActive((a) => Math.max(a - 1, 0)) }
    else if (e.key === 'Enter' && open && items[active]) { e.preventDefault(); pick(items[active]) }
    else if (e.key === 'Escape') setOpen(false)
  }

  return (
    <div className="relative">
      <label htmlFor={id} className="mb-1 block text-sm font-medium text-slate-800">Whom to meet</label>
      <input id={id} role="combobox" aria-expanded={open && enabled} aria-controls={`${id}-list`} aria-autocomplete="list" aria-invalid={!!error || undefined}
        aria-describedby={error ? `${id}-err` : `${id}-hint`} aria-activedescendant={open && items[active] ? `${id}-opt-${active}` : undefined}
        autoComplete="off" placeholder="Type the employee's name, email or code" value={text}
        className={`block w-full rounded-md border bg-white px-3 py-2 text-base text-slate-900 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-700 sm:text-sm ${error ? 'border-red-600' : 'border-slate-300'}`}
        onChange={(e) => { setText(e.target.value); setOpen(true) }} onFocus={() => setOpen(true)} onKeyDown={onKey} onBlur={() => setTimeout(() => setOpen(false), 150)} />
      {!error && <p id={`${id}-hint`} className="mt-1 text-xs text-slate-600">Type at least 2 characters, then choose the employee from the list.</p>}
      {error && <p id={`${id}-err`} role="alert" className="mt-1 text-sm text-red-700">{error}</p>}
      {open && enabled && (
        <ul id={`${id}-list`} role="listbox" aria-label="Matching employees" className="absolute z-20 mt-1 max-h-60 w-full overflow-auto rounded-md border border-slate-200 bg-white shadow-lg">
          {q.isPending && <li role="presentation" className="px-3 py-2 text-sm text-slate-600">Searching…</li>}
          {q.data && items.length === 0 && <li role="presentation" className="px-3 py-2 text-sm text-slate-600">No employee found</li>}
          {items.map((h, i) => (
            <li key={h.id} id={`${id}-opt-${i}`} role="option" aria-selected={i === active} onMouseDown={(e) => { e.preventDefault(); pick(h) }}
              className={`cursor-pointer px-3 py-2 text-sm ${i === active ? 'bg-blue-50' : ''}`}>
              <span className="font-medium text-slate-900">{h.name}</span> <span className="text-slate-600">· {h.department ?? 'No department'} · {h.employeeCode}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
