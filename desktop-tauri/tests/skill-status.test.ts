import test from "node:test";
import assert from "node:assert/strict";
import {isTicketSkillRun,launchStageOf,recordedReason,rememberLaunchStage,skillId,skillStatus,skillStatusText,skillStatusTitle} from "../src/domain/skill-status.ts";
import type {Run} from "../src/domain/execution.ts";
import type {RunResult,SkillRecord} from "../src/domain/workflow.ts";

const DASH=" "+String.fromCharCode(0x2014)+" ";
const run=(over:Partial<Run>={}):Run=>({id:"r",projectId:"p",taskId:"t",skill:"clarify",status:"running",createdAt:"2026-11-15",conversation:true,...over});
const record=(over:Partial<SkillRecord>={}):SkillRecord=>({id:"r",taskId:"t",skillId:"clarify",status:"running",...over});
const result=(activity:Partial<SkillRecord>,status="to_clarify",extra:Partial<RunResult>={}):RunResult=>({activity:record(activity),task:{status},...extra});
const kind=(...args:Parameters<typeof skillStatus>)=>skillStatus(...args)?.kind;

test("row 1: a requested stop not taken effect yet reads Stopping, before any verdict",()=>{
 const status=skillStatus(run({cancelRequested:true}),result({status:"completed"},"to_specify"));
 assert.equal(status?.label,"Stopping");
 assert.equal(status?.detail,"Waiting for the session to exit");
 assert.equal(kind(run({cancelRequested:true,status:"canceled"})),"stopped");
});

test("row 2: a completed skill whose task reached its expected stage is Done with its outcome and next step",()=>{
 const status=skillStatus(run({status:"completed"}),result({status:"completed"},"to_specify"),{nextStep:"Specify"});
 assert.equal(status?.kind,"done");
 assert.equal(status?.label,"Done · Clarified");
 assert.equal(status?.next,"Next: Specify");
 assert.equal(skillStatusText(status!),"Done · Clarified · Next: Specify");
});

test("US2.2: a session left open after the skill completed is Done, not Working",()=>{
 assert.equal(kind(run({status:"running"}),result({status:"completed"},"to_specify")),"done");
});

test("row 3: a completed skill whose task is behind its expected stage needs verification",()=>{
 const status=skillStatus(run({status:"completed"}),result({status:"completed"},"to_clarify"));
 assert.equal(status?.kind,"verify");
 assert.equal(status?.detail,"Clarify reported completion; the task is still at new");
 assert.equal(skillStatus(run({status:"completed"}),{activity:record({status:"completed"})})?.detail,"Clarify reported completion; the task is still at an unknown stage");
});

test("row 4: a failed skill could not finish and says why and what to do",()=>{
 const live=skillStatus(run(),result({status:"failed",error:"git push rejected\nhint: fetch first"}));
 assert.equal(live?.kind,"failed");
 assert.equal(live?.detail,"git push rejected");
 assert.equal(live?.reason,"git push rejected\nhint: fetch first");
 assert.equal(live?.next,"Open the console");
 assert.equal(skillStatusText(live!),"Could not finish · git push rejected");
 assert.equal(skillStatus(run({status:"failed"}),result({status:"failed",error:"boom"}))?.next,"Relaunch Clarify");
});

test("row 5: a canceled skill is Stopped with its recorded reason",()=>{
 const status=skillStatus(run({status:"canceled"}),result({status:"canceled",summary:"Owner asked to stop"}));
 assert.equal(status?.kind,"stopped");
 assert.equal(status?.detail,"Owner asked to stop");
});

test("row 6: a running process with a declared wait needs an answer",()=>{
 assert.equal(skillStatus(run({waitingSince:"2026-11-15T10:00:00Z"}))?.label,"Needs your answer");
 assert.equal(skillStatus(run({waitingSince:"2026-11-15T10:00:00Z"}))?.detail,"Answer in the console");
});

test("US2.3: zero-date marks are no wait, a pending question is one, a stale or outranked wait is ignored",()=>{
 assert.equal(kind(run({waitingSince:"0001-01-01T00:00:00Z"})),"working");
 assert.equal(kind(run(),undefined,{asking:true}),"waiting");
 assert.equal(kind(run({status:"completed",waitingSince:"2026-11-15T10:00:00Z"})),"verify");
 assert.equal(kind(run({status:"queued",waitingSince:"2026-11-15T10:00:00Z"})),"working");
 assert.equal(kind(run({waitingSince:"2026-11-15T10:00:00Z"}),result({status:"completed"},"to_specify"),{asking:true}),"done");
});

test("US2.4: a launch parked until a repository is chosen asks for one",()=>{
 assert.equal(skillStatus(run({status:"waiting"}))?.detail,"Choose a repository for this ticket");
 assert.equal(skillStatus(run({status:"queued"}),result({status:"running",waitingReason:"repository"}))?.detail,"Choose a repository for this ticket");
});

test("row 7: a queued, preparing or running process is Working with the skill name",()=>{
 assert.equal(skillStatusText(skillStatus(run({status:"queued"}))!),"Working · Clarify · Queued");
 assert.equal(skillStatus(run({status:"pending"}))?.detail,"Queued");
 assert.equal(skillStatus(run({status:"preparing"}))?.detail,"Preparing");
 assert.equal(skillStatusText(skillStatus(run({skill:"specify"}))!),"Working · Specify");
 assert.equal(skillStatus(run(),result({status:"running",summary:"Started"+DASH+"No MCP call for 10 minutes"}))?.detail,"No output for a while");
});

test("row 8: a clean exit without a skill verdict needs verification",()=>{
 const status=skillStatus(run({status:"completed"}));
 assert.equal(status?.kind,"verify");
 assert.equal(status?.detail,"The session ended without reporting a result");
});

test("row 9: a failed or canceled process without a verdict could not finish or stopped",()=>{
 const failed=skillStatus(run({status:"failed"}));
 assert.deepEqual([failed?.label,failed?.detail,failed?.next],["Could not finish","reason not recorded","Relaunch Clarify"]);
 assert.equal(skillStatus(run({status:"canceled"}))?.detail,"reason not recorded");
});

test("US2.5: a record of another execution, task or skill is ignored",()=>{
 for(const other of [{id:"x"},{taskId:"x"},{skillId:"specify"}]){
  assert.equal(kind(run({status:"completed"}),result({status:"completed",...other},"to_specify")),"verify");
  assert.match(skillStatus(run({status:"completed"}),result({status:"completed",...other},"to_specify"))!.diagnostics,/Skill record: none/);
 }
});

test("US2.6: without a result the process alone decides",()=>{
 assert.equal(kind(run()),"working");
 assert.equal(kind(run({status:"completed"}),undefined),"verify");
});

test("US3.2: every workflow skill names its outcome; a skill with no stage reports its own word",()=>{
 const cases:[string,string,string][]=[["clarify","to_specify","Done · Clarified"],["specify","to_implement","Done · Specification ready"],["implement","to_test","Done · Implemented"],["adjust","to_close","Done · Review complete"],["review","to_close","Done · Review complete"],["handoff","finished","Done · Handed off"],["rewrite_story","to_clarify","Done · reported by the skill"]];
 for(const [skill,status,label] of cases)assert.equal(skillStatus(run({skill,status:"completed"}),result({skillId:skill,status:"completed"},status))?.label,label,skill);
});

test("US3.3: a pickup is Done only when it moved its task forward from a known start stage",()=>{
 const pickup=run({skill:"pickup",status:"completed"});
 const done=result({skillId:"pickup",status:"completed"},"to_test");
 assert.equal(skillStatus(pickup,done,{launchStage:"clarified"})?.label,"Done · Now implemented");
 assert.equal(skillStatus(pickup,done,{launchStage:"implemented"})?.detail,"Pickup reported completion; the task is at implemented");
 assert.equal(kind(pickup,done),"verify");
 assert.equal(kind(run({skill:"pickup_issues",status:"completed"}),result({skillId:"pickup_issues",status:"completed"},"to_test"),{launchStage:"new"}),"done");
});

test("US3.4: the next step is shown only while the task is not finished",()=>{
 assert.equal(skillStatus(run({status:"completed"}),result({status:"completed"},"to_specify"),{nextStep:"Specify"})?.next,"Next: Specify");
 assert.equal(skillStatus(run({status:"completed"}),result({status:"completed"},"to_specify"))?.next,"");
 assert.equal(skillStatus(run({skill:"handoff",status:"completed"}),result({skillId:"handoff",status:"completed"},"finished"),{nextStep:"Handoff"})?.next,"");
});

test("US4: reasons are the first recorded line, error before note, without bookkeeping, cut to 120 characters",()=>{
 assert.equal(recordedReason(record({error:"denied",summary:"note"})),"denied");
 assert.equal(recordedReason(record({summary:"Execution reported by a local agent or native client"+DASH+"Tests failed"})),"Tests failed");
 assert.equal(recordedReason(record({summary:"No MCP call for 10 minutes"})),"");
 assert.equal(recordedReason(record()),null);
 const long="x".repeat(200);
 const failed=skillStatus(run(),result({status:"failed",error:long}));
 assert.equal(failed?.detail.length,120);
 assert.ok(failed?.detail.endsWith("…"));
 assert.equal(failed?.reason,long);
 assert.equal(skillStatus(run(),result({status:"failed",summary:"",error:""}))?.detail,"reason not recorded");
 assert.equal(skillStatus(run(),result({status:"failed"}))?.detail,"reason not available");
});

test("US5: Stopped by you and Session disconnected appear only on record",()=>{
 assert.equal(skillStatus(run({status:"canceled",cancelRequested:true}),result({status:"canceled",disconnected:true}))?.detail,"Stopped by you");
 assert.equal(skillStatus(run({status:"canceled"}),result({status:"canceled",disconnected:true}))?.detail,"Session disconnected");
 assert.equal(skillStatus(run({status:"canceled"}),result({status:"canceled",summary:""}))?.detail,"reason not recorded");
 assert.equal(skillStatus(run({status:"canceled"}),result({status:"canceled",summary:"Closed by the owner"}))?.detail,"Closed by the owner");
});

test("US6: free conversations and discussions have no skill status unless a stop is pending",()=>{
 assert.equal(skillStatus(undefined,undefined),null);
 assert.equal(skillStatus(run({taskId:""}),undefined),null);
 assert.equal(skillStatus(run({skill:"discuss"}),result({status:"completed"})),null);
 assert.equal(isTicketSkillRun(run({skill:"discuss"})),false);
 assert.equal(isTicketSkillRun(run()),true);
 const stopping=skillStatus(run({taskId:"",cancelRequested:true}),undefined);
 assert.equal(stopping?.label,"Stopping");
 assert.equal(stopping?.diagnostics,"Execution: Running");
});

test("US7: a successor owns the status and the launched skill stays in the diagnostics",()=>{
 const base=result({status:"completed"},"to_specify");
 const working=skillStatus(run(),{...base,successor:{...record({id:"s",skillId:"specify",status:"running"})}});
 assert.equal(working?.label,"Working · Specify");
 assert.equal(working?.diagnostics,"Execution: Running · Skill record: clarify completed; now specify running · Task stage: clarified");
 assert.equal(kind(run(),{...base,successor:{...record({id:"s",skillId:"specify",status:"running"}),waitingSince:"2026-11-15T10:00:00Z"}}),"waiting");
 assert.equal(kind(run({waitingSince:"2026-11-15T10:00:00Z"}),{...base,successor:{...record({id:"s",skillId:"specify",status:"running"}),waitingSince:"0001-01-01T00:00:00Z"}}),"working");
 assert.equal(skillStatus(run(),{activity:record({status:"completed"}),task:{status:"to_implement"},successor:record({id:"s",skillId:"specify",status:"completed"})})?.label,"Done · Specification ready");
});

test("US1.3: diagnostics name the run state, the record and the task stage",()=>{
 assert.equal(skillStatus(run({status:"completed"}),result({status:"completed"},"to_specify"))?.diagnostics,"Execution: Finished · Skill record: completed · Task stage: clarified");
 assert.equal(skillStatus(run({status:"queued"}))?.diagnostics,"Execution: Queued · Skill record: none · Task stage: unknown");
 assert.equal(skillStatus(run({waitingSince:"2026-11-15T10:00:00Z"}))?.diagnostics.startsWith("Execution: Waiting for you"),true);
 const title=skillStatusTitle(skillStatus(run({status:"completed"}),result({status:"completed"},"to_specify"),{nextStep:"Specify"})!);
 assert.equal(title,"Done · Clarified\nNext: Specify\nExecution: Finished · Skill record: completed · Task stage: clarified");
});

test("console-style skill names map onto workflow skills",()=>{
 assert.equal(skillId("sectile:specify-issue"),"specify");
 assert.equal(skillId("pickup-issues"),"pickup_issues");
 assert.equal(skillStatus(run({skill:"sectile:specify-issue",status:"completed"}),result({skillId:"sectile:specify-issue",status:"completed"},"to_implement"))?.label,"Done · Specification ready");
});

function memoryStorage(){
 const values=new Map<string,string>();
 return {getItem:(key:string)=>values.get(key)??null,setItem:(key:string,value:string)=>void values.set(key,value),values};
}

test("US3.3: a pickup's start stage is recorded once, while it runs, and pruned with its run",()=>{
 const storage=memoryStorage();
 const pickup=run({id:"p1",skill:"pickup"});
 rememberLaunchStage(pickup,result({id:"p1",skillId:"pickup",status:"running"},"to_specify"),["p1"],storage);
 assert.equal(launchStageOf("p1",storage),"clarified");
 rememberLaunchStage(pickup,result({id:"p1",skillId:"pickup",status:"running"},"to_test"),["p1"],storage);
 assert.equal(launchStageOf("p1",storage),"clarified");
 rememberLaunchStage(run({id:"p2",skill:"pickup",status:"completed"}),result({id:"p2",skillId:"pickup",status:"completed"},"to_test"),["p1","p2"],storage);
 assert.equal(launchStageOf("p2",storage),"");
 rememberLaunchStage(run({id:"p3",skill:"pickup"}),result({id:"p3",skillId:"pickup",status:"completed"},"to_test"),["p1","p3"],storage);
 assert.equal(launchStageOf("p3",storage),"");
 rememberLaunchStage(run({id:"c1"}),result({id:"c1",status:"running"}),["p1","c1"],storage);
 assert.equal(launchStageOf("c1",storage),"");
 rememberLaunchStage(run({id:"p4",skill:"pickup_issues"}),result({id:"p4",skillId:"pickup_issues",status:"running"},"to_implement"),["p4"],storage);
 assert.equal(launchStageOf("p4",storage),"specified");
 assert.equal(launchStageOf("p1",storage),"");
 assert.deepEqual(JSON.parse(storage.values.get("sectile:launchStages")!),{p4:"specified"});
});

test("launch stages survive a broken or missing storage",()=>{
 const broken={getItem:()=>{throw new Error("denied");},setItem:()=>{throw new Error("denied");}};
 rememberLaunchStage(run({skill:"pickup"}),result({skillId:"pickup",status:"running"}),[],broken);
 assert.equal(launchStageOf("r",broken),"");
 assert.equal(launchStageOf("r",undefined),"");
});
