import { useMutation, useQueryClient } from '@tanstack/react-query'
import { api } from '../../../shared/api'
import type { Visit } from '../../../shared/api'
import { useToast } from '../../../shared/ui'
import { errorMessage } from '../../../shared/api'

/** POST /visits/:id/<path>; refreshes every list, dashboard and detail afterwards. */
export function useVisitMutation() {
  const qc = useQueryClient()
  const toast = useToast()
  return useMutation({
    mutationFn: ({ id, path, body }: { id: string; path: string; body?: unknown; success: string }) =>
      api.post<Visit>(`/visits/${id}/${path}`, body),
    onSuccess: (_d, v) => {
      toast.success(v.success)
      return qc.invalidateQueries({ predicate: (q) => q.queryKey[0] !== 'me' })
    },
    onError: (e) => toast.error(errorMessage(e)),
  })
}
