import { createContext, useCallback, useContext, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { cx } from './cx'

type Kind = 'success' | 'error' | 'info'
interface Toast { id: number; kind: Kind; text: string }
interface Ctx { push: (kind: Kind, text: string) => void }

const ToastCtx = createContext<Ctx>({ push: () => {} })
let nextId = 1

export function ToastProvider({ children }: { children: ReactNode }) {
  const [items, setItems] = useState<Toast[]>([])
  const push = useCallback((kind: Kind, text: string) => {
    const id = nextId++
    setItems((l) => [...l, { id, kind, text }])
    setTimeout(() => setItems((l) => l.filter((t) => t.id !== id)), kind === 'error' ? 8000 : 4500)
  }, [])
  const value = useMemo(() => ({ push }), [push])
  return (
    <ToastCtx.Provider value={value}>
      {children}
      <div className="pointer-events-none fixed inset-x-0 bottom-4 z-50 flex flex-col items-center gap-2 px-4" aria-live="polite" role="status">
        {items.map((t) => (
          <div key={t.id} className={cx('pointer-events-auto max-w-md rounded-md px-4 py-3 text-sm font-medium shadow-lg',
            t.kind === 'success' && 'bg-green-800 text-white', t.kind === 'error' && 'bg-red-800 text-white', t.kind === 'info' && 'bg-slate-800 text-white')}>
            {t.text}
          </div>
        ))}
      </div>
    </ToastCtx.Provider>
  )
}

export function useToast() {
  const { push } = useContext(ToastCtx)
  return useMemo(
    () => ({
      success: (t: string) => push('success', t),
      error: (t: string) => push('error', t),
      info: (t: string) => push('info', t),
    }),
    [push],
  )
}
