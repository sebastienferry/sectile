import assert from 'node:assert/strict'
import { test } from 'node:test'
import { terminalSkillCommand } from '../src/lib/terminalSkillCommand.ts'

for (const [provider, command, override, expected] of [
  ['codex', '/clarify-issue', undefined, 'clarify-issue'],
  [' Codex ', '/clarify-issue', ' /clarify-workitem ', 'clarify-workitem'],
  ['codex', '/clarify-issue', 'clarify-workitem', 'clarify-workitem'],
  ['codex', '/clarify-issue', '  ', 'clarify-issue'],
  ['claude', '/clarify-issue', undefined, '/clarify-issue'],
  ['claude', '/clarify-issue', 'clarify-workitem', '/clarify-workitem'],
  ['gemini', '/clarify-issue', undefined, '/clarify-issue'],
  ['', '/clarify-issue', undefined, '/clarify-issue'],
]) {
  test(`${provider}: ${override ?? command}`, () => {
    assert.equal(terminalSkillCommand(provider, command, override), expected)
  })
}
