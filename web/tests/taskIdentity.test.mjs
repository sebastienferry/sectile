import assert from 'node:assert/strict'
import { test } from 'node:test'
import { sameTask, tasksInProject } from '../src/lib/taskIdentity.ts'

const fretzee = { id: 'gh-39', key: '#39', projectId: 'default', title: 'Fretzee' }
const taskflow = { id: 'gh-taskflow-39', key: '#39', projectId: 'taskflow', title: 'TaskFlow' }

test('identical public keys do not replace another project card or detail', () => {
  assert.equal(sameTask(fretzee, taskflow), false)
  const updated = { ...taskflow, title: 'Updated' }
  assert.equal(sameTask(taskflow, updated), true)
  assert.deepEqual([fretzee, taskflow].map(t => sameTask(t, updated) ? updated : t), [fretzee, updated])
  assert.equal(sameTask(fretzee, updated) ? updated : fretzee, fretzee)
})

test('project view rejects foreign tasks, including stale responses and new events', () => {
  assert.deepEqual(tasksInProject([fretzee, taskflow], 'default'), [fretzee])
  assert.deepEqual(tasksInProject([taskflow], 'default'), [])
  assert.deepEqual(tasksInProject([fretzee, taskflow], 'all'), [fretzee, taskflow])
})
