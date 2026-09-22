import assert from 'node:assert/strict'
import { test } from 'node:test'
import { getCommandPresets } from '../src/lib/commandPresets.ts'

test('command presets include default, Claude, AGY and Codex', () => {
  const presets = getCommandPresets()
  assert.equal(presets.length, 4)

  const labels = presets.map(p => p.label)
  assert.deepEqual(labels, ['Défaut du fournisseur', 'Claude', 'AGY', 'Codex'])

  const agy = presets.find(p => p.label === 'AGY')
  assert.ok(agy)
  assert.match(agy.cmd, /agy --dangerously-skip-permissions --model \{model\}/)
  assert.match(agy.autonomous, /agy --dangerously-skip-permissions --model \{model\} --output-format stream-json/)
  assert.match(agy.autonomous, /jq -rs/)

  const claude = presets.find(p => p.label === 'Claude')
  assert.ok(claude)
  assert.match(claude.cmd, /claude --model \{model\}/)
  assert.match(claude.autonomous, /claude -p --permission-mode bypassPermissions --output-format stream-json --verbose --model \{model\}/)

  const codex = presets.find(p => p.label === 'Codex')
  assert.ok(codex)
  assert.match(codex.cmd, /codex --model \{model\}/)
  assert.match(codex.autonomous, /codex exec --model \{model\}/)
})
