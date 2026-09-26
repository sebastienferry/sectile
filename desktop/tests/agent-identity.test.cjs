const {test} = require('node:test')
const assert = require('node:assert/strict')
const crypto = require('node:crypto')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const {fileSha256, agentOutdated} = require('../electron/agent-identity.cjs')

test('fileSha256 hashes the content and answers null for a missing file', async t => {
 const root = fs.mkdtempSync(path.join(os.tmpdir(), 'sectile-identity-'))
 t.after(() => fs.rmSync(root, {recursive: true, force: true}))
 const binary = path.join(root, 'sectile-agent')
 fs.writeFileSync(binary, 'agent build')
 assert.equal(await fileSha256(binary), crypto.createHash('sha256').update('agent build').digest('hex'))
 assert.equal(await fileSha256(path.join(root, 'missing')), null)
})

test('agentOutdated follows the decision table', () => {
 const cases = [
  {name: 'legacy agent, readable bundle', running: {version: 'v1.0.0'}, bundled: 'aaa', want: true},
  {name: 'same binary', running: {binarySha256: 'aaa'}, bundled: 'aaa', want: false},
  {name: 'rebuilt binary, same version', running: {version: 'dev', binarySha256: 'bbb'}, bundled: 'aaa', want: true},
  {name: 'unreadable bundle', running: {binarySha256: 'bbb'}, bundled: null, want: false},
  {name: 'legacy agent, unreadable bundle', running: {}, bundled: null, want: false},
  {name: 'no answer, readable bundle', running: null, bundled: 'aaa', want: false},
 ]
 for (const {name, running, bundled, want} of cases) assert.equal(agentOutdated({running, bundled}), want, name)
})
