// Sectile Desktop keeps what it stores in a file of its own, desktop.json in
// its data directory, and only reads the workstation settings.json, which the
// agent and `sectile-agent pair` write. The two files never share a writer, so
// a save of one process can no longer put back what the other just changed
// (#746, ADR 0053).
const path = require('node:path')
const fs = require('node:fs')
const credentials = require('./credential-store.cjs')

// DESKTOP_KEYS are what Desktop stores. Before #746 they lived in settings.json,
// from which the first start of this version copies them once.
const DESKTOP_KEYS = ['appearance', 'consoleView', 'repo', 'binary', 'server', 'deviceId', 'secret', 'apiKey', 'pairedAt']

// sameServer compares two server addresses as the agent stores them: trimmed, without a trailing slash.
function sameServer(a, b) {
 const canonical = value => String(value || '').trim().replace(/\/+$/, '')
 return Boolean(canonical(a)) && canonical(a) === canonical(b)
}

// pairedLater reports a pairing stamped after another one. An undated pairing,
// made before #746, is older than any dated one.
function pairedLater(a, b) {
 const time = value => (value ? Date.parse(value) : NaN)
 if (Number.isNaN(time(a))) return false
 return Number.isNaN(time(b)) || time(a) > time(b)
}

// settingsFiles binds the two files to their paths: desktopPath is Desktop's
// own, sharedPath the workstation settings.json, legacyPath the file of the
// versions that predate it.
function settingsFiles({desktopPath, sharedPath, legacyPath, io = fs}) {
 // readShared reads settings.json, else the legacy file; never writes either.
 function readShared() {
  try { return JSON.parse(io.readFileSync(sharedPath, 'utf8')) }
  catch (error) {
   if (error.code !== 'ENOENT') throw error
   try { return JSON.parse(io.readFileSync(legacyPath, 'utf8')) } catch { return {} }
  }
 }
 function write(saved) {
  io.mkdirSync(path.dirname(desktopPath), {recursive: true, mode: 0o700})
  io.writeFileSync(desktopPath + '.tmp', JSON.stringify(saved, null, 2), {mode: 0o600})
  io.renameSync(desktopPath + '.tmp', desktopPath)
 }
 // readDesktop reads desktop.json. When it does not exist yet, what Desktop
 // stored in settings.json is copied into it, once; settings.json is left as
 // it is, and the agent removes those keys from it.
 function readDesktop() {
  try { return JSON.parse(io.readFileSync(desktopPath, 'utf8')) }
  catch (error) { if (error.code !== 'ENOENT') throw error }
  let shared = {}
  try { shared = readShared() } catch {}
  const carried = {}
  for (const key of DESKTOP_KEYS) if (shared[key] !== undefined) carried[key] = shared[key]
  if (Object.keys(carried).length) write(carried)
  return carried
 }
 // updateDesktop applies change to Desktop's settings and saves them. Every
 // caller runs in the Electron main process and the cycle is synchronous, so
 // two of them never interleave.
 function updateDesktop(change) {
  let saved = {}
  try { saved = readDesktop() } catch {}
  saved = {...saved}
  change(saved)
  write(saved)
  return saved
 }
 return {readShared, readDesktop, updateDesktop}
}

// effectiveCredential is the pairing Desktop starts the agent on: its own,
// unless settings.json holds a key `sectile-agent pair` stored for the same
// server after it, or Desktop holds no key at all. The record returned carries
// the server, the device, the key fields and pairedAt; the key is read through
// the credential store, so an encrypted one never leaves this process.
function effectiveCredential(desktop, shared, store) {
 const own = desktop || {}
 const cli = shared || {}
 const ownHasKey = Boolean(own.secret || own.apiKey)
 const sameTarget = !own.server || !cli.server || sameServer(cli.server, own.server)
 const useShared = Boolean(cli.apiKey) && sameTarget && (!ownHasKey || pairedLater(cli.pairedAt, own.pairedAt))
 const record = useShared
  ? {server: cli.server || own.server, deviceId: cli.deviceId || own.deviceId, apiKey: cli.apiKey, pairedAt: cli.pairedAt}
  : {server: own.server, deviceId: own.deviceId, secret: own.secret, apiKey: own.apiKey, pairedAt: own.pairedAt}
 return {
  record,
  source: useShared ? 'shared' : 'desktop',
  state: credentials.keyState(record, store),
  token: () => credentials.storedKey(record, store),
 }
}

// pairedDeviceId is the device this workstation was paired as on that server,
// Desktop's own first, then the one `sectile-agent pair` stored, so a new
// pairing replaces its key rather than registering a second device (#717).
function pairedDeviceId(desktop, shared, server) {
 for (const saved of [desktop || {}, shared || {}]) {
  if (sameServer(saved.server, server) && saved.deviceId) return saved.deviceId
 }
 return ''
}

module.exports = {DESKTOP_KEYS, settingsFiles, effectiveCredential, pairedDeviceId, pairedLater, sameServer}
