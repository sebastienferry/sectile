import assert from 'node:assert/strict'
import { existsSync, mkdirSync, mkdtempSync, readdirSync, realpathSync, rmSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { test } from 'node:test'
import { pathToFileURL } from 'node:url'
import { browserRoot } from './browserRoot.mjs'

// A checkout laid out like a task worktree: <name>/web/tests and <name>/shared.
function checkout(name) {
  const base = mkdtempSync(join(tmpdir(), 'browser-root-test-'))
  const dir = join(base, name)
  mkdirSync(join(dir, 'web', 'tests'), { recursive: true })
  mkdirSync(join(dir, 'shared'))
  const testUrl = pathToFileURL(join(dir, 'web', 'tests', 'x.browser.mjs')).href
  return { base, dir, testUrl }
}

test('a path without # is served as it is', t => {
  const { base, dir, testUrl } = checkout('387')
  t.after(() => rmSync(base, { recursive: true, force: true }))
  const exits = []
  const { root, preserveSymlinks } = browserRoot(testUrl, { onExit: callback => exits.push(callback) })
  assert.equal(root, join(dir, 'web'))
  assert.equal(preserveSymlinks, false)
  assert.equal(exits.length, 0, 'nothing to clean up')
})

test('a path with # is served through a link to the same checkout', t => {
  const { base, dir, testUrl } = checkout('#387')
  t.after(() => rmSync(base, { recursive: true, force: true }))
  const links = mkdtempSync(join(tmpdir(), 'browser-root-links-'))
  t.after(() => rmSync(links, { recursive: true, force: true }))
  const exits = []
  const { root, preserveSymlinks } = browserRoot(testUrl, { tempDir: links, onExit: callback => exits.push(callback) })
  assert.equal(preserveSymlinks, true)
  assert.ok(!root.includes('#'), root)
  assert.ok(root.endsWith('/web'), root)
  assert.equal(realpathSync(root), realpathSync(join(dir, 'web')))
  assert.equal(realpathSync(join(root, '..', 'shared')), realpathSync(join(dir, 'shared')), 'web/ still reaches its sibling shared/')

  assert.equal(exits.length, 1)
  exits[0]()
  assert.deepEqual(readdirSync(links), [], 'the link and its directory are removed on exit')
  assert.ok(existsSync(join(dir, 'web', 'tests')), 'the checkout itself is left intact')
  assert.ok(existsSync(join(dir, 'shared')))
})

test('a temporary directory holding # is refused and removed', t => {
  const { base, testUrl } = checkout('#387')
  t.after(() => rmSync(base, { recursive: true, force: true }))
  const links = join(base, 'tmp#')
  mkdirSync(links)
  assert.throws(() => browserRoot(testUrl, { tempDir: links, onExit: () => assert.fail('no cleanup registered') }), /TMPDIR/)
  assert.deepEqual(readdirSync(links), [])
})
