/**
 * Closing a modal dialog by clicking beside it.
 *
 * The naive test — `event.target === event.currentTarget` on the backdrop's
 * `click` — closes on gestures that never meant to close anything. `click`
 * fires on the nearest common ancestor of the press and the release, so a
 * selection started inside the dialog and released over the backdrop reports
 * the backdrop as its target, and the dialog disappears with whatever was being
 * typed in it. The reverse, pressing beside the dialog and releasing over it,
 * reads exactly the same.
 *
 * So the whole gesture is followed: the dialog closes only when the press, the
 * release and the click all landed on the backdrop itself.
 */

/** True when this event landed on the backdrop rather than inside the dialog. */
export function landedOnBackdrop(target: EventTarget | null, backdrop: EventTarget | null): boolean {
  return target !== null && backdrop !== null && target === backdrop
}

export interface BackdropGesture {
  /** Records where a press landed, starting a new gesture. */
  press: (target: EventTarget | null, backdrop: EventTarget | null) => void
  /** Records where the release landed. */
  release: (target: EventTarget | null, backdrop: EventTarget | null) => void
  /** True when the gesture just completed asks for the dialog to close. */
  dismisses: (clickTarget: EventTarget | null, backdrop: EventTarget | null) => boolean
}

/**
 * Follows one press-and-release at a time. A gesture is spent as soon as it is
 * read, so a press inside the dialog can never lend its verdict to the next
 * click.
 */
export function createBackdropGesture(): BackdropGesture {
  let pressed = false
  let released = false
  return {
    press(target, backdrop) {
      pressed = landedOnBackdrop(target, backdrop)
      released = false
    },
    release(target, backdrop) {
      released = landedOnBackdrop(target, backdrop)
    },
    dismisses(clickTarget, backdrop) {
      const dismiss = pressed && released && landedOnBackdrop(clickTarget, backdrop)
      pressed = false
      released = false
      return dismiss
    },
  }
}
