// Zod mirrors of the server rules (backend/internal/platform/validation). The server stays the
// authority; src/shared/validation/vectors.test.ts runs the shared test vectors against both.
import { z } from 'zod'
import { addDays } from '../api/time'

const runes = (s: string) => Array.from(s).length

export const nameSchema = z
  .string()
  .trim()
  .min(1, 'Required')
  .refine((s) => runes(s) >= 2 && runes(s) <= 100, 'Must be 2–100 characters')
  .refine((s) => /^\p{L}[\p{L} .'-]*$/u.test(s), "Only letters, spaces and . ' - are allowed")

/** Indian mobile; output is normalised to +91XXXXXXXXXX (same as the server). */
export const mobileSchema = z
  .string()
  .transform((s) => s.trim().replace(/[ -]/g, ''))
  .pipe(z.string().min(1, 'Required'))
  .pipe(
    z.string().regex(/^(?:\+91|0)?[6-9][0-9]{9}$/, 'Enter a valid 10-digit Indian mobile number'),
  )
  .transform((s) => `+91${s.slice(-10)}`)

export const optionalEmailSchema = z
  .string()
  .trim()
  .refine((s) => s === '' || (s.length <= 254 && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(s)), 'Enter a valid email address')

export const emailSchema = optionalEmailSchema.refine((s) => s !== '', 'Required')

export const companySchema = z.string().trim().max(100, 'Too long')

export const purposeSchema = z
  .string()
  .trim()
  .min(1, 'Required')
  .refine((s) => runes(s) >= 3, 'Too short')
  .refine((s) => runes(s) <= 250, 'Too long')

export const durationSchema = z
  .number({ error: 'Choose a duration' })
  .int()
  .refine((n) => n >= 30 && n <= 480 && n % 15 === 0, 'Must be between 30 and 480 minutes, in steps of 15')

export const arrivalSchema = z.string().refine((s) => s === '' || /^([01][0-9]|2[0-3]):[0-5][0-9]$/.test(s), 'Use HH:MM (24-hour)')

export const reasonRequiredSchema = z
  .string()
  .trim()
  .min(1, 'Required')
  .refine((s) => runes(s) >= 5, 'Please give at least 5 characters')
  .refine((s) => runes(s) <= 250, 'Too long')

export const reasonOptionalSchema = z.string().trim().max(250, 'Too long')

export const idLast4Schema = z.string().refine((s) => s === '' || /^[A-Za-z0-9]{4}$/.test(s), 'Enter exactly 4 letters or digits')

export const passwordSchema = z
  .string()
  .refine((s) => {
    const n = new TextEncoder().encode(s).length
    return n >= 8 && n <= 72
  }, 'Must be 8–72 characters')
  .refine((s) => /[A-Za-z\u0080-￿]/.test(s) && /[0-9]/.test(s), 'Must contain at least one letter and one digit')

export function visitDateSchema(today: string, maxAdvanceDays: number) {
  return z
    .string()
    .refine((s) => /^\d{4}-\d{2}-\d{2}$/.test(s) && !Number.isNaN(Date.parse(`${s}T00:00:00Z`)) &&
      new Date(`${s}T00:00:00Z`).toISOString().slice(0, 10) === s, 'Enter a valid date')
    .refine((s) => s >= today, 'Visit date cannot be in the past')
    .refine((s) => s <= addDays(today, maxAdvanceDays), 'Visit date is too far ahead')
}

export const employeeCodeSchema = z
  .string()
  .trim()
  .min(1, 'Required')
  .regex(/^[A-Za-z0-9-]{3,20}$/, '3–20 letters, digits or -')

export const roleSchema = z.enum(['EMPLOYEE', 'SECURITY', 'ADMIN'], { error: 'Choose a role' })

/** Run a schema over a form's values and return per-field messages (first message per field). */
export function validateForm<S extends z.ZodType>(
  schema: S,
  values: unknown,
): { data: z.output<S>; errors: null } | { data: null; errors: Record<string, string> } {
  const r = schema.safeParse(values)
  if (r.success) return { data: r.data, errors: null }
  const errors: Record<string, string> = {}
  for (const issue of r.error.issues) {
    const key = issue.path.map(String).join('.') || '_'
    if (!(key in errors)) errors[key] = issue.message
  }
  return { data: null, errors }
}

export const plannedVisitSchema = (today: string, maxAdvanceDays: number) =>
  z.object({
    visitorName: nameSchema,
    visitorMobile: mobileSchema,
    visitorEmail: optionalEmailSchema,
    visitorCompany: companySchema,
    purpose: purposeSchema,
    visitDate: visitDateSchema(today, maxAdvanceDays),
    expectedArrival: arrivalSchema,
    expectedDurationMin: durationSchema,
  })

export const walkInSchema = z.object({
  visitorName: nameSchema,
  visitorMobile: mobileSchema,
  visitorEmail: optionalEmailSchema,
  visitorCompany: companySchema,
  purpose: purposeSchema,
  expectedDurationMin: durationSchema,
  hostId: z.string().min(1, 'Choose who to meet'),
})

export const loginSchema = z.object({
  email: z.string().trim().min(1, 'Required'),
  password: z.string().min(1, 'Required'),
})

export const changePasswordSchema = z
  .object({ currentPassword: z.string().min(1, 'Required'), newPassword: passwordSchema, confirm: z.string() })
  .refine((v) => v.newPassword === v.confirm, { path: ['confirm'], message: 'Passwords do not match' })

export const settingsSchema = z
  .object({
    approvalWindowMin: z.number({ error: 'Required' }).int().min(10, 'Between 10 and 120').max(120, 'Between 10 and 120'),
    approvalReminderMin: z.number({ error: 'Required' }).int().min(1, 'At least 1'),
    maxAdvanceDays: z.number({ error: 'Required' }).int().min(1, 'Between 1 and 90').max(90, 'Between 1 and 90'),
    photoRetentionDays: z.number({ error: 'Required' }).int().min(7, 'Between 7 and 365').max(365, 'Between 7 and 365'),
    recordRetentionDays: z.number({ error: 'Required' }).int().min(90, 'At least 90'),
  })
  .refine((v) => v.approvalReminderMin < v.approvalWindowMin, {
    path: ['approvalReminderMin'],
    message: 'Must be less than the approval window',
  })
