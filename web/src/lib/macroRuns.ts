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
  userId?: string
  projectId: string
}

/** The run that makes the macro busy, if any: the most recent running one. */
export function activeMacroRun(runs: MacroRun[]): MacroRun | null {
  return runs.find(run => run.status === 'running' || run.status === 'queued') || null
}

/**
 * Whether the launch button is offered, and the French reason when it is not.
 * An agent registered without a project serves every project, as the server's
 * routing does; only the signed-in user's agents count.
 */
export function macroLaunchBlocker(agents: AgentPresence[], projectId: string, runs: MacroRun[], userId = ''): string | null {
  // The server routes a launch to the caller's own agent: another person's
  // agent on a shared server does not make the button usable.
  const mine = userId ? agents.filter(agent => !agent.userId || agent.userId === userId) : agents
  if (!mine.some(agent => !agent.projectId || agent.projectId === projectId)) {
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

/** Stops a macro run; force closes it when the agent cannot be reached. */
export async function cancelMacroRun(projectId: string, macroKey: string, runId: string, force = false): Promise<void> {
  const res = await fetch(macroURL(projectId, macroKey, 'cancel-run'), {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ runId, force }),
  })
  if (!res.ok) {
    const data = await res.json().catch(() => ({}))
    throw new Error(data.error || `Arrêt refusé (${res.status})`)
  }
}
