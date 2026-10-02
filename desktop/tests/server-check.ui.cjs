const {test}=require('node:test')
const assert=require('node:assert/strict')
const {checkServer}=require('../electron/server-check.cjs')
test('offline server permits local startup, rejected authentication does not',async()=>{
 assert.equal(await checkServer('http://localhost:1','test',async()=>{throw new TypeError('fetch failed')}),false)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:401,ok:false})),/Authentication rejected/)
 assert.equal(await checkServer('http://localhost:1','test',async()=>({status:200,ok:true,json:async()=>({projects:[]})})),true)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:200,ok:true,json:async()=>({})})),/agent API/)
})
// A refused key asks for a new pairing (#716); a URL that is not Sectile does not.
test('only an authentication refusal is flagged as needing a pairing',async()=>{
 for(const status of [401,403])await assert.rejects(checkServer('http://localhost:1','test',async()=>({status,ok:false,json:async()=>({})})),err=>/Authentication rejected/.test(err.message)&&err.pairingNeeded===true)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:401,ok:false,json:async()=>({error:'API key expired'})})),err=>/has expired/.test(err.message)&&err.pairingNeeded===true)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:200,ok:true,json:async()=>({})})),err=>/agent API/.test(err.message)&&err.pairingNeeded===undefined)
 await assert.rejects(checkServer('http://localhost:1','test',async()=>({status:200,ok:true,json:async()=>{throw Error('not JSON')}})),err=>/agent API/.test(err.message)&&err.pairingNeeded===undefined)
 assert.equal(await checkServer('http://localhost:1','test',async()=>({status:500,ok:false})),false)
})
