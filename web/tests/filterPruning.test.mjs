import assert from 'node:assert/strict'
import { test } from 'node:test'
import { staleFilters } from '../src/lib/filterPruning.ts'

const UNASSIGNED = '__unassigned__'
const facets = (scope, sprints = [], teams = [], assignees = []) => ({ scope, sprints, teams, assignees })
const filters = (sprint = null, team = null, assignee = null) => ({ sprint, team, assignee })

test('values read for another project drop nothing', () => {
  const previous = facets('b', ['S1'], ['Ops'], ['Bob'])
  assert.deepEqual(staleFilters(filters('S12', 'Core', 'Alice'), previous, 'a', UNASSIGNED), [])
})

test('values not read yet drop nothing', () => {
  assert.deepEqual(staleFilters(filters('S12', 'Core', 'Alice'), facets(null, ['S1'], ['Ops'], ['Bob']), 'a', UNASSIGNED), [])
})

test('a view and a project are different scopes', () => {
  assert.deepEqual(staleFilters(filters(null, 'Core'), facets('a', [], ['Ops']), 'view_v1', UNASSIGNED), [])
})

test('a value gone from its own board is dropped', () => {
  const own = facets('a', ['S13'], ['Ops'], ['Bob'])
  assert.deepEqual(staleFilters(filters('S12', 'Core', 'Alice'), own, 'a', UNASSIGNED), ['sprint', 'team', 'assignee'])
})

test('a value still on its own board is kept', () => {
  const own = facets('a', ['S12'], ['Core'], ['Alice'])
  assert.deepEqual(staleFilters(filters('S12', 'Core', 'Alice'), own, 'a', UNASSIGNED), [])
})

test('an empty list drops nothing', () => {
  assert.deepEqual(staleFilters(filters('S12', 'Core', 'Alice'), facets('a'), 'a', UNASSIGNED), [])
})

test('no filter set, nothing to drop', () => {
  assert.deepEqual(staleFilters(filters(), facets('a', ['S1'], ['Ops'], ['Bob']), 'a', UNASSIGNED), [])
})

test('unassigned is never dropped', () => {
  assert.deepEqual(staleFilters(filters(null, null, UNASSIGNED), facets('a', [], [], ['Bob']), 'a', UNASSIGNED), [])
})
