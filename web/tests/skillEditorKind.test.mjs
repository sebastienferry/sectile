import assert from 'node:assert/strict'
import { test } from 'node:test'
import { kindStartContent } from '../src/lib/skillEditorKind.ts'

const builtin = { defaultContent: 'built-in skill', defaultWorkContent: '## Steps\nBuilt-in steps.' }

test('a skill without an override starts from the built-in content of the kind', () => {
  const entry = { ...builtin, isCustom: false, overrideKind: '', content: 'built-in skill' }
  assert.equal(kindStartContent(entry, 'work'), builtin.defaultWorkContent)
  assert.equal(kindStartContent(entry, ''), builtin.defaultContent)
})

// Switching to a full replacement and back to work only must not drop the
// stored work override for the built-in work sections (#732).
test('a stored work override is what switching back to work only starts from', () => {
  const entry = { ...builtin, isCustom: true, overrideKind: 'work', content: '## Steps\nThe team steps.' }
  assert.equal(kindStartContent(entry, 'work'), entry.content)
  assert.equal(kindStartContent(entry, ''), builtin.defaultContent)
})

test('a stored full replacement is what switching back to full starts from', () => {
  const entry = { ...builtin, isCustom: true, overrideKind: '', content: 'the team skill' }
  assert.equal(kindStartContent(entry, ''), entry.content)
  assert.equal(kindStartContent(entry, 'work'), builtin.defaultWorkContent)
})

test('a skill that is not overridable starts work only empty', () => {
  const entry = { defaultContent: 'built-in skill', isCustom: false, overrideKind: '', content: 'built-in skill' }
  assert.equal(kindStartContent(entry, 'work'), '')
})
