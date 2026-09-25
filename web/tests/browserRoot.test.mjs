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
  const { root, preserveSymlinks } = browserRoot(testUrl, { env: {}, onExit: callback => exits.push(callback), relaunch: () => assert.fail('no relaunch') })
  assert.equal(root, join(dir, 'web'))
  assert.equal(preserveSymlinks, false)
  assert.equal(exits.length, 0, 'nothing to clean up')
})

test('a path with # relaunches the test through a link to the same checkout', t => {
  const { base, dir, testUrl } = checkout('#387')
  t.after(() => rmSync(base, { recursive: true, force: true }))
  const links = mkdtempSync(join(tmpdir(), 'browser-root-links-'))
  t.after(() => rmSync(links, { recursive: true, force: true }))
  const exits = []
  const relaunches = []
  const result = browserRoot(testUrl, { tempDir: links, env: { KEEP: 'me' }, onExit: callback => exits.push(callback), relaunch: (script, env) => relaunches.push({ script, env }) })
  assert.equal(result, undefined, 'the test itself runs in the relaunched process')

  assert.equal(relaunches.length, 1)
  const [{ script, env }] = relaunches
  assert.ok(!script.includes('#'), script)
  assert.ok(script.endsWith(join('web', 'tests', 'x.browser.mjs')), script)
  const web = join(script, '..', '..')
  assert.equal(realpathSync(web), realpathSync(join(dir, 'web')))
  assert.equal(realpathSync(join(web, '..', 'shared')), realpathSync(join(dir, 'shared')), 'web/ still reaches its sibling shared/')
  assert.deepEqual(env, { KEEP: 'me', SECTILE_BROWSER_TEST_RELAUNCHED: '1' }, 'the environment is passed on, marked')

  // What the relaunched process gets back from the same call.
  const { root, preserveSymlinks } = browserRoot(pathToFileURL(script).href, { env, onExit: () => assert.fail('no cleanup in the child'), relaunch: () => assert.fail('no second relaunch') })
  assert.equal(root, web)
  assert.equal(preserveSymlinks, true, 'Vite keeps the link instead of resolving it back to the # path')

  assert.equal(exits.length, 1)
  exits[0]()
  assert.deepEqual(readdirSync(links), [], 'the link and its directory are removed on exit')
  assert.ok(existsSync(join(dir, 'web', 'tests')), 'the checkout itself is left intact')
  assert.ok(existsSync(join(dir, 'shared')))
})

test('a relaunched test whose path still holds # is refused instead of looping', t => {
  const { base, testUrl } = checkout('#387')
  t.after(() => rmSync(base, { recursive: true, force: true }))
  assert.throws(() => browserRoot(testUrl, { env: { SECTILE_BROWSER_TEST_RELAUNCHED: '1' }, relaunch: () => assert.fail('no relaunch') }), /still contains '#'/)
})

test('a temporary directory holding # is refused and removed', t => {
  const { base, testUrl } = checkout('#387')
  t.after(() => rmSync(base, { recursive: true, force: true }))
  const links = join(base, 'tmp#')
  mkdirSync(links)
  assert.throws(() => browserRoot(testUrl, { tempDir: links, env: {}, onExit: () => assert.fail('no cleanup registered'), relaunch: () => assert.fail('no relaunch') }), /TMPDIR/)
  assert.deepEqual(readdirSync(links), [])
})
