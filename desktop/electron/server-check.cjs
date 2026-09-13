// An unavailable server must not prevent the independent local agent starting.
async function checkServer(server,token,fetcher=fetch){
 let response
 try{
  response=await fetcher(new URL('/api/v1/agent/projects',server),{
   headers:{Authorization:'Bearer '+token},
   signal:AbortSignal.timeout(3000),redirect:'error'
  })
 }catch{return false}
 if(response.status===401||response.status===403)throw Error('Authentication rejected by the server. Check your token.')
 if(!response.ok)return false
 let catalog
 try{catalog=await response.json()}catch{throw Error('This URL does not return the TaskFlow agent API. Check the server address.')}
 if(!Array.isArray(catalog.projects))throw Error('This URL does not expose the TaskFlow agent API. Check the server address.')
 return true
}
module.exports={checkServer}
