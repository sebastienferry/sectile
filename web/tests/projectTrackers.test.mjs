import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  addTracker,
  effectiveDefaultTracker,
  isLocalIdentity,
  missingTrackerReason,
  moveTracker,
  projectSelectionPayload,
  removeTracker,
  selectedTrackerIds,
  trackerOptions,
} from '../src/lib/projectTrackers.ts'

// The "Trackers & label" tab of the project settings (#741): the order of the
// trackers, the default and its fallback, and the label.

const summaries = [
  { id: 'gode', name: 'GODE', provider: 'jira', site: '', scope: 'GODE', identity: 'jira|acme.atlassian.net|GODE' },
  { id: 'be', name: 'BE', provider: 'jira', site: '', scope: 'BE', identity: 'jira|acme.atlassian.net|BE' },
  { id: 'app', name: 'acme/app', provider: 'github', site: '', scope: 'acme/app', identity: 'github|api.github.com|acme/app' },
]

test('the selected trackers keep the order they are moved into', () => {
  let selected = addTracker([], 'gode')
  selected = addTracker(selected, 'be')
  selected = addTracker(selected, 'be')
  assert.deepEqual(selected, ['gode', 'be'])
  selected = moveTracker(selected, 1, -1)
  assert.deepEqual(selected, ['be', 'gode'])
  // Moving past either end changes nothing.
  assert.deepEqual(moveTracker(selected, 0, -1), ['be', 'gode'])
  assert.deepEqual(moveTracker(selected, 1, 1), ['be', 'gode'])
  assert.deepEqual(removeTracker(selected, 'be'), ['gode'])
})

test('the default falls back to the first tracker when it is not selected', () => {
  assert.equal(effectiveDefaultTracker(['gode', 'be'], 'be'), 'be')
  assert.equal(effectiveDefaultTracker(['gode', 'be'], ''), 'gode')
  assert.equal(effectiveDefaultTracker(['gode', 'be'], 'app'), 'gode')
  assert.equal(effectiveDefaultTracker([], 'app'), '')
})

test('the payload names the trackers in order with their identity, the default and the trimmed label', () => {
  const options = trackerOptions(summaries)
  assert.deepEqual(projectSelectionPayload(['be', 'gode'], 'gode', '  delivery-admin ', options), {
    label: 'delivery-admin',
    trackers: [
      { trackerId: 'be', identity: 'jira|acme.atlassian.net|BE' },
      { trackerId: 'gode', identity: 'jira|acme.atlassian.net|GODE' },
    ],
    defaultTrackerId: 'gode',
  })
  // An empty label is sent too: the project then shows every ticket of its trackers.
  assert.equal(projectSelectionPayload(['gode'], '', '', options).label, '')
  // A default that left the selection falls back to the first tracker.
  assert.equal(projectSelectionPayload(['be', 'gode'], 'app', '', options).defaultTrackerId, 'be')
})

test('an unchanged selection is not resent', () => {
  const options = trackerOptions(summaries)
  const original = [{ trackerId: 'gode', identity: 'jira|acme.atlassian.net|GODE' }]
  const unchanged = projectSelectionPayload(['gode'], 'gode', 'da', options, original)
  assert.equal(unchanged.trackers, undefined)
  assert.equal(unchanged.defaultTrackerId, 'gode')
})

test('a new project cannot be saved without a recorded tracker', () => {
  const options = trackerOptions(summaries)
  // A new project must pick one: the server never creates a tracker for it.
  assert.equal(missingTrackerReason(true, [], options), 'pick')
  assert.equal(missingTrackerReason(true, ['gode'], options), null)
  // With no tracker recorded at all, there is nothing to pick: an admin must add one.
  assert.equal(missingTrackerReason(true, [], trackerOptions([])), 'noneRecorded')
  assert.equal(missingTrackerReason(true, [], trackerOptions([{ id: 'loc', name: '', provider: 'local', site: '', scope: 'p1', identity: 'local||p1' }])), 'noneRecorded')
  // A saved project keeps the trackers it has, its own local board included.
  const own = [{ trackerId: 'loc', identity: 'local||p1' }]
  assert.equal(missingTrackerReason(false, ['loc'], trackerOptions([], own)), null)
  assert.equal(missingTrackerReason(false, [], options), null)
})

test("the project's own local board stays offered and selected", () => {
  // GET /api/trackers never lists local boards: a save built only from it
  // would unlink the board a local project's tickets live on.
  const own = [{ trackerId: 'loc', identity: 'local||p1' }]
  assert.equal(isLocalIdentity('local||p1'), true)
  assert.equal(isLocalIdentity('jira|acme|GODE'), false)
  const options = trackerOptions(summaries, own)
  const local = options.find(option => option.id === 'loc')
  assert.ok(local)
  assert.equal(local.local, true)
  assert.deepEqual(selectedTrackerIds(own), ['loc'])
  const payload = projectSelectionPayload(['loc', 'gode'], 'loc', '', options, own)
  assert.deepEqual(payload.trackers, [
    { trackerId: 'loc', identity: 'local||p1' },
    { trackerId: 'gode', identity: 'jira|acme.atlassian.net|GODE' },
  ])
})
