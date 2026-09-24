import assert from 'node:assert/strict'
import { test } from 'node:test'
import { formatProjectKeyList, parseProjectKeyList } from '../src/lib/roadmapProjects.ts'

test('the input is read as the server normalises it', () => {
  assert.deepEqual(parseProjectKeyList('abc, DEF abc; SFE', 'sfe'), ['ABC', 'DEF'])
  assert.deepEqual(parseProjectKeyList('   '), [])
})

test('the stored list is shown comma-separated', () => {
  assert.equal(formatProjectKeyList(['ABC', 'DEF']), 'ABC, DEF')
  assert.equal(formatProjectKeyList(undefined), '')
})
