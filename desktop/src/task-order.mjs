import { STAGES } from './workflow.mjs'

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
// With stageOf, a project grouped by stage lists its tasks in workflow order,
// the ones without a known stage last; the usual order breaks every tie.
export function orderedTaskGroups(groups,{stageOf}={}){
 const stageRank=group=>{
  const index=STAGES.indexOf(stageOf(group.run))
  return index===-1?STAGES.length:index
 }
 const compare=stageOf?(a,b)=>stageRank(a)-stageRank(b)||compareGroups(a,b):compareGroups
 return [...groups].map(executions=>{
  const run=[...executions].sort(compareRuns)[0]
  return {executions,run,skillRun:newestSkillRun(executions)||run,anchor:anchorTime(executions)}
 }).sort(compare)
}

// The execution a row's skill badge speaks for (#586): the newest one that runs
// a skill, so that a console left open after its skill ended does not keep its
// verdict on the row once another skill is launched on the task. The row is
// still led, ordered and selected by run.
function newestSkillRun(executions){
 return executions.filter(run=>run.kind!=='console'&&run.skill!=='discuss').sort((a,b)=>{
  const left=submissionTime(a),right=submissionTime(b)
  if(left!==right)return left>right?-1:1
  return identityCompare(a.id,b.id)
 })[0]
}

// The execution the console moves to after a poll (#639): among the executions
// next lists and previous did not, those of the displayed execution's row,
// ranked like the row ranks them. The displayed execution is looked up in
// previous, so the first poll after a start or a restart follows nothing.
// eligible leaves out what the console never follows.
export function followedExecution(previous,next,selectedId,keyOf,eligible){
 const shown=previous.find(run=>run.id===selectedId)
 if(!shown||!eligible(shown))return null
 const known=new Set(previous.map(run=>run.id)),key=keyOf(shown)
 return next.filter(run=>!known.has(run.id)&&keyOf(run)===key&&eligible(run)).sort(compareRuns)[0]||null
}
