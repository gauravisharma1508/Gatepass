import { useState } from 'react'
import { NavLink, Outlet, useNavigate } from 'react-router-dom'
import { useUser, useAuth } from '../shared/auth'
import type { Role } from '../shared/api'
import { Button, cx } from '../shared/ui'
import { NotificationBell } from '../modules/notifications'

const NAV: Record<Role, { to: string; label: string; end?: boolean }[]> = {
  EMPLOYEE: [
    { to: '/employee', label: 'Dashboard', end: true },
    { to: '/employee/visits', label: 'My visits', end: true },
    { to: '/employee/visits/new', label: 'New visit' },
  ],
  SECURITY: [
    { to: '/security', label: 'Gate dashboard', end: true },
    { to: '/security/walk-in', label: 'Register walk-in' },
  ],
  ADMIN: [
    { to: '/admin', label: 'Overview', end: true },
    { to: '/admin/visits', label: 'Visits' },
    { to: '/admin/users', label: 'Users' },
    { to: '/admin/settings', label: 'Settings' },
    { to: '/employee', label: 'Visitors I host', end: true },
  ],
}

export function AppLayout() {
  const user = useUser()
  const { logout } = useAuth()
  const nav = useNavigate()
  const [menu, setMenu] = useState(false)

  return (
    <div className="min-h-screen">
      <a href="#main" className="sr-only focus:not-sr-only focus:absolute focus:left-2 focus:top-2 focus:z-50 focus:rounded focus:bg-white focus:px-3 focus:py-2">Skip to content</a>
      <header className="sticky top-0 z-30 border-b border-slate-200 bg-white">
        <div className="mx-auto flex max-w-7xl items-center gap-3 px-4 py-2">
          <span className="text-lg font-bold text-blue-800">Gatepass</span>
          <button type="button" className="ml-auto inline-flex size-11 items-center justify-center rounded-md text-slate-700 hover:bg-slate-100 focus-visible:outline-2 focus-visible:outline-blue-700 md:hidden"
            aria-expanded={menu} aria-controls="main-nav" aria-label="Menu" onClick={() => setMenu((m) => !m)}>
            <svg viewBox="0 0 24 24" className="size-6" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden><path d="M4 7h16M4 12h16M4 17h16" strokeLinecap="round" /></svg>
          </button>
          <nav id="main-nav" aria-label="Main" className={cx('w-full flex-col gap-1 md:ml-4 md:flex md:w-auto md:flex-row', menu ? 'order-last flex' : 'hidden')}>
            {NAV[user.role].map((n) => (
              <NavLink key={n.to} to={n.to} end={n.end} onClick={() => setMenu(false)}
                className={({ isActive }) => cx('flex min-h-11 items-center rounded-md px-3 text-sm font-medium focus-visible:outline-2 focus-visible:outline-blue-700',
                  isActive ? 'bg-blue-50 text-blue-900' : 'text-slate-700 hover:bg-slate-100')}>
                {n.label}
              </NavLink>
            ))}
          </nav>
          <div className="flex items-center gap-2 md:ml-auto">
            <NotificationBell />
            <div className="hidden text-right leading-tight sm:block">
              <p className="text-sm font-medium text-slate-900" data-testid="user-name">{user.name}</p>
              <p className="text-xs text-slate-600">{user.role.charAt(0) + user.role.slice(1).toLowerCase()}</p>
            </div>
            <Button size="sm" variant="secondary" onClick={() => void logout().then(() => nav('/login', { replace: true }))}>Sign out</Button>
          </div>
        </div>
      </header>
      <main id="main" className="mx-auto max-w-7xl px-4 py-6"><Outlet /></main>
    </div>
  )
}
