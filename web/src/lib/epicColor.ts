import { ACCENT_COLORS, type AccentDefinition } from './accents.ts'

/**
 * 32-bit FNV-1a over the UTF-16 code units of `value`. Changing it repaints
 * every epic on every board, which is why a test pins its output.
 */
export function fnv1a(value: string): number {
  let hash = 0x811c9dc5
  for (let i = 0; i < value.length; i++) {
    hash ^= value.charCodeAt(i)
    hash = Math.imul(hash, 0x01000193)
  }
  return hash >>> 0
}

/**
 * The palette entry an epic is painted with, or null when the task has no
 * parent. It depends on the key alone, so an epic keeps one colour in every
 * view and for every user whatever other epics are shown; two epics may share
 * a colour once the palette is exhausted.
 */
export function epicColor(parentKey?: string | null): AccentDefinition | null {
  const key = (parentKey ?? '').trim()
  if (!key) return null
  return ACCENT_COLORS[fnv1a(key) % ACCENT_COLORS.length]
}

/** The hex colour of an epic, or null when the task has no parent. */
export function epicColorHex(parentKey?: string | null): string | null {
  return epicColor(parentKey)?.hex ?? null
}
