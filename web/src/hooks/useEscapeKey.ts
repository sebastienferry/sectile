import { useEffect } from 'react'

/**
 * Closes a dialog on `Escape`, and only that dialog.
 *
 * The interface already listens for `Escape` in several places at once: every
 * modal installs its own handler on `window`, and `AppContext` keeps a ranked
 * one for the surfaces it owns. Adding a plain `window` listener to a dialog
 * stacked over another — `TrackerSetup` over `ProjectModal`, say — would close
 * both on one key press, since listeners on the same target fire in the order
 * they were registered and the dialog underneath registered first.
 *
 * So the key is caught on the way down, on `document`, and stopped there: by
 * the time it would reach the `window` handlers, it has already been spent by
 * the topmost dialog. One press closes one layer, as one click does.
 */
export function useEscapeKey(enabled: boolean, onEscape: () => void): void {
  useEffect(() => {
    if (!enabled) return
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.stopPropagation()
      onEscape()
    }
    document.addEventListener('keydown', handleKeyDown, true)
    return () => document.removeEventListener('keydown', handleKeyDown, true)
  }, [enabled, onEscape])
}
