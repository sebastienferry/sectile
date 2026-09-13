import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BOARD_DISPLAY_MODE_STORAGE_KEY,
  LEGACY_BOARD_DISPLAY_MODE_STORAGE_KEY,
  loadBoardCardDisplayMode,
  saveBoardCardDisplayMode,
  toggleBoardCardDisplayMode,
  isBoardCardCondensed,
} from '../src/lib/boardDisplayMode.ts'

function createMockStorage(initial = {}) {
  const map = new Map(Object.entries(initial))
  return {
    getItem(key) {
      return map.has(key) ? map.get(key) : null
    },
    setItem(key, val) {
      map.set(key, String(val))
    },
    _map: map,
  }
}

test('defaults to condensed when storage is empty', () => {
  const storage = createMockStorage()
  assert.equal(loadBoardCardDisplayMode(storage), 'condensed')
  assert.equal(isBoardCardCondensed('condensed'), true)
  assert.equal(isBoardCardCondensed('expanded'), false)
})

test('loads expanded mode from primary storage key', () => {
  const storage = createMockStorage({ [BOARD_DISPLAY_MODE_STORAGE_KEY]: 'expanded' })
  assert.equal(loadBoardCardDisplayMode(storage), 'expanded')
})

test('loads condensed mode from primary storage key', () => {
  const storage = createMockStorage({ [BOARD_DISPLAY_MODE_STORAGE_KEY]: 'condensed' })
  assert.equal(loadBoardCardDisplayMode(storage), 'condensed')
})

test('falls back to legacy storage key when primary is absent', () => {
  const storage = createMockStorage({ [LEGACY_BOARD_DISPLAY_MODE_STORAGE_KEY]: 'expanded' })
  assert.equal(loadBoardCardDisplayMode(storage), 'expanded')
})

test('supports boolean representation fallbacks (true -> condensed, false -> expanded)', () => {
  const storageFalse = createMockStorage({ [BOARD_DISPLAY_MODE_STORAGE_KEY]: 'false' })
  assert.equal(loadBoardCardDisplayMode(storageFalse), 'expanded')

  const storageTrue = createMockStorage({ [BOARD_DISPLAY_MODE_STORAGE_KEY]: 'true' })
  assert.equal(loadBoardCardDisplayMode(storageTrue), 'condensed')
})

test('saves display mode to primary storage key', () => {
  const storage = createMockStorage()
  saveBoardCardDisplayMode('expanded', storage)
  assert.equal(storage.getItem(BOARD_DISPLAY_MODE_STORAGE_KEY), 'expanded')

  saveBoardCardDisplayMode('condensed', storage)
  assert.equal(storage.getItem(BOARD_DISPLAY_MODE_STORAGE_KEY), 'condensed')
})

test('toggles between condensed and expanded', () => {
  assert.equal(toggleBoardCardDisplayMode('condensed'), 'expanded')
  assert.equal(toggleBoardCardDisplayMode('expanded'), 'condensed')
})
