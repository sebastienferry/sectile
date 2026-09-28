import type { EngineReport } from '../types/index.ts'
import { normalizeEngineReport, UNKNOWN_ENGINE } from './aiModels.ts'

/** Shared reports and pending reads for every card on a board. */
export function createProjectEngineStore() {
  let reports: ReadonlyMap<string, EngineReport> = new Map()
  const pending = new Map<string, Promise<void>>()
  const listeners = new Set<() => void>()

  function refresh(projectId: string): Promise<void> {
    const existing = pending.get(projectId)
    if (existing) return existing

    // Register the promise before starting the read. Keep it until the body
    // has been consumed and published, so mounting cards join the same read.
    const request = Promise.resolve().then(async () => {
      let report: EngineReport
      try {
        const res = await fetch(`/api/projects/${encodeURIComponent(projectId)}/engine`)
        report = res.ok ? normalizeEngineReport(await res.json()) : UNKNOWN_ENGINE
      } catch {
        report = UNKNOWN_ENGINE
      }
      reports = new Map(reports).set(projectId, report)
      for (const listener of listeners) listener()
    }).finally(() => {
      pending.delete(projectId)
    })
    pending.set(projectId, request)
    return request
  }

  function subscribe(listener: () => void): () => void {
    listeners.add(listener)
    return () => { listeners.delete(listener) }
  }

  return { get reports() { return reports }, getSnapshot: () => reports, refresh, subscribe }
}
