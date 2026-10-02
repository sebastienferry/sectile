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
  // is opened by an admin, a revoked or unknown key is replaced by signing in.
  let detail=''
  try{detail=(await response.json()).error||''}catch{}
  if(/expired/i.test(detail))throw Error('This API key has expired. Renew it from your profile in the web interface, or sign in again.')
  if(/blocked/i.test(detail))throw Error('This account is blocked. Ask an admin to open it again.')
  throw Error('This API key was revoked or is unknown. Sign in again.')
 }
 // The server could not check the key; the agent starts and retries.
 if(response.status===503)return false
 if(!response.ok)return false
 let catalog
 try{catalog=await response.json()}catch{throw Error('This URL does not return the Sectile agent API. Check the server address.')}
 if(!Array.isArray(catalog.projects))throw Error('This URL does not expose the Sectile agent API. Check the server address.')
 return true
}
module.exports={checkServer}
