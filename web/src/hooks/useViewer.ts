import { useEffect, useState } from 'react'
import type { RunViewer } from '../lib/remoteRunIndicator'

// One read for the whole page. A badge sits on every card, and asking `/api/me`
// once per badge would cost a request per ticket for an answer that does not
// change while the page is open: signing out or in reloads the page.
let pending: Promise<RunViewer | null> | null = null

function readViewer(): Promise<RunViewer | null> {
  if (!pending) {
    pending = fetch('/api/me')
      .then(res => (res.ok ? res.json() : null))
      .then(user => (user ? { userId: user.userId, role: user.role } : null))
      .catch(() => {
        // A failed read is retried by the next badge rather than cached.
        pending = null
        return null
      })
  }
  return pending
}

/**
 * Who is looking at the board, as far as a run's controls care: their id and
 * their role. Nothing is known until the read answers, so a control that needs
 * it is simply not offered in the meantime.
 */
export function useViewer(): RunViewer | null {
  const [viewer, setViewer] = useState<RunViewer | null>(null)
  useEffect(() => {
    let alive = true
    void readViewer().then(value => { if (alive) setViewer(value) })
    return () => { alive = false }
  }, [])
  return viewer
}
