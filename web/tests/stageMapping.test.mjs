import assert from 'node:assert/strict'
import { test } from 'node:test'
import { editedStageMapping, ownStageMappings, projectForTracker, stageMappingPayload, trackerBoard } from '../src/lib/stageMapping.ts'
import { resolveTaskStage } from '../src/lib/workflow.ts'

// A project maps the workflow stages onto each of its trackers' columns its
// own way, or reads the tracker's (#741).

const columns = [
  { name: 'Doing', statuses: ['In Progress'] },
  { name: 'Review', statuses: ['In Review'] },
]
const project = {
  id: 'p',
  trackerColumns: columns,
  stageColumns: { implemented: ['Review'] },
  defaultTrackerId: 'gode',
  trackers: [
    { trackerId: 'gode', identity: 'jira|acme|GODE', trackerColumns: columns, trackerStageColumns: { implemented: ['Doing'] }, stageColumns: { implemented: ['Review'] }, ownStageColumns: true },
    { trackerId: 'be', identity: 'jira|acme|BE', trackerColumns: [{ name: 'QA', statuses: ['Testing'] }], trackerStageColumns: { implemented: ['QA'] }, stageColumns: { implemented: ['QA'] } },
  ],
}

test('a saved project holds its own mappings, the inherited ones as null', () => {
  assert.deepEqual(ownStageMappings(project.trackers), { gode: { implemented: ['Review'] }, be: null })
})

test('an edited mapping keeps its stages with columns, and reads the tracker once none is left', () => {
  assert.deepEqual(editedStageMapping({ implemented: ['Doing'], reviewed: ['Review'], new: [] }), { implemented: ['Doing'], reviewed: ['Review'] })
  assert.equal(editedStageMapping({ implemented: [] }), null)
})

test('a save sends the changed mappings only, a reset as an empty one', () => {
  const original = ownStageMappings(project.trackers)
  assert.equal(stageMappingPayload(original, original, ['gode', 'be']), undefined)
  assert.deepEqual(stageMappingPayload({ gode: null, be: { implemented: ['QA'], new: [] } }, original, ['gode', 'be']), {
    gode: {},
    be: { implemented: ['QA'] },
  })
  // A tracker the save no longer selects is not sent.
  assert.equal(stageMappingPayload({ gode: original.gode, be: { new: ['QA'] } }, original, ['gode']), undefined)
})

test("a ticket reads the columns and mapping of its own tracker in the project", () => {
  assert.deepEqual(trackerBoard(project, 'be').stageColumns, { implemented: ['QA'] })
  assert.deepEqual(trackerBoard(project, undefined).stageColumns, { implemented: ['Review'] })
  assert.equal(projectForTracker(project, 'be').trackerColumns[0].name, 'QA')
  assert.equal(projectForTracker(project, 'nowhere'), project)
  const ticket = { id: 't', labels: [], status: 'to_clarify', trackerStatus: 'Testing', trackerId: 'be' }
  assert.equal(resolveTaskStage(ticket, project), 'implemented')
})
