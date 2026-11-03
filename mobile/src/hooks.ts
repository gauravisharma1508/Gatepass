import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Alert } from 'react-native'
import { api, errorMessage } from './api'
import type { Visit } from './api'

export const keys = {
  dashboard: ['dashboard', 'employee'] as const,
  visits: (p: unknown) => ['visits', p] as const,
  visit: (id: string) => ['visit', id] as const,
  notifications: ['notifications'] as const,
}

/** Live-ish updates without SSE (EventSource does not exist in React Native): poll every 15 s while a screen is mounted. */
export const POLL_MS = 15_000

/** POST /visits/:id/<path>; refreshes every list and detail afterwards. */
export function useVisitAction() {
  const qc = useQueryClient()
  return useMutation({
    mutationFn: ({ id, path, body }: { id: string; path: string; body?: unknown }) => api.post<Visit>(`/visits/${id}/${path}`, body),
    onSuccess: () => qc.invalidateQueries(),
    onError: (e) => Alert.alert('Could not do that', errorMessage(e)),
  })
}
