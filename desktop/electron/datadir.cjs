// Electron derives userData from the package name, so renaming the app to
// Sectile moved its data directory and left the previous settings behind.
// These files are carried over once, rather than asking everyone to configure
// the app again.
const path = require('node:path')
const fs = require('node:fs')

// agent-connection.json is deliberately absent: it names a gateway port and a
// secret belonging to an agent session that is already over.
const CARRIED_FILES = ['settings.json', 'agent-settings.json']

// carryOverDataDirectory copies what the previous directory holds and the
// current one lacks. An existing file is never overwritten, so a fresh
// configuration always wins over an old one, and running twice changes
// nothing the first run did not.
function carryOverDataDirectory(previousDir, currentDir, io = fs) {
 const carried = []
 if (!previousDir || !currentDir || path.resolve(previousDir) === path.resolve(currentDir)) return carried
 for (const name of CARRIED_FILES) {
  const from = path.join(previousDir, name)
  const to = path.join(currentDir, name)
  try {
   if (!io.existsSync(from) || io.existsSync(to)) continue
   io.mkdirSync(currentDir, {recursive: true, mode: 0o700})
   io.copyFileSync(from, to)
   io.chmodSync(to, 0o600)
   carried.push(name)
  } catch (error) {
   // A failed carry-over must not stop the app: the worst case is the
   // configuration screen, not a crash.
   console.error('[Sectile] Could not carry over ' + name + ': ' + error.message)
  }
 }
 return carried
}

module.exports = {carryOverDataDirectory, CARRIED_FILES}
