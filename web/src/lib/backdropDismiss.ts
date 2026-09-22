/**
 * Closing a modal dialog by clicking beside it.
 *
 * The naive test — `event.target === event.currentTarget` on the backdrop's
 * `click` — closes on gestures that never meant to close anything. `click`
 * fires on the nearest common ancestor of the press and the release, so
 * selecting text inside the dialog and releasing the button over the backdrop
 * reports the backdrop as its target, and the dialog disappears with whatever
 * was being typed in it.
 *
 * So the press is checked too: the dialog closes only when the whole gesture,
 * press and release, happened on the backdrop itself.
 */

/** True when this event landed on the backdrop rather than inside the dialog. */
export function landedOnBackdrop(target: EventTarget | null, backdrop: EventTarget | null): boolean {
  return target !== null && backdrop !== null && target === backdrop
}

export interface BackdropGesture {
  /** Records where a press landed. */
  press: (target: EventTarget | null, backdrop: EventTarget | null) => void
  /** True when the completed gesture asks for the dialog to close. */
  release: (target: EventTarget | null, backdrop: EventTarget | null) => boolean
}

/**
 * Follows one press-and-release at a time. A gesture is spent as soon as it is
 * released, so a press inside the dialog can never lend its verdict to the next
 * click.
 */
export function createBackdropGesture(): BackdropGesture {
  let pressedBackdrop = false
  return {
    press(target, backdrop) {
      pressedBackdrop = landedOnBackdrop(target, backdrop)
    },
    release(target, backdrop) {
      const dismiss = pressedBackdrop && landedOnBackdrop(target, backdrop)
      pressedBackdrop = false
      return dismiss
    },
  }
}
