import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  OPTIONAL_VIEWS,
  enabledOptionalViews,
  isOptionalView,
  isViewAvailable,
  resolveAvailableView,
} from '../src/lib/optionalViews.ts'

const project = enabledViews => ({ id: 'p1', name: 'P', enabledViews })

test('the three planning views are the optional ones, in sidebar order', () => {
  assert.deepEqual(OPTIONAL_VIEWS, ['triage', 'roadmap', 'timeline'])
  for (const view of OPTIONAL_VIEWS) {
    assert.equal(isOptionalView(view), true)
  }
  for (const view of ['board', 'list', 'activities', 'sync', 'skills', 'team']) {
    assert.equal(isOptionalView(view), false)
  }
})

test('a project shows no optional view until it asks for one', () => {
  assert.deepEqual(enabledOptionalViews(project(undefined)), [])
  assert.deepEqual(enabledOptionalViews(project([])), [])
  assert.deepEqual(enabledOptionalViews(null), [])
  assert.deepEqual(enabledOptionalViews(undefined), [])
})

test('enabled views come back in canonical order, without the unknown ones', () => {
  assert.deepEqual(enabledOptionalViews(project(['timeline', 'triage'])), ['triage', 'timeline'])
  assert.deepEqual(enabledOptionalViews(project(['gantt', 'roadmap'])), ['roadmap'])
  assert.deepEqual(enabledOptionalViews(project([' Roadmap ', 'TIMELINE'])), ['roadmap', 'timeline'])
})

test('a non-optional view is always available, whatever the project says', () => {
  for (const view of ['board', 'list', 'activities']) {
    assert.equal(isViewAvailable(project([]), view), true)
    assert.equal(isViewAvailable(null, view), true)
  }
})

test('an optional view is available only to a project that enabled it', () => {
  assert.equal(isViewAvailable(project(['triage']), 'triage'), true)
  assert.equal(isViewAvailable(project(['triage']), 'roadmap'), false)
  assert.equal(isViewAvailable(project([]), 'timeline'), false)
  // Sans projet sélectionné, ces vues n'ont pas de données à montrer.
  assert.equal(isViewAvailable(null, 'roadmap'), false)
})

test('an unavailable view falls back rather than leaving a dead screen', () => {
  assert.equal(resolveAvailableView(project(['roadmap']), 'roadmap'), 'roadmap')
  assert.equal(resolveAvailableView(project(['roadmap']), 'timeline'), 'board')
  assert.equal(resolveAvailableView(project([]), 'triage', 'list'), 'list')
  assert.equal(resolveAvailableView(project([]), 'board'), 'board')
})
