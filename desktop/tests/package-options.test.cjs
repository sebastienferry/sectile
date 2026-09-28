const {test} = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const {spawnSync} = require('node:child_process')
const {agentName} = require('../electron/runtime.cjs')
const {packageOptions} = require('../electron/package-options.cjs')

const desktopRoot = path.resolve(__dirname, '..')

test('without a target the options are the host package make desktop-package builds', () => {
 const options = packageOptions()
 assert.deepEqual(Object.keys(options).sort(), ['dir', 'extraResource', 'icon', 'ignore', 'name', 'out', 'overwrite'])
 assert.equal(options.dir, desktopRoot)
 assert.equal(options.name, 'Sectile')
 assert.equal(options.out, path.join(desktopRoot, 'release'))
 assert.equal(options.overwrite, true)
 assert.equal(options.icon, path.join(desktopRoot, 'assets/icon'))
 assert.equal(options.extraResource, path.join(desktopRoot, 'bin', agentName()))
})

test('a target is passed to packager with its own agent name', () => {
 const options = packageOptions({platform: 'win32', arch: 'x64'})
 assert.equal(options.platform, 'win32')
 assert.equal(options.arch, 'x64')
 assert.equal(options.extraResource, path.join(desktopRoot, 'bin', 'sectile-agent.exe'))
 assert.equal(path.basename(packageOptions({platform: 'linux', arch: 'arm64'}).extraResource), 'sectile-agent')
})

test('a staged agent and an output directory are used as given', () => {
 const options = packageOptions({platform: 'darwin', arch: 'arm64', agent: 'staging/sectile-agent', out: 'dist-desktop'})
 assert.equal(options.extraResource, path.resolve('staging/sectile-agent'))
 assert.equal(options.out, path.resolve('dist-desktop'))
})

test('Go tokens and unknown targets are refused rather than guessed', () => {
 assert.throws(() => packageOptions({platform: 'windows'}), /Unknown platform windows/)
 assert.throws(() => packageOptions({platform: 'linux', arch: 'amd64'}), /Unknown arch amd64/)
 assert.throws(() => packageOptions({arch: 'ia32'}), /Unknown arch ia32/)
})

test('the package leaves out the development folders', () => {
 const {ignore} = packageOptions()
 for (const excluded of ['/bin', '/bin/sectile-agent', '/tests', '/tests/runtime.test.cjs', '/release', '/release-linux/Sectile']) {
  assert.ok(ignore.test(excluded), excluded)
 }
 for (const kept of ['/electron/main.cjs', '/dist/index.html', '/package.json', '/binary.js']) {
  assert.ok(!ignore.test(kept), kept)
 }
})

test('package.cjs refuses a missing agent before packaging anything', () => {
 const out = fs.mkdtempSync(path.join(os.tmpdir(), 'sectile-package-test-'))
 try {
  const result = spawnSync(process.execPath, [path.join(desktopRoot, 'electron/package.cjs'), '--platform', 'linux', '--arch', 'x64', '--agent', path.join(out, 'missing'), '--out', out], {encoding: 'utf8'})
  assert.equal(result.status, 1)
  assert.match(result.stderr, /Agent not found/)
  assert.deepEqual(fs.readdirSync(out), [])
 } finally {
  fs.rmSync(out, {recursive: true, force: true})
 }
})
