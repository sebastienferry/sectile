import { useEffect } from 'react'
import type { RefObject } from 'react'

type Target = RefObject<HTMLElement | null>

/**
 * Closes an anchored menu when the press lands outside it.
 *
 * Unlike a modal dialog, a menu has no backdrop to click on: it floats over
 * whatever is underneath, so the only way to know the press went elsewhere is
 * to watch the document. `mousedown` rather than `click`, so the menu is gone
 * before whatever sits under the pointer reacts.
 *
 * Several targets can be given, for a menu whose panel is rendered through a
 * portal and is therefore not inside its own anchor. Refs are stable for the
 * life of the component, so only `enabled` and `onOutside` decide when the
 * listener is replaced — pass a memoised `onOutside`.
 *
 * Two call sites deliberately keep their own listener: `TaskCard`, whose menu
 * also closes on scroll and on resize and ranks two levels of `Escape`, and
 * `McpSessions`, which installs its `mousedown` and its `keydown` in one
 * effect.
 */
export function useClickOutside(targets: Target | Target[], onOutside: () => void, enabled = true): void {
  useEffect(() => {
    if (!enabled) return
    const refs = Array.isArray(targets) ? targets : [targets]
    const onDocClick = (e: MouseEvent) => {
      const target = e.target as Node
      if (refs.some(ref => ref.current?.contains(target))) return
      onOutside()
    }
    document.addEventListener('mousedown', onDocClick)
    return () => document.removeEventListener('mousedown', onDocClick)
  }, [enabled, onOutside, targets])
}
