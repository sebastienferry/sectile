import assert from 'node:assert/strict'
import { test } from 'node:test'
import { skillForStage, getNextStepInfo, prRecoverySkill, isTaskScopedSkill } from '../src/lib/workflow.ts'
test('adjustment precedes the human merge boundary', () => {
 assert.equal(skillForStage('implemented'), 'adjust')
 assert.equal(skillForStage('reviewed'), 'handoff')
 assert.equal(skillForStage('finished'), null)
})
test('an existing PR does not skip adjustment', () => {
 const result = getNextStepInfo({ labels: ['#implemented'], status: 'to_test', prUrl: 'https://forge/pull/1' })
 assert.equal(result.nextSkillId, 'adjust')
})

test('missing PR recovery follows earlier creation policy', () => {
 assert.equal(prRecoverySkill(), 'implement')
 assert.equal(prRecoverySkill({ prCreationStage: 'specified' }), 'specify')
 assert.equal(prRecoverySkill({ prCreationStage: 'implemented' }), 'implement')
})

test('a ticket offers neither the macro nor the batch skills', () => {
 for (const id of ['refine_macro', 'realign_macro', 'pickup_issues']) assert.equal(isTaskScopedSkill(id), false, id)
 for (const id of ['clarify', 'specify', 'implement', 'adjust', 'handoff', 'pickup', 'rewrite_story', 'create_pr']) assert.equal(isTaskScopedSkill(id), true, id)
})
