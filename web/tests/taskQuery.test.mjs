import assert from 'node:assert/strict'
import { test } from 'node:test'
import { SELF_FILTERING_VIEWS, sendsServerSearch } from '../src/lib/taskQuery.ts'

test('the roadmap and the timeline filter their own rows', () => {
  assert.deepEqual([...SELF_FILTERING_VIEWS].sort(), ['roadmap', 'timeline'])
  assert.equal(sendsServerSearch('roadmap'), false)
  assert.equal(sendsServerSearch('timeline'), false)
})

test('every other view sends the header search to the server', () => {
  for (const view of ['board', 'list', 'triage', 'activities', 'sync', 'skills', 'team', 'admin']) {
    assert.equal(sendsServerSearch(view), true, view)
  }
})
