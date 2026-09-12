import { useState, useEffect, useCallback } from 'react'

/** Snapshot of a connected local agent, as returned by /api/agent/status. */
export interface AgentConnInfo {
  userId: string
  projectId: string
  deviceId: string
  connectedAt: string
  lastPingAt: string
}

interface AgentStatusState {
  agents: AgentConnInfo[]
  isConnected: boolean
  isLoading: boolean
  error: string | null
}

const API_BASE = '/api'

/**
 * useAgentStatus polls the remote agent connection status so the UI can show
 * a live "Local Agent Connected / Disconnected" indicator. It also listens
 * to SSE events for immediate updates when an agent connects or disconnects.
 */
export function useAgentStatus(pollIntervalMs = 30_000): AgentStatusState {
  const [state, setState] = useState<AgentStatusState>({
    agents: [],
    isConnected: false,
    isLoading: true,
    error: null,
  })

  const fetchStatus = useCallback(async () => {
    try {
      const res = await fetch(`${API_BASE}/agent/status`)
      if (!res.ok) {
        throw new Error(`HTTP ${res.status}`)
      }
      const data = await res.json()
      const agents: AgentConnInfo[] = data.agents || []
      setState({
        agents,
        isConnected: agents.length > 0,
        isLoading: false,
        error: null,
      })
    } catch (err: unknown) {
      setState(prev => ({
        ...prev,
        isLoading: false,
        error: err instanceof Error ? err.message : 'Unknown error',
      }))
    }
  }, [])

  // Initial fetch + periodic polling.
  useEffect(() => {
    fetchStatus()
    const id = setInterval(fetchStatus, pollIntervalMs)
    return () => clearInterval(id)
  }, [fetchStatus, pollIntervalMs])

  // Listen for SSE agent_connected / agent_disconnected events for instant updates.
  useEffect(() => {
    let es: EventSource | null = null
    try {
      es = new EventSource(`${API_BASE}/events/sse`)
      const handler = (event: MessageEvent) => {
        try {
          const data = JSON.parse(event.data)
          if (data.type === 'agent_connected' || data.type === 'agent_disconnected') {
            fetchStatus()
          }
        } catch {
          // Ignore malformed SSE data.
        }
      }
      es.addEventListener('message', handler)
    } catch {
      // SSE not available; rely on polling.
    }
    return () => {
      es?.close()
    }
  }, [fetchStatus])

  return state
}
