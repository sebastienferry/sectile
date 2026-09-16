import { useCallback, useEffect, useState } from 'react'
import { LogIn, LogOut, UserRound } from 'lucide-react'

type CurrentUser = {
  userId: string
  signedIn: boolean
  identityProvider: boolean
  email?: string
  displayName?: string
}

export function SignInStatus() {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [status, setStatus] = useState('')

  const load = useCallback(async () => {
    try {
      const res = await fetch('/api/me')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setUser(await res.json())
    } catch {
      setStatus('Could not read the current session.')
    }
  }, [])

  useEffect(() => { void load() }, [load])

  async function signOut() {
    try {
      const res = await fetch('/auth/logout', { method: 'POST' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      window.location.assign('/')
    } catch {
      setStatus('Could not sign out.')
    }
  }

  // Without a provider the deployment has a single implicit user; showing a
  // sign-in button there would promise something that does not exist.
  if (!user?.identityProvider) return null

  return (
    <section className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4" aria-labelledby="signin-title">
      <h3 id="signin-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
        <UserRound size={16} /> Account
      </h3>
      {user.signedIn ? (
        <>
          <p className="text-[var(--text-muted)]">
            Signed in as <span className="font-semibold text-[var(--text-primary)]">{user.displayName || user.email || user.userId}</span>.
          </p>
          <button
            type="button"
            onClick={signOut}
            className="flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)]"
          >
            <LogOut size={14} /> Sign out
          </button>
        </>
      ) : (
        <>
          <p className="text-[var(--text-muted)]">You are not signed in.</p>
          <a
            href={`/auth/login?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`}
            className="inline-flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)]"
          >
            <LogIn size={14} /> Sign in
          </a>
        </>
      )}
      <p role="status" className="text-[var(--text-muted)]">{status}</p>
    </section>
  )
}
