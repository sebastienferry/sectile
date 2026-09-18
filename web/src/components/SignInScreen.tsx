import { useState } from 'react'
import { KeyRound, LogIn, Mail, ShieldAlert } from 'lucide-react'
import type { CurrentUser } from '../lib/session'
import { redirectFromSearch, unlockSealedCredentials } from '../lib/session'

/**
 * The sign-in screen. Signing in is mandatory (ADR 0015): a signed-out visitor
 * sees this and nothing else. With an identity provider it is one link; without
 * one it is the temporary local sign-in, an e-mail address which identifies
 * people without authenticating them. The screen says so, because a mode that
 * looks like a login and is not one would mislead.
 *
 * The only secret the form ever asks for is the sealing passphrase of ADR 0014,
 * and only for whoever chose to seal their tracker tokens. It is optional, and
 * a wrong one never refuses the sign-in: that would turn it into a password.
 */

export function SignInScreen({ user, onSignedIn }: { user: CurrentUser; onSignedIn: () => void }) {
  const [email, setEmail] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  // A refused passphrase holds the screen instead of handing over to the
  // application: the session is open, but the notice has to be read, and
  // signalling the sign-in would unmount this screen along with the message.
  const [notice, setNotice] = useState('')
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
      // A refused passphrase is reported and nothing else: the session is open
      // and the tokens simply stay locked.
      const refusal = await unlockSealedCredentials(passphrase)
      if (refusal) {
        setNotice(refusal)
        return
      }
      enterApplication()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not sign in.')
    } finally {
      setSubmitting(false)
    }
  }

  function enterApplication() {
    onSignedIn()
    window.location.assign(returnTo)
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

        {notice ? (
          <div className="space-y-4">
            <p role="alert" className="flex gap-2 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-[11px] leading-relaxed text-[var(--text-secondary)]">
              <ShieldAlert size={16} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
              <span>{notice}</span>
            </p>
            <button type="button" onClick={enterApplication} className={button}>
              <LogIn size={16} /> Continue
            </button>
          </div>
        ) : user.mode === 'oidc' ? (
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
            <label className="block text-xs font-medium text-[var(--text-secondary)]">
              Sealing passphrase <span className="text-[var(--text-muted)]">(optional)</span>
              <div className="relative mt-1">
                <input
                  type="password" autoComplete="off" value={passphrase}
                  onChange={event => setPassphrase(event.target.value)}
                  placeholder="Only if you sealed your tracker tokens" className={field}
                />
                <KeyRound size={15} className="absolute left-3 top-2.5 text-[var(--text-muted)]" />
              </div>
            </label>
            <button type="submit" disabled={submitting || !email.trim()} className={button}>
              <LogIn size={16} /> {submitting ? 'Signing in' : 'Sign in'}
            </button>
            <p role="note" className="flex gap-2 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-[11px] leading-relaxed text-[var(--text-secondary)]">
              <ShieldAlert size={16} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
              <span>
                No login password is asked: this mode identifies people without authenticating them
                and is meant for a trusted network until an identity provider is connected. The first
                account created becomes the admin. The passphrase above is only the one sealing your
                own tracker tokens; leaving it empty signs you in with those tokens locked.
              </span>
            </p>
          </form>
        )}
        <p role="alert" className="min-h-4 text-xs text-rose-400">{error}</p>
      </section>
    </main>
  )
}
