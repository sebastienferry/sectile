// Exchanging a pairing code is the one moment the desktop app talks to the
// server without a credential: the code is the proof, and it is spent here.
const os = require('node:os')

async function exchangePairingCode(server, code, label = os.hostname(), fetcher = fetch) {
 if (!String(code || '').trim()) throw Error('A pairing code is required')
 const url = new URL(server)
 if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password) {
  throw Error('Use an HTTP or HTTPS server URL')
 }
 let response
 try {
  response = await fetcher(new URL('/api/v1/agent/pair', server), {
   method: 'POST',
   headers: {'Content-Type': 'application/json'},
   body: JSON.stringify({code: String(code).trim(), label}),
   signal: AbortSignal.timeout(5000),
   redirect: 'error'
  })
 } catch {
  throw Error('Could not reach the server. Check the server address.')
 }
 if (response.status === 401) throw Error('Invalid or expired pairing code. Generate a new one.')
 if (!response.ok) throw Error('The server refused the pairing request (HTTP ' + response.status + ').')
 let body
 try { body = await response.json() } catch { throw Error('This URL does not expose the Sectile agent API.') }
 if (!body || typeof body.token !== 'string' || !body.token) {
  throw Error('The server returned no device credential.')
 }
 return {token: body.token, deviceId: body.deviceId, userId: body.userId}
}

// Pairing is the only way in: the form asks for a code, which is spent once for
// a device credential. `token` is not something a user types any more, it is the
// credential an earlier pairing stored; the code wins when both are present,
// because typing one is a deliberate re-pairing.
async function resolveConnectCredential(settings, exchange = exchangePairingCode, label = os.hostname()) {
 const code = String(settings.code || '').trim()
 if (!code) {
  const token = String(settings.token || '').trim()
  if (!token) throw Error('Enter a pairing code from your profile in the web interface')
  return {token, paired: false}
 }
 const credential = await exchange(settings.server, code, label)
 return {token: credential.token, deviceId: credential.deviceId, paired: true}
}

module.exports = {exchangePairingCode, resolveConnectCredential}
