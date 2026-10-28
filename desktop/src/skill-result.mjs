import {STAGES,taskStage,skillLabel} from './workflow.mjs'
import {RUN_SILENCE_PREFIX,isSilentSummary,runStateLabel,runStateOf} from '../../shared/runStates.ts'

// The stage a workflow skill leaves its task on, and the outcome it names once
// the task got there.
const expected={clarify:'clarified',specify:'specified',implement:'implemented',adjust:'reviewed',review:'reviewed',handoff:'finished'}
const outcomes={clarify:'Clarified',specify:'Specification ready',implement:'Implemented',adjust:'Review complete',review:'Review complete',handoff:'Handed off'}
const pickups=new Set(['pickup','pickup_issues'])
const ended=run=>['completed','failed','canceled'].includes(run.status)
// The summary the server gives a remote run when it starts: bookkeeping, never
// a reason.
const LAUNCH_PLACEHOLDER='Execution reported by a local agent or native client'
// The separator the server joins the sentences of a summary with.
const SUMMARY_SEPARATOR=' '+String.fromCharCode(0x2014)+' '
export const REASON_LIMIT=120

// The skill id a recorded skill name stands for: a skill started by hand in a
// console reports the name it was invoked under (`specify-issue`,
// `sectile:pickup-issues`), a launched one its id.
function skillId(name){
 return String(name||'').trim().replace(/^[^:]*:/,'').replace(/-issues$/,'_issues').replace(/-issue$/,'').replace(/-/g,'_')
}

// The workflow skill a recorded skill name stands for. '' when the name is no
// workflow skill.
export function workflowSkill(name){
 const skill=skillId(name)
 return Object.hasOwn(expected,skill)?skill:''
}

// What the server recorded as the reason a skill ended: the first line of its
// error, otherwise of its note, without the server's own bookkeeping. '' when
// nothing was recorded, null when the agent is too old to send it.
export function recordedReason(record){
 if(!record||!('summary' in record||'error' in record))return null
 const error=String(record.error||'').trim()
 const note=String(record.summary||'').split(SUMMARY_SEPARATOR)
  .filter(part=>part.trim()&&part.trim()!==LAUNCH_PLACEHOLDER&&!part.includes(RUN_SILENCE_PREFIX))
  .join(SUMMARY_SEPARATOR).trim()
 return error||note
}
const firstLine=text=>text.split(/\r?\n/).find(line=>line.trim())?.trim()||''
const shorten=text=>text.length>REASON_LIMIT?text.slice(0,REASON_LIMIT-1).trimEnd()+'…':text
// The detail and the full text of a reason, saying so when there is none.
function reasonOf(record){
 const reason=recordedReason(record)
 if(reason===null)return {detail:'reason not available',reason:''}
 if(!reason)return {detail:'reason not recorded',reason:''}
 return {detail:shorten(firstLine(reason)),reason}
}

const status=(kind,label,{detail='',next='',reason=''}={})=>({kind,label,detail,next,reason})

// The one status Desktop shows for a task's execution (#878): what the user's
// situation is, with the outcome, the reason or the next step. The process
// state and the skill record are separate signals that can disagree, so they
// are reconciled here, in the order of the precedence table of
// specs/878-clear-skill-status/spec.md, rather than shown side by side. The
// first row that matches wins.
//
// run is the execution, result what the local agent read of its skill on the
// server. nextStep is the label of the task's next workflow step, launchStage
// the stage the task was at when the run started, when known. Returns null for
// no run, and for a free console or a discussion that is not stopping: they
// run no skill, and their caller shows the run state as it is.
export function primaryStatus(run,result,{nextStep='',launchStage=''}={}){
 if(!run)return null
 const free=run.kind==='console'||run.skill==='discuss'
 // Row 1: a stop asked for and not taken effect yet.
 const stopping=run.cancelRequested&&!ended(run)
 // A free run has no skill record nor task stage to diagnose.
 if(free)return stopping?{...status('stopping','Stopping',{detail:'Waiting for the session to exit'}),diagnostics:'Execution: '+runStateLabel(runStateOf(run))}:null
 const activity=result?.activity
 const own=activity?.id===run.id&&activity.taskId===run.taskId&&activity.skillId===run.skill?activity:null
 // A console left open after its skill ended may have gone on to run another
 // one (#586): the status then speaks for that skill, never for the one before.
 const successor=own&&result.successor||null
 const record=successor||own
 const skill=successor?successor.skillId:run.skill
 const name=skillLabel(skillId(skill))||'Skill'
 const stage=result?.task?taskStage(result.task):''
 const shown=stopping?status('stopping','Stopping',{detail:'Waiting for the session to exit'})
  :verdict(record,skill,name,stage,run,{nextStep,launchStage})
  ||processStatus(run,record,successor,name)
 return withDiagnostics(shown,run,own,successor,result)
}

// Rows 2 to 5: what the server recorded for the skill, once it ended.
function verdict(record,skill,name,stage,run,{nextStep,launchStage}){
 const state=record?.status
 if(state==='completed'){
  const id=skillId(skill),target=expected[workflowSkill(skill)]
  const done=outcome=>status('done','Done · '+outcome,{next:stage!=='finished'&&nextStep?'Next: '+nextStep:''})
  if(pickups.has(id)){
   // A pickup has no stage of its own: its outcome is how far it moved the task.
   const from=STAGES.indexOf(launchStage),to=STAGES.indexOf(stage)
   if(from>=0&&to>from)return done('Now '+stage)
   return status('verify','Needs verification',{detail:'Pickup reported completion; the task is at '+(stage||'an unknown stage')})
  }
  // Row 2: a skill with no stage is done on its own word.
  if(!target)return done('reported by the skill')
  if(STAGES.indexOf(stage)>=STAGES.indexOf(target))return done(outcomes[workflowSkill(skill)])
  // Row 3: reported, not validated.
  return status('verify','Needs verification',{detail:name+' reported completion; the task is still at '+(stage||'an unknown stage')})
 }
 // Row 4.
 if(state==='failed'){
  const {detail,reason}=reasonOf(record)
  return status('failed','Could not finish',{detail,reason,next:ended(run)?'Relaunch '+name:'Open the console'})
 }
 // Row 5: only a stop on record names who or what stopped it.
 if(state==='canceled')return stopped(run,record)
 return null
}
function stopped(run,record){
 if(run.cancelRequested)return status('stopped','Stopped',{detail:'Stopped by you'})
 if(record?.disconnected)return status('stopped','Stopped',{detail:'Session disconnected'})
 if(!record)return status('stopped','Stopped',{detail:'reason not recorded'})
 const {detail,reason}=reasonOf(record)
 return status('stopped','Stopped',{detail,reason})
}

// Rows 6 to 9: the skill has not ended on record, so the process says the rest.
function processStatus(run,record,successor,name){
 // Row 6: only a live process can be asking; a mark left on a queued or ended
 // run is stale.
 const waitingSince=successor?successor.waitingSince:run.waitingSince
 if(run.status==='running'&&waitingSince)return status('waiting','Needs your answer',{detail:'Answer in the console'})
 // A launch the local agent parked until its ticket is pinned to a repository.
 if(run.status==='waiting'||record?.waitingReason==='repository')return status('waiting','Needs your answer',{detail:'Choose a repository for this ticket'})
 // Row 7.
 if(['queued','pending','preparing','running'].includes(run.status)){
  const detail=run.status==='preparing'?'Preparing':run.status!=='running'?'Queued'
   :isSilentSummary(run.summary)||isSilentSummary(record?.summary)?'No output for a while':''
  return status('working','Working · '+name,{detail})
 }
 // Row 8: a clean exit is no result.
 if(run.status==='completed')return status('verify','Needs verification',{detail:'The session ended without reporting a result'})
 // Row 9.
 if(run.status==='canceled')return stopped(run,null)
 if(run.status==='failed')return status('failed','Could not finish',{detail:'reason not recorded',next:'Relaunch '+name})
 return status('working','Working · '+name)
}

// The signals the status was reconciled from, verbatim, so nothing that could
// be inspected before is lost.
function withDiagnostics(shown,run,own,successor,result){
 const records=successor?(own.skillId+' '+own.status+'; now '+successor.skillId+' '+successor.status):own?own.status:'none'
 const stage=result?.task?taskStage(result.task):'unknown'
 return {...shown,diagnostics:'Execution: '+runStateLabel(runStateOf(run))+' · Skill record: '+records+' · Task stage: '+stage}
}
