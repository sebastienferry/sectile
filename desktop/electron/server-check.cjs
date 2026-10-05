const {pairingNeeded}=require('./pairing.cjs')
// An unavailable server must not prevent the independent local agent starting.
async function checkServer(server,token,fetcher=fetch){
 let response
 try{
  response=await fetcher(new URL('/api/v1/agent/projects',server),{
   headers:{Authorization:'Bearer '+token},
   signal:AbortSignal.timeout(3000),redirect:'error'
  })
 }catch{return false}
 if(response.status===401||response.status===403){
  // Each refusal names its remedy: an expired key is renewed, a blocked account
  // is opened by an admin, a revoked or unknown key is replaced by pairing again.
  let detail=''
  try{detail=(await response.json()).error||''}catch{}
  if(/expired/i.test(detail))throw pairingNeeded('This API key has expired. Renew it from your profile in the web interface, or pair again.')
  // A new pairing would not open a blocked account, so this refusal does not ask for one.
  if(/blocked/i.test(detail))throw Error('This account is blocked. Ask an admin to open it again.')
  throw pairingNeeded('This API key was revoked or is unknown.')
 }
 // The server could not check the key (a 503 or any other failure): the agent starts and retries.
 if(!response.ok)return false
 let catalog
 try{catalog=await response.json()}catch{throw Error('This URL does not return the Sectile agent API. Check the server address.')}
 if(!Array.isArray(catalog.projects))throw Error('This URL does not expose the Sectile agent API. Check the server address.')
 return true
}
module.exports={checkServer}
