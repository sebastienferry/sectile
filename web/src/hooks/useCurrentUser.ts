import { useCallback, useEffect, useState } from 'react'
import type { CurrentUser } from '../lib/session'

interface CurrentUserState {
  user: CurrentUser | null
  loading: boolean
  error: string
  reload: () => Promise<void>
  /**
   * Renames the signed-in account. It resolves to the server's message when the
   * name was refused, and to an empty string when it was taken: the caller
   * shows one line either way rather than throwing at the person.
   */
  rename: (displayName: string) => Promise<string>
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

  const rename = useCallback(async (displayName: string) => {
    try {
      const res = await fetch('/api/me', {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ displayName }),
      })
      // The answer is the profile itself, so a successful rename needs no
      // second read: the account section updates from what came back.
      const body = await res.json().catch(() => null)
      if (!res.ok) return body?.error || `HTTP ${res.status}`
      setUser(body)
      return ''
    } catch (err) {
      return err instanceof Error ? err.message : 'Could not save the display name.'
    }
  }, [])

  useEffect(() => { void reload() }, [reload])

  return { user, loading, error, reload, rename }
}
