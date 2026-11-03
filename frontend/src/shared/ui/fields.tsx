import { useId } from 'react'
import type { InputHTMLAttributes, SelectHTMLAttributes, TextareaHTMLAttributes, ReactNode } from 'react'
import { cx } from './cx'

const control =
  'block w-full rounded-md border bg-white px-3 py-2 text-base sm:text-sm text-slate-900 placeholder:text-slate-400 focus-visible:outline-2 focus-visible:outline-offset-1 focus-visible:outline-blue-700 disabled:bg-slate-100'

interface Common {
  label: string
  error?: string
  hint?: string
}

function Wrap({ id, label, error, hint, children }: Common & { id: string; children: ReactNode }) {
  return (
    <div>
      <label htmlFor={id} className="mb-1 block text-sm font-medium text-slate-800">{label}</label>
      {children}
      {hint && !error && <p id={`${id}-hint`} className="mt-1 text-xs text-slate-600">{hint}</p>}
      {error && <p id={`${id}-err`} role="alert" className="mt-1 text-sm text-red-700">{error}</p>}
    </div>
  )
}

function describe(id: string, error?: string, hint?: string) {
  return error ? `${id}-err` : hint ? `${id}-hint` : undefined
}

export function TextField({ label, error, hint, className, ...rest }: Common & InputHTMLAttributes<HTMLInputElement>) {
  const id = useId()
  return (
    <Wrap id={id} label={label} error={error} hint={hint}>
      <input id={id} aria-invalid={!!error || undefined} aria-describedby={describe(id, error, hint)}
        className={cx(control, error ? 'border-red-600' : 'border-slate-300', className)} {...rest} />
    </Wrap>
  )
}

export function TextAreaField({ label, error, hint, className, ...rest }: Common & TextareaHTMLAttributes<HTMLTextAreaElement>) {
  const id = useId()
  return (
    <Wrap id={id} label={label} error={error} hint={hint}>
      <textarea id={id} aria-invalid={!!error || undefined} aria-describedby={describe(id, error, hint)} rows={3}
        className={cx(control, error ? 'border-red-600' : 'border-slate-300', className)} {...rest} />
    </Wrap>
  )
}

export function SelectField({ label, error, hint, className, children, ...rest }: Common & SelectHTMLAttributes<HTMLSelectElement>) {
  const id = useId()
  return (
    <Wrap id={id} label={label} error={error} hint={hint}>
      <select id={id} aria-invalid={!!error || undefined} aria-describedby={describe(id, error, hint)}
        className={cx(control, error ? 'border-red-600' : 'border-slate-300', className)} {...rest}>
        {children}
      </select>
    </Wrap>
  )
}
