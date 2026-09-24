import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  boardViewFormError,
  filterScopeKey,
  foldViewLabel,
  initialLabelsForView,
  normalizeViewLabels,
  readViewParam,
  withViewParam,
} from '../src/lib/boardViews.ts'

const view = labels => ({ id: 'v1', name: 'Platform', projectIds: ['a'], labels, createdAt: '', updatedAt: '' })

test('a view remembers its filters apart from every project', () => {
  assert.equal(filterScopeKey('p1', null), 'p1')
  assert.equal(filterScopeKey('all', null), 'all')
  assert.equal(filterScopeKey('', null), 'all')
  assert.equal(filterScopeKey('all', 'v1'), 'view_v1')
  assert.equal(filterScopeKey('p1', 'v1'), 'view_v1')
})

test('the address names the open view', () => {
  assert.equal(readViewParam('?view=v1'), 'v1')
  assert.equal(readViewParam('?task=%23387&view=v1'), 'v1')
  assert.equal(readViewParam('?view='), null)
  assert.equal(readViewParam(''), null)
})

test('opening and leaving a view keeps the other parameters', () => {
  assert.equal(withViewParam('http://x/?task=t1', 'v1'), '/?task=t1&view=v1')
  assert.equal(withViewParam('http://x/?view=old', 'v2'), '/?view=v2')
  assert.equal(withViewParam('http://x/?task=t1&view=v1#h', null), '/?task=t1#h')
  assert.equal(withViewParam('http://x/', null), '/')
})

test('only a single-label view prefills the label of a new ticket', () => {
  assert.deepEqual(initialLabelsForView(view(['platform'])), ['platform'])
  assert.deepEqual(initialLabelsForView(view(['api', 'ui'])), [])
  assert.deepEqual(initialLabelsForView(view([])), [])
  assert.deepEqual(initialLabelsForView(null), [])
})

test('labels are trimmed, deduplicated regardless of case, first spelling kept', () => {
  assert.deepEqual(normalizeViewLabels([' Platform ', '', 'platform', 'ops', 'OPS']), ['Platform', 'ops'])
  // Accents are not case folded: the selection treats them as two labels, so
  // the form must keep both rather than silently drop one (docs/adrs/0025).
  assert.deepEqual(normalizeViewLabels(['\u00c9quipe', '\u00e9quipe']), ['\u00c9quipe', '\u00e9quipe'])
  assert.equal(foldViewLabel('\u00c9QUIPE'), '\u00c9quipe')
})

test('the form says what prevents saving', () => {
  const others = [{ id: 'v1', name: 'Platform' }]
  assert.equal(boardViewFormError('  ', ['a'], others, null), 'name')
  assert.equal(boardViewFormError(' platform ', ['a'], others, null), 'duplicate')
  assert.equal(boardViewFormError('PLATFORM', ['a'], others, 'v1'), null)
  assert.equal(boardViewFormError('Other', [], others, null), 'projects')
  assert.equal(boardViewFormError('Other', ['a'], others, null), null)
})
