import { useCallback, useEffect, useState } from 'react'
import { Ban, Trash2, Undo2, Users } from 'lucide-react'
import type { Role } from '../lib/session'
import { useApp } from '../context/AppContext'
import { relativeTime } from '../lib/adminStats'

interface UserRow {
  id: string
  subject: string
  email: string
  displayName: string
  role: Role
  createdAt: string
  lastSignIn?: string
  blocked?: boolean
  blockedAt?: string
  /** When a browser session of this account last reached the server. */
  lastActiveAt?: string
  /** Seen within the server's active window. */
  active?: boolean
}

function when(value?: string): string {
  if (!value) return 'never'
  const at = Date.parse(value)
  return Number.isNaN(at) ? value : new Date(at).toLocaleString()
}

/**
 * The admin's users view: every account with its role, a selector to change
 * it, and the two ways an account stops being usable. Blocking keeps the
 * account and everything it owns and only closes the door; deleting removes
 * the account and its credentials while leaving its work on the board. The
 * server refuses to demote, block or delete the last admin and says so; when
 * the identity provider supplies roles, a manual change lasts only until that
 * user's next sign-in, and the panel says that too.
 */
interface UsersPanelProps {
  currentUserId: string
  embedded?: boolean
  /** A change reloads the list, so it can follow the page it sits on. */
  reloadKey?: number
  /** Called after a change to an account, for the figures that count them. */
  onChange?: () => void
}

export function UsersPanel({ currentUserId, embedded = false, reloadKey = 0, onChange }: UsersPanelProps) {
  const { t, settings } = useApp()
  const [users, setUsers] = useState<UserRow[]>([])
  const [rolesFromProvider, setRolesFromProvider] = useState(false)
  const [status, setStatus] = useState('')
  // The instant the list was read: "seen 3 minutes ago" is measured against it,
  // so a render never reads the clock.
  const [loadedAt, setLoadedAt] = useState(0)

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/users')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const body = await res.json()
      setUsers(body.users || [])
      setLoadedAt(Date.now())
      setRolesFromProvider(!!body.rolesFromProvider)
    } catch {
      setStatus('Could not load the users.')
    }
  }, [])

  useEffect(() => { void load() }, [load, reloadKey])

  const label = (user: UserRow) => user.displayName || user.email || user.id

  // The three operations differ only in what they send and what they say, so
  // they share one call: the server's own refusal is what the panel shows,
  // because it is the layer that knows why a change was not allowed.
  async function apply(user: UserRow, init: RequestInit, done: string, failed: string) {
    setStatus('')
    try {
      const res = await fetch('/api/users/' + encodeURIComponent(user.id), init)
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error || `HTTP ${res.status}`)
      }
      await load()
      onChange?.()
      setStatus(done)
    } catch (err) {
      setStatus(err instanceof Error ? err.message : failed)
    }
  }

  async function changeRole(user: UserRow, role: Role) {
    await apply(user, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ role }),
    }, `${label(user)} is now ${role}.`, 'Could not change the role.')
  }

  async function setBlocked(user: UserRow, blocked: boolean) {
    if (blocked && !window.confirm(`Block ${label(user)}? Their sessions end immediately and their workstation keys stop working. Nothing they own is deleted.`)) {
      return
    }
    await apply(user, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ blocked }),
    }, blocked ? `${label(user)} is blocked.` : `${label(user)} can sign in again.`, 'Could not change the account.')
  }

  async function remove(user: UserRow) {
    if (!window.confirm(`Delete ${label(user)}? The account, its sessions and its workstation keys are removed for good. The tasks, comments and executions it owns stay on the board, with no owner.`)) {
      return
    }
    await apply(user, { method: 'DELETE' }, `${label(user)} is deleted.`, 'Could not delete the account.')
  }

  return (
    <section className={embedded ? 'space-y-4' : 'space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4'} aria-labelledby="users-title">
      {!embedded && (
        <h3 id="users-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
          <Users size={16} /> Users
        </h3>
      )}
      <p className="text-[var(--text-muted)]">
        Admins manage the accounts: who exists, what role they hold, and whether their account still opens.
        Members work on the shared board, open and configure projects, and act only on their own agent and executions.
        {rolesFromProvider && ' Roles come from the identity provider: a change here lasts until that person signs in again.'}
      </p>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="text-[var(--text-muted)]">
            <tr>
              <th scope="col" className="py-1 pr-3 font-medium">User</th>
              <th scope="col" className="py-1 pr-3 font-medium">{t.admin.activity}</th>
              <th scope="col" className="py-1 pr-3 font-medium">Last sign-in</th>
              <th scope="col" className="py-1 pr-3 font-medium">Role</th>
              <th scope="col" className="py-1 font-medium">Account</th>
            </tr>
          </thead>
          <tbody>
            {users.map(user => (
              <tr key={user.id} className="border-t border-[var(--border-color)]">
                <td className="py-2 pr-3">
                  <div className="flex items-center gap-1.5 font-semibold text-[var(--text-primary)]">
                    <span>{user.displayName || user.email || user.id}{user.id === currentUserId ? ' (you)' : ''}</span>
                    {user.blocked && (
                      <span className="rounded bg-rose-500/20 px-1 py-0.5 text-[9px] font-bold uppercase tracking-wider text-rose-500">Blocked</span>
                    )}
                  </div>
                  {user.email && user.displayName && user.displayName !== user.email && <div className="text-[var(--text-muted)]">{user.email}</div>}
                </td>
                <td className="py-2 pr-3" title={user.lastActiveAt ? when(user.lastActiveAt) : undefined}>
                  {user.active ? (
                    <span className="inline-flex items-center gap-1.5 font-semibold text-emerald-400">
                      <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" aria-hidden="true" />
                      {t.admin.online}
                    </span>
                  ) : (
                    <span className="text-[var(--text-muted)]">
                      {t.admin.lastActive} {relativeTime(user.lastActiveAt, loadedAt, settings.language) ?? t.admin.never}
                    </span>
                  )}
                </td>
                <td className="py-2 pr-3 text-[var(--text-muted)]">{when(user.lastSignIn)}</td>
                <td className="py-2 pr-3">
                  <select
                    aria-label={`Role of ${label(user)}`}
                    value={user.role}
                    onChange={event => void changeRole(user, event.target.value as Role)}
                    className="rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] px-2 py-1 text-[var(--text-primary)]"
                  >
                    <option value="admin">Admin</option>
                    <option value="member">Member</option>
                  </select>
                </td>
                <td className="py-2">
                  {/* Neither action is offered on your own row: the server
                      refuses both, and a control that always fails is worse
                      than no control. */}
                  {user.id === currentUserId ? (
                    <span className="text-[var(--text-muted)]">&mdash;</span>
                  ) : (
                    <div className="flex items-center gap-1">
                      <button
                        type="button"
                        onClick={() => void setBlocked(user, !user.blocked)}
                        aria-label={`${user.blocked ? 'Unblock' : 'Block'} ${label(user)}`}
                        title={user.blocked ? 'Let this account sign in again' : 'Close this account without deleting anything'}
                        className="flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-[var(--text-primary)] transition-colors hover:bg-[var(--bg-tertiary)] cursor-pointer"
                      >
                        {user.blocked ? <Undo2 size={12} /> : <Ban size={12} />}
                        <span>{user.blocked ? 'Unblock' : 'Block'}</span>
                      </button>
                      <button
                        type="button"
                        onClick={() => void remove(user)}
                        aria-label={`Delete ${label(user)}`}
                        title="Remove the account and its credentials for good"
                        className="flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-rose-500 transition-colors hover:bg-rose-500/10 cursor-pointer"
                      >
                        <Trash2 size={12} />
                        <span>Delete</span>
                      </button>
                    </div>
                  )}
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
