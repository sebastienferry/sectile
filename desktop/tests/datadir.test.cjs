const test = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const {carryOverDataDirectory} = require('../electron/datadir.cjs')

function directories() {
 const root = fs.mkdtempSync(path.join(os.tmpdir(), 'sectile-datadir-'))
 const previous = path.join(root, 'taskflow-desktop')
 const current = path.join(root, 'sectile-desktop')
 fs.mkdirSync(previous, {recursive: true})
 return {previous, current}
}

test('settings written under the previous name are carried over', () => {
 const {previous, current} = directories()
 fs.writeFileSync(path.join(previous, 'settings.json'), '{"server":"http://localhost:8090"}')

 const carried = carryOverDataDirectory(previous, current)

 assert.deepStrictEqual(carried, ['settings.json'])
 assert.strictEqual(fs.readFileSync(path.join(current, 'settings.json'), 'utf8'), '{"server":"http://localhost:8090"}')
})

test('a configuration already in place is never overwritten', () => {
 const {previous, current} = directories()
 fs.mkdirSync(current, {recursive: true})
 fs.writeFileSync(path.join(previous, 'settings.json'), '{"server":"old"}')
 fs.writeFileSync(path.join(current, 'settings.json'), '{"server":"current"}')

 assert.deepStrictEqual(carryOverDataDirectory(previous, current), [])
 assert.strictEqual(fs.readFileSync(path.join(current, 'settings.json'), 'utf8'), '{"server":"current"}')
})

test('running twice changes nothing the first run did not', () => {
 const {previous, current} = directories()
 fs.writeFileSync(path.join(previous, 'settings.json'), '{"server":"http://localhost:8090"}')

 assert.deepStrictEqual(carryOverDataDirectory(previous, current), ['settings.json'])
 assert.deepStrictEqual(carryOverDataDirectory(previous, current), [])
})

test('nothing to carry over is not an error', () => {
 const {previous, current} = directories()
 assert.deepStrictEqual(carryOverDataDirectory(previous, current), [])
 assert.deepStrictEqual(carryOverDataDirectory(path.join(previous, 'absent'), current), [])
})

test('the same directory is left alone', () => {
 const {previous} = directories()
 fs.writeFileSync(path.join(previous, 'settings.json'), '{}')
 assert.deepStrictEqual(carryOverDataDirectory(previous, previous), [])
})

// The connection file names a gateway port and a secret from an agent session
// that has already ended; carrying it over would point the app at nothing.
test('the stale agent connection file is not carried over', () => {
 const {previous, current} = directories()
 fs.writeFileSync(path.join(previous, 'agent-connection.json'), '{"url":"http://127.0.0.1:8091","token":"stale"}')

 assert.deepStrictEqual(carryOverDataDirectory(previous, current), [])
 assert.strictEqual(fs.existsSync(path.join(current, 'agent-connection.json')), false)
})

test('the carried settings stay private to the user', () => {
 const {previous, current} = directories()
 fs.writeFileSync(path.join(previous, 'settings.json'), '{"secret":"x"}', {mode: 0o644})

 carryOverDataDirectory(previous, current)

 const mode = fs.statSync(path.join(current, 'settings.json')).mode & 0o777
 assert.strictEqual(mode, 0o600, 'carried settings should not be readable by others')
})

test('a failure on one file does not stop the others', () => {
 const {previous, current} = directories()
 fs.writeFileSync(path.join(previous, 'settings.json'), '{"a":1}')
 fs.writeFileSync(path.join(previous, 'agent-settings.json'), '{"b":2}')

 const io = {
  existsSync: fs.existsSync,
  mkdirSync: fs.mkdirSync,
  chmodSync: fs.chmodSync,
  copyFileSync: (from, to) => {
   if (from.endsWith('settings.json') && !from.endsWith('agent-settings.json')) throw Error('disk full')
   return fs.copyFileSync(from, to)
  }
 }
 assert.deepStrictEqual(carryOverDataDirectory(previous, current, io), ['agent-settings.json'])
})
