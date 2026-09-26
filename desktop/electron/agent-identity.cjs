const crypto = require('node:crypto')
const fs = require('node:fs')

// The SHA-256 of a file's content, null when it cannot be read. The agent
// reports the same fingerprint of the executable it was started from, which is
// the only identity that changes on a rebuild reporting the same version.
function fileSha256(path) {
 return new Promise(resolve => {
  const hash = crypto.createHash('sha256')
  const stream = fs.createReadStream(path)
  stream.on('error', () => resolve(null))
  stream.on('data', chunk => hash.update(chunk))
  stream.on('end', () => resolve(hash.digest('hex')))
 })
}

// Whether the running agent is not the binary this app would start. An agent
// that cannot say which binary it runs predates the fingerprint, so it is older
// than this app. An agent that did not answer, or a bundled binary that cannot
// be read, is no reason to prompt: nothing is known, and the start path already
// reports a missing binary.
function agentOutdated({running, bundled}) {
 if (!bundled || !running) return false
 return !running?.binarySha256 || running.binarySha256 !== bundled
}

module.exports = {fileSha256, agentOutdated}
