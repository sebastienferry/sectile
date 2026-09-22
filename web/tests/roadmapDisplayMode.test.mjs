import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  ROADMAP_DISPLAY_MODE_STORAGE_KEY,
  loadRoadmapRowDisplayMode,
  saveRoadmapRowDisplayMode,
  toggleRoadmapRowDisplayMode,
  isRoadmapRowCondensed,
  CONDENSED_HORIZONS,
  HORIZON_SHORT,
} from '../src/lib/roadmapDisplayMode.ts'

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
  assert.equal(loadRoadmapRowDisplayMode(createMockStorage()), 'expanded')
  assert.equal(isRoadmapRowCondensed('condensed'), true)
  assert.equal(isRoadmapRowCondensed('expanded'), false)
})

test('reads back what was saved', () => {
  const storage = createMockStorage()
  saveRoadmapRowDisplayMode('condensed', storage)
  assert.equal(storage._map.get(ROADMAP_DISPLAY_MODE_STORAGE_KEY), 'condensed')
  assert.equal(loadRoadmapRowDisplayMode(storage), 'condensed')

  saveRoadmapRowDisplayMode('expanded', storage)
  assert.equal(loadRoadmapRowDisplayMode(storage), 'expanded')
})

test('a stored value that is no longer a known form falls back to expanded', () => {
  const storage = createMockStorage({ [ROADMAP_DISPLAY_MODE_STORAGE_KEY]: 'compact' })
  assert.equal(loadRoadmapRowDisplayMode(storage), 'expanded')
})

test('an unreadable storage is a missing preference, not a failure', () => {
  const broken = {
    getItem() {
      throw new Error('storage disabled')
    },
    setItem() {
      throw new Error('storage disabled')
    },
  }
  assert.equal(loadRoadmapRowDisplayMode(broken), 'expanded')
  assert.doesNotThrow(() => saveRoadmapRowDisplayMode('condensed', broken))
})

test('toggling goes back and forth', () => {
  assert.equal(toggleRoadmapRowDisplayMode('expanded'), 'condensed')
  assert.equal(toggleRoadmapRowDisplayMode('condensed'), 'expanded')
})

test('the condensed row offers the three horizons one classifies into, never hiding', () => {
  assert.deepEqual(CONDENSED_HORIZONS, ['now', 'next', 'later'])
  assert.ok(!CONDENSED_HORIZONS.includes('hidden'))
})

test('every horizon has a two-letter form, and they stay distinct', () => {
  const shorts = ['now', 'next', 'later', 'hidden'].map(h => HORIZON_SHORT[h])
  assert.equal(shorts.length, 4)
  for (const short of shorts) {
    assert.equal(short.length, 2, `« ${short} » devrait tenir en deux lettres`)
  }
  assert.equal(new Set(shorts).size, shorts.length, 'deux horizons ne peuvent pas partager leurs deux lettres')
})
