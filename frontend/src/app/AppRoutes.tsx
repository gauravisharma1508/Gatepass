import { Link, useRoutes } from 'react-router-dom'
import type { RouteObject } from 'react-router-dom'
import { HomeRedirect, RequireAuth } from '../shared/auth'
import { publicRoutes, changePasswordRoute } from '../modules/auth'
import { routes as visitRoutes } from '../modules/visits'
import { routes as gateRoutes } from '../modules/gate'
import { routes as walkInRoutes } from '../modules/walkins'
import { routes as reportRoutes } from '../modules/reports'
import { routes as userRoutes } from '../modules/users'
import { routes as settingsRoutes } from '../modules/settings'
import { AppLayout } from './AppLayout'

function NotFound() {
  return (
    <div className="mx-auto mt-16 max-w-md rounded-lg border border-slate-200 bg-white p-8 text-center">
      <h1 className="text-xl font-semibold">Page not found</h1>
      <Link className="mt-3 inline-block text-sm text-blue-800 underline" to="/">Go to your home page</Link>
    </div>
  )
}

export const appRoutes: RouteObject[] = [
  ...publicRoutes,
  {
    element: <RequireAuth />,
    children: [
      changePasswordRoute,
      {
        element: <AppLayout />,
        children: [
          { path: '/', element: <HomeRedirect /> },
          ...visitRoutes,
          ...gateRoutes,
          ...walkInRoutes,
          ...reportRoutes,
          ...userRoutes,
          ...settingsRoutes,
          { path: '*', element: <NotFound /> },
        ],
      },
    ],
  },
]

export function AppRoutes() {
  return useRoutes(appRoutes)
}
