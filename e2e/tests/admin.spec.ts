import { expect, test } from '@playwright/test'
import { api, asUser, captureBoth, createPlanned, uniq } from './helpers'

test('admin: overview, visits with audit + force-close, users, settings validation', async ({ browser }) => {
  const admin = await asUser(browser, 'admin', '/admin')
  const { page } = admin

  // overview tiles
  await expect(page.getByRole('heading', { name: 'Overview' })).toBeVisible()
  await expect(page.getByTestId('count-onPremises')).toBeVisible()

  // a visitor who is inside and whose host never marks Meeting Over -> force-close with a reason
  const name = `Force ${uniq()}`
  const host = await api('empA'); const guard = await api('guard')
  const v = await createPlanned(host, name)
  await captureBoth(guard, v.id)
  await guard.post(`/api/visits/${v.id}/check-in`)

  await page.goto('/admin/visits')
  await page.getByLabel('Search').fill(name)
  await expect(page.getByRole('link', { name: v.code })).toBeVisible()
  const csv = page.getByRole('link', { name: 'Export CSV' })
  await expect(csv).toHaveAttribute('href', /\/api\/reports\/visits\.csv/)
  await page.getByRole('link', { name: v.code }).click()
  await expect(page.getByRole('heading', { name: `Visit ${v.code}` })).toBeVisible()
  await expect(page.getByRole('heading', { name: 'Audit trail' })).toBeVisible()
  await expect(page.getByRole('img', { name: /Photo of the visitor's ID document/ })).toBeVisible() // admin may see the ID photo
  await page.getByRole('button', { name: 'Force-close' }).click()
  const dlg = page.getByRole('dialog')
  await dlg.getByRole('button', { name: 'Force-close' }).click() // reason is mandatory
  await expect(dlg.getByText(/at least 5 characters|Required/)).toBeVisible()
  await dlg.getByLabel(/Reason/).fill('Host on leave, visitor left at 5 pm')
  await dlg.getByRole('button', { name: 'Force-close' }).click()
  await expect(page.getByText('Checked out').first()).toBeVisible()
  await expect(page.getByText('FORCE_CLOSED')).toBeVisible()

  // users: create, find, deactivate
  const tag = uniq()
  await page.goto('/admin/users')
  await page.getByRole('button', { name: 'Add user' }).click()
  const form = page.getByRole('dialog')
  await form.getByLabel('Employee code').fill(`E2E-${tag}`)
  await form.getByLabel('Full name').fill(`Eve Tester${tag.replace(/x/g, '')}`)
  await form.getByLabel('Email').fill(`e2e.${tag}@gatepass.local`)
  await form.getByRole('button', { name: 'Create user' }).click()
  await page.getByLabel('Search').fill(`e2e.${tag}@gatepass.local`)
  const row = page.getByRole('row').filter({ hasText: `e2e.${tag}@gatepass.local` })
  await expect(row).toContainText('Active')
  await row.getByRole('button', { name: /^Deactivate/ }).click()
  await page.getByRole('dialog').getByRole('button', { name: 'Deactivate' }).click()
  await expect(row).toContainText('Inactive')

  // duplicate email is reported on the field
  await page.getByRole('button', { name: 'Add user' }).click()
  const dup = page.getByRole('dialog')
  await dup.getByLabel('Employee code').fill(`E2E-${tag}b`)
  await dup.getByLabel('Full name').fill('Dup Tester')
  await dup.getByLabel('Email').fill(`e2e.${tag}@gatepass.local`)
  await dup.getByRole('button', { name: 'Create user' }).click()
  await expect(dup.getByText('A user with this email already exists')).toBeVisible()
  await dup.getByRole('button', { name: 'Cancel' }).click()

  // settings: inline validation, nothing is saved
  await page.goto('/admin/settings')
  const reminder = page.getByLabel(/Reminder before the window ends/)
  const window_ = page.getByLabel(/Walk-in approval window/)
  await expect(window_).not.toHaveValue('')
  const w = Number(await window_.inputValue())
  await reminder.fill(String(w))
  await expect(page.getByText('Must be less than the approval window')).toBeVisible()
  await expect(page.getByRole('button', { name: 'Save settings' })).toBeDisabled()
  await window_.fill('5')
  await expect(page.getByText('Between 10 and 120')).toBeVisible()
  await admin.ctx.close()
})
