import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  canEditEpicLabels,
  epicLabelInventory,
  freeEpicLabels,
  isEpicAxisLabel,
  matchesEpicLabels,
  pruneSelectedLabels,
} from '../src/lib/roadmap.ts'

const row = (key, labels) => ({ key, meta: labels === undefined ? undefined : { labels } })

test('the roadmap, priority and quarter axes are protected, whatever the case or a leading #', () => {
  assert.equal(isEpicAxisLabel('roadmap:now'), true)
  assert.equal(isEpicAxisLabel('Roadmap:Later'), true)
  assert.equal(isEpicAxisLabel('#roadmap:next'), true)
  assert.equal(isEpicAxisLabel(' roadmap:hidden '), true)
  assert.equal(isEpicAxisLabel('priority:p2'), true)
  assert.equal(isEpicAxisLabel('Quarter:2026-q1'), true)
  assert.equal(isEpicAxisLabel('2026-Q3'), true)
  assert.equal(isEpicAxisLabel('#2026.q4'), true)
  assert.equal(isEpicAxisLabel('2026-Q5'), false)
  assert.equal(isEpicAxisLabel('roadmap'), false)
  assert.equal(isEpicAxisLabel('client-acme'), false)
})

test('free labels keep the tracker order and leave the axes out', () => {
  assert.deepEqual(freeEpicLabels({ labels: ['domain-billing', 'roadmap:now', 'client-acme', ' '] }), ['domain-billing', 'client-acme'])
  assert.deepEqual(freeEpicLabels({ labels: ['#roadmap:later'] }), [])
  assert.deepEqual(freeEpicLabels({ labels: ['priority:p1', '2026-Q3', 'client-acme'] }), ['client-acme'])
  assert.deepEqual(freeEpicLabels(undefined), [])
  assert.deepEqual(freeEpicLabels({}), [])
})

test('the inventory counts epics per label, one entry per spelling', () => {
  const inventory = epicLabelInventory([
    row('PE-1', ['client-acme', 'domain-billing', 'roadmap:now']),
    row('PE-2', ['Client-Acme', 'client-acme']),
    row('PE-3', ['roadmap:later']),
    row('PE-4', undefined),
  ])
  assert.deepEqual(inventory, [
    { label: 'client-acme', count: 2 },
    { label: 'domain-billing', count: 1 },
  ])
  assert.deepEqual(epicLabelInventory([row('PE-3', ['roadmap:later'])]), [])
})

test('the filter keeps an epic carrying any selected label', () => {
  const billing = row('PE-1', ['domain-billing'])
  const acme = row('PE-2', ['Client-Acme'])
  const bare = row('PE-3', [])
  assert.equal(matchesEpicLabels(bare, []), true)
  assert.equal(matchesEpicLabels(billing, ['client-acme']), false)
  assert.equal(matchesEpicLabels(acme, ['client-acme']), true)
  assert.equal(matchesEpicLabels(billing, ['client-acme', 'domain-billing']), true)
  // An axis label never matches, even if someone selected it by hand.
  assert.equal(matchesEpicLabels(row('PE-4', ['roadmap:now']), ['roadmap:now']), false)
})

test('a selected label nobody carries any more is dropped', () => {
  const selected = ['client-acme', 'gone']
  assert.deepEqual(pruneSelectedLabels(selected, [{ label: 'Client-Acme', count: 1 }]), ['client-acme'])
  const unchanged = ['client-acme']
  assert.equal(pruneSelectedLabels(unchanged, [{ label: 'client-acme', count: 3 }]), unchanged)
})

test('labels are editable only on a Jira epic of the project', () => {
  const jira = { issueTracker: 'jira', jiraProject: 'PE' }
  assert.equal(canEditEpicLabels(jira, row('PE-1', [])), true)
  assert.equal(canEditEpicLabels(jira, row('OPS-4', [])), false)
  assert.equal(canEditEpicLabels(jira, row('M-3', [])), false)
  assert.equal(canEditEpicLabels({ issueTracker: 'github' }, row('M-3', [])), false)
  assert.equal(canEditEpicLabels({ issueTracker: 'gitlab' }, row('12', [])), false)
  assert.equal(canEditEpicLabels(null, row('PE-1', [])), false)
  // The server's verdict wins when it sends one.
  assert.equal(canEditEpicLabels(jira, { key: 'PE-1', meta: { labels: [], labelsWritable: false } }), false)
  assert.equal(canEditEpicLabels(jira, { key: 'OPS-4', meta: { labels: [], labelsWritable: true } }), true)
  assert.equal(canEditEpicLabels({ issueTracker: 'github' }, { key: 'M-3', meta: { labelsWritable: true } }), false)
})
