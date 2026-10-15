import test from 'node:test'
import assert from 'node:assert/strict'
import { browserAgent } from '../browser-agent.mjs'

function invoke(middleware, overrides = {}) {
  const req = { url: '/__desktop/status', method: 'GET', headers: {
    host: '127.0.0.1:5173', 'x-sectile-preview': '1', origin: 'http://127.0.0.1:5173',
  }, ...overrides }
  const res = { statusCode: 200, setHeader() {}, end(body) { this.body = body } }
  return middleware(req, res, () => { res.next = true }).then(() => res)
}

test('the browser bridge rejects external origins, forged hosts, writes and unknown routes before reading credentials', async () => {
  const middleware = browserAgent({ readConnection: () => { throw Error('Credentials must not be read') } })
  for (const request of [
    { headers: { host: 'evil.example', 'x-sectile-preview': '1' } },
    { headers: { host: '127.0.0.1:5173', 'x-sectile-preview': '1', origin: 'https://evil.example' } },
    { headers: { host: '127.0.0.1:5173' } },
  ]) assert.equal((await invoke(middleware, request)).statusCode, 403)
  assert.equal((await invoke(middleware, { method: 'POST' })).statusCode, 405)
  assert.equal((await invoke(middleware, { url: '/__desktop/mcp' })).statusCode, 404)
})

test('the agent credential stays in the upstream request and queries are preserved', async () => {
  const middleware = browserAgent({
    readConnection: async () => ({ url: 'http://127.0.0.1:8091', token: 'private-test-key' }),
    request: async (url, options) => {
      assert.equal(url.href, 'http://127.0.0.1:8091/desktop/project?id=example')
      assert.equal(options.headers.Authorization, 'Bearer private-test-key')
      assert.equal(options.redirect, 'error')
      return new Response('{"id":"example"}')
    },
  })
  const result = await invoke(middleware, { url: '/__desktop/project?id=example' })
  assert.equal(result.body, '{"id":"example"}')
  assert.equal(result.statusCode, 200)
})

test('invalid upstream targets and missing agents return a useful error without disclosing credentials', async () => {
  const middleware = browserAgent({
    readConnection: async () => ({ url: 'https://example.com', token: 'private-test-key' }),
    request: () => { assert.fail('Must not contact a nonlocal address') },
  })
  const result = await invoke(middleware)
  assert.equal(result.statusCode, 503)
  assert.match(result.body, /Open Sectile Desktop/)
  assert.ok(!result.body.includes('private-test-key'))
})
