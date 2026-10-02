const test = require('node:test')
const assert = require('node:assert')
const {storeKey, storedKey, keyStatus} = require('../electron/credential-store.cjs')

// A host offering a secret service, and one offering none: the two persistence
// paths the desktop application has to survive.
const keyring = {
 isEncryptionAvailable: () => true,
 encryptString: text => Buffer.from('enc:' + text),
 decryptString: buffer => String(buffer).replace(/^enc:/, '')
}
const bare = {
 isEncryptionAvailable: () => false,
 encryptString: () => { throw Error('no encryption store here') },
 decryptString: () => { throw Error('no encryption store here') }
}
// What an earlier version wrote on a keyring host: the key encrypted, nothing in clear.
const legacySecret = token => keyring.encryptString(token).toString('base64')

// The key is kept in clear in the 0600 settings file so the standalone agent
// reads the same key Desktop stores (ADR 0049): no encrypted copy is written.
test('a host with a secure store keeps the credential in clear, as the standalone agent reads it', () => {
 const saved = {server: 'http://127.0.0.1:8090'}
 storeKey(saved, 'device-token')

 assert.strictEqual(saved.apiKey, 'device-token')
 assert.strictEqual(saved.secret, undefined, 'no encrypted copy is written')
 assert.strictEqual(storedKey(saved, keyring), 'device-token')
})

// Refusing to save here would be the safer-looking option and the worse one: the
// code is already spent, so the credential would be lost for good.
test('a host without a secure store still persists the credential, in clear', () => {
 const saved = {server: 'http://127.0.0.1:8090'}
 storeKey(saved, 'device-token')

 assert.strictEqual(saved.apiKey, 'device-token')
 assert.strictEqual(saved.secret, undefined)
 assert.strictEqual(storedKey(saved, bare), 'device-token', 'a later launch connects with it')
})

test('re-pairing on a keyring host leaves no encrypted key from an earlier version', () => {
 const saved = {server: 'http://127.0.0.1:8090', secret: legacySecret('old-token')}
 storeKey(saved, 'fresh-token')

 assert.strictEqual(saved.apiKey, 'fresh-token')
 assert.strictEqual(saved.secret, undefined)
 assert.strictEqual(storedKey(saved, keyring), 'fresh-token')
})

test('a legacy encrypted key is still read, and the next store replaces it', () => {
 const saved = {server: 'http://127.0.0.1:8090', secret: legacySecret('legacy-token')}
 const token = storedKey(saved, keyring)
 assert.strictEqual(token, 'legacy-token')

 storeKey(saved, token)
 assert.strictEqual(saved.apiKey, 'legacy-token')
 assert.strictEqual(saved.secret, undefined, 'the encrypted form is retired')
})

test('a clear key written by `sectile-agent pair` outranks an older encrypted one', () => {
 const saved = {server: 'http://127.0.0.1:8090', secret: legacySecret('old-token'), apiKey: 'cli-token'}

 assert.strictEqual(storedKey(saved, keyring), 'cli-token')
})

// A key that cannot be read is said so rather than read as no key: the user is
// told to sign in again instead of being shown an unexplained pairing screen.
test('losing the keyring makes a legacy encrypted key unreadable, not absent', () => {
 const saved = {server: 'http://127.0.0.1:8090', secret: legacySecret('device-token')}

 assert.throws(() => storedKey(saved, bare), /cannot be read on this machine/)
})

test('settings holding no credential read back empty', () => {
 assert.strictEqual(storedKey({server: 'http://127.0.0.1:8090'}, keyring), '')
})

test('keyStatus: none when no key is stored', () => {
 assert.strictEqual(keyStatus({server: 'http://127.0.0.1:8090'}, keyring), 'none')
 assert.strictEqual(keyStatus(undefined, keyring), 'none')
})

test('keyStatus: ok when a key is stored', () => {
 assert.strictEqual(keyStatus({apiKey: 'device-token'}, bare), 'ok')
 assert.strictEqual(keyStatus({secret: legacySecret('device-token')}, keyring), 'ok')
})

test('keyStatus: unreadable when decrypt throws', () => {
 const broken = {...keyring, decryptString: () => { throw Error('the key was sealed by another user') }}
 assert.strictEqual(keyStatus({secret: legacySecret('device-token')}, broken), 'unreadable')
})

test('keyStatus: unreadable when the store is unavailable', () => {
 assert.strictEqual(keyStatus({secret: legacySecret('device-token')}, bare), 'unreadable')
})
