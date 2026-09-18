import { useCallback, useEffect, useState } from 'react'
import { LogIn, LogOut, ShieldAlert, UserRound } from 'lucide-react'
import { useCurrentUser } from '../hooks/useCurrentUser'
import { useAgentStatus } from '../hooks/useAgentStatus'
import { describeRole, describeSignInMode } from '../lib/session'

/**
 * Who is signed in, how this deployment signs people in, what role they hold,
 * and where their agent is registered. Without any sign-in at all the section
 * still shows the mode, so a personal deployment knows why it is never asked
 * for anything.
 */
export function SignInStatus({ projects }: { projects?: { id: string; name: string }[] }) {
  const { user, reload } = useCurrentUser()
  const { agents } = useAgentStatus()
  const [status, setStatus] = useState('')

  const projectName = useCallback(
    (id: string) => projects?.find(project => project.id === id)?.name || id,
    [projects],
  )
  const [mine, setMine] = useState<string[]>([])
  useEffect(() => {
    if (!user) return
    setMine(agents.filter(agent => agent.userId === user.userId).map(agent => projectName(agent.projectId)))
  }, [agents, user, projectName])

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

  return (
    <section className="space-y-3 rounded-xl border border-[var(--border-color)] bg-[var(--bg-secondary)] p-4" aria-labelledby="signin-title">
      <h3 id="signin-title" className="flex items-center gap-2 font-bold text-[var(--text-primary)]">
        <UserRound size={16} /> Account
      </h3>

      {user.signedIn ? (
        <>
          <p className="text-[var(--text-muted)]">
            Signed in as <span className="font-semibold text-[var(--text-primary)]">{user.displayName || user.email || user.userId}</span>
            {user.role && <> as <span className="font-semibold text-[var(--text-primary)]">{describeRole(user.role)}</span></>}.
          </p>
          <p className="text-[var(--text-muted)]">
            Sign-in: {describeSignInMode(user.mode)}.
            {mine.length > 0
              ? ` Agent registered on ${mine.join(', ')}.`
              : ' No agent registered on any project.'}
          </p>
          {user.mode === 'local' && (
            <p role="note" className="flex gap-2 rounded-lg border border-amber-500/40 bg-amber-500/10 p-2.5 text-[11px] leading-relaxed text-[var(--text-secondary)]">
              <ShieldAlert size={14} className="mt-0.5 shrink-0 text-amber-400" aria-hidden="true" />
              <span>
                The local sign-in identifies people without authenticating them: anyone who types a
                colleague's address is that colleague. Keep it to a trusted network and connect an
                identity provider to replace it.
              </span>
            </p>
          )}
          {user.mode !== 'implicit' && (
            <button
              type="button"
              onClick={() => void signOut()}
              className="flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)]"
            >
              <LogOut size={14} /> Sign out
            </button>
          )}
        </>
      ) : (
        <>
          <p className="text-[var(--text-muted)]">You are not signed in. Sign-in: {describeSignInMode(user.mode)}.</p>
          <a
            href={`/signin?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`}
            className="inline-flex items-center gap-1 rounded-lg border border-[var(--border-color)] px-3 py-2 hover:bg-[var(--bg-hover)]"
            onClick={() => void reload()}
          >
            <LogIn size={14} /> Sign in
          </a>
        </>
      )}
      <p role="status" className="text-[var(--text-muted)]">{status}</p>
    </section>
  )
}
