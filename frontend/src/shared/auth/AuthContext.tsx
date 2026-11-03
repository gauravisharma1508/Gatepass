import { createContext, useCallback, useContext, useEffect, useMemo } from 'react'
import type { ReactNode } from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { api, ApiError, setUnauthorizedHandler } from '../api/client'
import type { Role, SessionUser } from '../api/types'

interface AuthValue {
  user: SessionUser | null
  loading: boolean
  login: (email: string, password: string) => Promise<SessionUser>
  logout: () => Promise<void>
  refresh: () => Promise<void>
}

const Ctx = createContext<AuthValue | null>(null)
export const ME_KEY = ['me'] as const

export function homeFor(role: Role): string {
  return role === 'SECURITY' ? '/security' : role === 'ADMIN' ? '/admin' : '/employee'
}

export function AuthProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const me = useQuery({
    queryKey: ME_KEY,
    retry: false,
    staleTime: Infinity,
    queryFn: async (): Promise<SessionUser | null> => {
      try {
        return (await api.get<{ user: SessionUser }>('/auth/me', undefined, { noAuthRedirect: true })).user
      } catch (e) {
        if (e instanceof ApiError && e.status === 401) return null
        throw e
      }
    },
  })

  // Any 401 from the API means the session ended: forget the user, the guards redirect to /login.
  useEffect(() => {
    setUnauthorizedHandler(() => qc.setQueryData(ME_KEY, null))
  }, [qc])

  const login = useCallback(async (email: string, password: string) => {
    const r = await api.post<{ user: SessionUser }>('/auth/login', { email, password }, { noAuthRedirect: true })
    qc.setQueryData(ME_KEY, r.user)
    return r.user
  }, [qc])

  const logout = useCallback(async () => {
    try {
      await api.post('/auth/logout')
    } catch {
      /* already signed out */
    }
    qc.removeQueries({ predicate: (q) => q.queryKey[0] !== 'me' })
    qc.setQueryData(ME_KEY, null)
  }, [qc])

  const refresh = useCallback(async () => {
    await qc.invalidateQueries({ queryKey: ME_KEY })
  }, [qc])

  const value = useMemo<AuthValue>(
    () => ({ user: me.data ?? null, loading: me.isPending, login, logout, refresh }),
    [me.data, me.isPending, login, logout, refresh],
  )
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth(): AuthValue {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth must be used inside <AuthProvider>')
  return v
}

/** The signed-in user; only call inside routes protected by <RequireAuth>. */
export function useUser(): SessionUser {
  const { user } = useAuth()
  if (!user) throw new Error('No signed-in user')
  return user
}
