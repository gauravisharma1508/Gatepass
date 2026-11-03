import { api, ApiError } from '../../../shared/api'
import { SelectField, TextField } from '../../../shared/ui'
import type { FormErrors } from '../../../shared/ui'
import { mobileSchema } from '../../../shared/validation'

export interface VisitorValues {
  visitorName: string
  visitorMobile: string
  visitorEmail: string
  visitorCompany: string
  purpose: string
  expectedDurationMin: number
}

interface Props<T extends VisitorValues> {
  values: T
  errors: FormErrors
  set: <K extends keyof T>(key: K, value: T[K]) => void
}

export const DURATIONS = Array.from({ length: 31 }, (_, i) => 30 + i * 15)
export const durationLabel = (m: number) => (m < 60 ? `${m} minutes` : m % 60 === 0 ? `${m / 60} hour${m === 60 ? '' : 's'}` : `${Math.floor(m / 60)} h ${m % 60} min`)

/** Shared by the planned-visit form and the walk-in form. */
export function VisitorFields<T extends VisitorValues>({ values, errors, set }: Props<T>) {
  // A known mobile number pre-fills what is still empty (404 = never seen, nothing to do).
  async function lookup() {
    const m = mobileSchema.safeParse(values.visitorMobile)
    if (!m.success) return
    try {
      const r = await api.get<{ name: string; email: string | null; company: string | null }>('/visitors/lookup', { mobile: m.data })
      if (!values.visitorName) set('visitorName', r.name as T['visitorName'])
      if (!values.visitorEmail && r.email) set('visitorEmail', r.email as T['visitorEmail'])
      if (!values.visitorCompany && r.company) set('visitorCompany', r.company as T['visitorCompany'])
    } catch (e) {
      if (!(e instanceof ApiError && e.status === 404)) return
    }
  }
  return (
    <>
      <TextField label="Visitor mobile number" type="tel" inputMode="tel" autoComplete="off" value={values.visitorMobile} error={errors.visitorMobile}
        hint="10-digit Indian mobile number. Known visitors are filled in automatically."
        onChange={(e) => set('visitorMobile', e.target.value as T['visitorMobile'])} onBlur={lookup} />
      <TextField label="Visitor full name" autoComplete="off" value={values.visitorName} error={errors.visitorName}
        onChange={(e) => set('visitorName', e.target.value as T['visitorName'])} />
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField label="Visitor email (optional)" type="email" autoComplete="off" value={values.visitorEmail} error={errors.visitorEmail}
          onChange={(e) => set('visitorEmail', e.target.value as T['visitorEmail'])} />
        <TextField label="Company (optional)" autoComplete="off" value={values.visitorCompany} error={errors.visitorCompany}
          onChange={(e) => set('visitorCompany', e.target.value as T['visitorCompany'])} />
      </div>
      <TextField label="Purpose of visit" value={values.purpose} error={errors.purpose}
        onChange={(e) => set('purpose', e.target.value as T['purpose'])} />
      <SelectField label="Expected duration" value={values.expectedDurationMin} error={errors.expectedDurationMin}
        onChange={(e) => set('expectedDurationMin', Number(e.target.value) as T['expectedDurationMin'])}>
        {DURATIONS.map((d) => <option key={d} value={d}>{durationLabel(d)}</option>)}
      </SelectField>
    </>
  )
}
