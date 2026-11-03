import { useState } from 'react'
import { ApiError } from '../api/client'

export type FormErrors = Record<string, string>

/** Minimal form state: values, per-field errors (cleared as the user edits that field). */
export function useForm<T extends object>(initial: T) {
  const [values, setValues] = useState<T>(initial)
  const [errors, setErrors] = useState<FormErrors>({})
  function set<K extends keyof T>(key: K, value: T[K]) {
    setValues((p) => ({ ...p, [key]: value }))
    setErrors((p) => {
      if (!(key as string in p)) return p
      const next = { ...p }
      delete next[key as string]
      return next
    })
  }
  return { values, setValues, set, errors, setErrors }
}

/** Per-field messages from a 400/409 response (`error.details.fields`), or null. */
export function fieldErrors(e: unknown): FormErrors | null {
  return e instanceof ApiError && Object.keys(e.fields).length > 0 ? e.fields : null
}
