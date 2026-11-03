import { readFileSync } from 'fs'
import { resolve } from 'path'
import type { ZodType } from 'zod'
import {
  arrivalSchema, durationSchema, mobileSchema, nameSchema, optionalEmailSchema, passwordSchema, purposeSchema, visitDateSchema,
  plannedVisitSchema, validateForm, changePasswordSchema,
} from '../src/validation'

// the same vectors file the Go and web tests use
const v = JSON.parse(readFileSync(resolve(__dirname, '../../testdata/validation_vectors.json'), 'utf8'))

function suite(name: string, schema: ZodType, valid: unknown[], invalid: unknown[]) {
  describe(name, () => {
    it.each(valid.map((x) => [JSON.stringify(x).slice(0, 40), x]))('accepts %s', (_l, x) => expect(schema.safeParse(x).success).toBe(true))
    it.each(invalid.map((x) => [JSON.stringify(x).slice(0, 40), x]))('rejects %s', (_l, x) => expect(schema.safeParse(x).success).toBe(false))
  })
}

suite('name', nameSchema, v.name.valid, v.name.invalid)
suite('mobile', mobileSchema, v.mobile.valid.map((p: [string, string]) => p[0]), v.mobile.invalid)
suite('email', optionalEmailSchema, v.email.valid, v.email.invalid)
suite('purpose', purposeSchema, v.purpose.valid, v.purpose.invalid)
suite('duration', durationSchema, v.duration.valid, v.duration.invalid)
suite('arrival', arrivalSchema, v.arrival.valid, v.arrival.invalid)
suite('password', passwordSchema, v.password.valid, v.password.invalid)
suite('visitDate', visitDateSchema(v.visitDate.today, v.visitDate.maxAdvanceDays), v.visitDate.valid, v.visitDate.invalid)

it('normalises mobile numbers', () => {
  for (const [input, want] of v.mobile.valid) expect(mobileSchema.parse(input)).toBe(want)
})

describe('planned visit form', () => {
  const good = { visitorName: 'Rahul Verma', visitorMobile: '98765 43210', visitorEmail: '', visitorCompany: '', purpose: 'Project review',
    visitDate: '2026-11-04', expectedArrival: '', expectedDurationMin: 60 }
  const schema = plannedVisitSchema('2026-11-03', 30)
  it('accepts a good form and normalises the mobile', () => {
    const r = validateForm(schema, good)
    expect(r.errors).toBeNull()
    expect(r.data?.visitorMobile).toBe('+919876543210')
  })
  it('reports per-field errors', () => {
    const r = validateForm(schema, { ...good, visitorName: '', visitDate: '2026-11-02', expectedDurationMin: 40 })
    expect(Object.keys(r.errors ?? {}).sort()).toEqual(['expectedDurationMin', 'visitDate', 'visitorName'])
  })
})

it('change-password needs matching confirmation', () => {
  const r = validateForm(changePasswordSchema, { currentPassword: 'x', newPassword: 'Newpass123', confirm: 'Different1' })
  expect(r.errors?.confirm).toBe('Passwords do not match')
})
