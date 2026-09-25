import { useCallback, useEffect, useState } from 'react'
import { sortSessions, type McpSession, type McpSessionsResponse } from '../lib/mcpSessions'

interface McpSessionsState {
  sessions: McpSession[]
  isLoading: boolean
  error: string | null
}

/**
 * useMcpSessions polls the live MCP clients so the status bar can show who is
 * attached to the board right now.
 *
 * Connecting and disconnecting raise no server event, so this polls rather than
 * listening: the interval is what bounds how stale the list can be. It also
 * refreshes when the window regains focus, because the interesting moment is
 * usually the one right after the user started or stopped a client elsewhere.
 */
export function useMcpSessions(pollIntervalMs = 15_000): McpSessionsState {
  const [state, setState] = useState<McpSessionsState>({ sessions: [], isLoading: true, error: null })

  const fetchSessions = useCallback(async () => {
    try {
      const response = await fetch('/api/mcp/sessions')
      if (!response.ok) throw new Error(`HTTP ${response.status}`)
      const data: McpSessionsResponse = await response.json()
      setState({ sessions: sortSessions(data.sessions || []), isLoading: false, error: null })
    } catch (error: unknown) {
      // Keep the last known list: a failed poll says the status is stale, not
      // that every client went away.
      setState(previous => ({
        ...previous,
        isLoading: false,
        error: error instanceof Error ? error.message : 'Unknown error',
      }))
    }
  }, [])

  useEffect(() => {
    void fetchSessions()
    const interval = setInterval(() => void fetchSessions(), pollIntervalMs)
    const onFocus = () => void fetchSessions()
    window.addEventListener('focus', onFocus)
    return () => {
      clearInterval(interval)
      window.removeEventListener('focus', onFocus)
    }
  }, [fetchSessions, pollIntervalMs])

  return state
}
