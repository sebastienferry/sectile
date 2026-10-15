/**
 * Macro addresses for views that only hold tickets (#809). The Timeline knows a
 * ticket's macro by its key; the address of that macro comes from the
 * project's macro list, where the server computes it. No address is ever
 * derived from a key: a macro without one stays plain text.
 */
import type { MacroMeta, Task } from '../types'

/** The projects whose macro list is needed: those of tickets with a macro, sorted. */
export function macroProjectIds(tasks: Task[], fallbackProjectId = ''): string[] {
  const ids = new Set<string>()
  for (const task of tasks) {
    if (!task.parentKey?.trim()) continue
    const id = task.projectId || fallbackProjectId
    if (id) ids.add(id)
  }
  return [...ids].sort()
}

/** Index key of a macro address: one project can reuse another's macro key. */
export function macroUrlKey(projectId: string, macroKey: string): string {
  return `${projectId}\n${macroKey}`
}

/** Builds the address index from each project's macro list; macros without externalUrl are left out. */
export function macroUrlIndex(byProject: Record<string, MacroMeta[]>): Map<string, string> {
  const index = new Map<string, string>()
  for (const [projectId, macros] of Object.entries(byProject)) {
    for (const macro of macros) {
      if (macro.externalUrl) index.set(macroUrlKey(projectId, macro.key), macro.externalUrl)
    }
  }
  return index
}
