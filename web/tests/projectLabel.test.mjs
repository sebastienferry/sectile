import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  membershipParam,
  projectLabelState,
  withProjectLabel,
  withoutProjectLabel,
} from '../src/lib/projectLabel.ts'

test('membership is only sent when the board is widened', () => {
  assert.equal(membershipParam(false), null)
  assert.equal(membershipParam(true), 'all')
})

test('a project without a label offers nothing', () => {
  assert.deepEqual(projectLabelState(['bug'], undefined), { label: '', applicable: false, carries: false })
  assert.deepEqual(projectLabelState(['bug'], '   '), { label: '', applicable: false, carries: false })
})

test('a ticket lacking the label is offered the addition', () => {
  const state = projectLabelState(['bug'], 'team-alpha')
  assert.equal(state.applicable, true)
  assert.equal(state.carries, false)
  assert.equal(state.label, 'team-alpha')
})

test('a ticket carrying the label in another case is offered the removal', () => {
  assert.equal(projectLabelState(['Team-Alpha'], 'team-alpha').carries, true)
})

test('a look-alike label is not the project label', () => {
  assert.equal(projectLabelState(['team-alphabet'], 'team-alpha').carries, false)
})

test('a ticket with no label at all is handled', () => {
  assert.equal(projectLabelState(undefined, 'team-alpha').carries, false)
})

test('adding appends once and never duplicates', () => {
  assert.deepEqual(withProjectLabel(['bug'], 'team-alpha'), ['bug', 'team-alpha'])
  assert.deepEqual(withProjectLabel(['bug', 'TEAM-ALPHA'], 'team-alpha'), ['bug', 'TEAM-ALPHA'])
  assert.deepEqual(withProjectLabel(undefined, 'team-alpha'), ['team-alpha'])
})

test('removing drops whatever case the ticket carries and leaves the rest', () => {
  assert.deepEqual(withoutProjectLabel(['bug', 'TEAM-ALPHA'], 'team-alpha'), ['bug'])
  assert.deepEqual(withoutProjectLabel(['bug', 'team-alphabet'], 'team-alpha'), ['bug', 'team-alphabet'])
  assert.deepEqual(withoutProjectLabel(undefined, 'team-alpha'), [])
})
