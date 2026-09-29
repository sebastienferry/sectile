import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  PROJECT_HISTORY_KEY,
  PROJECT_HISTORY_LIMIT,
  parseProjectHistory,
  readProjectHistory,
  recordProjectOpening,
  writeProjectHistory,
} from '../src/lib/projectHistory.ts'

const at = iso => new Date(iso)

test('an opening goes to the front with its time, once', () => {
  const history = [
    { id: 'a', openedAt: '2026-09-01T10:00:00.000Z' },
    { id: 'b', openedAt: '2026-08-01T10:00:00.000Z' },
  ]
  const next = recordProjectOpening(history, 'b', at('2026-09-28T09:00:00.000Z'))
  assert.deepEqual(next, [
    { id: 'b', openedAt: '2026-09-28T09:00:00.000Z' },
    { id: 'a', openedAt: '2026-09-01T10:00:00.000Z' },
  ])
  // The input is left untouched.
  assert.equal(history[0].id, 'a')
})

test('the history keeps the most recent openings only', () => {
  let history = []
  for (let i = 0; i < PROJECT_HISTORY_LIMIT + 5; i++) {
    history = recordProjectOpening(history, `p${i}`, at('2026-09-28T09:00:00.000Z'))
  }
  assert.equal(history.length, PROJECT_HISTORY_LIMIT)
  assert.equal(history[0].id, `p${PROJECT_HISTORY_LIMIT + 4}`)
  assert.ok(!history.some(entry => entry.id === 'p0'))
})

test('unreadable stored values yield an empty history', () => {
  for (const raw of [null, '', '{', '"a"', '{"id":"a"}', '42', 'null']) {
    assert.deepEqual(parseProjectHistory(raw), [], `raw ${raw}`)
  }
})

test('malformed entries are dropped, the others kept', () => {
  const raw = JSON.stringify([
    { id: 'a', openedAt: '2026-09-28T09:00:00.000Z' },
    { id: '', openedAt: '2026-09-28T09:00:00.000Z' },
    { id: 42, openedAt: '2026-09-28T09:00:00.000Z' },
    { id: 'b', openedAt: 'yesterday' },
    { id: 'c' },
    null,
    'd',
    { id: 'a', openedAt: '2026-01-01T09:00:00.000Z' },
    { id: 'e', openedAt: '2026-09-27T09:00:00.000Z' },
  ])
  assert.deepEqual(parseProjectHistory(raw), [
    { id: 'a', openedAt: '2026-09-28T09:00:00.000Z' },
    { id: 'e', openedAt: '2026-09-27T09:00:00.000Z' },
  ])
})

function withStorage(storage, run) {
  const previous = Object.getOwnPropertyDescriptor(globalThis, 'localStorage')
  Object.defineProperty(globalThis, 'localStorage', { configurable: true, get: () => storage })
  try {
    run()
  } finally {
    if (previous) Object.defineProperty(globalThis, 'localStorage', previous)
    else delete globalThis.localStorage
  }
}

test('the history round-trips through storage', () => {
  const store = new Map()
  const storage = {
    getItem: key => (store.has(key) ? store.get(key) : null),
    setItem: (key, value) => store.set(key, String(value)),
  }
  withStorage(storage, () => {
    const history = recordProjectOpening([], 'a', at('2026-09-28T09:00:00.000Z'))
    writeProjectHistory(history)
    assert.ok(store.has(PROJECT_HISTORY_KEY))
    assert.deepEqual(readProjectHistory(), history)
  })
})

test('a storage that throws neither breaks reading nor writing', () => {
  const storage = {
    getItem: () => { throw new Error('SecurityError') },
    setItem: () => { throw new Error('QuotaExceededError') },
  }
  withStorage(storage, () => {
    assert.deepEqual(readProjectHistory(), [])
    assert.doesNotThrow(() => writeProjectHistory([{ id: 'a', openedAt: '2026-09-28T09:00:00.000Z' }]))
  })
  // No storage at all, as in a non-browser context.
  withStorage(undefined, () => {
    assert.deepEqual(readProjectHistory(), [])
    assert.doesNotThrow(() => writeProjectHistory([]))
  })
})
