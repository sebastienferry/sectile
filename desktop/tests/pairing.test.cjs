const test = require('node:test')
const assert = require('node:assert')
const {exchangePairingCode, resolveConnectCredential} = require('../electron/pairing.cjs')

function responder(status, body) {
 return async () => ({
  status,
  ok: status >= 200 && status < 300,
  json: async () => body
 })
}

test('a successful exchange returns the device credential', async () => {
 const result = await exchangePairingCode('http://127.0.0.1:8090', 'code-1', 'laptop',
  responder(201, {token: 'device-token', deviceId: 'dev_1', userId: 'usr_1'}))
 assert.strictEqual(result.token, 'device-token')
 assert.strictEqual(result.deviceId, 'dev_1')
})

test('the code and the label reach the server', async () => {
 let seen
 await exchangePairingCode('http://127.0.0.1:8090', '  code-2  ', 'workstation', async (_, options) => {
  seen = JSON.parse(options.body)
  return {status: 201, ok: true, json: async () => ({token: 't'})}
 })
 assert.strictEqual(seen.code, 'code-2', 'the code is trimmed before it is sent')
 assert.strictEqual(seen.label, 'workstation')
})

test('a rejected code is reported as expired rather than as a server error', async () => {
 await assert.rejects(
  exchangePairingCode('http://127.0.0.1:8090', 'stale', 'laptop', responder(401, {})),
  /Invalid or expired pairing code/)
})

test('an empty code never reaches the network', async () => {
 await assert.rejects(
  exchangePairingCode('http://127.0.0.1:8090', '   ', 'laptop', () => {
   throw Error('the network must not be reached')
  }),
  /pairing code is required/)
})

test('a credentialed URL is refused', async () => {
 await assert.rejects(
  exchangePairingCode('http://user:secret@127.0.0.1:8090', 'code', 'laptop', responder(201, {token: 't'})),
  /HTTP or HTTPS server URL/)
})

test('a response without a token is an error, not a silent success', async () => {
 await assert.rejects(
  exchangePairingCode('http://127.0.0.1:8090', 'code', 'laptop', responder(201, {deviceId: 'dev_1'})),
  /no device credential/)
})

test('a pairing code from the connect form is exchanged for a token', async () => {
 let seen
 const credential = await resolveConnectCredential(
  {server: 'http://127.0.0.1:8090', code: 'code-3', token: ''},
  async (server, code, label) => { seen = {server, code, label}; return {token: 'device-token', deviceId: 'dev_9'} },
  'laptop')
 assert.strictEqual(credential.token, 'device-token')
 assert.strictEqual(credential.deviceId, 'dev_9')
 assert.strictEqual(credential.paired, true)
 assert.deepStrictEqual(seen, {server: 'http://127.0.0.1:8090', code: 'code-3', label: 'laptop'})
})

test('a workstation that is already paired connects on its token alone', async () => {
 const credential = await resolveConnectCredential(
  {server: 'http://127.0.0.1:8090', code: '', token: '  kept-token  '},
  () => { throw Error('an exchange must not be attempted without a code') })
 assert.strictEqual(credential.token, 'kept-token')
 assert.strictEqual(credential.paired, false)
})

// Typing a code is a deliberate act of re-pairing, so it outranks the credential
// an earlier pairing stored.
test('a pairing code outranks a token left in the form', async () => {
 const credential = await resolveConnectCredential(
  {server: 'http://127.0.0.1:8090', code: 'code-4', token: 'stale-token'},
  async () => ({token: 'fresh-token', deviceId: 'dev_10'}))
 assert.strictEqual(credential.token, 'fresh-token')
 assert.strictEqual(credential.paired, true)
})

test('an empty form with no stored credential is refused before anything is spent', async () => {
 await assert.rejects(
  resolveConnectCredential({server: 'http://127.0.0.1:8090', code: '  ', token: '  '},
   () => { throw Error('the network must not be reached') }),
  /Enter a pairing code/)
})

test('a server that answers nothing is reported as unreachable, pointing at the address', async () => {
 await assert.rejects(
  exchangePairingCode('http://127.0.0.1:8090', 'code', 'laptop', async () => { throw Error('ECONNREFUSED') }),
  /Could not reach the server. Check the server address./)
})

test('an address that answers without JSON is reported as not being the Sectile agent API', async () => {
 await assert.rejects(
  exchangePairingCode('http://127.0.0.1:8090', 'code', 'laptop', async () => ({
   status: 200, ok: true, json: async () => { throw Error('not JSON') }
  })),
  /does not expose the Sectile agent API/)
})
