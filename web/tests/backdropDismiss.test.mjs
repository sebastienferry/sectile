import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createBackdropGesture, landedOnBackdrop } from '../src/lib/backdropDismiss.ts'

// The two nodes a gesture can land on: the dialog's backdrop, and anything the
// dialog itself renders.
const backdrop = { node: 'backdrop' }
const inside = { node: 'dialog panel' }

// A press and a release always produce a click on their common ancestor, which
// is the backdrop as soon as either of them landed on it.
const gestureOn = (pressTarget, releaseTarget) => {
  const gesture = createBackdropGesture()
  gesture.press(pressTarget, backdrop)
  gesture.release(releaseTarget, backdrop)
  const clickTarget = pressTarget === releaseTarget ? pressTarget : backdrop
  return gesture.dismisses(clickTarget, backdrop)
}

test('an event landing on the dialog is not on the backdrop', () => {
  assert.equal(landedOnBackdrop(backdrop, backdrop), true)
  assert.equal(landedOnBackdrop(inside, backdrop), false)
})

test('a press and a release beside the dialog close it', () => {
  assert.equal(gestureOn(backdrop, backdrop), true)
})

test('a click inside the dialog leaves it open', () => {
  assert.equal(gestureOn(inside, inside), false)
})

test('a selection dragged out of the dialog leaves it open', () => {
  // The click of such a drag reports the backdrop as its target, since that is
  // the common ancestor of the press and the release. Only the press tells the
  // two gestures apart.
  assert.equal(gestureOn(inside, backdrop), false)
})

test('a press beside the dialog released over it leaves it open', () => {
  assert.equal(gestureOn(backdrop, inside), false)
})

test('a gesture is spent once read', () => {
  const gesture = createBackdropGesture()
  gesture.press(backdrop, backdrop)
  gesture.release(backdrop, backdrop)
  assert.equal(gesture.dismisses(backdrop, backdrop), true)
  // A click with no gesture of its own — a synthetic one, or a release the
  // browser reported without its press — closes nothing.
  assert.equal(gesture.dismisses(backdrop, backdrop), false)
})

test('a missing target never closes the dialog', () => {
  // A press begun outside the window can leave an event without a usable
  // target; two absent targets are not the same node.
  assert.equal(landedOnBackdrop(null, backdrop), false)
  assert.equal(landedOnBackdrop(backdrop, null), false)
  assert.equal(landedOnBackdrop(null, null), false)
  assert.equal(gestureOn(null, backdrop), false)
})
