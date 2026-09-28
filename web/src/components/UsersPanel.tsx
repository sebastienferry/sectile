import { useCallback, useEffect, useState } from 'react'
import { Ban, Trash2, Undo2, Users } from 'lucide-react'
import type { Role } from '../lib/session'
import { useApp } from '../context/AppContext'
import { relativeTime } from '../lib/adminStats'
import { format, formatDateTime, parseDateInput, type Locale } from '../lib/i18n'

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

// A date the server sent, in the UI language; an unreadable one is shown as it came.
function when(value: string | undefined, locale: Locale, never: string): string {
  if (!value) return never
  return parseDateInput(value) ? formatDateTime(locale, value) : value
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
  const text = t.signIn.users
  const [users, setUsers] = useState<UserRow[]>([])
  const [rolesFromProvider, setRolesFromProvider] = useState(false)
  const [status, setStatus] = useState('')
  // The instant the list was read: "seen 3 minutes ago" is measured against it,
  // so a render never reads the clock.
  const [loadedAt, setLoadedAt] = useState(0)
  // A flag rather than a message, so the text follows a language switch
  // without reloading the list.
  const [loadFailed, setLoadFailed] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/users')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      const body = await res.json()
      setUsers(body.users || [])
      setLoadedAt(Date.now())
      setRolesFromProvider(!!body.rolesFromProvider)
      setLoadFailed(false)
    } catch {
      setLoadFailed(true)
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
        throw new Error(body.error || format(text.httpFailure, { message: failed, status: res.status }))
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
    }, format(text.roleChanged, { name: label(user), role: t.signIn.status.roles[role].toLowerCase() }), text.roleFailed)
  }

  async function setBlocked(user: UserRow, blocked: boolean) {
    if (blocked && !window.confirm(format(text.confirmBlock, { name: label(user) }))) {
      return
    }
    await apply(user, {
      method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ blocked }),
    }, format(blocked ? text.blockedDone : text.unblockedDone, { name: label(user) }), text.accountFailed)
  }

  async function remove(user: UserRow) {
    if (!window.confirm(format(text.confirmDelete, { name: label(user) }))) {
      return
    }
    await apply(user, { method: 'DELETE' }, format(text.deletedDone, { name: label(user) }), text.deleteFailed)
  }

  return (
    <section className={embedded ? 'space-y-4' : 'space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4'} aria-labelledby="users-title">
      {!embedded && (
        <h3 id="users-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
          <Users size={16} /> {text.title}
        </h3>
      )}
      <p className="text-[var(--text-muted)]">
        {text.intro}
        {rolesFromProvider && ` ${text.rolesFromProvider}`}
      </p>
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="text-[var(--text-muted)]">
            <tr>
              <th scope="col" className="py-1 pr-3 font-medium">{text.columnUser}</th>
              <th scope="col" className="py-1 pr-3 font-medium">{t.admin.activity}</th>
              <th scope="col" className="py-1 pr-3 font-medium">{text.columnLastSignIn}</th>
              <th scope="col" className="py-1 pr-3 font-medium">{text.columnRole}</th>
              <th scope="col" className="py-1 font-medium">{text.columnAccount}</th>
            </tr>
          </thead>
          <tbody>
            {users.map(user => (
              <tr key={user.id} className="border-t border-[var(--border-color)]">
                <td className="py-2 pr-3">
                  <div className="flex items-center gap-1.5 font-semibold text-[var(--text-primary)]">
                    <span>{user.displayName || user.email || user.id}{user.id === currentUserId ? ` ${text.you}` : ''}</span>
                    {user.blocked && (
                      <span className="rounded bg-rose-500/20 px-1 py-0.5 text-[9px] font-bold uppercase tracking-wider text-rose-500">{text.blocked}</span>
                    )}
                  </div>
                  {user.email && user.displayName && user.displayName !== user.email && <div className="text-[var(--text-muted)]">{user.email}</div>}
                </td>
                <td className="py-2 pr-3" title={user.lastActiveAt ? when(user.lastActiveAt, settings.language, text.never) : undefined}>
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
                <td className="py-2 pr-3 text-[var(--text-muted)]">{when(user.lastSignIn, settings.language, text.never)}</td>
                <td className="py-2 pr-3">
                  <select
                    aria-label={format(text.roleOf, { name: label(user) })}
                    value={user.role}
                    onChange={event => void changeRole(user, event.target.value as Role)}
                    className="rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] px-2 py-1 text-[var(--text-primary)]"
                  >
                    <option value="admin">{t.signIn.status.roles.admin}</option>
                    <option value="member">{t.signIn.status.roles.member}</option>
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
                        aria-label={format(user.blocked ? text.unblockAria : text.blockAria, { name: label(user) })}
                        title={user.blocked ? text.unblockTitle : text.blockTitle}
                        className="flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-[var(--text-primary)] transition-colors hover:bg-[var(--bg-tertiary)] cursor-pointer"
                      >
                        {user.blocked ? <Undo2 size={12} /> : <Ban size={12} />}
                        <span>{user.blocked ? text.unblock : text.block}</span>
                      </button>
                      <button
                        type="button"
                        onClick={() => void remove(user)}
                        aria-label={format(text.deleteAria, { name: label(user) })}
                        title={text.deleteTitle}
                        className="flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-2 py-1 text-rose-500 transition-colors hover:bg-rose-500/10 cursor-pointer"
                      >
                        <Trash2 size={12} />
                        <span>{text.delete}</span>
                      </button>
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <p role="status" className="text-[var(--text-muted)]">{status || (loadFailed ? t.signIn.users.loadFailed : '')}</p>
    </section>
  )
}
