import assert from 'node:assert/strict'
import { test } from 'node:test'
import { createProjectEngineStore } from '../src/lib/projectEngineStore.ts'

function deferred() {
  let resolve
  const promise = new Promise(done => { resolve = done })
  return { promise, resolve }
}

test('hundreds of mounted cards share one request until its body is read', async t => {
  const body = deferred()
  const urls = []
  t.mock.method(globalThis, 'fetch', async url => {
    urls.push(url)
    return { ok: true, json: () => body.promise }
  })
  const store = createProjectEngineStore()
  let notifications = 0
  const unsubscribe = store.subscribe(() => { notifications++ })
  const reads = Array.from({ length: 500 }, () => store.refresh('project/a'))
  await Promise.resolve()
  reads.push(store.refresh('project/a'))
  assert.deepEqual(urls, ['/api/projects/project%2Fa/engine'])
  assert.equal(store.reports.size, 0)
  body.resolve({ state: 'reported', provider: 'claude' })
  await Promise.all(reads)
  assert.equal(notifications, 1)
  assert.equal(store.reports.get('project/a').provider, 'claude')
  unsubscribe()
  await store.refresh('project/a')
  assert.equal(urls.length, 2)
  assert.equal(notifications, 1)
})

test('different projects can load independently', async t => {
  const first = deferred()
  const urls = []
  t.mock.method(globalThis, 'fetch', async url => {
    urls.push(url)
    return { ok: true, json: () => url.includes('/first/') ? first.promise : { state: 'unknown' } }
  })
  const store = createProjectEngineStore()
  const pending = store.refresh('first')
  await store.refresh('second')
  assert.equal(urls.length, 2)
  assert.equal(store.reports.has('first'), false)
  assert.equal(store.reports.get('second').state, 'unknown')
  first.resolve({ state: 'unknown' })
  await pending
})

for (const failure of ['network', 'http', 'json']) {
  test(`a ${failure} failure releases the pending read for a later refresh`, async t => {
    let calls = 0
    t.mock.method(globalThis, 'fetch', async () => {
      calls++
      if (calls === 1) {
        if (failure === 'network') throw new TypeError('Failed to fetch')
        return {
          ok: failure !== 'http',
          json: async () => { throw new SyntaxError('Invalid JSON') },
        }
      }
      return { ok: true, json: async () => ({ state: 'reported', provider: 'claude' }) }
    })
    const store = createProjectEngineStore()
    await Promise.all(Array.from({ length: 500 }, () => store.refresh('project')))
    assert.equal(calls, 1)
    assert.equal(store.reports.get('project').state, 'unknown')
    await store.refresh('project')
    assert.equal(calls, 2)
    assert.equal(store.reports.get('project').provider, 'claude')
  })
}
