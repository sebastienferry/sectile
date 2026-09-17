const test = require('node:test')
const assert = require('node:assert')
const {storeKey, storedKey} = require('../electron/credential-store.cjs')

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

test('a host with a secure store keeps the credential encrypted and nowhere in clear', () => {
 const saved = {server: 'http://127.0.0.1:8090'}
 storeKey(saved, 'device-token', keyring)

 assert.strictEqual(saved.apiKey, undefined, 'no clear copy remains')
 assert.notStrictEqual(saved.secret, undefined)
 assert.ok(!JSON.stringify(saved).includes('device-token'), 'the token does not appear in the settings file')
 assert.strictEqual(storedKey(saved, keyring), 'device-token')
})

// Refusing to save here would be the safer-looking option and the worse one: the
// code is already spent, so the credential would be lost for good.
test('a host without a secure store still persists the credential, in clear', () => {
 const saved = {server: 'http://127.0.0.1:8090'}
 storeKey(saved, 'device-token', bare)

 assert.strictEqual(saved.apiKey, 'device-token')
 assert.strictEqual(saved.secret, undefined)
 assert.strictEqual(storedKey(saved, bare), 'device-token', 'a later launch connects with it')
})

test('re-pairing on a keyring host leaves no clear key from an earlier bare launch', () => {
 const saved = {server: 'http://127.0.0.1:8090'}
 storeKey(saved, 'old-token', bare)
 storeKey(saved, 'fresh-token', keyring)

 assert.strictEqual(saved.apiKey, undefined)
 assert.strictEqual(storedKey(saved, keyring), 'fresh-token')
})

test('losing the keyring drops the encrypted form rather than failing to read', () => {
 const saved = {server: 'http://127.0.0.1:8090'}
 storeKey(saved, 'device-token', keyring)

 assert.strictEqual(storedKey(saved, bare), '', 'an unreadable secret reads empty instead of throwing')
})

test('settings holding no credential read back empty', () => {
 assert.strictEqual(storedKey({server: 'http://127.0.0.1:8090'}, keyring), '')
})
