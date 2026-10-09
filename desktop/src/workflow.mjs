import { explicitStage, stageFromColumn } from '../../shared/workflowStage.mjs'

// The workflow stages in their order, shared by every list that sorts on them.
export const STAGES=['new','clarified','specified','implemented','reviewed','finished']
const skills={new:['clarify','Clarify'],clarified:['specify','Specify'],specified:['implement','Implement'],implemented:implementedStep,reviewed:['handoff','Handoff']}

// An implemented task is adjusted once it records a pull request. Without one, the
// pull request is recovered through the specification when the project opens it
// there, and through the implementation otherwise, a project that opens it at
// clarification included: re-running a clarification to publish a branch makes no
// sense. The stage-neutral create_pr skill never records the link, so it would
// leave the task at implemented for good.
function implementedStep(task,project){
 if(typeof task.prUrl==='string'&&task.prUrl.trim())return ['adjust','Adjust']
 return [project?.server?.prCreationStage==='specified'?'specify':'implement','Create PR']
}

// A launched skill reads as the workflow names it; any other id, such as a
// pickup or a discussion, is shown with its first letter capitalized.
const skillLabels={clarify:'Clarify',specify:'Specify',implement:'Implement',adjust:'Adjust',handoff:'Handoff',create_pr:'Create PR'}
export function skillLabel(skillId){
 const id=String(skillId||'').trim()
 return skillLabels[id]||(id?id[0].toUpperCase()+id.slice(1):'')
}

// The stage of a task: a finished status, then an explicit workflow label, then
// the stage the project maps the task's tracker column to when `board` carries
// the mapping (/desktop/project's board field), then the status. With a board it
// answers as the web board does, so the board, the tickets list and the
// sidebar's grouping agree (#806).
export function taskStage(task,board){
 if(['finished','done'].includes(task.status))return 'finished'
 const labelled=explicitStage({...task,labels:(task.labels||[]).map(label=>String(label).trim())})
 if(labelled)return labelled
 const mapped=board&&stageFromColumn(task,board)
 if(mapped)return mapped
 return ({to_clarify:'new',backlog:'new',untouched:'new',to_specify:'clarified',to_implement:'specified',in_progress:'specified',to_test:'implemented',to_validate:'implemented',to_close:'reviewed'})[task.status]||task.status||'Unknown'
}

export function nextTaskStep(task,project){
 const stage=taskStage(task)
 if(stage==='finished')return {stage,message:'Task finished'}
 const next=skills[stage]
 if(!next)return {stage,message:'No next step for this workflow state'}
 const [skillId,label]=typeof next==='function'?next(task,project):next
 if(!project?.configured)return {stage,message:'Configure the local project to continue'}
 if(!project.server?.skills?.some(skill=>skill.id===skillId))return {stage,message:'Next skill is unavailable: '+label}
 return {stage,skillId,label,message:'Ready for the next step'}
}
