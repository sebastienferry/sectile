import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  BOARD_GROUPING_OPTIONS,
  BOARD_GROUPING_SIZES,
} from '../src/lib/boardGrouping.ts'

test('the workflow grouping is offered first, the status grouping second', () => {
  assert.deepEqual(
    BOARD_GROUPING_OPTIONS.map(option => option.id),
    ['workflow', 'status'],
  )
})

test('each option carries its icon, label key and tooltip key', () => {
  const [workflow, status] = BOARD_GROUPING_OPTIONS
  assert.equal(workflow.icon, 'sparkles')
  assert.equal(workflow.labelKey, 'workflow')
  assert.equal(workflow.tooltipKey, 'workflowTooltip')
  assert.equal(status.icon, 'kanban')
  assert.equal(status.labelKey, 'status')
  assert.equal(status.tooltipKey, 'statusTooltip')
})

test('options are distinct and every mode is represented exactly once', () => {
  const ids = BOARD_GROUPING_OPTIONS.map(option => option.id)
  assert.equal(new Set(ids).size, ids.length)
  assert.equal(ids.length, 2)
})

test('only density differs between the two sizes', () => {
  const { sm, md } = BOARD_GROUPING_SIZES
  assert.ok(sm.iconSize < md.iconSize)
  assert.notEqual(sm.buttonPadding, md.buttonPadding)
  assert.equal(Object.keys(sm).length, Object.keys(md).length)
})
