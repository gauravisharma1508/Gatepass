import { defineConfig } from '@playwright/test'

// Run against a running stack: backend on :8080 and the web app on :5173 (`make run`, `make web-dev`),
// seeded users (`make seed`). Override with BASE_URL.
export default defineConfig({
  testDir: './tests',
  timeout: 90_000,
  expect: { timeout: 10_000 },
  workers: 1, // tests share the seeded users and the gate
  reporter: [['list']],
  use: {
    baseURL: process.env.BASE_URL ?? 'http://127.0.0.1:5173',
    trace: 'retain-on-failure',
    launchOptions: { args: ['--use-fake-device-for-media-stream', '--use-fake-ui-for-media-stream'] },
    permissions: ['camera'],
  },
  projects: [{ name: 'chromium', use: { browserName: 'chromium' } }],
})
