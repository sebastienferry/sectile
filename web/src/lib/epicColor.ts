import type { Project } from '../types/index.ts'
import { ACCENT_COLORS, type AccentDefinition } from './accents.ts'
import { epicColorIndex } from '../../../shared/epicColor.mjs'

/**
 * 32-bit FNV-1a over the UTF-16 code units of a key, shared with the desktop
 * app so a macro keeps one colour in both clients.
 */
export { fnv1a } from '../../../shared/epicColor.mjs'

/**
 * The palette entry an epic is painted with, or null when the task has no
 * parent. It depends on the key alone, so an epic keeps one colour in every
 * view and for every user whatever other epics are shown; two epics may share
 * a colour once the palette is exhausted.
 */
export function epicColor(parentKey?: string | null): AccentDefinition | null {
  const index = epicColorIndex(parentKey)
  return index < 0 ? null : ACCENT_COLORS[index]
}

/** The hex colour of an epic, or null when the task has no parent. */
export function epicColorHex(parentKey?: string | null): string | null {
  return epicColor(parentKey)?.hex ?? null
}

/**
 * Whether cards of this project carry their epic's colour. The setting is off
 * unless the project asks for it. A task that does not name its project is read
 * against `fallback`, the project the workspace is showing.
 */
export function epicColorsEnabled(
  projects: readonly Pick<Project, 'id' | 'epicColors'>[],
  projectId?: string | null,
  fallback?: Pick<Project, 'id' | 'epicColors'> | null
): boolean {
  const project = projectId ? projects.find(p => p.id === projectId) : fallback
  return project?.epicColors === true
}
