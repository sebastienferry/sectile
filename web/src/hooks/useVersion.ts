import { useCallback, useEffect, useState } from 'react'

/** The build the server this interface is served by was cut from. */
export interface VersionInfo {
  version: string
  commit?: string
  date?: string
}

/**
 * useVersion reads the running server's build.
 *
 * The interface never carries a version of its own: it is compiled into the
 * server binary and served by it, so the only honest answer to "which version
 * am I looking at" is the one the server gives. A version baked into the
 * bundle at build time would keep claiming the release the tab was loaded on
 * long after the server behind it was upgraded.
 *
 * It is fetched once. The server cannot change version without restarting, and
 * a restart reloads the interface.
 */
export function useVersion(): { version: VersionInfo | null; error: string | null } {
  const [version, setVersion] = useState<VersionInfo | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    ;(async () => {
      try {
        const res = await fetch('/api/version')
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        const data: VersionInfo = await res.json()
        if (!cancelled) setVersion(data)
      } catch (err: unknown) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Unknown error')
      }
    })()
    return () => {
      cancelled = true
    }
  }, [])

  return { version, error }
}

/** The changelog as the server embeds it, Markdown included. */
export function useChangelog(enabled: boolean): {
  markdown: string | null
  isLoading: boolean
  error: string | null
  reload: () => void
} {
  const [markdown, setMarkdown] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [attempt, setAttempt] = useState(0)

  const reload = useCallback(() => setAttempt(value => value + 1), [])

  // Only fetched when something actually shows it: the changelog is a panel
  // somebody opens, not part of the board's first paint.
  useEffect(() => {
    if (!enabled) return
    let cancelled = false
    setIsLoading(true)
    setError(null)
    ;(async () => {
      try {
        const res = await fetch('/api/changelog')
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        const data: { markdown?: string } = await res.json()
        if (!cancelled) setMarkdown(data.markdown || '')
      } catch (err: unknown) {
        if (!cancelled) setError(err instanceof Error ? err.message : 'Unknown error')
      } finally {
        if (!cancelled) setIsLoading(false)
      }
    })()
    return () => {
      cancelled = true
    }
  }, [enabled, attempt])

  return { markdown, isLoading, error, reload }
}
