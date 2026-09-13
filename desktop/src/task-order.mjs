const stateRank=run=>['running','preparing'].includes(run.status)?0:run.status==='queued'?1:2
const identityCompare=(a,b)=>a<b?-1:a>b?1:0
function executionTime(run){
 const started=['queued','preparing'].includes(run.status)?NaN:Date.parse(run.startedAt)
 const created=Date.parse(run.createdAt)
 return Number.isFinite(started)?started:Number.isFinite(created)?created:-Infinity
}
function compareRuns(a,b){
 const rank=stateRank(a)-stateRank(b)
 if(rank)return rank
 const left=executionTime(a),right=executionTime(b)
 if(left!==right)return left>right?-1:1
 return identityCompare(a.taskId,b.taskId)||identityCompare(a.id,b.id)
}

// Input groups already contain only visible executions from a single project.
export function orderedTaskGroups(groups){
 return [...groups].map(executions=>({
  executions,
  run:[...executions].sort(compareRuns)[0]
 })).sort((a,b)=>compareRuns(a.run,b.run))
}
