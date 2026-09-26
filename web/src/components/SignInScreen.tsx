import { useEffect, useState } from 'react'
import { KeyRound, LogIn, Mail, ShieldAlert } from 'lucide-react'
import type { CurrentUser, UnlockRefusal } from '../lib/session'
import { redirectFromSearch, unlockSealedCredentials } from '../lib/session'
import { applyDocumentLocale, format, rememberLocale, resolveInitialLocale, type Locale } from '../lib/i18n'
import { translations } from '../locales/translations'
import type { SignInStrings } from '../locales/signIn'

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
 *
 * The screen is rendered before the application context, so it has no personal
 * language setting to read: it starts from the language this browser used last,
 * else the browser's own, and offers a French/English switch that is remembered.
 */

const LOCALES: Locale[] = ['fr', 'en']

function describeRefusal(refusal: UnlockRefusal, text: SignInStrings['screen']): string {
  return refusal.code === 'unreadable'
    ? text.unlockUnreadable
    : format(text.unlockRefused, { trackers: refusal.trackers.join(', ') })
}

export function SignInScreen({ user, onSignedIn }: { user: CurrentUser; onSignedIn: () => void }) {
  const [locale, setLocale] = useState<Locale>(() => resolveInitialLocale())
  const t = translations[locale]
  const text = t.signIn.screen
  const [email, setEmail] = useState('')
  const [passphrase, setPassphrase] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState('')
  // A refused passphrase holds the screen instead of handing over to the
  // application: the session is open, but the notice has to be read, and
  // signalling the sign-in would unmount this screen along with the message.
  // The refusal is kept rather than its words, so a language switch rewords it.
  const [refusal, setRefusal] = useState<UnlockRefusal | null>(null)
  const returnTo = redirectFromSearch(window.location.search)

  useEffect(() => {
    applyDocumentLocale(locale, t.app.documentTitle)
  }, [locale, t.app.documentTitle])

  function chooseLocale(next: Locale) {
    setLocale(next)
    rememberLocale(next)
  }

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
        // The server's own reason is quoted as it comes; only its absence is worded here.
        throw new Error(body.error || format(text.signInFailedStatus, { status: res.status }))
      }
      // A refused passphrase is reported and nothing else: the session is open
      // and the tokens simply stay locked.
      const refused = await unlockSealedCredentials(passphrase)
      if (refused) {
        setRefusal(refused)
        return
      }
      enterApplication()
    } catch (err) {
      setError(err instanceof Error ? err.message : text.signInFailed)
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
        <div className="flex items-start justify-between gap-3">
          <div>
            <h1 id="signin-heading" className="text-lg font-bold text-[var(--text-primary)]">{text.title}</h1>
            <p className="mt-1 text-xs text-[var(--text-muted)]">
              {user.mode === 'oidc' ? text.oidcNotice : text.localNotice}
            </p>
          </div>
          <div
            role="group"
            aria-label={t.signIn.language.switchLabel}
            className="inline-flex shrink-0 gap-0.5 rounded-lg border border-[var(--border-color)] bg-[var(--bg-tertiary)] p-0.5"
          >
            {LOCALES.map(option => (
              <button
                key={option}
                type="button"
                lang={option}
                aria-pressed={locale === option}
                aria-label={t.signIn.language[option]}
                title={t.signIn.language[option]}
                onClick={() => chooseLocale(option)}
                className={`rounded-md px-2 py-0.5 text-[10.5px] font-semibold uppercase transition-colors cursor-pointer ${locale === option
                  ? 'bg-[var(--bg-secondary)] text-[var(--text-primary)] shadow-xs'
                  : 'text-[var(--text-muted)] hover:text-[var(--text-secondary)]'}`}
              >
                {option}
              </button>
            ))}
          </div>
        </div>

        {refusal ? (
          <div className="space-y-4">
            <p role="alert" className="flex gap-2 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-[11px] leading-relaxed text-[var(--text-secondary)]">
              <ShieldAlert size={16} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
              <span>{describeRefusal(refusal, text)}</span>
            </p>
            <button type="button" onClick={enterApplication} className={button}>
              <LogIn size={16} /> {text.continue}
            </button>
          </div>
        ) : user.mode === 'oidc' ? (
          <a href={`/auth/login?redirect=${encodeURIComponent(returnTo)}`} className={button}>
            <LogIn size={16} /> {text.continueWithProvider}
          </a>
        ) : (
          <form onSubmit={signInLocally} className="space-y-4">
            <label className="block text-xs font-medium text-[var(--text-secondary)]">
              {text.email}
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
              {text.passphrase} <span className="text-[var(--text-muted)]">{text.optional}</span>
              <div className="relative mt-1">
                <input
                  type="password" autoComplete="off" value={passphrase}
                  onChange={event => setPassphrase(event.target.value)}
                  placeholder={text.passphrasePlaceholder} className={field}
                />
                <KeyRound size={15} className="absolute left-3 top-2.5 text-[var(--text-muted)]" />
              </div>
            </label>
            <button type="submit" disabled={submitting || !email.trim()} className={button}>
              <LogIn size={16} /> {submitting ? text.submitting : text.submit}
            </button>
            <p role="note" className="flex gap-2 rounded-xl border border-amber-500/40 bg-amber-500/10 p-3 text-[11px] leading-relaxed text-[var(--text-secondary)]">
              <ShieldAlert size={16} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
              <span>{text.localWarning}</span>
            </p>
          </form>
        )}
        <p role="alert" className="min-h-4 text-xs text-rose-400">{error}</p>
      </section>
    </main>
  )
}
