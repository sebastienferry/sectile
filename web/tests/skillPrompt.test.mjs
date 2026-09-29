import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'
import { PICKUP_COMMAND, PICKUP_SKILL, SKILL_COMMANDS, taskSkillPrompt } from '../src/lib/skillPrompt.ts'

test('the prompt names the command, the task and the run contract', () => {
  assert.equal(
    taskSkillPrompt('/specify-issue', 'gh-p-612', 'p'),
    '/specify-issue gh-p-612. Use Sectile MCP to read the task and comments and record workflow transitions. First call start_run and save its returned ID. Call finish_run with that runId when this entire skill ends, including failure or stopping for user input. Task primary key: gh-p-612. Project primary key: p.',
  )
})

test('a task without a project names no project key', () => {
  const prompt = taskSkillPrompt('/clarify-issue', 'local-1')
  assert.ok(prompt.startsWith('/clarify-issue local-1. '))
  assert.ok(prompt.endsWith('Task primary key: local-1.'))
  assert.doesNotMatch(prompt, /Project primary key/)
})

test('every workflow step has a default command', () => {
  assert.deepEqual(SKILL_COMMANDS, {
    clarify: '/clarify-issue',
    specify: '/specify-issue',
    implement: '/implement-issue',
    adjust: '/adjust-issue',
    handoff: '/handoff-issue',
  })
  assert.equal(PICKUP_SKILL, 'pickup')
  assert.equal(PICKUP_COMMAND, '/pickup-issue')
})

test('the menu and the card copy through the same builder', async () => {
  // The card's icon and the menu entry must paste the same text: neither keeps
  // a template of its own.
  for (const name of ['CopyTaskSkillMenu.tsx', 'CopyStepPromptButton.tsx']) {
    const source = await readFile(new URL(`../src/components/${name}`, import.meta.url), 'utf8')
    assert.match(source, /taskSkillPrompt\(/, name)
    assert.doesNotMatch(source, /Use Sectile MCP/, name)
  }
})
