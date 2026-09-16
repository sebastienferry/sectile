const test = require('node:test')
const assert = require('node:assert')
const {exchangePairingCode} = require('../electron/pairing.cjs')

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
