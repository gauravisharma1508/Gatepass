import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'
import type { ZodType } from 'zod'
import {
  arrivalSchema, durationSchema, idLast4Schema, mobileSchema, nameSchema, optionalEmailSchema, passwordSchema,
  purposeSchema, reasonRequiredSchema, visitDateSchema,
} from './schemas'

const here = dirname(fileURLToPath(import.meta.url))
const file = resolve(here, '../../../../testdata/validation_vectors.json')

interface Vectors {
  name: { valid: string[]; invalid: string[]; normalize: [string, string][] }
  mobile: { valid: [string, string][]; invalid: string[] }
  email: { valid: string[]; invalid: string[] }
  purpose: { valid: string[]; invalid: string[] }
  duration: { valid: number[]; invalid: number[] }
  arrival: { valid: string[]; invalid: string[] }
  reasonRequired: { valid: string[]; invalid: string[] }
  idLast4: { valid: string[]; invalid: string[] }
  password: { valid: string[]; invalid: string[] }
  visitDate: { today: string; maxAdvanceDays: number; valid: string[]; invalid: string[] }
}
const v = JSON.parse(readFileSync(file, 'utf8')) as Vectors

const label = (x: unknown) => JSON.stringify(x).slice(0, 40)

function suite(name: string, schema: ZodType, valid: unknown[], invalid: unknown[]) {
  describe(name, () => {
    it.each(valid.map((x) => [label(x), x]))('accepts %s', (_l, x) => {
      expect(schema.safeParse(x).success).toBe(true)
    })
    it.each(invalid.map((x) => [label(x), x]))('rejects %s', (_l, x) => {
      expect(schema.safeParse(x).success).toBe(false)
    })
  })
}

describe('shared validation vectors (same file as the Go tests)', () => {
  suite('name', nameSchema, v.name.valid, v.name.invalid)
  suite('mobile', mobileSchema, v.mobile.valid.map(([i]) => i), v.mobile.invalid)
  suite('email', optionalEmailSchema, v.email.valid, v.email.invalid)
  suite('purpose', purposeSchema, v.purpose.valid, v.purpose.invalid)
  suite('duration', durationSchema, v.duration.valid, v.duration.invalid)
  suite('arrival', arrivalSchema, v.arrival.valid, v.arrival.invalid)
  suite('reasonRequired', reasonRequiredSchema, v.reasonRequired.valid, v.reasonRequired.invalid)
  suite('idLast4', idLast4Schema, v.idLast4.valid, v.idLast4.invalid)
  suite('password', passwordSchema, v.password.valid, v.password.invalid)
  suite('visitDate', visitDateSchema(v.visitDate.today, v.visitDate.maxAdvanceDays), v.visitDate.valid, v.visitDate.invalid)

  it('normalises mobile numbers to +91XXXXXXXXXX', () => {
    for (const [input, out] of v.mobile.valid) expect(mobileSchema.parse(input)).toBe(out)
  })
  it('trims names', () => {
    for (const [input, out] of v.name.normalize) expect(nameSchema.parse(input)).toBe(out)
  })
})
