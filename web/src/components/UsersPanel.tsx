import { useCallback, useEffect, useState } from 'react'
import { Users } from 'lucide-react'
import type { Role } from '../lib/session'

interface UserRow {
  id: string
  subject: string
  email: string
  displayName: string
  role: Role
  createdAt: string
  lastSignIn?: string
}

function when(value?: string): string {
  if (!value) return 'never'
  const at = Date.parse(value)
  return Number.isNaN(at) ? value : new Date(at).toLocaleString()
}

/**
 * The admin's users view: every account with its role, and a selector to
 * change it. The server refuses to demote the last admin and says so; when the
 * identity provider supplies roles, a manual change lasts only until that
 * user's next sign-in, and the panel says that too.
 */
export function UsersPanel({ currentUserId }: { currentUserId: string }) {
  const [users, setUsers] = useState<UserRow[]>([])
  const [rolesFromProvider, setRolesFromProvider] = useState(false)
  const [status, setStatus] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/users')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const body = await res.json()
      setUsers(body.users || [])
      setRolesFromProvider(!!body.rolesFromProvider)
    } catch {
      setStatus('Could not load the users.')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function changeRole(user: UserRow, role: Role) {
    setStatus('')
    try {
      const res = await fetch('/api/users/' + encodeURIComponent(user.id), {
        method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ role }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error || `HTTP ${res.status}`)
      }
      await load()
      setStatus(`${user.displayName || user.email} is now ${role}.`)
    } catch (err) {
      setStatus(err instanceof Error ? err.message : 'Could not change the role.')
    }
  }

  return (
    <section className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4" aria-labelledby="users-title">
      <h3 id="users-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
        <Users size={16} /> Users
      </h3>
      <p className="text-[var(--text-muted)]">
        Admins manage users, projects, global settings, tracker credentials and anyone's execution.
        Members work on the shared board and act only on their own agent and executions.
        {rolesFromProvider && ' Roles come from the identity provider: a change here lasts until that person signs in again.'}
      </p>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="text-[var(--text-muted)]">
            <tr>
              <th scope="col" className="py-1 pr-3 font-medium">User</th>
              <th scope="col" className="py-1 pr-3 font-medium">Last sign-in</th>
              <th scope="col" className="py-1 font-medium">Role</th>
            </tr>
          </thead>
          <tbody>
            {users.map(user => (
              <tr key={user.id} className="border-t border-[var(--border-color)]">
                <td className="py-2 pr-3">
                  <div className="font-semibold text-[var(--text-primary)]">{user.displayName || user.email || user.id}{user.id === currentUserId ? ' (you)' : ''}</div>
                  {user.email && user.displayName && user.displayName !== user.email && <div className="text-[var(--text-muted)]">{user.email}</div>}
                </td>
                <td className="py-2 pr-3 text-[var(--text-muted)]">{when(user.lastSignIn)}</td>
                <td className="py-2">
                  <select
                    aria-label={`Role of ${user.displayName || user.email || user.id}`}
                    value={user.role}
                    onChange={event => void changeRole(user, event.target.value as Role)}
                    className="rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] px-2 py-1 text-[var(--text-primary)]"
                  >
                    <option value="admin">Admin</option>
                    <option value="member">Member</option>
                  </select>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p role="status" className="text-[var(--text-muted)]">{status}</p>
    </section>
  )
}
