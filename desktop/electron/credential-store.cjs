// The API key is stored in clear in the 0600 settings file, as `sectile-agent pair` does, so the standalone agent reads
// the same key (ADR 0049). `secret` is what earlier versions encrypted with the OS store: read once, then replaced.
// Injecting the store keeps this pair testable without Electron.
function storeKey(saved,token){saved.apiKey=token;delete saved.secret}
function storedKey(saved,store){
 if(saved.apiKey)return saved.apiKey
 if(!saved.secret)return ''
 if(!store.isEncryptionAvailable())throw Error('The stored API key cannot be read on this machine. Sign in again.')
 return store.decryptString(Buffer.from(saved.secret,'base64'))
}
// keyStatus says why the desktop cannot start on its own: no key stored, or one it cannot read.
function keyStatus(saved,store){try{return storedKey(saved||{},store)?'ok':'none'}catch{return 'unreadable'}}
module.exports={storeKey,storedKey,keyStatus}
