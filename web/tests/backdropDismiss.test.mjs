import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createBackdropGesture, landedOnBackdrop } from '../src/lib/backdropDismiss.ts'

// The two nodes a gesture can land on: the dialog's backdrop, and anything the
// dialog itself renders.
const backdrop = { node: 'backdrop' }
const inside = { node: 'dialog panel' }

test('an event landing on the dialog is not on the backdrop', () => {
  assert.equal(landedOnBackdrop(backdrop, backdrop), true)
  assert.equal(landedOnBackdrop(inside, backdrop), false)
})

test('a press and a release on the backdrop close the dialog', () => {
  const gesture = createBackdropGesture()
  gesture.press(backdrop, backdrop)
  assert.equal(gesture.release(backdrop, backdrop), true)
})

test('a selection dragged out of the dialog does not close it', () => {
  // The click of such a drag reports the backdrop as its target, since that is
  // the common ancestor of the press and the release. Only the press tells the
  // two gestures apart.
  const gesture = createBackdropGesture()
  gesture.press(inside, backdrop)
  assert.equal(gesture.release(backdrop, backdrop), false)
})

test('a press on the backdrop released over the dialog does not close it', () => {
  const gesture = createBackdropGesture()
  gesture.press(backdrop, backdrop)
  assert.equal(gesture.release(inside, backdrop), false)
})

test('a gesture is spent once released', () => {
  const gesture = createBackdropGesture()
  gesture.press(backdrop, backdrop)
  assert.equal(gesture.release(backdrop, backdrop), true)
  // A click with no press of its own — a synthetic one, or the second release
  // of a double click reported without its press — closes nothing.
  assert.equal(gesture.release(backdrop, backdrop), false)
})

test('a missing target never closes the dialog', () => {
  // A press begun outside the window can leave an event without a usable
  // target; two absent targets are not the same node.
  assert.equal(landedOnBackdrop(null, backdrop), false)
  assert.equal(landedOnBackdrop(backdrop, null), false)
  assert.equal(landedOnBackdrop(null, null), false)

  const gesture = createBackdropGesture()
  gesture.press(null, backdrop)
  assert.equal(gesture.release(backdrop, backdrop), false)
})
