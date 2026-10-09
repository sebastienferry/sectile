// The colour a macro (an epic) is painted with on a board, decided in one
// place so that the web app and the desktop app paint a parent key alike
// (#806). The palette is the hex values of the web's ACCENT_COLORS, in order;
// a web test pins that the two stay equal.

export const EPIC_PALETTE = [
 '#6366f1', '#8b5cf6', '#10b981', '#f59e0b', '#f43f5e', '#06b6d4',
 '#3b82f6', '#f97316', '#00f0ff', '#d946ef', '#10f070', '#ffd000',
]

// 32-bit FNV-1a over the UTF-16 code units of `value`. Changing it repaints
// every epic on every board, which is why a test pins its output.
export function fnv1a(value) {
 let hash = 0x811c9dc5
 for (let i = 0; i < value.length; i++) {
  hash ^= value.charCodeAt(i)
  hash = Math.imul(hash, 0x01000193)
 }
 return hash >>> 0
}

// The palette index of a parent key, or -1 when the task has no parent. It
// depends on the trimmed key alone, so an epic keeps one colour in every view.
export function epicColorIndex(parentKey) {
 const key = (parentKey ?? '').trim()
 if (!key) return -1
 return fnv1a(key) % EPIC_PALETTE.length
}

// The hex colour of a parent key, or null when the task has no parent.
export function epicColorHex(parentKey) {
 const index = epicColorIndex(parentKey)
 return index < 0 ? null : EPIC_PALETTE[index]
}
