import assert from 'node:assert/strict'
import { test } from 'node:test'

const { nextStepOffer, blocksLaunch } = await import('../src/next-step-offer.mjs')

const project = { configured: true, server: { skills: ['clarify', 'specify', 'implement', 'adjust', 'handoff'].map(id => ({ id })) } }
const run = (fields = {}) => ({ id: 'chat', taskId: 'task', projectId: 'project', conversation: true, provider: 'claude', skill: 'specify', status: 'running', ...fields })
const result = (skill = 'specify', status = 'completed', stage = 'specified', fields = {}) => ({ activity: { id: 'chat', taskId: 'task', skillId: skill, status }, task: { status: 'open', labels: ['#' + stage] }, ...fields })
const task = (stage, fields = {}) => ({ id: 'task', status: 'open', labels: ['#' + stage], ...fields })
const idle = { id: 'chat', busy: false, readOnly: false, approvals: [] }
const offer = (fields = {}) => nextStepOffer({ run: run(), result: result(), conversation: idle, task: task('specified'), project, runs: [run()], ...fields })

test('a completed Specify at specified offers Implement', () => {
  assert.deepEqual(offer(), { skillId: 'implement', label: 'Implement' })
})

test('an implemented task offers Create PR without a pull request and Adjust with one', () => {
  const implemented = { run: run({ skill: 'implement' }), result: result('implement', 'completed', 'implemented') }
  assert.deepEqual(offer({ ...implemented, task: task('implemented') }), { skillId: 'implement', label: 'Create PR' })
  assert.deepEqual(offer({ ...implemented, task: task('implemented', { prUrl: 'https://github.com/o/r/pull/1' }) }), { skillId: 'adjust', label: 'Adjust' })
  assert.deepEqual(offer({ ...implemented, task: task('implemented'), project: { ...project, server: { ...project.server, prCreationStage: 'specified' } } }), { skillId: 'specify', label: 'Create PR' })
})

test('every offering skill names the step after its stage', () => {
  assert.deepEqual(offer({ run: run({ skill: 'clarify' }), result: result('clarify', 'completed', 'clarified'), task: task('clarified') }), { skillId: 'specify', label: 'Specify' })
  assert.deepEqual(offer({ run: run({ skill: 'adjust' }), result: result('adjust', 'completed', 'reviewed'), task: task('reviewed') }), { skillId: 'handoff', label: 'Handoff' })
})

test('a skill awaiting its stage validation offers nothing', () => {
  assert.equal(offer({ result: result('specify', 'completed', 'clarified') }).reason, 'skill-not-completed')
})

test('a task read older than the skill result waits for the stage move', () => {
  assert.equal(offer({ task: task('clarified') }).reason, 'task-behind')
})

test('a failed, stopped or unconfirmed skill offers nothing', () => {
  assert.equal(offer({ result: result('specify', 'failed') }).reason, 'skill-not-completed')
  assert.equal(offer({ result: result('specify', 'canceled') }).reason, 'skill-not-completed')
  assert.equal(offer({ result: result('specify', 'running') }).reason, 'skill-not-completed')
  assert.equal(offer({ result: null }).reason, 'skill-not-completed')
  assert.equal(offer({ result: { ...result(), activity: { ...result().activity, id: 'other' } } }).reason, 'skill-not-completed')
})

test('a conversation that answers, sends, asks or waits offers nothing', () => {
  assert.equal(offer({ conversation: { ...idle, busy: true } }).reason, 'busy')
  assert.equal(offer({ conversation: { ...idle, sending: true } }).reason, 'busy')
  assert.equal(offer({ conversation: { ...idle, approvals: [{ id: 'a', tool: 'Bash' }] } }).reason, 'approval')
  assert.equal(offer({ conversation: { ...idle, approvals: [{ id: 'q', tool: 'AskUserQuestion' }] } }).reason, 'approval')
  assert.equal(offer({ run: run({ waitingSince: '2026-10-28T10:00:00Z' }) }).reason, 'waiting')
  assert.equal(offer({ result: result('specify', 'completed', 'specified', { successor: { skillId: 'implement', status: 'running', waitingSince: '2026-10-28T10:00:00Z' } }) }).reason, 'waiting')
})

test('a read-only or not yet polled conversation offers nothing', () => {
  assert.equal(offer({ conversation: { ...idle, readOnly: true } }).reason, 'read-only')
  assert.equal(offer({ conversation: null }).reason, 'conversation-unknown')
  assert.equal(offer({ conversation: { ...idle, id: 'other' } }).reason, 'conversation-unknown')
})

test('a successor skill speaks for the conversation', () => {
  const running = result('specify', 'completed', 'specified', { successor: { skillId: 'implement-issue', status: 'running' } })
  assert.equal(offer({ result: running }).reason, 'skill-not-completed')
  const done = result('specify', 'completed', 'implemented', { successor: { skillId: 'sectile:implement-issue', status: 'completed' } })
  assert.deepEqual(offer({ result: done, task: task('implemented') }), { skillId: 'implement', label: 'Create PR' })
  const pickup = result('specify', 'completed', 'implemented', { successor: { skillId: 'pickup', status: 'completed' } })
  assert.equal(offer({ result: pickup, task: task('implemented') }).reason, 'skill')
})

test('a completed handoff and a finished task offer nothing', () => {
  assert.equal(offer({ run: run({ skill: 'handoff' }), result: result('handoff', 'completed', 'finished'), task: task('finished') }).reason, 'skill')
  assert.equal(offer({ run: run({ skill: 'adjust' }), result: result('adjust', 'completed', 'reviewed'), task: task('reviewed', { status: 'done' }) }).reason, 'finished')
})

test('a project not configured or without the next skill offers nothing', () => {
  assert.equal(offer({ project: { ...project, configured: false } }).reason, 'no-next-step')
  assert.equal(offer({ project: { configured: true, server: { skills: [{ id: 'specify' }] } } }).reason, 'no-next-step')
  assert.equal(offer({ project: null }).reason, 'task-unknown')
})

test('another active run or a launch in flight offers nothing', () => {
  assert.equal(offer({ runs: [run(), { id: 'other', taskId: 'task', projectId: 'project', status: 'running' }] }).reason, 'active-run')
  assert.equal(offer({ runs: [run(), { id: 'other', taskId: 'task', projectId: 'project', status: 'queued' }] }).reason, 'active-run')
  assert.deepEqual(offer({ runs: [run(), { id: 'other', taskId: 'task', projectId: 'project', status: 'completed' }, { id: 'elsewhere', taskId: 'other-task', projectId: 'project', status: 'running' }] }), { skillId: 'implement', label: 'Implement' })
  assert.equal(offer({ submitting: true }).reason, 'submitting')
})

test('Claude Code, Codex and Antigravity conversations offer the next step', () => {
  for (const provider of ['claude', 'codex', 'agy']) assert.deepEqual(offer({ run: run({ provider }) }), { skillId: 'implement', label: 'Implement' }, provider)
  assert.equal(offer({ run: run({ provider: 'gemini' }) }).reason, 'provider')
})

test('a project prompt, a macro run, a discussion and a terminal run offer nothing', () => {
  assert.equal(offer({ run: run({ kind: 'console', taskId: '' }) }).reason, 'not-a-task-conversation')
  assert.equal(offer({ run: run({ kind: 'console' }) }).reason, 'not-a-task-conversation')
  assert.equal(offer({ run: run({ macroKey: 'M-1' }) }).reason, 'not-a-task-conversation')
  assert.equal(offer({ run: run({ skill: 'discuss' }) }).reason, 'not-a-task-conversation')
  assert.equal(offer({ run: run({ conversation: false }) }).reason, 'not-a-task-conversation')
  assert.equal(offer({ run: run({ taskId: '' }) }).reason, 'not-a-task-conversation')
})

test('skills without an expected stage offer nothing', () => {
  for (const skill of ['pickup', 'pickup_issues', 'create_pr', 'rewrite_story']) assert.equal(offer({ run: run({ skill }), result: result(skill) }).reason, 'skill', skill)
})

test('blocksLaunch ignores the predecessor only', () => {
  assert.equal(blocksLaunch({ id: 'chat', status: 'running' }, 'chat'), false)
  assert.equal(blocksLaunch({ id: 'other', status: 'running' }, 'chat'), true)
  assert.equal(blocksLaunch({ id: 'other', status: 'preparing' }, 'chat'), true)
  assert.equal(blocksLaunch({ id: 'other', status: 'completed' }, 'chat'), false)
})
