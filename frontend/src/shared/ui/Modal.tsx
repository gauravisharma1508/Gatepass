import { useEffect, useId, useRef } from 'react'
import type { ReactNode } from 'react'

/** Accessible modal: focus moves in, Tab stays inside, Esc closes, focus returns to the opener. */
export function Modal({ open, title, onClose, children, wide }: { open: boolean; title: string; onClose: () => void; children: ReactNode; wide?: boolean }) {
  const id = useId()
  const panel = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!open) return
    const opener = document.activeElement as HTMLElement | null
    panel.current?.querySelector<HTMLElement>('input, select, textarea, button')?.focus()
    return () => opener?.focus?.()
  }, [open])
  if (!open) return null
  function onKey(e: React.KeyboardEvent) {
    if (e.key === 'Escape') onClose()
    if (e.key === 'Tab' && panel.current) {
      const f = panel.current.querySelectorAll<HTMLElement>('button:not(:disabled), input:not(:disabled), select, textarea, a[href]')
      const first = f[0]
      const last = f[f.length - 1]
      if (e.shiftKey && document.activeElement === first) { e.preventDefault(); last?.focus() }
      else if (!e.shiftKey && document.activeElement === last) { e.preventDefault(); first?.focus() }
    }
  }
  return (
    <div className="fixed inset-0 z-40 flex items-end justify-center overflow-y-auto bg-slate-900/50 p-4 sm:items-center" onKeyDown={onKey}>
      <div ref={panel} role="dialog" aria-modal="true" aria-labelledby={id} className={`my-auto w-full ${wide ? 'max-w-2xl' : 'max-w-md'} rounded-lg bg-white p-5 shadow-xl`}>
        <h2 id={id} className="mb-3 text-lg font-semibold text-slate-900">{title}</h2>
        {children}
      </div>
    </div>
  )
}
