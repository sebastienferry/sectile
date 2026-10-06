export interface AnchoredMenuPosition {
  left: number
  top?: number
  bottom?: number
  maxHeight: number
}

const MENU_MAX_HEIGHT = 380
const MENU_GAP = 6
const MARGIN = 8
// Room kept for the global status bar at the bottom of the window.
const BOTTOM_RESERVE = 36

/**
 * Where a fixed popup of `width` opens next to `anchor`: right-aligned with it,
 * kept on screen, below it unless there is clearly more room above.
 *
 * Popups of a card are rendered in a portal because the board columns scroll
 * and would clip them; the portal is zoomed along with the interface, so the
 * anchor's rectangle is divided by that zoom first.
 */
export function anchoredMenuPosition(anchor: HTMLElement, width: number): AnchoredMenuPosition {
  const rect = anchor.getBoundingClientRect()
  const zoom = parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--ui-zoom')) || 1

  const anchorTop = rect.top / zoom
  const anchorBottom = rect.bottom / zoom
  const anchorRight = rect.right / zoom
  const vpHeight = window.innerHeight / zoom
  const vpWidth = window.innerWidth / zoom

  const spaceAbove = anchorTop - MARGIN
  const spaceBelow = vpHeight - anchorBottom - BOTTOM_RESERVE
  const left = Math.max(MARGIN, Math.min(anchorRight - width, vpWidth - width - MARGIN))

  if (spaceBelow >= 220 || spaceBelow >= spaceAbove) {
    return { left, top: anchorBottom + MENU_GAP, maxHeight: Math.max(140, Math.min(MENU_MAX_HEIGHT, spaceBelow - MENU_GAP)) }
  }
  return { left, bottom: vpHeight - anchorTop + MENU_GAP, maxHeight: Math.max(140, Math.min(MENU_MAX_HEIGHT, spaceAbove - MENU_GAP)) }
}

/**
 * Arrow keys, Home and End move the focus between the items of a menu, wrapping
 * at both ends. Returns whether the key was one of them.
 */
export function moveMenuFocus(menu: HTMLElement, key: string): boolean {
  if (!['ArrowDown', 'ArrowUp', 'Home', 'End'].includes(key)) return false
  const items = Array.from(menu.querySelectorAll<HTMLElement>('[role^="menuitem"]:not(:disabled)'))
  if (items.length === 0) return true
  // Focus outside the items: ArrowDown starts at the first, ArrowUp at the last.
  const found = items.indexOf(document.activeElement as HTMLElement)
  const current = found >= 0 ? found : key === 'ArrowDown' ? -1 : 0
  const next = key === 'Home' ? 0
    : key === 'End' ? items.length - 1
      : key === 'ArrowDown' ? (current + 1) % items.length
        : (current - 1 + items.length) % items.length
  items[next].focus({ preventScroll: true })
  return true
}
