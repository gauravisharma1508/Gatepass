import { Link, Navigate, Outlet, useLocation } from 'react-router-dom'
import type { ReactNode } from 'react'
import { useAuth, homeFor } from './AuthContext'
import type { Role } from '../api/types'
import { Spinner } from '../ui'

/** Signed-in users only. Forces the password change first when the server asks for it. */
export function RequireAuth({ children }: { children?: ReactNode }) {
  const { user, loading } = useAuth()
  const loc = useLocation()
  if (loading) return <Spinner label="Checking your session…" />
  if (!user) {
    const returnTo = encodeURIComponent(loc.pathname + loc.search)
    return <Navigate to={`/login?returnTo=${returnTo}`} replace />
  }
  if (user.mustChangePassword && loc.pathname !== '/change-password') return <Navigate to="/change-password" replace />
  return <>{children ?? <Outlet />}</>
}

export function AccessDenied() {
  const { user } = useAuth()
  return (
    <div role="alert" className="mx-auto mt-16 max-w-md rounded-lg border border-slate-200 bg-white p-8 text-center shadow-sm">
      <h1 className="text-xl font-semibold text-slate-900">Access denied</h1>
      <p className="mt-2 text-sm text-slate-700">You do not have access to this page.</p>
      <Link className="mt-4 inline-block text-sm font-medium text-blue-800 underline" to={user ? homeFor(user.role) : '/login'}>Go to your home page</Link>
    </div>
  )
}

/** Renders children only for the listed roles, otherwise an access-denied page. */
export function RoleGuard({ roles, children }: { roles: Role[]; children?: ReactNode }) {
  const { user } = useAuth()
  if (!user || !roles.includes(user.role)) return <AccessDenied />
  return <>{children ?? <Outlet />}</>
}

/** Redirect "/" to the role's home page. */
export function HomeRedirect() {
  const { user, loading } = useAuth()
  if (loading) return <Spinner />
  return <Navigate to={user ? homeFor(user.role) : '/login'} replace />
}
