import { useEffect, useId, useRef, useState } from 'react'
import type { ReactNode } from 'react'
import { Button } from './Button'
import { TextAreaField } from './fields'
import { reasonOptionalSchema, reasonRequiredSchema } from '../validation'

interface Props {
  open: boolean
  title: string
  children?: ReactNode
  confirmLabel?: string
  danger?: boolean
  busy?: boolean
  /** shows a reason box; `required` reasons need ≥ 5 characters */
  reason?: { label: string; required: boolean }
  onConfirm: (reason: string) => void
  onCancel: () => void
}

/** Accessible modal confirm: focus moves in, Esc cancels, focus returns to the opener. */
export function ConfirmDialog({ open, title, children, confirmLabel = 'Confirm', danger, busy, reason, onConfirm, onCancel }: Props) {
  const titleId = useId()
  const [text, setText] = useState('')
  const [error, setError] = useState<string>()
  const panel = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    setText('')
    setError(undefined)
    const opener = document.activeElement as HTMLElement | null
    const first = panel.current?.querySelector<HTMLElement>('textarea, button')
    first?.focus()
    return () => opener?.focus?.()
  }, [open])

  if (!open) return null

  function submit() {
    if (reason) {
      const r = (reason.required ? reasonRequiredSchema : reasonOptionalSchema).safeParse(text)
      if (!r.success) {
        setError(r.error.issues[0]?.message)
        return
      }
      onConfirm(r.data)
    } else onConfirm('')
  }

  function onKey(e: React.KeyboardEvent) {
    if (e.key === 'Escape') onCancel()
    if (e.key === 'Tab' && panel.current) {
      const f = panel.current.querySelectorAll<HTMLElement>('button:not(:disabled), textarea, input, select, a[href]')
      const first = f[0]
      const last = f[f.length - 1]
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last?.focus() }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first?.focus() }
    }
  }

  return (
    <div className="fixed inset-0 z-40 flex items-end justify-center bg-slate-900/50 p-4 sm:items-center" onKeyDown={onKey}>
      <div ref={panel} role="dialog" aria-modal="true" aria-labelledby={titleId} className="w-full max-w-md rounded-lg bg-white p-5 shadow-xl">
        <h2 id={titleId} className="text-lg font-semibold text-slate-900">{title}</h2>
        {children && <div className="mt-2 text-sm text-slate-700">{children}</div>}
        {reason && (
          <div className="mt-3">
            <TextAreaField label={reason.label} value={text} error={error} onChange={(e) => setText(e.target.value)} />
          </div>
        )}
        <div className="mt-5 flex justify-end gap-2">
          <Button variant="secondary" onClick={onCancel} disabled={busy}>Cancel</Button>
          <Button variant={danger ? 'danger' : 'primary'} onClick={submit} loading={busy}>{confirmLabel}</Button>
        </div>
      </div>
    </div>
  )
}
