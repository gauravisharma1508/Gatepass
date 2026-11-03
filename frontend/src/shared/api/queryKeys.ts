export const keys = {
  employeeDashboard: ['dashboard', 'employee'] as const,
  securityDashboard: ['dashboard', 'security'] as const,
  visits: (params: unknown) => ['visits', params] as const,
  visit: (id: string) => ['visit', id] as const,
  audit: (id: string) => ['visit-audit', id] as const,
  notifications: ['notifications'] as const,
  users: (params: unknown) => ['users', params] as const,
  settings: ['settings'] as const,
}
