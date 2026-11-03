import { expect, test } from '@playwright/test'
import { api, asUser, captureBoth, card, createPlanned, hostId, mobile, todayIST, uiLogin, uniq } from './helpers'

test('planned visit happy path: create -> check in with camera -> meeting over -> check out', async ({ browser }) => {
  const name = `Plan ${uniq()}`
  const host = await asUser(browser, 'empA')
  const gate = await asUser(browser, 'guard')

  // employee plans the visit through the form
  await host.page.goto('/employee/visits/new')
  await host.page.getByLabel('Visitor mobile number').fill(mobile())
  await host.page.getByLabel('Visitor full name').fill(name)
  await host.page.getByLabel('Purpose of visit').fill('Quarterly review')
  await host.page.getByRole('button', { name: 'Create visit' }).click()
  await expect(host.page).toHaveURL(/\/employee\/visits$/)
  await expect(card(host.page, name)).toContainText('Scheduled')

  // guard finds it by name, captures both photos with the (fake) camera, checks in
  await gate.page.goto('/security')
  await gate.page.getByLabel('Find a visitor').fill(name)
  await card(gate.page, name).getByRole('link', { name: 'Check in' }).click()
  await expect(gate.page.getByRole('button', { name: 'Check in', exact: true })).toBeDisabled()

  const visitorSlot = gate.page.locator('section', { has: gate.page.getByRole('heading', { name: 'Visitor photo' }) })
  await visitorSlot.getByRole('button', { name: 'Take photo' }).click()
  await visitorSlot.getByRole('button', { name: 'Take photo' }).click()
  await visitorSlot.getByRole('button', { name: 'Use this photo' }).click()
  await expect(visitorSlot.getByText('Saved')).toBeVisible()

  const idSlot = gate.page.locator('section', { has: gate.page.getByRole('heading', { name: 'ID photo' }) })
  await expect(idSlot.getByText('Choose the type of ID first.')).toBeVisible()
  await idSlot.getByLabel('Type of ID').selectOption('PAN')
  await idSlot.getByRole('button', { name: 'Take photo' }).click()
  await idSlot.getByRole('button', { name: 'Take photo' }).click()
  await idSlot.getByRole('button', { name: 'Use this photo' }).click()
  await expect(idSlot.getByText('Saved')).toBeVisible()

  await expect(gate.page.getByRole('button', { name: 'Check in', exact: true })).toBeEnabled()
  await gate.page.getByRole('button', { name: 'Check in', exact: true }).click()
  await expect(gate.page).toHaveURL(/\/security$/)

  // host sees the arrival live (no reload) and marks the meeting over
  await host.page.goto('/employee')
  await expect(card(host.page, name)).toContainText('Checked in')
  await card(host.page, name).first().getByRole('button', { name: 'Meeting over' }).click()
  await host.page.getByRole('dialog').getByRole('button', { name: 'Meeting over' }).click()

  // guard sees it under "Ready for check-out" without reloading, and checks out
  await gate.page.getByRole('tab', { name: /Ready for check-out/ }).click()
  await expect(card(gate.page, name)).toBeVisible()
  await card(gate.page, name).getByRole('button', { name: 'Check out' }).click()
  await gate.page.getByRole('dialog').getByRole('button', { name: 'Check out' }).click()
  await expect(card(gate.page, name)).toHaveCount(0)

  await host.page.goto('/employee/visits')
  await expect(card(host.page, name)).toContainText('Checked out')
  await host.ctx.close(); await gate.ctx.close()
})

test('walk-in accepted by the host: appears live on the host screen, then approved live at the gate', async ({ browser }) => {
  const name = `Walk ${uniq()}`
  const host = await asUser(browser, 'empA')
  const gate = await asUser(browser, 'guard')
  await host.page.goto('/employee')

  await gate.page.goto('/security/walk-in')
  await gate.page.getByRole('combobox', { name: 'Whom to meet' }).fill('anit')
  await gate.page.getByRole('option', { name: /Anita Desai/ }).click()
  await gate.page.getByLabel('Visitor mobile number').fill(mobile())
  await gate.page.getByLabel('Visitor full name').fill(name)
  await gate.page.getByLabel('Purpose of visit').fill('Unplanned discussion')
  await gate.page.getByRole('button', { name: 'Ask the host to approve' }).click()
  await expect(gate.page.getByText(/Request sent/)).toBeVisible()
  await expect(card(gate.page, name)).toContainText('Awaiting approval')

  // the host's already-open dashboard shows the request with a running countdown, no reload
  const req = card(host.page, name)
  await expect(req).toBeVisible()
  await expect(req.getByRole('timer')).toHaveText(/^(2\d|30):\d\d$/)
  await req.getByRole('button', { name: 'Accept' }).click()

  // the guard's open screen turns to Approved and offers check-in, no reload
  await expect(card(gate.page, name)).toContainText('Approved')
  await expect(card(gate.page, name).getByRole('link', { name: 'Check in' })).toBeVisible()
  await host.ctx.close(); await gate.ctx.close()
})

test('walk-in rejected with a reason: the guard sees it and cannot check in', async ({ browser }) => {
  const name = `Rej ${uniq()}`
  const host = await asUser(browser, 'empA')
  const gate = await asUser(browser, 'guard')
  const guardApi = await api('guard')
  await guardApi.post('/api/visits/walk-in', { data: { visitorName: name, visitorMobile: mobile(), purpose: 'Cold call', expectedDurationMin: 30, hostId: await hostId(guardApi) } })

  await gate.page.goto('/security?tab=pending')
  await expect(card(gate.page, name)).toContainText('Awaiting approval')
  await host.page.goto('/employee')
  await card(host.page, name).getByRole('button', { name: 'Reject' }).click()
  const dialog = host.page.getByRole('dialog')
  await dialog.getByLabel('Reason (optional)').fill('Not expecting anyone')
  await dialog.getByRole('button', { name: 'Reject' }).click()

  await gate.page.getByLabel('Find a visitor').fill(name)
  const c = card(gate.page, name)
  await expect(c).toContainText('Rejected')
  await expect(c).toContainText('Not expecting anyone')
  await expect(c.getByRole('link', { name: 'Check in' })).toHaveCount(0)
  await host.ctx.close(); await gate.ctx.close()
})

test('check-out is blocked until the host marks Meeting Over', async ({ browser }) => {
  const name = `Block ${uniq()}`
  const hostApi = await api('empA')
  const guardApi = await api('guard')
  const v = await createPlanned(hostApi, name)
  await captureBoth(guardApi, v.id)
  expect((await guardApi.post(`/api/visits/${v.id}/check-in`)).status()).toBe(200)

  const gate = await asUser(browser, 'guard')
  await gate.page.goto('/security?tab=inside')
  const c = card(gate.page, name)
  await expect(c).toContainText('Waiting for host to mark Meeting Over')
  await expect(c.getByRole('button', { name: 'Check out' })).toHaveCount(0)

  // the API agrees: forcing it is refused
  const r = await guardApi.post(`/api/visits/${v.id}/check-out`)
  expect(r.status()).toBe(409)
  expect((await r.json()).error.message).toContain('Meeting Over')
  await gate.ctx.close()
})

test('access control: another employee cannot open the visit, security cannot open admin pages', async ({ browser }) => {
  const hostApi = await api('empA')
  const v = await createPlanned(hostApi, `Acl ${uniq()}`)
  const other = await asUser(browser, 'empB')
  await other.page.goto(`/visits/${v.id}`)
  await expect(other.page.getByRole('heading', { name: 'Access denied' })).toBeVisible()
  await expect(other.page.getByText('Acl')).toHaveCount(0)
  await other.page.goto('/security')
  await expect(other.page.getByRole('heading', { name: 'Access denied' })).toBeVisible()

  const gate = await asUser(browser, 'guard')
  await gate.page.goto('/admin/users')
  await expect(gate.page.getByRole('heading', { name: 'Access denied' })).toBeVisible()
  await other.ctx.close(); await gate.ctx.close()
})

test('signed-out visitors are sent to the login page and back again afterwards', async ({ page }) => {
  await page.goto('/employee/visits?status=SCHEDULED')
  await expect(page).toHaveURL(/\/login\?returnTo=%2Femployee%2Fvisits%3Fstatus%3DSCHEDULED/)
  await page.getByLabel('Email').fill('empA@gatepass.local')
  await page.getByLabel('Password').fill('Gatepass@123')
  await page.getByRole('button', { name: 'Sign in' }).click()
  await expect(page).toHaveURL(/\/employee\/visits\?status=SCHEDULED/)
})
