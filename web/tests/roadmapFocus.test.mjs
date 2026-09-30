import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  TICKET_VIEWS,
  canOpenEpicInRoadmap,
  isTicketView,
  locateEpic,
  projectOfTask,
  returnView,
} from '../src/lib/roadmapFocus.ts'

const project = enabledViews => ({ id: 'p1', name: 'P', enabledViews })
const row = (key, horizon, closed = false) => ({ key, horizon, closed })

test('the ticket views are the board, the list, the triage and the timeline', () => {
  assert.deepEqual(TICKET_VIEWS, ['board', 'list', 'triage', 'timeline'])
  for (const view of TICKET_VIEWS) assert.equal(isTicketView(view), true)
  for (const view of ['roadmap', 'activities', 'sync', 'skills', 'team', 'admin', null, undefined]) {
    assert.equal(isTicketView(view), false)
  }
})

test('a ticket opens its epic only with a parent key and a roadmap on its project', () => {
  const withRoadmap = project(['roadmap'])
  assert.equal(canOpenEpicInRoadmap({ parentKey: 'PE-1' }, withRoadmap), true)
  assert.equal(canOpenEpicInRoadmap({ parentKey: '' }, withRoadmap), false)
  assert.equal(canOpenEpicInRoadmap({ parentKey: '   ' }, withRoadmap), false)
  assert.equal(canOpenEpicInRoadmap({}, withRoadmap), false)
  assert.equal(canOpenEpicInRoadmap({ parentKey: 'PE-1' }, project(['triage', 'timeline'])), false)
  assert.equal(canOpenEpicInRoadmap({ parentKey: 'PE-1' }, project(undefined)), false)
  assert.equal(canOpenEpicInRoadmap({ parentKey: 'PE-1' }, null), false)
  assert.equal(canOpenEpicInRoadmap({ parentKey: 'PE-1' }, undefined), false)
})

test('an epic is found on the tab of its horizon', () => {
  const rows = [row('PE-1', 'now'), row('PE-2', 'next'), row('PE-3', 'later'), row('PE-4', 'hidden'), row('PE-5', '')]
  assert.deepEqual(locateEpic(rows, 'PE-1'), { tab: 'now', closed: false })
  assert.deepEqual(locateEpic(rows, 'PE-2'), { tab: 'next', closed: false })
  assert.deepEqual(locateEpic(rows, 'PE-3'), { tab: 'later', closed: false })
  assert.deepEqual(locateEpic(rows, 'PE-4'), { tab: 'hidden', closed: false })
  assert.deepEqual(locateEpic(rows, 'PE-5'), { tab: 'unclassified', closed: false })
})

test('a closed epic says so, so that closed epics can be shown', () => {
  assert.deepEqual(locateEpic([row('M-3', 'later', true)], 'M-3'), { tab: 'later', closed: true })
})

test('an epic the roadmap does not hold is not found, and keys match exactly', () => {
  const rows = [row('PE-10', 'now')]
  assert.equal(locateEpic(rows, 'PE-1'), null)
  assert.equal(locateEpic(rows, 'pe-10'), null)
  assert.equal(locateEpic([], 'PE-10'), null)
})

test('the way back lands on the ticket view left, when the project still shows it', () => {
  const all = project(['triage', 'roadmap', 'timeline'])
  for (const view of TICKET_VIEWS) assert.equal(returnView(view, all), view)
  const bare = project(['roadmap'])
  assert.equal(returnView('list', bare), 'list')
  assert.equal(returnView('triage', bare), 'board')
  assert.equal(returnView('timeline', bare), 'board')
})

test('without a ticket view left behind, the way back opens the board', () => {
  const all = project(['triage', 'roadmap', 'timeline'])
  assert.equal(returnView(null, all), 'board')
  assert.equal(returnView('activities', all), 'board')
  assert.equal(returnView('roadmap', all), 'board')
})

test("a ticket's project is found by id or slug, or is the one on screen without an id", () => {
  const a = { id: 'pa', slug: 'alpha', name: 'A' }
  const b = { id: 'pb', name: 'B' }
  assert.equal(projectOfTask({ projectId: 'pb' }, [a, b], a), b)
  assert.equal(projectOfTask({ projectId: 'alpha' }, [a, b], b), a)
  assert.equal(projectOfTask({ projectId: 'gone' }, [a, b], a), null)
  assert.equal(projectOfTask({}, [a, b], a), a)
  assert.equal(projectOfTask({}, [a, b], null), null)
})
