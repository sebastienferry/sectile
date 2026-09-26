import { useSyncExternalStore } from 'react'

/** Snapshot of a connected local agent, as returned by /api/agent/status. */
export interface AgentConnInfo {
  userId: string
  projectId: string
  deviceId: string
  connectedAt: string
  lastPingAt: string
}

export interface AgentStatusState {
  agents: AgentConnInfo[]
  isConnected: boolean
  isLoading: boolean
  error: string | null
}

const API_BASE = '/api'
const POLL_INTERVAL_MS = 30_000

// One store for the whole page: every task card reads the engine report, which
// follows the agent status, so a per-component poller and EventSource would
// open one connection per card.
let snapshot: AgentStatusState = {
  agents: [],
  isConnected: false,
  isLoading: true,
  error: null,
}
const listeners = new Set<() => void>()
let stopWatching: (() => void) | null = null

function publish(next: AgentStatusState) {
  snapshot = next
  for (const listener of listeners) listener()
}

async function fetchStatus() {
  try {
    const res = await fetch(`${API_BASE}/agent/status`)
    if (!res.ok) {
      throw new Error(`HTTP ${res.status}`)
    }
    const data = await res.json()
    const agents: AgentConnInfo[] = data.agents || []
    publish({
      agents,
      isConnected: agents.length > 0,
      isLoading: false,
      error: null,
    })
  } catch (err: unknown) {
    publish({
      ...snapshot,
      isLoading: false,
      error: err instanceof Error ? err.message : 'Unknown error',
    })
  }
}

function startWatching(): () => void {
  void fetchStatus()
  const interval = setInterval(fetchStatus, POLL_INTERVAL_MS)
  let es: EventSource | null = null
  try {
    es = new EventSource(`${API_BASE}/events/sse`)
    // The server names every event after its type, so these arrive as named
    // events: a listener on the unnamed 'message' event never fires.
    const handler = () => void fetchStatus()
    es.addEventListener('agent_connected', handler)
    es.addEventListener('agent_disconnected', handler)
  } catch {
    // SSE not available; rely on polling.
  }
  return () => {
    clearInterval(interval)
    es?.close()
  }
}

/**
 * Subscribes to the shared agent status. The first subscriber starts the
 * polling and the SSE listener, the last one to leave stops them.
 */
export function subscribeAgentStatus(listener: () => void): () => void {
  listeners.add(listener)
  if (!stopWatching) stopWatching = startWatching()
  return () => {
    listeners.delete(listener)
    if (listeners.size === 0 && stopWatching) {
      stopWatching()
      stopWatching = null
    }
  }
}

/** The current shared agent status, without subscribing. */
export function getAgentStatus(): AgentStatusState {
  return snapshot
}

/**
 * useAgentStatus exposes the remote agent connection status so the UI can show
 * a live "Local Agent Connected / Disconnected" indicator. It is refreshed by
 * polling and by SSE events when an agent connects or disconnects.
 */
export function useAgentStatus(): AgentStatusState {
  return useSyncExternalStore(subscribeAgentStatus, getAgentStatus)
}
