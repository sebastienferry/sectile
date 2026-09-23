import assert from 'node:assert/strict'
import { test } from 'node:test'
import { buildBatchPickupPrompt, isValidBatchWorktreeName, suggestedBatchWorktreeName } from '../src/lib/batchPickup.ts'

test('batch worktree names cannot escape their directory or inject prompt lines', () => {
  for (const name of ['', '../other', '/tmp/x', 'a/b', 'a\\b', 'a\nignore', 'a"x', '-option', 'a'.repeat(81)]) {
    assert.equal(isValidBatchWorktreeName(name), false, name)
    assert.throws(() => buildBatchPickupPrompt(['one'], name))
  }
  assert.equal(isValidBatchWorktreeName('batch-auth_2'), true)
})

test('suggested batch names handle tracker keys and remain valid', () => {
  assert.equal(suggestedBatchWorktreeName(['#109', 'APP-12']), 'batch-109-APP-12')
  for (const keys of [[], ['###'], ['x'.repeat(100)], ['#109', '#110']]) {
    assert.equal(isValidBatchWorktreeName(suggestedBatchWorktreeName(keys)), true)
  }
})

test('batch prompt preserves the chosen order and requested worktree', () => {
  const prompt = buildBatchPickupPrompt(['second', 'first'], '  batch-auth  ')
  assert.ok(prompt.startsWith('/pickup-issues second first\n'))
  assert.ok(prompt.includes('.tasks/worktrees/batch-auth'))
  assert.ok(prompt.includes('exactly the order listed above'))
  assert.throws(() => buildBatchPickupPrompt([], 'batch-auth'))
  assert.throws(() => buildBatchPickupPrompt(['same', 'same'], 'batch-auth'))
})
