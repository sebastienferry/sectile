import { useCallback, useEffect, useSyncExternalStore } from 'react'
import type { EngineReport } from '../types'
import { normalizeEngineReport, UNKNOWN_ENGINE } from '../lib/aiModels'
import { getAgentStatus, subscribeAgentStatus, type AgentStatusState } from './useAgentStatus'

const API_BASE = '/api'

// One cache for the whole page, keyed by project: a board shows many cards of
// the same project, and each would otherwise fetch the same report.
const reports = new Map<string, EngineReport>()
// Latest request per project, so an answer overtaken by a newer one is dropped.
const requestSeq = new Map<string, number>()
// Mounted readers per project; only these are refreshed.
const retained = new Map<string, number>()
const listeners = new Set<() => void>()
let stopWatching: (() => void) | null = null

function publish() {
  for (const listener of listeners) listener()
}

/** Fetches the caller's own workstation report for one project. */
export async function refreshProjectEngine(projectId: string): Promise<void> {
  const seq = (requestSeq.get(projectId) || 0) + 1
  requestSeq.set(projectId, seq)
  let report: EngineReport
  try {
    const res = await fetch(`${API_BASE}/projects/${encodeURIComponent(projectId)}/engine`)
    report = res.ok ? normalizeEngineReport(await res.json()) : UNKNOWN_ENGINE
  } catch {
    report = UNKNOWN_ENGINE
  }
  if (requestSeq.get(projectId) !== seq) return
  reports.set(projectId, report)
  publish()
}

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
  if (!reports.has(projectId)) void refreshProjectEngine(projectId)
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

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
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
  const getSnapshot = useCallback(() => (id ? reports.get(id) ?? null : null), [id])
  const report = useSyncExternalStore(subscribe, getSnapshot)
  useEffect(() => {
    if (!id) return
    retain(id)
    return () => release(id)
  }, [id])
  return report
}
