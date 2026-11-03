import { useState } from 'react'
import type { FormEvent } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { z } from 'zod'
import { api, errorMessage, keys } from '../../../shared/api'
import type { ImportResult, Page, Role, UserRow } from '../../../shared/api'
import { useUser } from '../../../shared/auth'
import { Button, ConfirmDialog, EmptyState, ErrorState, Modal, PageHeader, Pagination, SelectField, Spinner, TextField, fieldErrors, useForm, useToast } from '../../../shared/ui'
import { emailSchema, employeeCodeSchema, nameSchema, passwordSchema, roleSchema, validateForm } from '../../../shared/validation'

const ROLES: Role[] = ['EMPLOYEE', 'SECURITY', 'ADMIN']

function randomPassword() {
  const b = new Uint8Array(6)
  crypto.getRandomValues(b)
  return `Gp${Array.from(b, (x) => x.toString(16).padStart(2, '0')).join('')}9`
}

interface FormValues { employeeCode: string; name: string; email: string; phone: string; department: string; role: Role; tempPassword: string }

function UserFormModal({ user, onClose }: { user: UserRow | null; onClose: () => void }) {
  const qc = useQueryClient()
  const toast = useToast()
  const me = useUser()
  const form = useForm<FormValues>({
    employeeCode: user?.employeeCode ?? '', name: user?.name ?? '', email: user?.email ?? '', phone: user?.phone ?? '',
    department: user?.department ?? '', role: user?.role ?? 'EMPLOYEE', tempPassword: user ? '' : randomPassword(),
  })
  const [busy, setBusy] = useState(false)
  const schema = z.object({
    employeeCode: employeeCodeSchema, name: nameSchema, email: emailSchema, phone: z.string().trim().max(15, 'Too long'),
    department: z.string().trim().max(100, 'Too long'), role: roleSchema,
    tempPassword: user ? z.string() : passwordSchema,
  })

  async function submit(e: FormEvent) {
    e.preventDefault()
    const r = validateForm(schema, form.values)
    if (r.errors) return form.setErrors(r.errors)
    setBusy(true)
    try {
      if (user) {
        const { tempPassword: _t, ...patch } = r.data
        void _t
        await api.patch<UserRow>(`/users/${user.id}`, patch)
      } else await api.post<UserRow>('/users', r.data)
      await qc.invalidateQueries({ queryKey: ['users'] })
      toast.success(user ? 'User updated' : `User created. Temporary password: ${form.values.tempPassword}`)
      onClose()
    } catch (err) {
      const f = fieldErrors(err)
      if (f) form.setErrors(f)
      else toast.error(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }
  const editingSelf = user?.id === me.id

  return (
    <Modal open title={user ? `Edit ${user.name}` : 'Add a user'} onClose={onClose}>
      <form onSubmit={submit} noValidate className="space-y-3">
        <div className="grid gap-3 sm:grid-cols-2">
          <TextField label="Employee code" value={form.values.employeeCode} error={form.errors.employeeCode} onChange={(e) => form.set('employeeCode', e.target.value)} />
          <SelectField label="Role" value={form.values.role} error={form.errors.role} disabled={editingSelf}
            hint={editingSelf ? 'You cannot change your own role.' : undefined} onChange={(e) => form.set('role', e.target.value as Role)}>
            {ROLES.map((r) => <option key={r} value={r}>{r.charAt(0) + r.slice(1).toLowerCase()}</option>)}
          </SelectField>
        </div>
        <TextField label="Full name" value={form.values.name} error={form.errors.name} onChange={(e) => form.set('name', e.target.value)} />
        <TextField label="Email" type="email" value={form.values.email} error={form.errors.email} onChange={(e) => form.set('email', e.target.value)} />
        <div className="grid gap-3 sm:grid-cols-2">
          <TextField label="Phone (optional)" type="tel" value={form.values.phone} error={form.errors.phone} onChange={(e) => form.set('phone', e.target.value)} />
          <TextField label="Department (optional)" value={form.values.department} error={form.errors.department} onChange={(e) => form.set('department', e.target.value)} />
        </div>
        {!user && <TextField label="Temporary password" value={form.values.tempPassword} error={form.errors.tempPassword}
          hint="The user must change it at first sign-in. Share it securely." onChange={(e) => form.set('tempPassword', e.target.value)} />}
        <div className="flex justify-end gap-2 pt-2">
          <Button variant="secondary" onClick={onClose} disabled={busy}>Cancel</Button>
          <Button type="submit" loading={busy}>{user ? 'Save changes' : 'Create user'}</Button>
        </div>
      </form>
    </Modal>
  )
}

function ResetPasswordModal({ user, onClose }: { user: UserRow; onClose: () => void }) {
  const toast = useToast()
  const [pw, setPw] = useState(randomPassword())
  const [error, setError] = useState<string>()
  const m = useMutation({
    mutationFn: () => api.post(`/users/${user.id}/reset-password`, { tempPassword: pw }),
    onSuccess: () => { toast.success(`Password reset. Temporary password: ${pw}`); onClose() },
    onError: (e) => { const f = fieldErrors(e); if (f?.tempPassword) setError(f.tempPassword); else toast.error(errorMessage(e)) },
  })
  function submit(e: FormEvent) {
    e.preventDefault()
    const r = passwordSchema.safeParse(pw)
    if (!r.success) return setError(r.error.issues[0]?.message)
    m.mutate()
  }
  return (
    <Modal open title={`Reset password for ${user.name}`} onClose={onClose}>
      <form onSubmit={submit} noValidate className="space-y-3">
        <p className="text-sm text-slate-700">All their sessions end and they must choose a new password at the next sign-in.</p>
        <TextField label="Temporary password" value={pw} error={error} onChange={(e) => { setPw(e.target.value); setError(undefined) }} />
        <div className="flex justify-end gap-2">
          <Button variant="secondary" onClick={onClose} disabled={m.isPending}>Cancel</Button>
          <Button type="submit" variant="danger" loading={m.isPending}>Reset password</Button>
        </div>
      </form>
    </Modal>
  )
}

function ImportModal({ onClose }: { onClose: () => void }) {
  const qc = useQueryClient()
  const toast = useToast()
  const [file, setFile] = useState<File | null>(null)
  const [results, setResults] = useState<ImportResult[] | null>(null)
  const m = useMutation({
    mutationFn: () => { const fd = new FormData(); fd.append('file', file!); return api.upload<{ results: ImportResult[] }>('/users/import', fd) },
    onSuccess: (r) => { setResults(r.results); void qc.invalidateQueries({ queryKey: ['users'] }) },
    onError: (e) => toast.error(fieldErrors(e)?.file ?? errorMessage(e)),
  })
  return (
    <Modal open wide title="Import users from CSV" onClose={onClose}>
      <p className="mb-3 text-sm text-slate-700">Header row: <code className="rounded bg-slate-100 px-1">employeeCode,name,email,role</code> and optionally <code className="rounded bg-slate-100 px-1">department,phone,tempPassword</code>. Rows without a temporary password get a generated one, shown below once.</p>
      <div className="flex flex-wrap items-end gap-3">
        <div className="min-w-0 flex-1"><TextField label="CSV file" type="file" accept=".csv,text/csv" onChange={(e) => { setFile(e.target.files?.[0] ?? null); setResults(null) }} /></div>
        <Button disabled={!file} loading={m.isPending} onClick={() => m.mutate()}>Import</Button>
      </div>
      {results && (
        <div className="mt-4 max-h-72 overflow-auto rounded border border-slate-200">
          <table className="w-full text-left text-sm">
            <caption className="sr-only">Import results</caption>
            <thead><tr className="border-b bg-slate-50"><th scope="col" className="px-2 py-1">Row</th><th scope="col" className="px-2 py-1">Email</th><th scope="col" className="px-2 py-1">Result</th><th scope="col" className="px-2 py-1">Details</th></tr></thead>
            <tbody>{results.map((r) => (
              <tr key={r.row} className="border-b border-slate-100">
                <td className="px-2 py-1">{r.row}</td><td className="px-2 py-1">{r.email}</td>
                <td className={`px-2 py-1 font-medium ${r.status === 'created' ? 'text-green-800' : 'text-red-800'}`}>{r.status === 'created' ? 'Created' : 'Failed'}</td>
                <td className="px-2 py-1">{r.error ?? (r.tempPassword ? `Temporary password: ${r.tempPassword}` : '')}</td>
              </tr>))}</tbody>
          </table>
        </div>
      )}
      <div className="mt-4 flex justify-end"><Button variant="secondary" onClick={onClose}>Close</Button></div>
    </Modal>
  )
}

export function UsersPage() {
  const me = useUser()
  const qc = useQueryClient()
  const toast = useToast()
  const [f, setF] = useState({ q: '', role: '', isActive: '', page: 1 })
  const [edit, setEdit] = useState<UserRow | 'new' | null>(null)
  const [reset, setReset] = useState<UserRow | null>(null)
  const [toggle, setToggle] = useState<UserRow | null>(null)
  const [importing, setImporting] = useState(false)
  const params = { q: f.q || undefined, role: f.role || undefined, isActive: f.isActive || undefined, page: f.page, pageSize: 15 }
  const q = useQuery({ queryKey: keys.users(params), queryFn: () => api.get<Page<UserRow>>('/users', params), placeholderData: (p) => p })
  const setActive = useMutation({
    mutationFn: (u: UserRow) => api.post<UserRow>(`/users/${u.id}/${u.isActive ? 'deactivate' : 'activate'}`),
    onSuccess: (u) => { toast.success(u.isActive ? 'User activated' : 'User deactivated and signed out'); void qc.invalidateQueries({ queryKey: ['users'] }) },
    onError: (e) => toast.error(errorMessage(e)),
    onSettled: () => setToggle(null),
  })

  return (
    <>
      <PageHeader title="Users">
        <Button variant="secondary" onClick={() => setImporting(true)}>Import CSV</Button>
        <Button onClick={() => setEdit('new')}>Add user</Button>
      </PageHeader>
      <form role="search" className="mb-4 grid gap-3 rounded-lg border border-slate-200 bg-white p-4 sm:grid-cols-3" onSubmit={(e) => e.preventDefault()}>
        <TextField label="Search" placeholder="Name, email or employee code" value={f.q} onChange={(e) => setF({ ...f, q: e.target.value, page: 1 })} />
        <SelectField label="Role" value={f.role} onChange={(e) => setF({ ...f, role: e.target.value, page: 1 })}>
          <option value="">All roles</option>{ROLES.map((r) => <option key={r} value={r}>{r.charAt(0) + r.slice(1).toLowerCase()}</option>)}
        </SelectField>
        <SelectField label="Status" value={f.isActive} onChange={(e) => setF({ ...f, isActive: e.target.value, page: 1 })}>
          <option value="">Active and inactive</option><option value="true">Active</option><option value="false">Inactive</option>
        </SelectField>
      </form>
      {q.isPending && <Spinner />}
      {q.isError && <ErrorState error={q.error} onRetry={() => void q.refetch()} />}
      {q.data && (q.data.items.length === 0 ? <EmptyState title="No users found" /> : (
        <>
          <div className="overflow-x-auto rounded-lg border border-slate-200 bg-white">
            <table className="w-full min-w-[46rem] text-left text-sm">
              <caption className="sr-only">Users</caption>
              <thead><tr className="border-b border-slate-200 bg-slate-50 text-slate-700">
                {['Name', 'Email', 'Code', 'Role', 'Status', 'Actions'].map((h) => <th key={h} scope="col" className="px-3 py-2 font-medium">{h}</th>)}
              </tr></thead>
              <tbody>
                {q.data.items.map((u) => (
                  <tr key={u.id} className="border-b border-slate-100 align-top">
                    <td className="px-3 py-2">{u.name}<span className="block text-xs text-slate-600">{u.department ?? ''}</span></td>
                    <td className="px-3 py-2 break-all">{u.email}</td>
                    <td className="px-3 py-2 font-mono">{u.employeeCode}</td>
                    <td className="px-3 py-2">{u.role.charAt(0) + u.role.slice(1).toLowerCase()}</td>
                    <td className="px-3 py-2"><span className={`rounded-full border px-2 py-0.5 text-xs font-semibold ${u.isActive ? 'border-green-400 bg-green-100 text-green-900' : 'border-slate-400 bg-slate-100 text-slate-800'}`}>{u.isActive ? 'Active' : 'Inactive'}</span></td>
                    <td className="px-3 py-2"><div className="flex flex-wrap gap-1">
                      <Button size="sm" variant="secondary" onClick={() => setEdit(u)} aria-label={`Edit ${u.name}`}>Edit</Button>
                      <Button size="sm" variant="secondary" onClick={() => setReset(u)} aria-label={`Reset password of ${u.name}`}>Reset password</Button>
                      {u.id !== me.id && <Button size="sm" variant="secondary" onClick={() => setToggle(u)} aria-label={`${u.isActive ? 'Deactivate' : 'Activate'} ${u.name}`}>{u.isActive ? 'Deactivate' : 'Activate'}</Button>}
                    </div></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <Pagination page={q.data.page} pageSize={q.data.pageSize} total={q.data.total} onPage={(p) => setF({ ...f, page: p })} />
        </>
      ))}
      {edit && <UserFormModal user={edit === 'new' ? null : edit} onClose={() => setEdit(null)} />}
      {reset && <ResetPasswordModal user={reset} onClose={() => setReset(null)} />}
      {importing && <ImportModal onClose={() => setImporting(false)} />}
      <ConfirmDialog open={!!toggle} title={toggle?.isActive ? `Deactivate ${toggle.name}?` : `Activate ${toggle?.name}?`} danger={!!toggle?.isActive}
        confirmLabel={toggle?.isActive ? 'Deactivate' : 'Activate'} busy={setActive.isPending} onCancel={() => setToggle(null)} onConfirm={() => toggle && setActive.mutate(toggle)}>
        {toggle?.isActive ? 'They are signed out immediately and cannot sign in or be chosen as a host.' : 'They will be able to sign in again.'}
      </ConfirmDialog>
    </>
  )
}
