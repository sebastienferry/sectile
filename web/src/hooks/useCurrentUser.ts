import { useCallback, useEffect, useState } from 'react'
import type { CurrentUser } from '../lib/session'

interface CurrentUserState {
  user: CurrentUser | null
  loading: boolean
  error: string
  reload: () => Promise<void>
}

/**
 * Reads who the server says we are. `/api/me` is public, so this is the one
 * call that always answers, signed in or not, and it is what decides whether
 * the board or the sign-in screen is shown.
 */
export function useCurrentUser(): CurrentUserState {
  const [user, setUser] = useState<CurrentUser | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const reload = useCallback(async () => {
    try {
      const res = await fetch('/api/me')
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setUser(await res.json())
      setError('')
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Could not read the current session.')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { void reload() }, [reload])

  return { user, loading, error, reload }
}
