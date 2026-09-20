import { useState } from 'react'
import { Check, LogIn, LogOut } from 'lucide-react'
import { useCurrentUser } from '../hooks/useCurrentUser'
import { describeRole } from '../lib/session'

interface SignInStatusProps {
  projects?: { id: string; name: string }[]
  onOpenAdmin?: () => void
}

/**
 * Merged Account component:
 * Displays user identity (avatar, initials, display name, email, role),
 * an inline display name editor, and the sign-out action.
 */
export function SignInStatus({ projects: _projects, onOpenAdmin: _onOpenAdmin }: SignInStatusProps = {}) {
  const { user, reload, rename } = useCurrentUser()
  const [status, setStatus] = useState('')
  const [draftName, setDraftName] = useState<string | null>(null)
  const [saving, setSaving] = useState(false)

  async function saveName() {
    setSaving(true)
    setStatus('')
    const failure = await rename((draftName ?? '').trim())
    setSaving(false)
    if (!failure) setDraftName(null)
    setStatus(failure || 'Nom d\'affichage enregistré.')
  }

  async function signOut() {
    try {
      const res = await fetch('/auth/logout', { method: 'POST' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      window.location.assign('/')
    } catch {
      setStatus('Could not sign out.')
    }
  }

  if (!user) return null

  const initials = (user.displayName || user.email || user.userId || 'SF').substring(0, 2).toUpperCase()

  return (
    <section className="space-y-5 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-5 text-xs" aria-labelledby="account-title">
      {user.signedIn ? (
        <>
          {/* User Identity Header */}
          <div className="flex items-center justify-between gap-3">
            <div className="flex items-center gap-3.5 min-w-0">
              <div className="w-12 h-12 rounded-2xl flex items-center justify-center font-bold text-base accent-bg text-white shadow-md shrink-0">
                {initials}
              </div>
              <div className="min-w-0">
                <div className="flex items-center gap-2">
                  <h4 id="account-title" className="font-bold text-sm text-[var(--text-primary)] truncate">
                    {user.displayName || user.email || user.userId}
                  </h4>
                  {user.role && (
                    <span className="text-[10px] px-2 py-0.5 rounded-full font-bold uppercase tracking-wider bg-amber-500/15 text-amber-500 border border-amber-500/30 shrink-0">
                      {describeRole(user.role)}
                    </span>
                  )}
                </div>
                <div className="text-[11px] text-[var(--text-muted)] truncate">
                  {user.email || user.userId}
                </div>
              </div>
            </div>

            <button
              type="button"
              onClick={() => void signOut()}
              className="flex items-center gap-1.5 rounded-lg border border-[var(--border-color)] px-3 py-1.5 font-medium text-[var(--text-secondary)] hover:text-red-400 hover:border-red-400/40 hover:bg-red-500/5 transition-colors cursor-pointer shrink-0"
            >
              <LogOut size={13} /> Se déconnecter
            </button>
          </div>

          {/* Display Name Edit Form */}
          <div className="space-y-2 pt-3 border-t border-[var(--border-color)]">
            <label htmlFor="account-display-name" className="block text-[11px] font-medium text-[var(--text-secondary)]">
              Nom d'affichage (Display Name)
            </label>
            <div className="flex gap-2">
              <input
                id="account-display-name"
                type="text"
                maxLength={80}
                value={draftName ?? user.displayName ?? ''}
                onChange={event => setDraftName(event.target.value)}
                placeholder="Votre nom ou pseudo sur ce tableau..."
                className="flex-1 rounded-xl border border-[var(--border-color)] bg-[var(--bg-tertiary)] px-3 py-2 text-[var(--text-primary)] focus:border-[var(--accent-color)] focus:outline-none"
              />
              <button
                type="button"
                onClick={() => void saveName()}
                disabled={saving || draftName === null || draftName.trim() === (user.displayName || '')}
                className="flex items-center gap-1 rounded-xl border border-[var(--border-color)] px-3.5 py-2 hover:bg-[var(--bg-hover)] disabled:opacity-40 font-medium transition-colors cursor-pointer"
              >
                <Check size={14} /> Enregistrer
              </button>
            </div>
            <p className="text-[11px] text-[var(--text-muted)]">
              Votre nom tel qu'il apparaît sur les tâches, assignations, commentaires et activités. Effacez-le pour réutiliser votre e-mail.
            </p>
          </div>
        </>
      ) : (
        <div className="text-center py-4 space-y-3">
          <p className="text-[var(--text-muted)]">Vous n'êtes pas connecté.</p>
          <a
            href={`/signin?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`}
            className="inline-flex items-center gap-1.5 rounded-lg accent-bg text-white px-4 py-2 font-medium"
            onClick={() => void reload()}
          >
            <LogIn size={14} /> Se connecter
          </a>
        </div>
      )}

      {status && <p role="status" className="text-xs font-medium text-[var(--accent-color)]">{status}</p>}
    </section>
  )
}
