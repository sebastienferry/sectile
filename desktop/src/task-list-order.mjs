import { taskStage } from './workflow.mjs'

// The tickets pane sorts on the workstation: the server orders tasks by board
// position, which is the web board's manual order, not the reading order a
// list of tickets needs. The web backlog uses these same rules, so both
// surfaces agree on which ticket comes first.
export const PRIORITY_RANK={urgent:4,high:3,medium:2,low:1}
const STAGE_ORDER=['new','clarified','specified','implemented','reviewed','finished']
export const DEFAULT_SORT={field:'priority',ascending:false}
export const SORTABLE_FIELDS=['key','title','stage','priority']

const text=value=>String(value??'')
// Natural numeric comparison keeps #9 before #100 and PROJ-9 before PROJ-10,
// whatever tracker minted the key.
const natural=(a,b)=>text(a).localeCompare(text(b),undefined,{numeric:true,sensitivity:'base'})

// Identity is the last word in every ordering, so the row order is
// deterministic whatever order the server returned the tasks in.
export function compareIdentity(a,b){
 return natural(a.key||a.id,b.key||b.id)||natural(a.id,b.id)
}

const priorityRank=task=>PRIORITY_RANK[text(task.priority).toLowerCase()]||0
function stageRank(task){
 const index=STAGE_ORDER.indexOf(taskStage(task))
 return index===-1?STAGE_ORDER.length:index
}

export function compareBy(field){
 switch(field){
  case 'key':return compareIdentity
  case 'title':return (a,b)=>text(a.title).localeCompare(text(b.title),undefined,{sensitivity:'base'})
  case 'stage':return (a,b)=>stageRank(a)-stageRank(b)
  default:return (a,b)=>priorityRank(a)-priorityRank(b)
 }
}

// The direction applies to the chosen column only: a descending sort still
// lists equal rows by ascending identity, so flipping a column never reverses
// the tie-break the user did not ask about.
export function orderedTasks(tasks,{field=DEFAULT_SORT.field,ascending=DEFAULT_SORT.ascending}={}){
 const compare=compareBy(field)
 return [...tasks].sort((a,b)=>{
  const result=compare(a,b)
  if(result)return ascending?result:-result
  return compareIdentity(a,b)
 })
}

// A first click on a column reads it the natural way round, which for
// priority is highest first; a second click on the same column flips it.
export function nextSort(current,field){
 if(current?.field===field)return {field,ascending:!current.ascending}
 return {field,ascending:field!=='priority'}
}
