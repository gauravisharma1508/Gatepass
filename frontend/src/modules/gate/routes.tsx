import type { RouteObject } from 'react-router-dom'
import { RoleGuard } from '../../shared/auth'
import { CheckInPage } from './pages/CheckInPage'
import { SecurityDashboardPage } from './pages/SecurityDashboardPage'

export const routes: RouteObject[] = [
  {
    element: <RoleGuard roles={['SECURITY']} />,
    children: [
      { path: '/security', element: <SecurityDashboardPage /> },
      { path: '/security/visits/:id/check-in', element: <CheckInPage /> },
    ],
  },
]
