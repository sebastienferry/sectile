import assert from 'node:assert/strict'
import { test } from 'node:test'
import { initialQuickAddMacro, quickAddMacroOptions } from '../src/lib/quickAdd.ts'

const macro = (key, title, closed = false) => ({ projectId: 'p', key, title, closed, horizon: '', description: '', todos: [], updatedAt: '' })

test('only open macros are offered, in the natural order of their keys', () => {
  const options = quickAddMacroOptions([macro('M-10', 'Ten'), macro('M-2', 'Two'), macro('M-3', 'Done', true), macro('', 'Keyless')])
  assert.deepEqual(options.map(m => m.key), ['M-2', 'M-10'])
})

test('the list given is left untouched', () => {
  const macros = [macro('M-10', 'Ten'), macro('M-2', 'Two')]
  quickAddMacroOptions(macros)
  assert.deepEqual(macros.map(m => m.key), ['M-10', 'M-2'])
})

test('the board macro filter pre-selects the macro it names, by key or by title', () => {
  const options = [macro('M-2', 'Two'), macro('M-7', 'Ux improvements')]
  assert.equal(initialQuickAddMacro('M-7', options), 'M-7')
  assert.equal(initialQuickAddMacro('Ux improvements', options), 'M-7')
})

test('any other filter selects none', () => {
  const options = [macro('M-2', 'Two')]
  assert.equal(initialQuickAddMacro(null, options), '')
  assert.equal(initialQuickAddMacro('', options), '')
  assert.equal(initialQuickAddMacro('__no_macro__', options), '')
  assert.equal(initialQuickAddMacro('none', options), '')
  // A macro of another project, or a closed one, is not among the options.
  assert.equal(initialQuickAddMacro('M-9', options), '')
})
