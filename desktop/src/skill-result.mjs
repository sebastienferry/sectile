import {taskStage} from './workflow.mjs'
const stages=['new','clarified','specified','implemented','reviewed','finished']
const expected={clarify:'clarified',specify:'specified',implement:'implemented',adjust:'reviewed',review:'reviewed',handoff:'finished'}
export function skillResult(run,result){
 if(!run)return null
 const activity=result?.activity
 const matched=activity?.id===run.id&&activity.taskId===run.taskId&&activity.skillId===run.skill
 const state=matched?activity.status:null
 if(state==='completed'){
  const target=expected[run.skill]
  if(target&&stages.indexOf(taskStage(result.task||{}))<stages.indexOf(target))return {kind:'pending',icon:'◷',label:'Awaiting stage validation'}
  return {kind:'completed',icon:'✓',label:'Skill completed'}
 }
 if(state==='failed'||state==='canceled')return {kind:state,icon:state==='failed'?'!':'⊘',label:state==='failed'?'Skill failed':'Skill canceled'}
 if(run.cancelRequested)return {kind:'pending',icon:'◷',label:'Stopping execution'}
 if(run.status==='failed'||run.status==='canceled')return {kind:run.status,icon:run.status==='failed'?'!':'⊘',label:run.status==='failed'?'Execution failed':'Execution canceled'}
 if(run.status==='completed')return {kind:'pending',icon:'◷',label:'Execution ended · skill completion unconfirmed'}
 return {kind:'pending',icon:'◷',label:run.status==='queued'?'Queued':run.status==='preparing'?'Preparing':'In progress'}
}
