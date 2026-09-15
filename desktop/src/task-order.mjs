const stateRank=run=>['running','preparing'].includes(run.status)?0:run.status==='queued'?1:2
const identityCompare=(a,b)=>a<b?-1:a>b?1:0
// Submission time is the only ordering basis: unlike the actual start time it
// never changes once a run is recorded, so a row cannot move when its
// execution starts.
function submissionTime(run){
 const created=Date.parse(run.createdAt)
 return Number.isFinite(created)?created:-Infinity
}
function compareRuns(a,b){
 const rank=stateRank(a)-stateRank(b)
 if(rank)return rank
 const left=submissionTime(a),right=submissionTime(b)
 if(left!==right)return left>right?-1:1
 return identityCompare(a.taskId,b.taskId)||identityCompare(a.id,b.id)
}
// A group is anchored on its earliest visible submission, so relaunching a
// skill on a listed task adds an execution without repositioning the row.
function anchorTime(executions){
 let anchor=null
 for(const run of executions){
  const time=submissionTime(run)
  if(Number.isFinite(time)&&(anchor===null||time<anchor))anchor=time
 }
 return anchor===null?-Infinity:anchor
}
function compareGroups(a,b){
 const rank=stateRank(a.run)-stateRank(b.run)
 if(rank)return rank
 if(a.anchor!==b.anchor)return a.anchor>b.anchor?-1:1
 return identityCompare(a.run.taskId,b.run.taskId)||identityCompare(a.run.id,b.run.id)
}

// Input groups already contain only visible executions from a single project.
export function orderedTaskGroups(groups){
 return [...groups].map(executions=>({
  executions,
  run:[...executions].sort(compareRuns)[0],
  anchor:anchorTime(executions)
 })).sort(compareGroups)
}
