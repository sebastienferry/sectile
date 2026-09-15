import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sameTask, tasksInProject } from '../src/lib/taskIdentity.ts'

const fretzee = { id: 'gh-39', key: '#39', projectId: 'default', title: 'Fretzee' }
const sectile = { id: 'gh-sectile-39', key: '#39', projectId: 'sectile', title: 'Sectile' }

test('identical public keys do not replace another project card or detail', () => {
  assert.equal(sameTask(fretzee, sectile), false)
  const updated = { ...sectile, title: 'Updated' }
  assert.equal(sameTask(sectile, updated), true)
  assert.deepEqual([fretzee, sectile].map(t => sameTask(t, updated) ? updated : t), [fretzee, updated])
  assert.equal(sameTask(fretzee, updated) ? updated : fretzee, fretzee)
})

test('project view rejects foreign tasks, including stale responses and new events', () => {
  assert.deepEqual(tasksInProject([fretzee, sectile], 'default'), [fretzee])
  assert.deepEqual(tasksInProject([sectile], 'default'), [])
  assert.deepEqual(tasksInProject([fretzee, sectile], 'all'), [fretzee, sectile])
})
