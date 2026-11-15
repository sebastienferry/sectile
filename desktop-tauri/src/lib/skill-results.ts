// One skill result reader per run, shared by every surface showing it (#908):
// the conversation status strip, the ticket workflow panel and the sidebar row.
// Each surface asks on every run list poll; the throttling rule of
// `domain/skill-result-refresh.ts` decides whether a read is due, and a read in
// flight is never doubled, so a run costs the same reads whatever shows it.
import {useEffect,useMemo,useRef,useSyncExternalStore} from "react";
import type {Task} from "../domain/board";
import type {Run} from "../domain/execution";
import {SETTLE_MS,skillEnded,skillResultDue,skillResultStamp,type SkillResultStamp} from "../domain/skill-result-refresh";
import {isTicketSkillRun,launchStageOf,rememberLaunchStage,skillStatus,type SkillStatus} from "../domain/skill-status";
import {nextSkill,type RunResult} from "../domain/workflow";
import {runResult,type ProjectContext} from "./workspace";

interface Entry {
 result?:RunResult;
 stamp?:SkillResultStamp;
 reading:boolean;
 subscribers:number;
 listeners:Set<()=>void>;
}
const entries=new Map<string,Entry>();
// Below the run list's 3 s poll: a surface that renders a fresh run object
// more often than the monitor polls never turns the settling window into a loop.
const READ_FLOOR_MS=2_000;

function entryFor(runId:string){
 let entry=entries.get(runId);
 if(!entry){entry={reading:false,subscribers:0,listeners:new Set()};entries.set(runId,entry);}
 return entry;
}
function subscribeTo(runId:string){
 return (listener:()=>void)=>{
  if(!runId)return ()=>{};
  const entry=entryFor(runId);
  entry.subscribers++;entry.listeners.add(listener);
  return ()=>{
   entry.listeners.delete(listener);
   // Released with its last surface; a read still in flight is then dropped.
   if(--entry.subscribers<=0&&entries.get(runId)===entry)entries.delete(runId);
  };
 };
}

function read(run:Run,force:boolean,runIds?:string[]){
 const entry=entryFor(run.id);
 const now=Date.now();
 if(entry.reading)return;
 if(!force&&(entry.stamp&&now-entry.stamp.at<READ_FLOOR_MS||!skillResultDue(run,entry.stamp,now)))return;
 entry.reading=true;
 const last=entry.stamp;
 void runResult(run.id).then(result=>{
  if(entries.get(run.id)!==entry)return;
  entry.result=result;
  entry.stamp=skillResultStamp(run,Date.now(),last,skillEnded(result));
  rememberLaunchStage(run,result,runIds);
 },()=>{
  if(entries.get(run.id)!==entry)return;
  // A failed read keeps the last result and waits for the next regular read
  // rather than retrying at every poll.
  entry.stamp={...skillResultStamp(run,Date.now(),last,last?.ended??false),changedAt:Date.now()-SETTLE_MS};
 }).finally(()=>{
  entry.reading=false;
  for(const listener of entry.listeners)listener();
 });
}

/**
 * The last skill result read for a ticket skill run, refreshed when due on each
 * run list poll and when the conversation turn goes from busy to idle. Free
 * runs are never read and return undefined. `runIds` is the run list, used to
 * prune the remembered pickup start stages.
 */
export function useSkillResult(run:Run|undefined,turnBusy?:boolean,runIds?:string[]):RunResult|undefined{
 const runId=run&&isTicketSkillRun(run)?run.id:"";
 const subscribe=useMemo(()=>subscribeTo(runId),[runId]);
 const result=useSyncExternalStore(subscribe,()=>runId?entries.get(runId)?.result:undefined);
 const wasBusy=useRef(turnBusy);
 // The monitor gives a new run object on every poll: that is the moment to
 // ask whether a read is due.
 useEffect(()=>{if(runId&&run)read(run,false,runIds);},[runId,run]);
 useEffect(()=>{
  const settled=wasBusy.current===true&&turnBusy===false;
  wasBusy.current=turnBusy;
  if(settled&&runId&&run)read(run,true,runIds);
 },[turnBusy]);
 return result;
}

export interface SkillStatusInputs {
 task?:Task;
 project?:ProjectContext;
 /** The conversation shows a question or an approval waiting for the user. */
 asking?:boolean;
 turnBusy?:boolean;
 runIds?:string[];
}
/** The skill status of a run, as every surface shows it. Null for a free run. */
export function useSkillStatus(run:Run|undefined,{task,project,asking,turnBusy,runIds}:SkillStatusInputs={}):SkillStatus|null{
 const result=useSkillResult(run,turnBusy,runIds);
 const step=task&&project?nextSkill(task,project):undefined;
 const nextStep=step?.skillId?step.label:"";
 const launchStage=run?launchStageOf(run.id):"";
 return useMemo(()=>skillStatus(run,result,{nextStep,launchStage,asking}),[run,result,nextStep,launchStage,asking]);
}

/** Forgets every reader. For tests only. */
export function resetSkillResults(){
 entries.clear();
}
