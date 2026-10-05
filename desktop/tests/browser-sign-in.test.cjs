const test = require('node:test')
const assert = require('node:assert')
const http = require('node:http')
const {browserSignIn} = require('../electron/browser-sign-in.cjs')

// Stands for the browser and the server's /auth/workstation: it reads the port
// and state the desktop put in the URL and calls the loopback callback back.
function callback(href, {code = 'pairing-code', state} = {}) {
 const url = new URL(href)
 const params = new URLSearchParams({code, state: state ?? url.searchParams.get('state')})
 return fetch('http://127.0.0.1:' + url.searchParams.get('port') + '/callback?' + params)
}

test('the browser sign-in returns the pairing code the server sent back', async () => {
 let answered
 const code = await browserSignIn('http://127.0.0.1:8090', {open: href => {
  answered = callback(href, {code: 'code-1'}).then(async response => ({status: response.status, text: await response.text()}))
 }})
 assert.strictEqual(code, 'code-1')
 const page = await answered
 assert.strictEqual(page.status, 200)
 assert.match(page.text, /You can close this tab and return to Sectile Desktop/)
})

test('a wrong state is answered 400 and the flow still completes with the right one', async () => {
 let refused
 const code = await browserSignIn('http://127.0.0.1:8090', {open: async href => {
  refused = (await callback(href, {code: 'forged', state: 'not-the-state'})).status
  await callback(href, {code: 'genuine'})
 }})
 assert.strictEqual(refused, 400)
 assert.strictEqual(code, 'genuine')
})

test('a sign-in nobody completes times out', async () => {
 await assert.rejects(
  browserSignIn('http://127.0.0.1:8090', {open: () => {}, timeout: 50}),
  /Sign-in timed out. Try again, or use a pairing code./)
})

test('an aborted sign-in is cancelled and stops listening', async () => {
 const abort = new AbortController()
 let port
 const pending = browserSignIn('http://127.0.0.1:8090', {signal: abort.signal, open: href => {
  port = new URL(href).searchParams.get('port')
  abort.abort()
 }})
 await assert.rejects(pending, /Sign-in cancelled/)
 await assert.rejects(fetch('http://127.0.0.1:' + port + '/callback'), 'the listener is closed')
})

test('the opened URL is {server}/auth/workstation with only port and state', async () => {
 let opened
 await browserSignIn('https://sectile.example.test', {open: async href => { opened = new URL(href); await callback(href) }})
 assert.strictEqual(opened.origin + opened.pathname, 'https://sectile.example.test/auth/workstation')
 assert.deepStrictEqual([...opened.searchParams.keys()].sort(), ['port', 'state'])
 assert.match(opened.searchParams.get('port'), /^\d+$/)
 assert.ok(opened.searchParams.get('state').length >= 43, 'the state carries 32 random bytes')
})

test('a browser that cannot be opened is reported', async () => {
 await assert.rejects(
  browserSignIn('http://127.0.0.1:8090', {open: async () => { throw Error('no browser here') }}),
  /Could not open the browser: no browser here/)
})

test('a credentialed server URL is refused before listening', async () => {
 const listen = http.Server.prototype.listen
 let listened = false
 http.Server.prototype.listen = function (...args) { listened = true; return listen.apply(this, args) }
 try {
  await assert.rejects(
   async () => browserSignIn('http://user:secret@127.0.0.1:8090', {open: () => { throw Error('the browser must not be opened') }}),
   /Use an HTTP or HTTPS server URL/)
  assert.strictEqual(listened, false)
 } finally {
  http.Server.prototype.listen = listen
 }
})
