import assert from 'node:assert/strict'
import { test } from 'node:test'
import { readFile } from 'node:fs/promises'
import ts from 'typescript'

// Use the project's TypeScript compiler so tests also run on Node versions
// supported by Vite that do not natively strip TypeScript types.
const source = await readFile(new URL('../src/lib/terminalSkillCommand.ts', import.meta.url), 'utf8')
const { outputText } = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
})
const { terminalSkillCommand } = await import(`data:text/javascript;base64,${Buffer.from(outputText).toString('base64')}`)

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
