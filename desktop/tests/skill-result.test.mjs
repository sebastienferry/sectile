import {test} from 'node:test'
import assert from 'node:assert/strict'
import {primaryStatus,recordedReason,workflowSkill,REASON_LIMIT} from '../src/skill-result.mjs'

// One test per row of the precedence table of specs/878-clear-skill-status/spec.md,
// then the cases the ticket names. A row number in a test name is the row it pins.
const DASH=' '+String.fromCharCode(0x2014)+' '
const run={id:'run',taskId:'task',skill:'clarify',status:'running'}
const record=(fields={})=>({id:'run',taskId:'task',skillId:'clarify',status:'running',summary:'',error:'',disconnected:false,...fields})
const result=(fields={},labels=['new'],extra={})=>({activity:record(fields),successor:null,task:{labels},...extra})
const status=(...args)=>{const value=primaryStatus(...args);return value&&{kind:value.kind,label:value.label,detail:value.detail,next:value.next}}

test('row 1: a stop asked for and not taken effect yet is Stopping',()=>{
 assert.deepEqual(status({...run,cancelRequested:true},result()),{kind:'stopping',label:'Stopping',detail:'Waiting for the session to exit',next:''})
 // It outranks a verdict the skill reached a moment before.
 assert.equal(primaryStatus({...run,cancelRequested:true},result({status:'completed'},['clarified'])).kind,'stopping')
 // Once the process exited, the stop is no longer pending.
 assert.equal(primaryStatus({...run,status:'canceled',cancelRequested:true},null).kind,'stopped')
})

test('row 2: a completion on a reached stage is Done and names its outcome',()=>{
 assert.deepEqual(status(run,result({status:'completed'},['clarified']),{nextStep:'Specify'}),{kind:'done',label:'Done · Clarified',detail:'',next:'Next: Specify'})
 const outcome=(skill,labels)=>primaryStatus({...run,skill},result({skillId:skill,status:'completed'},labels)).label
 assert.equal(outcome('specify',['specified']),'Done · Specification ready')
 assert.equal(outcome('implement',['implemented']),'Done · Implemented')
 assert.equal(outcome('adjust',['reviewed']),'Done · Review complete')
 assert.equal(outcome('handoff',['finished']),'Done · Handed off')
 // A stage past the expected one is reached too.
 assert.equal(outcome('clarify',['specified']),'Done · Clarified')
 // No hint when there is no next step, nor once the task is finished.
 assert.equal(primaryStatus(run,result({status:'completed'},['clarified'])).next,'')
 assert.equal(primaryStatus({...run,skill:'handoff'},result({skillId:'handoff',status:'completed'},['finished']),{nextStep:'Clarify'}).next,'')
})

test('row 2: an open console after completion is Done, not Working',()=>{
 const shown=primaryStatus({...run,status:'running'},result({status:'completed'},['clarified']))
 assert.equal(shown.kind,'done')
 assert.match(shown.diagnostics,/^Execution: Running · Skill record: completed · Task stage: clarified$/)
})

test('row 2: a skill with no stage is done on its own word',()=>{
 for(const skill of ['create_pr','rewrite_story','refine_macro','realign_macro','rewrite-story','sectile:create-pr']){
  assert.equal(primaryStatus({...run,skill},result({skillId:skill,status:'completed'})).label,'Done · reported by the skill',skill)
 }
})

test('row 3: a completion before stage validation needs verification, then is Done',()=>{
 assert.deepEqual(status(run,result({status:'completed'},['new'])),{kind:'verify',label:'Needs verification',detail:'Clarify reported completion; the task is still at new',next:''})
 assert.equal(primaryStatus(run,result({status:'completed'},['clarified'])).kind,'done')
})

test('row 4: a failed skill could not finish and says why',()=>{
 const failed=fields=>primaryStatus(run,result({status:'failed',...fields}))
 assert.deepEqual(status(run,result({status:'failed',error:'git push refused\nremote: denied'})),{kind:'failed',label:'Could not finish',detail:'git push refused',next:'Open the console'})
 assert.equal(failed({error:'git push refused\nremote: denied'}).reason,'git push refused\nremote: denied')
 // The note stands in for a missing error, without the server's bookkeeping.
 assert.equal(failed({summary:'Execution reported by a local agent or native client'+DASH+'Les tests échouent'}).detail,'Les tests échouent')
 assert.equal(failed({summary:'No MCP call for 12 minutes'+DASH+'Blocked on credentials'}).detail,'Blocked on credentials')
 assert.equal(failed({summary:'Execution reported by a local agent or native client'+DASH+'No MCP call for 12 minutes'}).detail,'reason not recorded')
 // An agent older than the reason fields sends neither key.
 assert.equal(primaryStatus(run,{activity:{id:'run',taskId:'task',skillId:'clarify',status:'failed'},task:{labels:['new']}}).detail,'reason not available')
 // Once the console exited, the next step is a relaunch.
 assert.equal(primaryStatus({...run,status:'completed'},result({status:'failed'})).next,'Relaunch Clarify')
})

test('row 4: a long reason is shortened to about 120 characters',()=>{
 const long='x'.repeat(300)
 const shown=primaryStatus(run,result({status:'failed',error:long}))
 assert.equal(shown.detail.length,REASON_LIMIT)
 assert.ok(shown.detail.endsWith('…'))
 assert.equal(shown.reason,long)
})

test('row 5: a stopped skill names only the reason on record',()=>{
 assert.deepEqual(status({...run,status:'canceled',cancelRequested:true},result({status:'canceled'})),{kind:'stopped',label:'Stopped',detail:'Stopped by you',next:''})
 assert.equal(primaryStatus({...run,status:'canceled'},result({status:'canceled',disconnected:true})).detail,'Session disconnected')
 assert.equal(primaryStatus({...run,status:'canceled'},result({status:'canceled'})).detail,'reason not recorded')
 assert.equal(primaryStatus({...run,status:'canceled'},result({status:'canceled',summary:'Superseded by a new launch'})).detail,'Superseded by a new launch')
})

test('row 6: a declared wait needs your answer, only while the process runs',()=>{
 const waiting={...run,waitingSince:'2026-09-25T10:00:00Z'}
 assert.deepEqual(status(waiting,result()),{kind:'waiting',label:'Needs your answer',detail:'Answer in the console',next:''})
 assert.equal(primaryStatus(waiting,null).kind,'waiting')
 // A skill verdict outranks a wait mark left behind.
 assert.equal(primaryStatus(waiting,result({status:'completed'},['clarified'])).kind,'done')
 // A queued or ended run asks nothing, whatever mark it still carries.
 assert.equal(primaryStatus({...waiting,status:'queued'},null).kind,'working')
 assert.equal(primaryStatus({...waiting,status:'failed'},null).kind,'failed')
})

test('row 6: a launch parked for a repository needs your answer',()=>{
 assert.deepEqual(status({...run,status:'waiting'},null),{kind:'waiting',label:'Needs your answer',detail:'Choose a repository for this ticket',next:''})
 assert.equal(primaryStatus({...run,status:'queued'},result({waitingReason:'repository'})).detail,'Choose a repository for this ticket')
})

test('row 7: a live process is Working on its skill',()=>{
 assert.deepEqual(status(run,result()),{kind:'working',label:'Working · Clarify',detail:'',next:''})
 assert.equal(primaryStatus({...run,status:'queued'},null).detail,'Queued')
 assert.equal(primaryStatus({...run,status:'preparing'},null).detail,'Preparing')
 assert.equal(primaryStatus(run,result({summary:'Execution reported by a local agent or native client'+DASH+'No MCP call for 12 minutes'})).detail,'No output for a while')
 assert.equal(primaryStatus({...run,skill:'pickup'},null).label,'Working · Pickup')
})

test('row 8: an exit without a report needs verification, never Done',()=>{
 assert.deepEqual(status({...run,status:'completed'},null),{kind:'verify',label:'Needs verification',detail:'The session ended without reporting a result',next:''})
 assert.equal(primaryStatus({...run,status:'completed'},result()).kind,'verify')
})

test('row 9: a failed or canceled process with no skill record says so',()=>{
 assert.deepEqual(status({...run,status:'failed'},null),{kind:'failed',label:'Could not finish',detail:'reason not recorded',next:'Relaunch Clarify'})
 assert.deepEqual(status({...run,status:'canceled'},null),{kind:'stopped',label:'Stopped',detail:'reason not recorded',next:''})
 assert.equal(primaryStatus({...run,status:'canceled',cancelRequested:true},null).detail,'Stopped by you')
})

test('a skill record of another execution is ignored',()=>{
 for(const other of [{id:'old'},{taskId:'elsewhere'},{skillId:'specify'}]){
  assert.equal(primaryStatus(run,result({status:'completed',...other},['clarified'])).kind,'working')
 }
})

test('a pickup is done only once it moved the task forward',()=>{
 const pickup={...run,skill:'pickup'}
 const completed=labels=>result({skillId:'pickup',status:'completed'},labels)
 assert.equal(primaryStatus(pickup,completed(['implemented']),{launchStage:'new'}).label,'Done · Now implemented')
 assert.equal(primaryStatus(pickup,completed(['new']),{launchStage:'new'}).detail,'Pickup reported completion; the task is at new')
 // Without the stage it started from, nothing tells it moved anything.
 assert.equal(primaryStatus(pickup,completed(['implemented'])).kind,'verify')
 assert.equal(primaryStatus({...run,skill:'pickup_issues'},result({skillId:'pickup_issues',status:'completed'},['reviewed']),{launchStage:'clarified'}).label,'Done · Now reviewed')
})

test('a successor skill in the same console owns the status',()=>{
 const own=record({status:'completed'})
 const after=(successor,labels=['clarified'])=>({activity:own,successor:{id:'next',taskId:'task',skillId:'specify-issue',summary:'',error:'',...successor},task:{labels}})
 assert.equal(primaryStatus(run,after({status:'running'})).label,'Working · Specify')
 assert.equal(primaryStatus(run,after({status:'running',waitingSince:'2026-10-06T09:00:00Z'})).kind,'waiting')
 assert.equal(primaryStatus(run,after({status:'completed'})).kind,'verify')
 assert.equal(primaryStatus(run,after({status:'completed'},['specified'])).label,'Done · Specification ready')
 assert.equal(primaryStatus(run,after({status:'failed',error:'boom'})).detail,'boom')
 assert.equal(primaryStatus(run,after({status:'canceled'})).kind,'stopped')
 assert.equal(primaryStatus({...run,cancelRequested:true},after({status:'running'})).kind,'stopping')
 // The console exited while the successor still runs: nothing was reported.
 assert.equal(primaryStatus({...run,status:'completed'},after({status:'running'})).detail,'The session ended without reporting a result')
 assert.equal(primaryStatus({...run,status:'completed'},after({status:'running',waitingSince:'2026-10-06T09:00:00Z'})).kind,'verify')
 // The launched skill's outcome stays in the diagnostics.
 assert.equal(primaryStatus(run,after({status:'running'})).diagnostics,'Execution: Running · Skill record: clarify completed; now specify-issue running · Task stage: clarified')
 // A successor only counts beside the console's own record.
 assert.equal(primaryStatus(run,{...after({status:'completed'},['specified']),activity:{...own,id:'other'}}).label,'Working · Clarify')
})

test('a free console and a discussion have no skill status',()=>{
 assert.equal(primaryStatus({...run,kind:'console'},null),null)
 assert.equal(primaryStatus({...run,kind:'console',status:'canceled'},null),null)
 assert.equal(primaryStatus({...run,kind:'console',cancelRequested:true},null).label,'Stopping')
 assert.equal(primaryStatus({...run,kind:'console',cancelRequested:true},null).diagnostics,'Execution: Running')
 const discussion={...run,skill:'discuss'}
 assert.equal(primaryStatus(discussion,result({skillId:'discuss',status:'completed'})),null)
 assert.equal(primaryStatus({...discussion,cancelRequested:true},null).kind,'stopping')
 assert.equal(primaryStatus(null,result()),null)
})

test('diagnostics keep every signal the status was drawn from',()=>{
 assert.equal(primaryStatus({...run,status:'completed'},null).diagnostics,'Execution: Finished · Skill record: none · Task stage: unknown')
 assert.equal(primaryStatus(run,result({status:'failed'},['specified'])).diagnostics,'Execution: Running · Skill record: failed · Task stage: specified')
})

test('reason and skill helpers',()=>{
 assert.equal(recordedReason(null),null)
 assert.equal(recordedReason({status:'failed'}),null)
 assert.equal(recordedReason({summary:'',error:''}),'')
 assert.equal(recordedReason({summary:'note',error:' error '}),'error')
 assert.equal(workflowSkill('specify-issue'),'specify')
 assert.equal(workflowSkill('sectile:implement-issue'),'implement')
 assert.equal(workflowSkill('adjust'),'adjust')
 assert.equal(workflowSkill('pickup-issue'),'')
 assert.equal(workflowSkill(undefined),'')
})
