import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  ROADMAP_PANEL_HIDDEN_STORAGE_KEY,
  ROADMAP_TAB_STORAGE_KEY,
  loadRoadmapFlag,
  loadRoadmapSelectedKey,
  loadRoadmapTab,
  roadmapSelectedKeyStorageKey,
  saveRoadmapFlag,
  saveRoadmapSelectedKey,
  saveRoadmapTab,
} from '../src/lib/roadmapViewPrefs.ts'

function createMockStorage(initial = {}) {
  const map = new Map(Object.entries(initial))
  return {
    getItem(key) {
      return map.has(key) ? map.get(key) : null
    },
    setItem(key, val) {
      map.set(key, String(val))
    },
    map,
  }
}

const refusingStorage = {
  getItem() {
    throw new Error('denied')
  },
  setItem() {
    throw new Error('denied')
  },
}

test('an empty storage answers the defaults', () => {
  const storage = createMockStorage()
  assert.equal(loadRoadmapTab(storage), 'now')
  assert.equal(loadRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, false, storage), false)
  assert.equal(loadRoadmapFlag('sectile_roadmap_description_open', true, storage), true)
  assert.equal(loadRoadmapSelectedKey('p1', storage), null)
})

test('the tab round-trips, and an unknown one answers NOW', () => {
  const storage = createMockStorage()
  for (const tab of ['now', 'next', 'later', 'unclassified', 'hidden']) {
    saveRoadmapTab(tab, storage)
    assert.equal(loadRoadmapTab(storage), tab)
  }
  storage.setItem(ROADMAP_TAB_STORAGE_KEY, 'someday')
  assert.equal(loadRoadmapTab(storage), 'now')
})

test('a flag round-trips, and a foreign value answers the fallback', () => {
  const storage = createMockStorage()
  saveRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, true, storage)
  assert.equal(storage.map.get(ROADMAP_PANEL_HIDDEN_STORAGE_KEY), '1')
  assert.equal(loadRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, false, storage), true)
  saveRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, false, storage)
  assert.equal(loadRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, true, storage), false)
  storage.setItem(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, 'true')
  assert.equal(loadRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, false, storage), false)
})

test('the selected macro is kept per project', () => {
  const storage = createMockStorage()
  saveRoadmapSelectedKey('project-a', 'M-12', storage)
  saveRoadmapSelectedKey('project-b', 'M-30', storage)
  assert.equal(loadRoadmapSelectedKey('project-a', storage), 'M-12')
  assert.equal(loadRoadmapSelectedKey('project-b', storage), 'M-30')
  assert.equal(storage.map.get(roadmapSelectedKeyStorageKey('project-a')), 'M-12')

  saveRoadmapSelectedKey('project-a', null, storage)
  assert.equal(loadRoadmapSelectedKey('project-a', storage), null)
  assert.equal(loadRoadmapSelectedKey('project-b', storage), 'M-30')
  assert.equal(loadRoadmapSelectedKey('', storage), null)
})

test('a refusing storage answers the defaults and never throws', () => {
  assert.equal(loadRoadmapTab(refusingStorage), 'now')
  assert.equal(loadRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, false, refusingStorage), false)
  assert.equal(loadRoadmapSelectedKey('p1', refusingStorage), null)
  assert.doesNotThrow(() => saveRoadmapTab('later', refusingStorage))
  assert.doesNotThrow(() => saveRoadmapFlag(ROADMAP_PANEL_HIDDEN_STORAGE_KEY, true, refusingStorage))
  assert.doesNotThrow(() => saveRoadmapSelectedKey('p1', 'M-1', refusingStorage))
})

test('the grouping axis round trips and falls back to none', async () => {
  const { loadRoadmapGroupAxis, saveRoadmapGroupAxis, ROADMAP_GROUP_AXIS_STORAGE_KEY } = await import('../src/lib/roadmapViewPrefs.ts')
  const storage = createMockStorage()
  assert.equal(loadRoadmapGroupAxis(storage), 'none')
  saveRoadmapGroupAxis('quarter', storage)
  assert.equal(storage.map.get(ROADMAP_GROUP_AXIS_STORAGE_KEY), 'quarter')
  assert.equal(loadRoadmapGroupAxis(storage), 'quarter')
  assert.equal(loadRoadmapGroupAxis(createMockStorage({ [ROADMAP_GROUP_AXIS_STORAGE_KEY]: 'readiness' })), 'none')
  assert.equal(loadRoadmapGroupAxis(refusingStorage), 'none')
})

test('the folded sections round trip and tolerate foreign values', async () => {
  const { loadRoadmapFoldedSections, saveRoadmapFoldedSections, ROADMAP_FOLDED_SECTIONS_STORAGE_KEY } = await import('../src/lib/roadmapViewPrefs.ts')
  const storage = createMockStorage()
  assert.deepEqual(loadRoadmapFoldedSections(storage), [])
  saveRoadmapFoldedSections(new Set(['priority:p1', 'quarter:none']), storage)
  assert.deepEqual(loadRoadmapFoldedSections(storage), ['priority:p1', 'quarter:none'])
  assert.deepEqual(loadRoadmapFoldedSections(createMockStorage({ [ROADMAP_FOLDED_SECTIONS_STORAGE_KEY]: '{"a":1}' })), [])
  assert.deepEqual(loadRoadmapFoldedSections(createMockStorage({ [ROADMAP_FOLDED_SECTIONS_STORAGE_KEY]: 'nope' })), [])
  assert.deepEqual(loadRoadmapFoldedSections(createMockStorage({ [ROADMAP_FOLDED_SECTIONS_STORAGE_KEY]: '["x",2]' })), ['x'])
  assert.deepEqual(loadRoadmapFoldedSections(refusingStorage), [])
  saveRoadmapFoldedSections(['x'], refusingStorage)
})
