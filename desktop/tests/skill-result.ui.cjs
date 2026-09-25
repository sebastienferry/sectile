const {test}=require('node:test')
const assert=require('node:assert/strict')
test('skill checkmark requires matching execution and workflow progress',async()=>{
 const {skillResult}=await import('../src/skill-result.mjs')
 const run={id:'new',taskId:'task',skill:'clarify',status:'running'}
 const result={activity:{id:'new',taskId:'task',skillId:'clarify',status:'completed'},task:{labels:['clarified']}}
 assert.equal(skillResult(run,result).kind,'completed')
 // An activity belonging to another execution is no verdict on this one, and a
 // running execution has nothing else to report that the run state does not.
 assert.equal(skillResult(run,{...result,activity:{...result.activity,id:'old'}}),null)
 assert.equal(skillResult(run,null),null)
 assert.equal(skillResult({...run,status:'queued'},null),null)
 assert.equal(skillResult({...run,status:'failed'},null),null)
 assert.equal(skillResult({...run,status:'canceled'},null),null)
 // A free console runs no skill; only a pending stop is still worth a word.
 assert.equal(skillResult({...run,kind:'console'},null),null)
 assert.equal(skillResult({...run,kind:'console',status:'canceled'},null),null)
 assert.equal(skillResult({...run,kind:'console',cancelRequested:true},null).label,'Stopping console')
 // Nor does a discussion, whatever the server recorded for it.
 const discussion={...run,skill:'discuss'}
 assert.equal(skillResult(discussion,{...result,activity:{...result.activity,skillId:'discuss'}}),null)
 assert.equal(skillResult({...discussion,status:'completed'},null),null)
 assert.equal(skillResult({...discussion,status:'canceled'},{...result,activity:{...result.activity,skillId:'discuss',status:'canceled'}}),null)
 assert.equal(skillResult({...discussion,cancelRequested:true},null).label,'Stopping discussion')
 // A stop already taken effect is no longer pending.
 assert.equal(skillResult({...run,status:'canceled',cancelRequested:true},null),null)
 assert.equal(skillResult(run,{...result,task:{labels:['new']}}).label,'Awaiting stage validation')
 assert.equal(skillResult({...run,status:'completed'},null).kind,'pending')
 assert.equal(skillResult(run,{...result,activity:{...result.activity,status:'failed'}}).kind,'failed')
 assert.equal(skillResult(run,{...result,activity:{...result.activity,status:'canceled'}}).kind,'canceled')
 assert.equal(skillResult({...run,cancelRequested:true},null).label,'Stopping execution')
 assert.equal(skillResult(null,result),null)
})
test('skill badge reports a declared wait for the user',async()=>{
 const {skillResult}=await import('../src/skill-result.mjs')
 const run={id:'new',taskId:'task',skill:'clarify',status:'running',waitingSince:'2026-09-25T10:00:00Z'}
 const waiting=skillResult(run,null)
 assert.equal(waiting.kind,'waiting')
 assert.equal(waiting.label,'Waiting for your answer')
 // A server verdict outranks a start of wait left behind on the run.
 const done={activity:{id:'new',taskId:'task',skillId:'clarify',status:'completed'},task:{labels:['clarified']}}
 assert.equal(skillResult(run,done).kind,'completed')
 // A pending stop is what the user is about to see happen, not a question.
 assert.equal(skillResult({...run,cancelRequested:true},null).label,'Stopping execution')
 // An ended or queued run asks nothing, whatever mark it still carries.
 assert.equal(skillResult({...run,status:'failed'},null),null)
 assert.equal(skillResult({...run,status:'queued'},null),null)
 // A console and a discussion run no skill, so no skill is waiting.
 assert.equal(skillResult({...run,kind:'console'},null),null)
 assert.equal(skillResult({...run,skill:'discuss'},null),null)
})
