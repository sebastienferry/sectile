// The daemon sequence is authoritative; older agents expose only creation time.
export function orderedQueueRuns(runs){
 return runs.filter(run=>run.status==='queued'&&!run.cancelRequested).sort((a,b)=>{
  if(a.queueSequence>0&&b.queueSequence>0)return a.queueSequence-b.queueSequence
  const left=Date.parse(a.createdAt),right=Date.parse(b.createdAt)
  if(Number.isFinite(left)&&Number.isFinite(right)&&left!==right)return left-right
  return a.id<b.id?-1:a.id>b.id?1:0
 })
}
