import type { RouteObject } from 'react-router-dom'
import { RoleGuard } from '../../shared/auth'
import { SettingsPage } from './pages/SettingsPage'

export const routes: RouteObject[] = [
  { element: <RoleGuard roles={['ADMIN']} />, children: [{ path: '/admin/settings', element: <SettingsPage /> }] },
]
