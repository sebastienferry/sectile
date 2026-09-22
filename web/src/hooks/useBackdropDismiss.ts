import { useCallback, useMemo } from 'react'
import type { MouseEvent } from 'react'
import { createBackdropGesture } from '../lib/backdropDismiss'

interface BackdropProps {
  onMouseDown: (e: MouseEvent) => void
  onClick: (e: MouseEvent) => void
}

/**
 * The props a modal dialog spreads on its backdrop to close on a click beside
 * it: `const backdrop = useBackdropDismiss(onClose)`, then
 * `<div className="fixed …" {...backdrop}>`.
 *
 * Spreading both handlers at once is the point — wiring the click alone brings
 * back the drag that closes the dialog (see `lib/backdropDismiss.ts`).
 *
 * The handlers live on the backdrop element rather than on `document`, so an
 * open dialog costs nothing to the board's drag-and-drop, and a dialog stacked
 * over another closes alone: the event never reaches the backdrop underneath.
 *
 * `onClose` is called as given. Each dialog passes the very function its close
 * button uses, so clicking outside can never save or discard more than the
 * button does.
 */
export function useBackdropDismiss(onClose: () => void): BackdropProps {
  // The gesture is state that must never cause a render: it is written by the
  // press and read by the click that immediately follows it.
  const gesture = useMemo(() => createBackdropGesture(), [])

  const onMouseDown = useCallback((e: MouseEvent) => {
    gesture.press(e.target, e.currentTarget)
  }, [gesture])

  const onClick = useCallback((e: MouseEvent) => {
    if (gesture.release(e.target, e.currentTarget)) onClose()
  }, [gesture, onClose])

  return { onMouseDown, onClick }
}
