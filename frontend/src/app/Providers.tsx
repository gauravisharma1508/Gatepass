import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { ApiError } from '../shared/api'
import { AuthProvider } from '../shared/auth'
import { ToastProvider } from '../shared/ui'

export function makeQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 10_000,
        // 4xx answers will not change by asking again
        retry: (count, err) => !(err instanceof ApiError && err.status >= 400 && err.status < 500) && count < 2,
        refetchOnWindowFocus: true,
      },
    },
  })
}

export function Providers({ client, children }: { client?: QueryClient; children: ReactNode }) {
  return (
    <QueryClientProvider client={client ?? makeQueryClient()}>
      <ToastProvider>
        <AuthProvider>{children}</AuthProvider>
      </ToastProvider>
    </QueryClientProvider>
  )
}
