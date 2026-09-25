import { useCallback, useEffect, useState } from 'react'
import type { AdminStats } from '../lib/adminStats'

interface AdminStatsState {
  stats: AdminStats | null
  error: string | null
  isLoading: boolean
  /** Bumped on every successful read, so views that load alongside can follow. */
  revision: number
  refresh: () => Promise<void>
}

/**
 * useAdminStats polls the admin summary while the page is visible. A hidden
 * tab stops asking: nobody reads the figures it would fetch.
 */
export function useAdminStats(enabled: boolean, pollIntervalMs = 15_000): AdminStatsState {
  const [stats, setStats] = useState<AdminStats | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [isLoading, setIsLoading] = useState(true)
  const [revision, setRevision] = useState(0)

  const refresh = useCallback(async () => {
    try {
      const response = await fetch('/api/admin/stats')
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      setStats(await response.json())
      setError(null)
      setRevision(value => value + 1)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setIsLoading(false)
    }
  }, [])

  useEffect(() => {
    if (!enabled) return
    void refresh()
    const timer = window.setInterval(() => {
      if (document.visibilityState === 'visible') void refresh()
    }, pollIntervalMs)
    return () => window.clearInterval(timer)
  }, [enabled, refresh, pollIntervalMs])

  return { stats, error, isLoading, revision, refresh }
}
