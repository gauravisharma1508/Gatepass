import type { RouteObject } from 'react-router-dom'
import { RoleGuard } from '../../shared/auth'
import { EmployeeDashboardPage } from './pages/EmployeeDashboardPage'
import { MyVisitsPage } from './pages/MyVisitsPage'
import { VisitFormPage } from './pages/VisitFormPage'
import { VisitDetailPage } from './pages/VisitDetailPage'

const host = ['EMPLOYEE', 'ADMIN'] as const

export const routes: RouteObject[] = [
  {
    element: <RoleGuard roles={[...host]} />,
    children: [
      { path: '/employee', element: <EmployeeDashboardPage /> },
      { path: '/employee/visits', element: <MyVisitsPage /> },
      { path: '/employee/visits/new', element: <VisitFormPage /> },
      { path: '/employee/visits/:id/edit', element: <VisitFormPage /> },
    ],
  },
  { path: '/visits/:id', element: <VisitDetailPage /> },
]
