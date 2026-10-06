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
test('skill badge follows the next skill started in a console left open',async()=>{
 const {skillResult,workflowSkill}=await import('../src/skill-result.mjs')
 const run={id:'console',taskId:'task',skill:'clarify',status:'running'}
 const own={id:'console',taskId:'task',skillId:'clarify',status:'completed'}
 const after=(successor,labels=['clarified'])=>({activity:own,successor:{id:'next',taskId:'task',skillId:'specify-issue',...successor},task:{labels}})
 // The ended skill's verdict is dropped as soon as another one runs.
 assert.equal(skillResult(run,after({status:'running'})),null)
 assert.equal(skillResult(run,after({status:'running',waitingSince:'2026-10-06T09:00:00Z'})).kind,'waiting')
 // A wait is only asked by a live console.
 assert.equal(skillResult({...run,status:'completed'},after({status:'running',waitingSince:'2026-10-06T09:00:00Z'})),null)
 assert.equal(skillResult({...run,cancelRequested:true},after({status:'running'})).label,'Stopping execution')
 assert.equal(skillResult(run,after({status:'completed'})).label,'Awaiting stage validation')
 assert.equal(skillResult(run,after({status:'completed'},['specified'])).kind,'completed')
 assert.equal(skillResult(run,after({status:'failed'})).kind,'failed')
 assert.equal(skillResult(run,after({status:'canceled'})).kind,'canceled')
 // A skill with no stage of its own is done once it completes.
 assert.equal(skillResult(run,after({status:'completed',skillId:'rewrite-story'})).kind,'completed')
 // A successor only counts beside the console's own activity.
 assert.equal(skillResult(run,{...after({status:'running'}),activity:{...own,id:'other'}}),null)
 // A free console and a discussion still report no skill.
 assert.equal(skillResult({...run,kind:'console'},after({status:'completed'},['specified'])),null)
 assert.equal(skillResult({...run,skill:'discuss'},{...after({status:'completed'},['specified']),activity:{...own,skillId:'discuss'}}),null)
 // Without a successor, nothing changes.
 assert.equal(skillResult(run,{activity:own,successor:null,task:{labels:['clarified']}}).kind,'completed')
 assert.equal(workflowSkill('specify-issue'),'specify')
 assert.equal(workflowSkill('sectile:implement-issue'),'implement')
 assert.equal(workflowSkill('adjust'),'adjust')
 assert.equal(workflowSkill('pickup-issue'),'')
 assert.equal(workflowSkill(undefined),'')
})
