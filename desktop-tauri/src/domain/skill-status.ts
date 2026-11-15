// The one status Sectile Desktop shows for a ticket skill (#908).
//
// A port of the Electron client's `desktop/src/skill-result.mjs` (#878), which
// follows the precedence table of `specs/878-clear-skill-status/spec.md`.
// desktop-tauri imports nothing from `desktop/` or `shared/`, so the two clients
// carry the same rules side by side: change them together. Labels, details and
// reasons are the Electron wording, word for word.
import type {Task} from "./board.ts";
import {hasWaitMark,type Run} from "./execution.ts";
import {skillLabels,stages,taskStage,type RunResult,type SkillRecord} from "./workflow.ts";

export type SkillStatusKind="working"|"waiting"|"done"|"verify"|"failed"|"stopped"|"stopping";
export interface SkillStatus {
 kind:SkillStatusKind;
 label:string;
 detail:string;
 next:string;
 /** The full recorded reason, when the detail shows a shortened one. */
 reason:string;
 /** The signals the status was reconciled from, verbatim. */
 diagnostics:string;
}
export interface SkillStatusOptions {
 /** Label of the task's next workflow step, when it can be launched. */
 nextStep?:string;
 /** The stage the task was at when a pickup started, '' when unknown. */
 launchStage?:string;
 /** The conversation shows a question or an approval waiting for the user. */
 asking?:boolean;
}
type Successor=NonNullable<RunResult["successor"]>;

// The stage a workflow skill leaves its task on, and the outcome it names once
// the task got there.
const expected:Record<string,string>={clarify:"clarified",specify:"specified",implement:"implemented",adjust:"reviewed",review:"reviewed",handoff:"finished"};
const outcomes:Record<string,string>={clarify:"Clarified",specify:"Specification ready",implement:"Implemented",adjust:"Review complete",review:"Review complete",handoff:"Handed off"};
const pickups=new Set(["pickup","pickup_issues"]);
const ENDED=new Set(["completed","failed","canceled"]);
const ended=(run:Run)=>ENDED.has(run.status);
// The summary the server gives a remote run when it starts: bookkeeping, never
// a reason.
const LAUNCH_PLACEHOLDER="Execution reported by a local agent or native client";
// The separator the server joins the sentences of a summary with.
const SUMMARY_SEPARATOR=" "+String.fromCharCode(0x2014)+" ";
export const REASON_LIMIT=120;
// The sentence the server appends to a run whose MCP session stopped speaking
// (`shared/runStates.ts`, pinned to `models.RunSilencePrefix` by a Go test).
const RUN_SILENCE_PREFIX="No MCP call for ";
const isSilentSummary=(summary?:string|null)=>!!summary&&summary.includes(RUN_SILENCE_PREFIX);

// The run state of `shared/runStates.ts`, for the diagnostics line. A Tauri run
// carries no summary, so it never reads as silent.
const runStateLabels:Record<string,string>={waiting:"Waiting for you",silent:"Silent",running:"Running",queued:"Queued",completed:"Finished",failed:"Failed",canceled:"Stopped"};
function runStateOf(run:Run){
 if(ENDED.has(run.status))return run.status;
 if(hasWaitMark(run.waitingSince)||run.status==="waiting")return "waiting";
 if(["queued","preparing","pending"].includes(run.status))return "queued";
 return "running";
}
const runStateLabel=(run:Run)=>{const id=runStateOf(run);return runStateLabels[id]??id;};

// The skill id a recorded skill name stands for: a skill started by hand in a
// console reports the name it was invoked under (`specify-issue`,
// `sectile:pickup-issues`), a launched one its id.
export function skillId(name?:string){
 return String(name||"").trim().replace(/^[^:]*:/,"").replace(/-issues$/,"_issues").replace(/-issue$/,"").replace(/-/g,"_");
}
// The workflow skill a recorded skill name stands for, '' when it is none.
export function workflowSkill(name?:string){
 const skill=skillId(name);
 return Object.prototype.hasOwnProperty.call(expected,skill)?skill:"";
}
// The stage a workflow skill is expected to bring its task to; '' for any other.
export const expectedStage=(skill:string)=>Object.prototype.hasOwnProperty.call(expected,skill)?expected[skill]:"";

// What the server recorded as the reason a skill ended: the first line of its
// error, otherwise of its note, without the server's own bookkeeping. '' when
// nothing was recorded, null when the agent is too old to send it.
export function recordedReason(record?:SkillRecord|null):string|null{
 if(!record||!("summary" in record||"error" in record))return null;
 const error=String(record.error||"").trim();
 const note=String(record.summary||"").split(SUMMARY_SEPARATOR)
  .filter(part=>part.trim()&&part.trim()!==LAUNCH_PLACEHOLDER&&!part.includes(RUN_SILENCE_PREFIX))
  .join(SUMMARY_SEPARATOR).trim();
 return error||note;
}
const firstLine=(text:string)=>text.split(/\r?\n/).find(line=>line.trim())?.trim()||"";
const shorten=(text:string)=>text.length>REASON_LIMIT?text.slice(0,REASON_LIMIT-1).trimEnd()+"…":text;
// The detail and the full text of a reason, saying so when there is none.
function reasonOf(record?:SkillRecord|null){
 const reason=recordedReason(record);
 if(reason===null)return {detail:"reason not available",reason:""};
 if(!reason)return {detail:"reason not recorded",reason:""};
 return {detail:shorten(firstLine(reason)),reason};
}

type Shown=Omit<SkillStatus,"diagnostics">;
const status=(kind:SkillStatusKind,label:string,{detail="",next="",reason=""}:{detail?:string;next?:string;reason?:string}={}):Shown=>({kind,label,detail,next,reason});
const stopping=()=>status("stopping","Stopping",{detail:"Waiting for the session to exit"});
const stageOf=(run:Run,result?:RunResult)=>result?.task?taskStage({id:run.taskId,title:"",...result.task} as Task)??"":"";

/** A conversation that runs a ticket skill: it has a ticket and is no discussion. */
export function isTicketSkillRun(run:Run){
 return !!run.taskId&&run.skill!=="discuss";
}

// The one status Desktop shows for a ticket skill: what the user's situation
// is, with the outcome, the reason or the next step. The process state and the
// skill record are separate signals that can disagree, so they are reconciled
// here, in the order of the precedence table of
// specs/878-clear-skill-status/spec.md, rather than shown side by side. The
// first row that matches wins.
//
// Returns null for no run, and for a free conversation, a discussion or a free
// console that is not stopping: they run no skill, and their caller shows the
// run state as it is.
export function skillStatus(run:Run|undefined,result:RunResult|undefined,{nextStep="",launchStage="",asking=false}:SkillStatusOptions={}):SkillStatus|null{
 if(!run)return null;
 // Row 1: a stop asked for and not taken effect yet.
 const stopRequested=!!run.cancelRequested&&!ended(run);
 // A free run has no skill record nor task stage to diagnose.
 if(!isTicketSkillRun(run))return stopRequested?{...stopping(),diagnostics:"Execution: "+runStateLabel(run)}:null;
 const activity=result?.activity;
 const own=activity&&activity.id===run.id&&activity.taskId===run.taskId&&activity.skillId===run.skill?activity:null;
 // A console left open after its skill ended may have gone on to run another
 // one (#586): the status then speaks for that skill, never for the one before.
 const successor=own&&result?.successor||null;
 const record=successor||own;
 const skill=successor?successor.skillId:run.skill;
 const name=skillLabels[skillId(skill)]||skillId(skill)||"Skill";
 const stage=stageOf(run,result);
 const shown=stopRequested?stopping()
  :verdict(record,skill,name,stage,run,nextStep,launchStage)
  ||processStatus(run,record,successor,name,asking);
 return withDiagnostics(shown,run,own,successor,result);
}

// Rows 2 to 5: what the server recorded for the skill, once it ended.
function verdict(record:SkillRecord|null,skill:string,name:string,stage:string,run:Run,nextStep:string,launchStage:string):Shown|null{
 const state=record?.status;
 if(state==="completed"){
  const id=skillId(skill),target=expected[workflowSkill(skill)];
  const done=(outcome:string)=>status("done","Done · "+outcome,{next:stage!=="finished"&&nextStep?"Next: "+nextStep:""});
  if(pickups.has(id)){
   // A pickup has no stage of its own: its outcome is how far it moved the task.
   const from=stages.indexOf(launchStage as never),to=stages.indexOf(stage as never);
   if(from>=0&&to>from)return done("Now "+stage);
   return status("verify","Needs verification",{detail:"Pickup reported completion; the task is at "+(stage||"an unknown stage")});
  }
  // Row 2: a skill with no stage is done on its own word.
  if(!target)return done("reported by the skill");
  if(stages.indexOf(stage as never)>=stages.indexOf(target as never))return done(outcomes[workflowSkill(skill)]);
  // Row 3: reported, not validated.
  return status("verify","Needs verification",{detail:name+" reported completion; the task is still at "+(stage||"an unknown stage")});
 }
 // Row 4.
 if(state==="failed"){
  const {detail,reason}=reasonOf(record);
  return status("failed","Could not finish",{detail,reason,next:ended(run)?"Relaunch "+name:"Open the console"});
 }
 // Row 5: only a stop on record names who or what stopped it.
 if(state==="canceled")return stopped(run,record);
 return null;
}
function stopped(run:Run,record:SkillRecord|null):Shown{
 if(run.cancelRequested)return status("stopped","Stopped",{detail:"Stopped by you"});
 if(record?.disconnected)return status("stopped","Stopped",{detail:"Session disconnected"});
 if(!record)return status("stopped","Stopped",{detail:"reason not recorded"});
 const {detail,reason}=reasonOf(record);
 return status("stopped","Stopped",{detail,reason});
}

// Rows 6 to 9: the skill has not ended on record, so the process says the rest.
function processStatus(run:Run,record:SkillRecord|null,successor:Successor|null,name:string,asking:boolean):Shown{
 // Row 6: only a live process can be asking; a mark left on a queued or ended
 // run is stale. A question pending in the conversation is a wait too.
 const waitingSince=successor?successor.waitingSince:run.waitingSince;
 if(run.status==="running"&&(hasWaitMark(waitingSince)||asking))return status("waiting","Needs your answer",{detail:"Answer in the console"});
 // A launch the local agent parked until its ticket is pinned to a repository.
 if(run.status==="waiting"||record?.waitingReason==="repository")return status("waiting","Needs your answer",{detail:"Choose a repository for this ticket"});
 // Row 7.
 if(["queued","pending","preparing","running"].includes(run.status)){
  const detail=run.status==="preparing"?"Preparing":run.status!=="running"?"Queued"
   :isSilentSummary(record?.summary)?"No output for a while":"";
  return status("working","Working · "+name,{detail});
 }
 // Row 8: a clean exit is no result.
 if(run.status==="completed")return status("verify","Needs verification",{detail:"The session ended without reporting a result"});
 // Row 9.
 if(run.status==="canceled")return stopped(run,null);
 if(run.status==="failed")return status("failed","Could not finish",{detail:"reason not recorded",next:"Relaunch "+name});
 return status("working","Working · "+name);
}

// The signals the status was reconciled from, verbatim, so nothing that could
// be inspected before is lost.
function withDiagnostics(shown:Shown,run:Run,own:SkillRecord|null,successor:Successor|null,result?:RunResult):SkillStatus{
 const records=successor&&own?own.skillId+" "+own.status+"; now "+successor.skillId+" "+successor.status:own?own.status:"none";
 const stage=stageOf(run,result)||"unknown";
 return {...shown,diagnostics:"Execution: "+runStateLabel(run)+" · Skill record: "+records+" · Task stage: "+stage};
}

/** The one-line form: the label, then the detail or the next step. */
export function skillStatusText(value:SkillStatus){
 return [value.label,value.detail||value.next].filter(Boolean).join(" · ");
}
/** The tooltip: label, detail, next step and diagnostics, one per line. */
export function skillStatusTitle(value:SkillStatus){
 return [value.label,value.detail,value.next,value.diagnostics].filter(Boolean).join("\n");
}

// The stage a pickup's task was at when the run started. A pickup has no stage
// of its own, so its outcome is how far it moved the task; the start stage is
// kept in localStorage so a restart of Desktop does not lose it.
const LAUNCH_STAGES_KEY="sectile:launchStages";
type StageStorage=Pick<Storage,"getItem"|"setItem">;
function defaultStorage():StageStorage|undefined{
 try{return globalThis.localStorage;}catch{return undefined;}
}
function readLaunchStages(storage?:StageStorage):Record<string,string>{
 try{
  const value=JSON.parse(storage?.getItem(LAUNCH_STAGES_KEY)||"{}");
  return value&&typeof value==="object"&&!Array.isArray(value)?value:{};
 }catch{return {};}
}
/**
 * Records the stage a pickup's task is at, the first time its result is read
 * while it still runs. Entries of runs no longer in `runIds` are pruned when it
 * is given. Storage errors are ignored (restricted environments).
 */
export function rememberLaunchStage(run:Run,result:RunResult|undefined,runIds?:string[],storage:StageStorage|undefined=defaultStorage()){
 if(!pickups.has(run.skill)||!result?.task||ENDED.has(result.activity?.status??"")||ended(run))return;
 const known=readLaunchStages(storage);
 if(Object.prototype.hasOwnProperty.call(known,run.id))return;
 known[run.id]=stageOf(run,result);
 if(runIds)for(const id of Object.keys(known))if(id!==run.id&&!runIds.includes(id))delete known[id];
 try{storage?.setItem(LAUNCH_STAGES_KEY,JSON.stringify(known));}catch{/* Ignore storage errors in restricted environments. */}
}
/** The recorded start stage of a pickup run, '' when unknown. */
export function launchStageOf(runId:string,storage:StageStorage|undefined=defaultStorage()){
 const value=readLaunchStages(storage)[runId];
 return typeof value==="string"?value:"";
}
