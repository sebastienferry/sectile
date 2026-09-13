const stages=['new','clarified','specified','implemented','reviewed','finished']
const skills={new:['clarify','Clarify'],clarified:['specify','Specify'],specified:['implement','Implement'],implemented:['create_pr','Review and create PR']}

export function taskStage(task){
 const labels=(task.labels||[]).map(label=>label.trim().replace(/^#+/,'').toLowerCase())
 if(['finished','done'].includes(task.status)||labels.some(label=>['finished','closed','done'].includes(label)))return 'finished'
 for(const stage of [...stages].reverse())if(labels.includes(stage))return stage
 return ({to_clarify:'new',backlog:'new',untouched:'new',to_specify:'clarified',to_implement:'specified',in_progress:'specified',to_test:'implemented',to_validate:'implemented',to_close:'reviewed'})[task.status]||task.status||'Unknown'
}

export function nextTaskStep(task,project){
 const stage=taskStage(task)
 if(stage==='finished')return {stage,message:'Task finished'}
 if(stage==='reviewed')return {stage,message:'Awaiting human merge'}
 const next=skills[stage]
 if(!next)return {stage,message:'No next step for this workflow state'}
 const [skillId,label]=next
 if(!project?.configured)return {stage,message:'Configure the local project to continue'}
 if(!project.server?.skills?.some(skill=>skill.id===skillId))return {stage,message:'Next skill is unavailable: '+label}
 return {stage,skillId,label,message:'Ready for the next step'}
}
