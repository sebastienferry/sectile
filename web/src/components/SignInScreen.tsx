import { useState } from 'react'
import { LogIn, Mail, ShieldAlert } from 'lucide-react'
import type { CurrentUser } from '../lib/session'
import { redirectFromSearch } from '../lib/session'

/**
 * The sign-in screen. With an identity provider it is one link; without one
 * it is the temporary local sign-in: an e-mail address and nothing else, which
 * identifies people without authenticating them. The screen says so, because a
 * mode that looks like a login and is not one would mislead.
 */
export function SignInScreen({ user, onSignedIn }: { user: CurrentUser; onSignedIn: () => void }) {
  const [email, setEmail] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  const returnTo = redirectFromSearch(window.location.search)

  async function signInLocally(event: React.FormEvent) {
    event.preventDefault()
    setSubmitting(true)
    setError('')
    try {
      const res = await fetch('/auth/local', {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ email }),
      })
      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        throw new Error(body.error || `HTTP ${res.status}`)
      }
      onSignedIn()
      window.location.assign(returnTo)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not sign in.')
    } finally {
      setSubmitting(false)
    }
  }

  const card = 'w-full max-w-sm space-y-5 rounded-2xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-6 shadow-2xl'
  const field = 'w-full rounded-xl border border-[var(--border-color)] bg-[var(--bg-tertiary)] py-2 pl-9 pr-3 text-sm text-[var(--text-primary)] focus:border-[var(--accent-color)] focus:outline-none'
  const button = 'inline-flex w-full items-center justify-center gap-2 rounded-xl accent-bg px-4 py-2 text-sm font-semibold text-white disabled:opacity-50'

  return (
    <main className="flex h-[var(--app-h)] w-[var(--app-w)] items-center justify-center bg-[var(--bg-primary)] p-6" aria-labelledby="signin-heading">
      <section className={card}>
        <div>
          <h1 id="signin-heading" className="text-lg font-bold text-[var(--text-primary)]">Sign in to Sectile</h1>
          <p className="mt-1 text-xs text-[var(--text-muted)]">
            {user.mode === 'oidc'
              ? 'This deployment signs people in through its identity provider.'
              : 'This deployment uses the temporary local sign-in.'}
          </p>
        </div>

        {user.mode === 'oidc' ? (
          <a href={`/auth/login?redirect=${encodeURIComponent(returnTo)}`} className={button}>
            <LogIn size={16} /> Continue with the identity provider
          </a>
        ) : (
          <form onSubmit={signInLocally} className="space-y-4">
            <label className="block text-xs font-medium text-[var(--text-secondary)]">
              E-mail address
              <div className="relative mt-1">
                <input
                  type="email" required autoFocus autoComplete="email" value={email}
                  onChange={event => setEmail(event.target.value)}
                  placeholder="you@example.com" className={field}
                />
                <Mail size={15} className="absolute left-3 top-2.5 text-[var(--text-muted)]" />
              </div>
            </label>
            <button type="submit" disabled={submitting || !email.trim()} className={button}>
              <LogIn size={16} /> {submitting ? 'Signing in' : 'Sign in'}
            </button>
            <p role="note" className="flex gap-2 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-[11px] leading-relaxed text-[var(--text-secondary)]">
              <ShieldAlert size={16} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
              <span>
                No password is asked: this mode identifies people without authenticating them and is
                meant for a trusted network until an identity provider is connected. The first account
                created becomes the admin.
              </span>
            </p>
          </form>
        )}
        <p role="alert" className="min-h-4 text-xs text-rose-400">{error}</p>
      </section>
    </main>
  )
}
