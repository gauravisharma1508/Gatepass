import '@testing-library/jest-dom/vitest'
import { afterAll, afterEach, beforeAll } from 'vitest'
import { cleanup } from '@testing-library/react'
import { setupServer } from 'msw/node'
import { resetServerTime } from '../shared/api'

// Node's fetch needs absolute URLs; the app calls relative `/api/...`.
const realFetch = globalThis.fetch
globalThis.fetch = ((input: RequestInfo | URL, init?: RequestInit) =>
  typeof input === 'string' && input.startsWith('/') ? realFetch(`http://localhost${input}`, init) : realFetch(input, init)) as typeof fetch

export const server = setupServer()

beforeAll(() => server.listen({ onUnhandledRequest: 'error' }))
afterEach(() => {
  cleanup()
  server.resetHandlers()
  resetServerTime()
})
afterAll(() => server.close())
