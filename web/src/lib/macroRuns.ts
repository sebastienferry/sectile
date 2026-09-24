/**
 * Macro skill runs: a skill such as realign-macro launched on the local agent
 * for a macro rather than for a task. The server records them on the macro
 * (GET /api/projects/{id}/macros/{key}/runs) and refuses a second launch while
 * one is running.
 */

export interface MacroRun {
  id: string
  macroKey?: string
  skillName: string
  status: string
  summary: string
  createdAt: string
  completedAt?: string
}

export interface AgentPresence {
  projectId: string
}

/** The run that makes the macro busy, if any: the most recent running one. */
export function activeMacroRun(runs: MacroRun[]): MacroRun | null {
  return runs.find(run => run.status === 'running' || run.status === 'queued') || null
}

/**
 * Whether the launch button is offered, and the French reason when it is not.
 * An agent registered without a project serves every project, as the server's
 * routing does.
 */
export function macroLaunchBlocker(agents: AgentPresence[], projectId: string, runs: MacroRun[]): string | null {
  if (!agents.some(agent => !agent.projectId || agent.projectId === projectId)) {
    return "Connectez l'agent local pour lancer le réalignement."
  }
  if (activeMacroRun(runs)) {
    return 'Un réalignement est déjà en cours sur cette macro.'
  }
  return null
}

const API_BASE = '/api'

function macroURL(projectId: string, macroKey: string, action: string): string {
  return `${API_BASE}/projects/${encodeURIComponent(projectId)}/macros/${encodeURIComponent(macroKey)}/${action}`
}

export async function fetchMacroRuns(projectId: string, macroKey: string): Promise<MacroRun[]> {
  const res = await fetch(macroURL(projectId, macroKey, 'runs'))
  if (!res.ok) return []
  const data = await res.json().catch(() => ({}))
  return Array.isArray(data.runs) ? data.runs : []
}

/** Launches a macro skill; resolves with the run, rejects with the server's reason. */
export async function launchMacroSkill(projectId: string, macroKey: string, skillId: string): Promise<MacroRun> {
  const res = await fetch(macroURL(projectId, macroKey, 'run-skill'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ skillId }),
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || `Lancement refusé (${res.status})`)
  return data.activity
}
