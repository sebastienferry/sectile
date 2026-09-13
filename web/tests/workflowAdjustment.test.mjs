import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'
import ts from 'typescript'
const source = await readFile(new URL('../src/lib/workflow.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 } })
const { skillForStage, getNextStepInfo, prRecoverySkill } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)
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
