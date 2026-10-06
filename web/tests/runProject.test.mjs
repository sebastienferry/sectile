import assert from 'node:assert/strict'
import { test } from 'node:test'
import { boardRunProject, runProjectCandidates } from '../src/lib/runProject.ts'

// The project a run works for (#741).

const projects = [{ id: 'a', slug: 'alpha' }, { id: 'b', slug: 'beta' }]

test('a launch from a project board names that project when the ticket is in it', () => {
  assert.equal(boardRunProject({ projectId: 'a', projectIds: ['a', 'b'] }, 'b', projects), 'b')
  assert.equal(boardRunProject({ projectId: 'a', projectIds: ['a', 'b'] }, 'alpha', projects), 'a', 'a slug selects its project')
  // All projects, an unknown selection, or a ticket the project does not show
  // leave the choice to the server.
  assert.equal(boardRunProject({ projectIds: ['a', 'b'] }, 'all', projects), undefined)
  assert.equal(boardRunProject({ projectIds: ['a'] }, 'b', projects), undefined)
  assert.equal(boardRunProject({ projectIds: ['a'] }, 'gone', projects), undefined)
  // A ticket read before memberships existed keeps the board's project.
  assert.equal(boardRunProject({ projectId: 'a' }, 'a', projects), 'a')
  assert.equal(boardRunProject(undefined, 'a', projects), 'a')
})

test('only a refusal listing candidates asks which project', () => {
  assert.deepEqual(runProjectCandidates(409, { error: 'x', candidates: [{ id: 'a', name: 'Alpha' }, { id: 'b' }], unattended: false }), [
    { id: 'a', name: 'Alpha' },
    { id: 'b', name: 'b' },
  ])
  // A busy ticket is a 409 too, without candidates.
  assert.equal(runProjectCandidates(409, { error: 'busy', active: { id: 'r1' } }), null)
  assert.equal(runProjectCandidates(409, { candidates: [] }), null)
  assert.equal(runProjectCandidates(409, null), null)
  assert.equal(runProjectCandidates(409, 'nope'), null)
})

test('an unattended refusal is shown, never answered with the picker', () => {
  const candidates = [{ id: 'a', name: 'Alpha' }, { id: 'b', name: 'Beta' }]
  // An autonomous run or a batch is refused with a 400 that lists the
  // candidates: retrying it for one of them would override the refusal.
  assert.equal(runProjectCandidates(400, { error: 'refused', candidates, unattended: true }), null)
  // The flag alone refuses, whatever the status; the status alone too.
  assert.equal(runProjectCandidates(409, { error: 'refused', candidates, unattended: true }), null)
  assert.equal(runProjectCandidates(400, { error: 'refused', candidates }), null)
})
