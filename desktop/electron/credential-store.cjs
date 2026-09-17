// The API key goes in the settings file either way: encrypted with the OS store
// when there is one, in clear under the same 0600 permissions otherwise. A
// pairing code is single use, so a key that was not saved would be lost and the
// next launch would demand a fresh code, indefinitely, on a host without a
// secret service. Injecting the store keeps this pair testable without Electron.
function storeKey(saved,token,store){
 if(store.isEncryptionAvailable()){saved.secret=store.encryptString(token).toString('base64');delete saved.apiKey}
 else{saved.apiKey=token;delete saved.secret}
}
function storedKey(saved,store){
 if(saved.secret&&store.isEncryptionAvailable())return store.decryptString(Buffer.from(saved.secret,'base64'))
 return saved.apiKey||''
}
module.exports={storeKey,storedKey}
