import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  EMPTY_TRACKER_DRAFT,
  labellingProjects,
  normalizeTrackerScope,
  trackerDisplayName,
  trackerDraftOf,
  trackerDraftProblem,
  trackerPayload,
  trackerProjects,
  trackerSourceLocked,
} from '../src/lib/trackers.ts'

// The admin Trackers screen (#741): what a tracker draft must hold before it
// is sent, and what the creation or rewrite carries.

test('a tracker draft names its provider and its scope', () => {
  assert.equal(trackerDraftProblem(EMPTY_TRACKER_DRAFT), 'provider')
  assert.equal(trackerDraftProblem({ ...EMPTY_TRACKER_DRAFT, provider: 'jira' }), 'scope')
  assert.equal(trackerDraftProblem({ ...EMPTY_TRACKER_DRAFT, provider: 'jira', scope: '  ' }), 'scope')
  assert.equal(trackerDraftProblem({ ...EMPTY_TRACKER_DRAFT, provider: 'jira', scope: 'gode' }), null)
  assert.equal(trackerDraftProblem({ ...EMPTY_TRACKER_DRAFT, provider: 'gitlab', scope: 'group/sub/app' }), null)
})

test('a GitHub tracker names one owner/repo', () => {
  assert.equal(trackerDraftProblem({ ...EMPTY_TRACKER_DRAFT, provider: 'github', scope: 'acme' }), 'githubScope')
  assert.equal(trackerDraftProblem({ ...EMPTY_TRACKER_DRAFT, provider: 'github', scope: 'acme/app/extra' }), 'githubScope')
  assert.equal(trackerDraftProblem({ ...EMPTY_TRACKER_DRAFT, provider: 'github', scope: 'https://github.com/acme/app.git' }), null)
})

test('the scope is shown as the server stores it', () => {
  assert.equal(normalizeTrackerScope('jira', ' gode '), 'GODE')
  assert.equal(normalizeTrackerScope('github', 'git@github.com:acme/app.git'), 'acme/app')
  assert.equal(normalizeTrackerScope('github', 'https://github.com/acme/app'), 'acme/app')
  assert.equal(normalizeTrackerScope('gitlab', '/group/app/'), 'group/app')
})

test('a creation carries the trimmed source and default auto-sync settings', () => {
  const payload = trackerPayload({ provider: 'jira', name: ' Gode ', site: ' https://acme.atlassian.net ', scope: 'gode' })
  assert.deepEqual(payload, {
    provider: 'jira',
    name: 'Gode',
    site: 'https://acme.atlassian.net',
    scope: 'GODE',
    autoSyncEnabled: false,
    autoSyncIntervalMin: 5,
  })
})

test('a rewrite keeps the board mirror of the tracker it replaces', () => {
  // The server replaces the whole tracker: a rewrite that dropped the columns
  // would wipe the status mapping every project of the tracker reads.
  const current = {
    id: 't1', provider: 'jira', name: 'Gode', site: '', scope: 'GODE', identity: 'jira|acme|GODE',
    boardId: '7', trackerColumns: [{ name: 'Done', statuses: ['Done'] }], stageColumns: { finished: ['Done'] },
    issueTypes: ['Story'], autoSyncEnabled: true, autoSyncIntervalMin: 10,
  }
  const payload = trackerPayload({ provider: 'jira', name: 'GODE space', site: '', scope: 'GODE' }, current)
  assert.equal(payload.id, 't1')
  assert.equal(payload.name, 'GODE space')
  assert.equal(payload.boardId, '7')
  assert.deepEqual(payload.trackerColumns, current.trackerColumns)
  assert.deepEqual(payload.stageColumns, current.stageColumns)
  assert.deepEqual(payload.issueTypes, ['Story'])
  assert.equal(payload.autoSyncEnabled, true)
  assert.equal(payload.autoSyncIntervalMin, 10)
})

test('a tracker holding tickets keeps its source, and its count is never sent back', () => {
  assert.equal(trackerSourceLocked({}), false)
  assert.equal(trackerSourceLocked({ ticketCount: 0 }), false)
  assert.equal(trackerSourceLocked({ ticketCount: 3 }), true)
  const current = { id: 't1', provider: 'jira', name: 'Gode', site: '', scope: 'GODE', identity: 'jira|acme|GODE', autoSyncEnabled: false, autoSyncIntervalMin: 5, ticketCount: 3 }
  const payload = trackerPayload({ provider: 'jira', name: 'Gode', site: '', scope: 'GODE' }, current)
  assert.equal('ticketCount' in payload, false)
  assert.equal(payload.id, 't1')
})

test('a tracker reopens in the form as it was saved', () => {
  assert.deepEqual(trackerDraftOf({ provider: 'github', name: 'App', site: '', scope: 'acme/app' }), {
    provider: 'github', name: 'App', site: '', scope: 'acme/app',
  })
})

test('the projects of a tracker are those selecting it, and only labelled ones take a backlog ticket', () => {
  const projects = [
    { id: 'da', name: 'Delivery admin', label: 'delivery-admin', trackers: [{ trackerId: 'gode' }, { trackerId: 'be' }] },
    { id: 'ba', name: 'Bidder admin', label: '', trackers: [{ trackerId: 'gode' }] },
    { id: 'x', name: 'Other', label: 'other', trackers: [{ trackerId: 'be' }] },
    { id: 'old', name: 'Old', label: 'old' },
  ]
  assert.deepEqual(trackerProjects('gode', projects).map(p => p.id), ['da', 'ba'])
  assert.deepEqual(labellingProjects('gode', projects).map(p => p.id), ['da'])
  assert.deepEqual(trackerProjects('none', projects), [])
})

test('a tracker reads as its name, then its scope when they differ', () => {
  assert.equal(trackerDisplayName({ name: 'GODE', scope: 'GODE' }), 'GODE')
  assert.equal(trackerDisplayName({ name: 'Delivery', scope: 'GODE' }), 'Delivery (GODE)')
  assert.equal(trackerDisplayName({ name: '', scope: 'acme/app' }), 'acme/app')
})
