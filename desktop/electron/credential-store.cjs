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
// keyState says whether a key is usable, never what it is: a secret the OS store can no longer read counts as unreadable.
function keyState(saved,store){
 if(saved.secret){if(!store.isEncryptionAvailable())return 'unreadable';try{return store.decryptString(Buffer.from(saved.secret,'base64'))?'present':'unreadable'}catch{return 'unreadable'}}
 return saved.apiKey?'present':'missing'
}
module.exports={storeKey,storedKey,keyState}
