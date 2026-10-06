const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const {settingsFiles, effectiveCredential, pairedDeviceId} = require('../electron/settings-file.cjs')

// An encryption store standing in for Electron's safeStorage.
const store = {
 isEncryptionAvailable: () => true,
 encryptString: value => Buffer.from('enc:' + value),
 decryptString: buffer => buffer.toString().replace(/^enc:/, ''),
}
const encrypted = key => Buffer.from('enc:' + key).toString('base64')

function files() {
 const root = fs.mkdtempSync(path.join(os.tmpdir(), 'sectile-settings-'))
 const paths = {desktopPath: path.join(root, 'data', 'desktop.json'), sharedPath: path.join(root, 'config', 'settings.json'), legacyPath: path.join(root, 'data', 'agent-settings.json')}
 return {...paths, ...settingsFiles(paths)}
}
const write = (file, value) => { fs.mkdirSync(path.dirname(file), {recursive: true}); fs.writeFileSync(file, JSON.stringify(value)) }
const read = file => JSON.parse(fs.readFileSync(file, 'utf8'))

test('the first start copies what Desktop stored in settings.json, once, without writing it', () => {
 const f = files()
 const shared = {server: 'https://sectile.test', deviceId: 'dev_1', secret: encrypted('desktop-key'), appearance: 'dark', consoleView: 'terminal', repo: '/r', layout: 4, defaults: {parallelism: 2}}
 write(f.sharedPath, shared)
 const before = fs.readFileSync(f.sharedPath, 'utf8')

 const desktop = f.readDesktop()

 assert.deepStrictEqual(desktop, {appearance: 'dark', consoleView: 'terminal', repo: '/r', server: 'https://sectile.test', deviceId: 'dev_1', secret: shared.secret})
 assert.deepStrictEqual(read(f.desktopPath), desktop)
 assert.strictEqual(fs.readFileSync(f.sharedPath, 'utf8'), before)

 write(f.sharedPath, {...shared, appearance: 'light'})
 assert.strictEqual(f.readDesktop().appearance, 'dark', 'the migration ran twice')
})

test('a key stored in clear and the legacy file migrate the same way', () => {
 const f = files()
 write(f.legacyPath, {server: 'https://sectile.test', apiKey: 'clear-key'})

 assert.deepStrictEqual(f.readDesktop(), {server: 'https://sectile.test', apiKey: 'clear-key'})
})

test('nothing to migrate writes no file', () => {
 const f = files()
 assert.deepStrictEqual(f.readDesktop(), {})
 assert.strictEqual(fs.existsSync(f.desktopPath), false)
})

test('a save writes Desktop\'s own file and never settings.json', () => {
 const f = files()
 write(f.sharedPath, {layout: 4, server: 'https://sectile.test'})
 const before = fs.readFileSync(f.sharedPath, 'utf8')

 f.updateDesktop(saved => { saved.appearance = 'light' })
 f.updateDesktop(saved => { saved.consoleView = 'conversation' })

 assert.deepStrictEqual(read(f.desktopPath), {server: 'https://sectile.test', appearance: 'light', consoleView: 'conversation'})
 assert.strictEqual(fs.readFileSync(f.sharedPath, 'utf8'), before)
 if (process.platform !== 'win32') assert.strictEqual(fs.statSync(f.desktopPath).mode & 0o777, 0o600)
})

test('the newest pairing wins between Desktop and sectile-agent pair', () => {
 const desktop = {server: 'https://sectile.test', secret: encrypted('desktop-key'), pairedAt: '2026-10-06T07:00:00.000Z'}
 const later = {server: 'https://sectile.test/', apiKey: 'cli-key', pairedAt: '2026-10-06T08:00:00Z'}
 const earlier = {...later, pairedAt: '2026-10-06T06:00:00Z'}
 const undated = {server: 'https://sectile.test', apiKey: 'cli-key'}
 const elsewhere = {...later, server: 'https://other.test'}

 assert.strictEqual(effectiveCredential(desktop, later, store).token(), 'cli-key')
 for (const shared of [earlier, undated, elsewhere, {}]) {
  const current = effectiveCredential(desktop, shared, store)
  assert.strictEqual(current.token(), 'desktop-key', JSON.stringify(shared))
  assert.strictEqual(current.source, 'desktop')
 }
 assert.strictEqual(effectiveCredential({server: 'https://sectile.test'}, undated, store).token(), 'cli-key', 'Desktop holds no key')
 assert.strictEqual(effectiveCredential({}, {}, store).state, 'missing')
})

test('a pairing reuses the device either side stored for that server', () => {
 assert.strictEqual(pairedDeviceId({server: 'https://sectile.test', deviceId: 'dev_desktop'}, {server: 'https://sectile.test', deviceId: 'dev_cli'}, 'https://sectile.test/'), 'dev_desktop')
 assert.strictEqual(pairedDeviceId({}, {server: 'https://sectile.test', deviceId: 'dev_cli'}, 'https://sectile.test'), 'dev_cli')
 assert.strictEqual(pairedDeviceId({}, {server: 'https://other.test', deviceId: 'dev_cli'}, 'https://sectile.test'), '')
})
