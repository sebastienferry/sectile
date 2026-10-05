const {test}=require('node:test')
const assert=require('node:assert/strict')
const {checkServer}=require('../electron/server-check.cjs')
test('offline server permits local startup, rejected authentication does not',async()=>{
 assert.equal(await checkServer('http://localhost:1','test',async()=>{throw new TypeError('fetch failed')}),false)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:401,ok:false})),/revoked or is unknown/)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:401,ok:false,json:async()=>({error:'API key expired'})})),/has expired/)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:401,ok:false,json:async()=>({error:'Account blocked'})})),/account is blocked/)
 // A server that could not check the key does not stop the agent starting.
 assert.equal(await checkServer('http://localhost:1','test',async()=>({status:503,ok:false,json:async()=>({error:'Authentication temporarily unavailable'})})),false)
 assert.equal(await checkServer('http://localhost:1','test',async()=>({status:200,ok:true,json:async()=>({projects:[]})})),true)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:200,ok:true,json:async()=>({})})),/agent API/)
})
