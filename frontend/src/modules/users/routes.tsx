import type { RouteObject } from 'react-router-dom'
import { RoleGuard } from '../../shared/auth'
import { UsersPage } from './pages/UsersPage'

export const routes: RouteObject[] = [
  { element: <RoleGuard roles={['ADMIN']} />, children: [{ path: '/admin/users', element: <UsersPage /> }] },
]
