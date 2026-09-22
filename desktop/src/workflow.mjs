const stages=['new','clarified','specified','implemented','reviewed','finished']
const skills={new:['clarify','Clarify'],clarified:['specify','Specify'],specified:['implement','Implement'],implemented:implementedStep,reviewed:['handoff','Handoff']}

// An implemented task is adjusted once it records a pull request. Without one, the
// pull request is recovered through the owner the project configured for its
// creation: the stage-neutral create_pr skill never records the link, so it would
// leave the task at implemented for good.
function implementedStep(task,project){
 if(typeof task.prUrl==='string'&&task.prUrl.trim())return ['adjust','Adjust']
 return [project?.server?.prCreationStage==='specified'?'specify':'implement','Create PR']
}

export function taskStage(task){
 const labels=(task.labels||[]).map(label=>label.trim().replace(/^#+/,'').toLowerCase())
 if(['finished','done'].includes(task.status)||labels.some(label=>['finished','closed','done'].includes(label)))return 'finished'
 for(const stage of [...stages].reverse())if(labels.includes(stage))return stage
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
