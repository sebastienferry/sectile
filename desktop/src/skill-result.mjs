import {taskStage} from './workflow.mjs'
const stages=['new','clarified','specified','implemented','reviewed','finished']
const expected={clarify:'clarified',specify:'specified',implement:'implemented',adjust:'reviewed',review:'reviewed',handoff:'finished'}
const ended=run=>['completed','failed','canceled'].includes(run.status)
// The workflow skill a recorded skill name stands for: a skill started by hand
// in a console reports the name it was invoked under (`specify-issue`,
// `sectile:specify-issue`), a launched one its workflow id. '' when the name is
// no workflow skill.
export function workflowSkill(name){
 const skill=String(name||'').replace(/^[^:]*:/,'').replace(/-issue$/,'')
 return Object.hasOwn(expected,skill)?skill:''
}
// The verdict of an ended skill, its stage check included; null before it ends.
function verdict(state,skill,task){
 if(state==='completed'){
  const target=expected[skill]
  if(target&&stages.indexOf(taskStage(task||{}))<stages.indexOf(target))return {kind:'pending',icon:'◷',label:'Awaiting stage validation'}
  return {kind:'completed',icon:'✓',label:'Skill completed'}
 }
 if(state==='failed'||state==='canceled')return {kind:state,icon:state==='failed'?'!':'⊘',label:state==='failed'?'Skill failed':'Skill canceled'}
 return null
}
// A stop that has been asked for and has not taken effect yet is the one process
// transient the shared run state has no word for, so it is the one this badge
// still reports about the process itself.
const stopping=(run,label)=>run.cancelRequested&&!ended(run)?{kind:'pending',icon:'◷',label}:null

// What the server knows about the launched skill - a different question from
// whether the process is still alive, which the run state shown beside this
// badge already answers. Anything this could only restate in other words -
// queued, preparing, running, failed, canceled - is reported as no result at
// all rather than as a second glyph saying the same thing. A declared wait is
// the exception: the skill, not the process, is asking its user something, and
// the badge beside the task is where that user looks for what the skill needs.
export function skillResult(run,result){
 if(!run)return null
 // A free console and a discussion run no skill, so they have no skill result
 // to report.
 if(run.kind==='console')return stopping(run,'Stopping console')
 if(run.skill==='discuss')return stopping(run,'Stopping discussion')
 const activity=result?.activity
 const matched=activity?.id===run.id&&activity.taskId===run.taskId&&activity.skillId===run.skill
 const state=matched?activity.status:null
 // A console left open after its skill ended may have gone on to run another
 // one (#586): the badge then speaks for that skill, never for the one before.
 const successor=matched&&result.successor
 if(successor){
  const next=verdict(successor.status,workflowSkill(successor.skillId),result.task)
  if(next)return next
  const pending=stopping(run,'Stopping execution')
  if(pending)return pending
  if(run.status==='running'&&successor.waitingSince)return {kind:'waiting',icon:'?',label:'Waiting for your answer'}
  return null
 }
 const own=verdict(state,run.skill,result?.task)
 if(own)return own
 const pending=stopping(run,'Stopping execution')
 if(pending)return pending
 // Only a live process can be asking: a mark left on a queued run is stale.
 if(run.status==='running'&&run.waitingSince)return {kind:'waiting',icon:'?',label:'Waiting for your answer'}
 // An execution that ended well without the server recording its skill is the
 // gap worth naming: the run state says it finished, and nothing else would say
 // the work was never registered.
 if(run.status==='completed')return {kind:'pending',icon:'◷',label:'Execution ended · skill completion unconfirmed'}
 return null
}
