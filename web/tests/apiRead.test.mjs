import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  CORE_READS,
  coreFailures,
  failureDetail,
  formatReadFailure,
  readJson,
  withOutcome,
} from '../src/lib/apiRead.ts'

/** Un `fetch` de remplacement qui rend exactement la réponse demandée. */
function stubFetch(handler) {
  const previous = globalThis.fetch
  globalThis.fetch = handler
  return () => { globalThis.fetch = previous }
}

function jsonResponse(status, body, { invalid = false } = {}) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => {
      if (invalid) throw new Error('Unexpected token < in JSON')
      return body
    },
  }
}

test('a body that parses is the only outcome that carries data', async () => {
  const restore = stubFetch(async () => jsonResponse(200, [{ id: 'p1' }]))
  try {
    const outcome = await readJson('/api/projects')
    assert.equal(outcome.kind, 'ok')
    assert.deepEqual(outcome.data, [{ id: 'p1' }])
  } finally {
    restore()
  }
})

test('the 500 that looked like an empty deployment is now a failed read', async () => {
  const restore = stubFetch(async () => jsonResponse(500, { error: 'column p.enabled_views does not exist' }))
  try {
    const outcome = await readJson('/api/projects')
    assert.equal(outcome.kind, 'failed')
    assert.equal(outcome.status, 500)
    assert.equal(outcome.detail, 'column p.enabled_views does not exist')
    assert.equal(failureDetail(outcome), 'HTTP 500 — column p.enabled_views does not exist')
  } finally {
    restore()
  }
})

test('a non-ok answer with no usable body still names its status', async () => {
  const restore = stubFetch(async () => jsonResponse(503, null, { invalid: true }))
  try {
    const outcome = await readJson('/api/tasks')
    assert.equal(outcome.kind, 'failed')
    assert.equal(outcome.detail, '')
    assert.equal(failureDetail(outcome), 'HTTP 503')
  } finally {
    restore()
  }
})

test('a 401 stays silent: the sign-in redirect owns it', async () => {
  const restore = stubFetch(async () => jsonResponse(401, { error: 'sign in' }))
  try {
    const outcome = await readJson('/api/settings')
    assert.equal(outcome.kind, 'silent')
    assert.equal(outcome.status, 401)
  } finally {
    restore()
  }
})

test('a request that never reached the server has no status to quote', async () => {
  const restore = stubFetch(async () => { throw new TypeError('Failed to fetch') })
  try {
    const outcome = await readJson('/api/projects')
    assert.equal(outcome.kind, 'failed')
    assert.equal(outcome.status, null)
    assert.equal(failureDetail(outcome), 'Failed to fetch')
  } finally {
    restore()
  }
})

test('a 200 whose body is not JSON is a failed read, not an empty one', async () => {
  const restore = stubFetch(async () => jsonResponse(200, null, { invalid: true }))
  try {
    const outcome = await readJson('/api/projects')
    assert.equal(outcome.kind, 'failed')
    assert.equal(outcome.status, 200)
    assert.match(outcome.detail, /Unexpected token/)
  } finally {
    restore()
  }
})

test('the detail never repeats the status it already carries', () => {
  assert.equal(failureDetail({ status: 500, detail: 'HTTP 500' }), 'HTTP 500')
  assert.equal(failureDetail({ status: 500, detail: '  ' }), 'HTTP 500')
  assert.equal(failureDetail({ status: null, detail: '' }), 'network error')
})

test('a translated sentence takes the resource and the detail, once each', () => {
  assert.equal(
    formatReadFailure('{resource} : le serveur a répondu {detail}.', 'Projets', 'HTTP 500'),
    'Projets : le serveur a répondu HTTP 500.',
  )
  // Un détail qui contient lui-même un marqueur ne s'étend pas une seconde fois.
  assert.equal(
    formatReadFailure('{resource}: {detail}', 'Tickets', '{resource}'),
    'Tickets: {resource}',
  )
})

test('a read that comes back drops out of the failing list', () => {
  const failing = withOutcome([], 'projects', { kind: 'failed', status: 500, detail: '' })
  assert.deepEqual(failing, [{ resource: 'projects', detail: 'HTTP 500' }])
  assert.deepEqual(withOutcome(failing, 'projects', { kind: 'ok', data: [] }), [])
})

test('a 401 changes nothing, so signing in never clears a real failure', () => {
  const failing = [{ resource: 'projects', detail: 'HTTP 500' }]
  assert.equal(withOutcome(failing, 'projects', { kind: 'silent', status: 401 }), failing)
})

test('an unchanged failure keeps its identity, so polling does not rerender', () => {
  const failing = [{ resource: 'tasks', detail: 'HTTP 500' }]
  const again = withOutcome(failing, 'tasks', { kind: 'failed', status: 500, detail: '' })
  assert.equal(again, failing)
  const healthy = []
  assert.equal(withOutcome(healthy, 'tasks', { kind: 'ok', data: [] }), healthy)
})

test('a failure that changes keeps its place in the list', () => {
  const failing = [
    { resource: 'projects', detail: 'HTTP 500' },
    { resource: 'tasks', detail: 'HTTP 500' },
  ]
  const next = withOutcome(failing, 'projects', { kind: 'failed', status: 503, detail: '' })
  assert.deepEqual(next.map(f => f.resource), ['projects', 'tasks'])
  assert.equal(next[0].detail, 'HTTP 503')
})

test('only projects and tasks degrade the whole interface', () => {
  assert.deepEqual(CORE_READS, ['projects', 'tasks'])
  const failures = [
    { resource: 'settings', detail: 'HTTP 500' },
    { resource: 'projects', detail: 'HTTP 500' },
    { resource: 'boardViews', detail: 'HTTP 500' },
  ]
  assert.deepEqual(coreFailures(failures), [{ resource: 'projects', detail: 'HTTP 500' }])
  assert.deepEqual(coreFailures([{ resource: 'settings', detail: 'HTTP 500' }]), [])
})
