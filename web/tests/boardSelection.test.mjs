import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  SELECTABLE_STAGES,
  isSelectableStage,
  isSelectionClick,
  toggleSelected,
  pruneSelection,
  orderSelection,
  shouldEscapeClearSelection,
} from '../src/lib/boardSelection.ts'

test('only the new and clarified stages are selectable', () => {
  assert.deepEqual([...SELECTABLE_STAGES], ['new', 'clarified'])
  assert.equal(isSelectableStage('new'), true)
  assert.equal(isSelectableStage('clarified'), true)
  for (const stage of ['specified', 'implemented', 'reviewed', 'finished']) {
    assert.equal(isSelectableStage(stage), false, stage)
  }
})

test('Ctrl and Cmd make a selection click, a bare click does not', () => {
  assert.equal(isSelectionClick({ ctrlKey: true, metaKey: false }), true)
  assert.equal(isSelectionClick({ ctrlKey: false, metaKey: true }), true)
  assert.equal(isSelectionClick({ ctrlKey: false, metaKey: false }), false)
})

test('toggling adds then removes an id without mutating its input', () => {
  const empty = new Set()
  const one = toggleSelected(empty, 'a')
  assert.deepEqual([...one], ['a'])
  assert.equal(empty.size, 0)
  const none = toggleSelected(one, 'a')
  assert.equal(none.size, 0)
  assert.deepEqual([...one], ['a'])
})

test('pruning returns the same Set when every id is still selectable on screen', () => {
  const selected = new Set(['a', 'b'])
  assert.equal(pruneSelection(selected, ['b', 'c', 'a']), selected)
  const empty = new Set()
  assert.equal(pruneSelection(empty, []), empty)
})

test('pruning drops the ids that left the screen or are no longer selectable', () => {
  const selected = new Set(['a', 'b', 'c'])
  const pruned = pruneSelection(selected, ['c', 'a'])
  assert.notEqual(pruned, selected)
  assert.deepEqual([...pruned].sort(), ['a', 'c'])
  assert.equal(pruneSelection(selected, []).size, 0)
  assert.equal(selected.size, 3)
})

test('the selection comes out in board order, whatever order it was built in', () => {
  const selected = new Set(['clarified-1', 'new-2', 'new-1'])
  const board = ['new-1', 'new-2', 'new-3', 'clarified-1', 'specified-1']
  assert.deepEqual(orderSelection(selected, board), ['new-1', 'new-2', 'clarified-1'])
})

test('ordering ignores the ids that are not on the board', () => {
  assert.deepEqual(orderSelection(new Set(['gone', 'a']), ['a', 'b']), ['a'])
})

const bareEscape = {
  key: 'Escape',
  defaultPrevented: false,
  appSurfaceOpen: false,
  inputFocused: false,
  modalOpen: false,
}

test('a bare Escape clears the selection', () => {
  assert.equal(shouldEscapeClearSelection(bareEscape), true)
})

test('Escape leaves the selection alone when something else takes the key', () => {
  assert.equal(shouldEscapeClearSelection({ ...bareEscape, key: 'Enter' }), false)
  assert.equal(shouldEscapeClearSelection({ ...bareEscape, defaultPrevented: true }), false, 'a card menu spent it')
  assert.equal(shouldEscapeClearSelection({ ...bareEscape, appSurfaceOpen: true }), false, 'an AppContext surface or a search')
  assert.equal(shouldEscapeClearSelection({ ...bareEscape, inputFocused: true }), false, 'a field or a terminal')
  assert.equal(shouldEscapeClearSelection({ ...bareEscape, modalOpen: true }), false, 'an aria-modal dialog')
})
