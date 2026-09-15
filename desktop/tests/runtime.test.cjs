const {test} = require('node:test')
const assert = require('node:assert/strict')
const fs = require('node:fs')
const os = require('node:os')
const path = require('node:path')
const {agentName, resolveAgentBinary} = require('../electron/runtime.cjs')

test('packaged desktop resolves only its platform agent', t => {
 const root = fs.mkdtempSync(path.join(os.tmpdir(), 'sectile-runtime-'))
 t.after(() => fs.rmSync(root, {recursive: true, force: true}))
 for (const platform of ['darwin', 'linux', 'win32']) {
  const resourcesPath = path.join(root, platform)
  fs.mkdirSync(resourcesPath)
  fs.writeFileSync(path.join(resourcesPath, 'sectile-server'), '')
  const options = {packaged: true, resourcesPath, directory: root, platform}
  assert.throws(() => resolveAgentBinary(options), /agent is missing/)
  const binary = path.join(resourcesPath, agentName(platform))
  fs.writeFileSync(binary, '')
  assert.equal(resolveAgentBinary(options), binary)
 }
})
