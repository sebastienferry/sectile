import assert from 'node:assert/strict'
import { test } from 'node:test'
import { checkBranchNameFormat, renderBranchNameFormat, validBranchName, BRANCH_NAME_PRESETS } from '../src/lib/branchNameFormat.ts'

// The same cases as TestTaskBranchName in internal/models: the example the
// settings show must be the branch the agent creates.
test('a format renders the branch the agent creates', () => {
  const title = 'Allow the user to choose the format'
  const cases = [
    ['', '#621', title, 'feat/621'],
    ['', 'AUC-1234', title, 'feat/auc-1234'],
    ['   ', 'AUC-1234', title, 'feat/auc-1234'],
    ['', 'GL#12', '', 'feat/gl-12'],
    ['', 'a_b.c', '', 'feat/a-b-c'],
    ['{key}', 'AUC-1234', title, 'AUC-1234'],
    ['{key}', '#621', title, '621'],
    ['{key_lower}', 'AUC-1234', title, 'auc-1234'],
    ['feat/{key}-{title}', 'AUC-1234', title, 'feat/AUC-1234-allow-the-user-to-choose-the-f'],
    ['feat/{key}-{title}', 'AUC-1234', 'Déploiement : phase 2 !', 'feat/AUC-1234-d-ploiement-phase-2'],
    ['feat/{key}-{title}', 'AUC-1234', '', 'feat/AUC-1234'],
    ['feat/{key}-{title}', 'AUC-1234', '!!!', 'feat/AUC-1234'],
    ['{title}-{key}', 'AUC-1234', '', 'AUC-1234'],
    ['feat/{title}/{key}', 'AUC-1234', '', 'feat/AUC-1234'],
    ['{key}/{title}', 'AUC-1234', '', 'AUC-1234'],
    ['{key}/{title}', 'AUC-1234', 'Fix', 'AUC-1234/fix'],
  ]
  for (const [format, key, taskTitle, want] of cases) {
    assert.deepEqual(renderBranchNameFormat(format, key, taskTitle), { ok: true, branch: want }, `${format} ${key} ${taskTitle}`)
  }
})

test('the presets are all accepted', () => {
  for (const preset of BRANCH_NAME_PRESETS) assert.equal(checkBranchNameFormat(preset).ok, true, preset)
  assert.equal(checkBranchNameFormat('').ok, true)
})

test('a format is refused for the reasons the server gives', () => {
  const cases = [
    ['feat/{title}', 'key'],
    ['main', 'key'],
    ['feat/{id}-{key}', 'placeholder'],
    ['feat/{key', 'brace'],
    ['feat/{key}}', 'brace'],
    ['feat/{{key}}', 'brace'],
    ['feat//{key}', 'git'],
    ['{key}.lock', 'git'],
    ['feat {key}', 'git'],
    ['-{key}', 'git'],
    ['feat/{key}/', 'git'],
    ['feat/.{key}', 'git'],
    ['feat/{key}~1', 'git'],
  ]
  for (const [format, problem] of cases) {
    const render = checkBranchNameFormat(format)
    assert.equal(render.ok, false, format)
    assert.equal(render.problem, problem, format)
  }
})

test('branch names follow git check-ref-format', () => {
  for (const name of ['feat/621', 'AUC-1234', 'users/me/a_b.c', 'v1.2']) assert.equal(validBranchName(name), true, name)
  for (const name of ['', '@', 'HEAD', '-x', '/x', 'x/', 'x.', 'a..b', 'a@{b', 'a//b', 'a b', 'a~b', 'a^b', 'a:b', 'a?b', 'a*b', 'a[b', 'a\\b', 'a\tb', '.a', 'a/.b', 'a.lock', 'a/b.lock/c']) {
    assert.equal(validBranchName(name), false, JSON.stringify(name))
  }
})
