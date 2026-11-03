import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import * as SecureStore from 'expo-secure-store'
import { useQueryClient } from '@tanstack/react-query'
import { api, setToken, setUnauthorizedHandler } from '../api'
import type { SessionUser } from '../api'

const KEY = 'gatepass.token'

interface Auth {
  user: SessionUser | null
  loading: boolean
  signIn: (email: string, password: string) => Promise<SessionUser>
  signOut: () => Promise<void>
  refreshUser: () => Promise<void>
}
const Ctx = createContext<Auth | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const [user, setUser] = useState<SessionUser | null>(null)
  const [loading, setLoading] = useState(true)

  const clear = useCallback(async () => {
    setToken(null)
    setUser(null)
    qc.clear()
    await SecureStore.deleteItemAsync(KEY).catch(() => {})
  }, [qc])

  useEffect(() => {
    setUnauthorizedHandler(() => void clear())
    ;(async () => {
      try {
        const t = await SecureStore.getItemAsync(KEY)
        if (t) {
          setToken(t)
          const r = await api.get<{ user: SessionUser }>('/auth/me')
          setUser(r.user)
        }
      } catch {
        /* expired token or offline: stay signed out (a 401 already cleared it) */
      } finally {
        setLoading(false)
      }
    })()
  }, [clear])

  const value = useMemo<Auth>(() => ({
    user,
    loading,
    async signIn(email, password) {
      const r = await api.post<{ user: SessionUser; token: string }>('/auth/login', { email: email.trim(), password }, true)
      setToken(r.token)
      await SecureStore.setItemAsync(KEY, r.token)
      setUser(r.user)
      return r.user
    },
    async signOut() {
      try {
        await api.post('/auth/logout', undefined, true)
      } catch {
        /* sign out locally even if the server is unreachable */
      }
      await clear()
    },
    async refreshUser() {
      const r = await api.get<{ user: SessionUser }>('/auth/me')
      setUser(r.user)
    },
  }), [user, loading, clear])

  return <Ctx.Provider value={value}>{children}</Ctx.Provider>
}

export function useAuth(): Auth {
  const v = useContext(Ctx)
  if (!v) throw new Error('useAuth outside AuthProvider')
  return v
}
