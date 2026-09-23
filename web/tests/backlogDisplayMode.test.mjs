import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BACKLOG_DISPLAY_MODE_STORAGE_KEY,
  loadBacklogRowDisplayMode,
  saveBacklogRowDisplayMode,
  toggleBacklogRowDisplayMode,
  isBacklogRowCondensed,
} from '../src/lib/backlogDisplayMode.ts'

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

test('defaults to the expanded row when storage is empty', () => {
  assert.equal(loadBacklogRowDisplayMode(createMockStorage()), 'expanded')
  assert.equal(isBacklogRowCondensed('condensed'), true)
  assert.equal(isBacklogRowCondensed('expanded'), false)
})

test('reads back what was saved', () => {
  const storage = createMockStorage()
  saveBacklogRowDisplayMode('condensed', storage)
  assert.equal(storage._map.get(BACKLOG_DISPLAY_MODE_STORAGE_KEY), 'condensed')
  assert.equal(loadBacklogRowDisplayMode(storage), 'condensed')

  saveBacklogRowDisplayMode('expanded', storage)
  assert.equal(loadBacklogRowDisplayMode(storage), 'expanded')
})

test('an unknown stored value answers the default rather than nothing', () => {
  const storage = createMockStorage({ [BACKLOG_DISPLAY_MODE_STORAGE_KEY]: 'dense' })
  assert.equal(loadBacklogRowDisplayMode(storage), 'expanded')
})

test('casing and surrounding spaces do not change the shape read', () => {
  const storage = createMockStorage({ [BACKLOG_DISPLAY_MODE_STORAGE_KEY]: '  CONDENSED ' })
  assert.equal(loadBacklogRowDisplayMode(storage), 'condensed')
})

test('a storage that throws leaves the default in place', () => {
  const throwing = {
    getItem() {
      throw new Error('storage disabled')
    },
    setItem() {
      throw new Error('storage disabled')
    },
  }
  assert.equal(loadBacklogRowDisplayMode(throwing), 'expanded')
  assert.doesNotThrow(() => saveBacklogRowDisplayMode('condensed', throwing))
})

test('the toggle goes both ways', () => {
  assert.equal(toggleBacklogRowDisplayMode('expanded'), 'condensed')
  assert.equal(toggleBacklogRowDisplayMode('condensed'), 'expanded')
})

test('the backlog keeps its own key, apart from the board and the roadmap', () => {
  assert.equal(BACKLOG_DISPLAY_MODE_STORAGE_KEY, 'sectile_backlog_display_mode')
  assert.notEqual(BACKLOG_DISPLAY_MODE_STORAGE_KEY, 'sectile_board_display_mode')
  assert.notEqual(BACKLOG_DISPLAY_MODE_STORAGE_KEY, 'sectile_roadmap_display_mode')
})
