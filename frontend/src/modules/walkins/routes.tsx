import type { RouteObject } from 'react-router-dom'
import { RoleGuard } from '../../shared/auth'
import { WalkInPage } from './pages/WalkInPage'

export const routes: RouteObject[] = [
  { element: <RoleGuard roles={['SECURITY']} />, children: [{ path: '/security/walk-in', element: <WalkInPage /> }] },
]
