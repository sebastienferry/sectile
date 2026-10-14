export const isValidBatchWorktreeName=value=>/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,79}$/.test(value.trim())
export const suggestedBatchWorktreeName=keys=>('batch-'+keys.map(key=>key.replace(/[^a-zA-Z0-9_-]/g,'')).filter(Boolean).join('-')).slice(0,80).replace(/[-_]+$/,'')
export function batchMembers(ids,tasks,projectId){
 if(ids.length<2||new Set(ids).size!==ids.length)throw Error('Select at least two distinct tickets.')
 return ids.map(id=>{
  const task=tasks.find(task=>task.id===id)
  if(!task)throw Error('A selected ticket is no longer available. Clear the selection and select the batch again.')
  const projects=[task.projectId,...(task.projectIds||[])]
  if(!projects.includes(projectId))throw Error('Every selected ticket must belong to this project.')
  return task
 })
}
export function moveBatchMember(ids,index,offset){
 const result=[...ids],target=index+offset
 if(index>=0&&index<ids.length&&target>=0&&target<ids.length)[result[index],result[target]]=[result[target],result[index]]
 return result
}
export function buildBatchPickupPrompt(ids,worktreeName){
 if(ids.length<2||new Set(ids).size!==ids.length||ids.some(id=>!id||/\s/.test(id)))throw Error('Select at least two distinct full task IDs.')
 const name=worktreeName.trim()
 if(!isValidBatchWorktreeName(name))throw Error('Use 1–80 letters, digits, hyphens or underscores, starting with a letter or digit.')
 return `/pickup-issues ${ids.join(' ')}\n\nUse one dedicated Git worktree named "${name}" at .tasks/worktrees/${name} for this entire batch. `+
  'This is the workspace explicitly chosen by the user for the batch. Preserve any existing work when preparing it; do not overwrite an unrelated worktree. '+
  'Process the tickets sequentially in exactly the order listed above and produce one combined pull request.'
}
