const {test}=require('node:test')
const assert=require('node:assert/strict')
test('skill checkmark requires matching execution and workflow progress',async()=>{
 const {skillResult}=await import('../src/skill-result.mjs')
 const run={id:'new',taskId:'task',skill:'clarify',status:'running'}
 const result={activity:{id:'new',taskId:'task',skillId:'clarify',status:'completed'},task:{labels:['clarified']}}
 assert.equal(skillResult(run,result).kind,'completed')
 assert.equal(skillResult(run,{...result,activity:{...result.activity,id:'old'}}).label,'In progress')
 assert.equal(skillResult(run,{...result,task:{labels:['new']}}).label,'Awaiting stage validation')
 assert.equal(skillResult({...run,status:'completed'},null).kind,'pending')
 assert.equal(skillResult(run,{...result,activity:{...result.activity,status:'failed'}}).kind,'failed')
 assert.equal(skillResult(run,{...result,activity:{...result.activity,status:'canceled'}}).kind,'canceled')
 assert.equal(skillResult({...run,cancelRequested:true},null).label,'Stopping execution')
 assert.equal(skillResult(null,result),null)
})
