import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  applyTargetPickerValue,
  isDefaultOriginSelection,
  loadOriginSelection,
  macroOrigin,
  matchesOrigins,
  normalizeOriginSelection,
  offeredOrigins,
  roadmapOriginsStorageKey,
  roadmapTargetOptions,
  saveOriginSelection,
  targetPickerValue,
} from '../src/lib/roadmapOrigins.ts'

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

const project = { issueTracker: 'jira', jiraProject: 'PE', roadmapProjects: ['DATA', 'OPS'] }
const rows = [
  { key: 'PE-1' },
  { key: 'PE-2' },
  { key: 'DATA-12', meta: { origin: 'DATA' } },
  { key: 'OLD-3' },
  { key: 'M-4' },
]

test('macroOrigin reads the key prefix and ignores milestones', () => {
  assert.equal(macroOrigin('DATA-12'), 'DATA')
  assert.equal(macroOrigin('pe-2'), 'PE')
  assert.equal(macroOrigin('M-4'), '')
  assert.equal(macroOrigin('NOKEY'), '')
})

test('the own key comes first, then the declared keys, then the carried ones', () => {
  assert.deepEqual(offeredOrigins(project, rows), [
    { key: 'PE', count: 3, own: true },
    { key: 'DATA', count: 1, own: false },
    { key: 'OPS', count: 0, own: false },
    { key: 'OLD', count: 1, own: false },
  ])
})

test('the counts follow the other filters while the list does not', () => {
  const offered = offeredOrigins(project, rows, [{ key: 'PE-1' }])
  assert.deepEqual(
    offered.map(o => [o.key, o.count]),
    [
      ['PE', 1],
      ['DATA', 0],
      ['OPS', 0],
      ['OLD', 0],
    ]
  )
})

test('nothing is offered without a declaration or a foreign epic, or off Jira', () => {
  assert.deepEqual(offeredOrigins({ issueTracker: 'jira', jiraProject: 'PE' }, [{ key: 'PE-1' }]), [])
  assert.deepEqual(offeredOrigins({ ...project, issueTracker: 'github' }, rows), [])
  assert.deepEqual(offeredOrigins(null, rows), [])
  assert.equal(offeredOrigins({ issueTracker: 'jira', jiraProject: 'PE' }, [{ key: 'OLD-3' }]).length, 2)
})

test('the selection drops what is no longer offered and falls back to the own key', () => {
  const offered = offeredOrigins(project, rows)
  assert.deepEqual(normalizeOriginSelection([], offered, 'PE'), ['PE'])
  assert.deepEqual(normalizeOriginSelection(['GONE'], offered, 'PE'), ['PE'])
  assert.deepEqual(normalizeOriginSelection(['data', 'DATA', 'OPS'], offered, 'PE'), ['DATA', 'OPS'])
  assert.ok(isDefaultOriginSelection(['PE'], 'PE'))
  assert.ok(!isDefaultOriginSelection(['PE', 'DATA'], 'PE'))
})

test('a row passes when its origin is selected; rows without one belong to the own key', () => {
  assert.ok(matchesOrigins({ key: 'PE-1' }, ['PE'], 'PE'))
  assert.ok(!matchesOrigins({ key: 'DATA-12' }, ['PE'], 'PE'))
  assert.ok(matchesOrigins({ key: 'DATA-12' }, ['PE', 'DATA'], 'PE'))
  assert.ok(matchesOrigins({ key: 'M-4' }, ['PE'], 'PE'))
  assert.ok(matchesOrigins({ key: 'DATA-12' }, [], 'PE'))
})

test('the selection is kept per project and tolerates a refusing or damaged storage', () => {
  const storage = createMockStorage()
  saveOriginSelection('p1', ['PE', 'DATA'], storage)
  assert.deepEqual(loadOriginSelection('p1', storage), ['PE', 'DATA'])
  assert.deepEqual(loadOriginSelection('p2', storage), [])
  storage.map.set(roadmapOriginsStorageKey('p3'), '{not json')
  assert.deepEqual(loadOriginSelection('p3', storage), [])
  assert.deepEqual(loadOriginSelection('p1', refusingStorage), [])
  assert.doesNotThrow(() => saveOriginSelection('p1', ['PE'], refusingStorage))
})

test('the declared keys of a Jira project are the roadmap targets', () => {
  assert.deepEqual(roadmapTargetOptions(project), ['DATA', 'OPS'])
  assert.deepEqual(roadmapTargetOptions({ ...project, roadmapProjects: ['data', 'PE', 'DATA'] }), ['DATA'])
  assert.deepEqual(roadmapTargetOptions({ ...project, issueTracker: 'gitlab' }), [])
})

test('the picker value keeps one target per line', () => {
  assert.equal(targetPickerValue({}, 'p1'), '')
  assert.equal(targetPickerValue({ targetProjectId: 'p1' }, 'p1'), '')
  assert.equal(targetPickerValue({ targetProjectId: 'p2' }, 'p1'), 'p2')
  assert.equal(targetPickerValue({ targetProjectId: 'p2', targetTrackerProject: 'data' }, 'p1'), 'tracker:DATA')

  const line = { id: 'l1', text: 'Do it', targetProjectId: 'p2' }
  assert.deepEqual(applyTargetPickerValue(line, 'tracker:DATA'), { id: 'l1', text: 'Do it', targetProjectId: undefined, targetTrackerProject: 'DATA' })
  assert.deepEqual(applyTargetPickerValue({ ...line, targetTrackerProject: 'DATA' }, 'p3'), { id: 'l1', text: 'Do it', targetProjectId: 'p3', targetTrackerProject: undefined })
  assert.deepEqual(applyTargetPickerValue(line, ''), { id: 'l1', text: 'Do it', targetProjectId: undefined, targetTrackerProject: undefined })
})
