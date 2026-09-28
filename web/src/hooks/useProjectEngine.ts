import { useCallback, useEffect, useSyncExternalStore } from 'react'
import type { EngineReport } from '../types'
import { createProjectEngineStore } from '../lib/projectEngineStore'
import { getAgentStatus, subscribeAgentStatus, type AgentStatusState } from './useAgentStatus'

const store = createProjectEngineStore()
const { refresh: refreshProjectEngine, subscribe } = store
export { refreshProjectEngine }

// Mounted readers per project; only these are refreshed.
const retained = new Map<string, number>()
let stopWatching: (() => void) | null = null

function refreshRetained() {
  for (const projectId of retained.keys()) void refreshProjectEngine(projectId)
}

// The agents the report depends on: one connecting, leaving or reconnecting
// changes what the caller's workstation reports.
function agentSignature(status: AgentStatusState): string {
  return status.agents
    .map(agent => `${agent.projectId}|${agent.deviceId}|${agent.connectedAt}`)
    .sort()
    .join(',')
}

// How long a newly connected agent is given to send its capability report.
const REPORT_SETTLE_MS = 3_000

function startWatching(): () => void {
  let followUp: ReturnType<typeof setTimeout> | undefined
  let previous = getAgentStatus()
  let signature = agentSignature(previous)
  const unsubscribe = subscribeAgentStatus(() => {
    const status = getAgentStatus()
    const next = agentSignature(status)
    const firstLoad = previous.isLoading
    previous = status
    if (next === signature) return
    signature = next
    // The first answer of the status is not a change: the reports were just
    // fetched on mount.
    if (firstLoad) return
    refreshRetained()
    // An agent that just connected sends its report right after registering,
    // so the first read may come too early: read again once it had the time.
    clearTimeout(followUp)
    followUp = setTimeout(refreshRetained, REPORT_SETTLE_MS)
  })
  // Coming back from the desktop app, where the settings were saved, is the
  // moment the report is most likely to have moved.
  const onFocus = () => refreshRetained()
  window.addEventListener('focus', onFocus)
  return () => {
    clearTimeout(followUp)
    unsubscribe()
    window.removeEventListener('focus', onFocus)
  }
}

function retain(projectId: string) {
  retained.set(projectId, (retained.get(projectId) || 0) + 1)
  if (!stopWatching) stopWatching = startWatching()
  if (!store.reports.has(projectId)) void refreshProjectEngine(projectId)
}

function release(projectId: string) {
  const count = (retained.get(projectId) || 0) - 1
  if (count > 0) retained.set(projectId, count)
  else retained.delete(projectId)
  if (retained.size === 0 && stopWatching) {
    stopWatching()
    stopWatching = null
  }
}

/**
 * The engine the caller's own workstation reported for a project (#305), or
 * null while the first answer is pending. Refreshed when the agent status
 * changes and when the window regains focus. A project no agent of the caller
 * serves reads as `{ state: 'unknown' }`.
 */
export function useProjectEngine(projectId: string | null | undefined): EngineReport | null {
  const id = (projectId || '').trim()
  const getSnapshot = useCallback(() => (id ? store.reports.get(id) ?? null : null), [id])
  const report = useSyncExternalStore(subscribe, getSnapshot)
  useEffect(() => {
    if (!id) return
    retain(id)
    return () => release(id)
  }, [id])
  return report
}

/** Retains each board project once; cards consume the resulting shared snapshot. */
export function useProjectEngines(projectIds: readonly (string | null | undefined)[]): ReadonlyMap<string, EngineReport> {
  const projectKey = JSON.stringify([...new Set(projectIds.map(id => (id || '').trim()).filter(Boolean))].sort())
  const reports = useSyncExternalStore(subscribe, store.getSnapshot)
  useEffect(() => {
    const ids: string[] = JSON.parse(projectKey)
    for (const id of ids) retain(id)
    return () => { for (const id of ids) release(id) }
  }, [projectKey])
  return reports
}
