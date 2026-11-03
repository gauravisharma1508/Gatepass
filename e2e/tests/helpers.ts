import { expect, request as pwRequest } from '@playwright/test'
import type { APIRequestContext, Browser, BrowserContext, Page } from '@playwright/test'
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { dirname, resolve } from 'node:path'

export const PASSWORD = 'Gatepass@123'
export const USERS = {
  empA: 'empA@gatepass.local', empB: 'empB@gatepass.local', guard: 'guard@gatepass.local', admin: 'admin@gatepass.local',
} as const
export type Who = keyof typeof USERS

const here = dirname(fileURLToPath(import.meta.url))
export const PHOTO = readFileSync(resolve(here, '../fixtures/photo.jpg'))
const BASE = process.env.BASE_URL ?? 'http://127.0.0.1:5173'

export const uniq = () => Math.random().toString(36).slice(2, 7).replace(/[0-9]/g, 'x')
export const mobile = () => '9' + String(Math.floor(Math.random() * 1e9)).padStart(9, '0')
/** today's date in the office time zone */
export const todayIST = () => new Intl.DateTimeFormat('en-CA', { timeZone: 'Asia/Kolkata' }).format(new Date())

const clients = new Map<Who, Promise<APIRequestContext>>()

/**
 * Signed-in API client, one login per role per run: the backend limits sign-ins to 10 per minute per IP,
 * so tests share each role's session (cookie) instead of signing in again and again.
 */
export function api(who: Who): Promise<APIRequestContext> {
  let c = clients.get(who)
  if (!c) {
    c = (async () => {
      const ctx = await pwRequest.newContext({ baseURL: BASE, extraHTTPHeaders: { 'X-Requested-With': 'gatepass' } })
      const r = await ctx.post('/api/auth/login', { data: { email: USERS[who], password: PASSWORD } })
      expect(r.ok(), `API login as ${who}`).toBeTruthy()
      return ctx
    })()
    clients.set(who, c)
  }
  return c
}

/** Sign in through the real login form (used by the login test only). */
export async function uiLogin(page: Page, who: Who) {
  await page.goto('/login')
  await page.getByLabel('Email').fill(USERS[who])
  await page.getByLabel('Password').fill(PASSWORD)
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page.getByTestId('user-name')).toBeVisible()
}

/** A separate browser context (own tabs, own event stream) already signed in as one role. */
export async function asUser(browser: Browser, who: Who, path = '/'): Promise<{ ctx: BrowserContext; page: Page }> {
  const storageState = await (await api(who)).storageState()
  const ctx = await browser.newContext({ permissions: ['camera'], storageState })
  const page = await ctx.newPage()
  await page.goto(path)
  await expect(page.getByTestId('user-name')).toBeVisible()
  return { ctx, page }
}

export async function createPlanned(host: APIRequestContext, name: string) {
  const r = await host.post('/api/visits/planned', {
    data: { visitorName: name, visitorMobile: mobile(), visitorCompany: 'E2E Ltd', purpose: 'Smoke test', visitDate: todayIST(), expectedDurationMin: 60 },
  })
  expect(r.status()).toBe(201)
  return (await r.json()) as { id: string; code: string }
}

export async function captureBoth(guard: APIRequestContext, id: string) {
  const file = { name: 'p.jpg', mimeType: 'image/jpeg', buffer: PHOTO }
  expect((await guard.put(`/api/visits/${id}/photos/VISITOR`, { multipart: { file } })).status()).toBe(204)
  expect((await guard.put(`/api/visits/${id}/photos/ID_DOCUMENT`, { multipart: { file, idType: 'PAN', idLast4: 'AB12' } })).status()).toBe(204)
}

export async function hostId(guard: APIRequestContext, q = 'anita') {
  const r = await guard.get(`/api/employees/search?q=${q}`)
  return ((await r.json()) as { items: { id: string }[] }).items[0]!.id
}

export const card = (page: Page, name: string) => page.getByRole('article').filter({ hasText: name })
