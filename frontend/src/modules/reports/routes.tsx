import type { RouteObject } from 'react-router-dom'
import { RoleGuard } from '../../shared/auth'
import { AdminHomePage } from './pages/AdminHomePage'
import { AdminVisitsPage } from './pages/AdminVisitsPage'

export const routes: RouteObject[] = [
  {
    element: <RoleGuard roles={['ADMIN']} />,
    children: [
      { path: '/admin', element: <AdminHomePage /> },
      { path: '/admin/visits', element: <AdminVisitsPage /> },
    ],
  },
]
