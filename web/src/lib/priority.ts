import type { Priority } from '../types/index.ts'

/** Every priority list, from the most to the least urgent. */
export const PRIORITY_LEVELS: readonly Priority[] = ['urgent', 'high', 'medium', 'low']

/**
 * The colour of the priority dot, shared by the card, the backlog row, the
 * filter bar, the pinned bar and the edit forms so that they cannot drift apart.
 * The roadmap badges use their own palette (`PRIORITY_META` in roadmap.ts).
 */
export const PRIORITY_COLORS: Record<Priority, string> = {
  urgent: 'var(--status-danger)',
  high: 'var(--status-warn)',
  medium: 'var(--status-info)',
  low: 'var(--text-muted)',
}

const FALLBACK_COLOR = 'var(--text-muted)'

/** The dot colour; a value outside the four levels, which a tracker can send, reads as low. */
export function priorityColor(priority: string | null | undefined): string {
  return Object.hasOwn(PRIORITY_COLORS, priority ?? '') ? PRIORITY_COLORS[priority as Priority] : FALLBACK_COLOR
}
