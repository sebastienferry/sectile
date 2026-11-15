import {lazy,Suspense,useEffect,useRef,useState} from "react";
import type {Task} from "../domain/board";
import {pullRequestLabel,pullRequestStateLabel,taskPullRequests} from "../domain/pull-requests";
import {isActive,launchInput,readTaskLaunchView,saveTaskLaunchView,statusLabel,type LaunchOptions,type Run,type TaskLaunchView} from "../domain/execution";
import {chainProgress,nextSkill,skillLabels,stageLabels,stages,taskStage} from "../domain/workflow";
import {skillStatusText,skillStatusTitle} from "../domain/skill-status";
import {executionSnapshot,launchTask,listTasks,stopRun} from "../lib/agent";
import type {TurnOptions} from "../lib/conversation";
import {conversationProfiles,type ProjectProfiles} from "../lib/profiles";
import {declareReviewed,openExternalLink,projectContext,providerReadiness,type ProjectContext,type Readiness} from "../lib/workspace";
import {useConversation} from "../lib/conversation-controller";
import {useSkillStatus} from "../lib/skill-results";
import {useExecutionMonitor} from "./ExecutionMonitor";
import {Markdown} from "./Markdown";
import {RunReview} from "./RunReview";
import {TurnSettings} from "./TurnSettings";
const RunConsole=lazy(()=>import("./RunConsole").then(module=>({default:module.RunConsole})));
const ProjectPrompt=lazy(()=>import("./ProjectPrompt"));

type WorkflowTaskProps={projectId:string;taskId:string;onBack:()=>void;onSetup:()=>void;onEdit?:()=>void;onExecution?:(runId:string)=>void;onOpenConversation?:(runId:string)=>void;onArchiveConversation?:(runId:string)=>void;onTasksLoaded?:(tasks:Task[])=>void};
export function WorkflowTask(props:WorkflowTaskProps){
 return <WorkflowTaskView key={JSON.stringify([props.projectId,props.taskId])} {...props}/>;
}
function WorkflowTaskView({projectId,taskId,onBack,onSetup,onEdit,onExecution,onOpenConversation,onArchiveConversation,onTasksLoaded}:WorkflowTaskProps){
 const monitor=useExecutionMonitor();
 const [task,setTask]=useState<Task>();const [project,setProject]=useState<ProjectContext>();
 const [selected,setSelected]=useState("");
 const [readiness,setReadiness]=useState<Readiness>();
 const [error,setError]=useState("");const [readError,setReadError]=useState("");const [notice,setNotice]=useState("");const [pending,setPending]=useState(false);
 const [draft,setDraft]=useState("");const [customSkill,setCustomSkill]=useState("");
 const [reviewNote,setReviewNote]=useState("");
 const [submitted,setSubmitted]=useState<{ids:string[];skill:string}>();
 const [launchView,setLaunchView]=useState<TaskLaunchView>(()=>readTaskLaunchView());
 const [provider,setProvider]=useState<"claude"|"codex">("claude");
 const [turnOptions,setTurnOptions]=useState<TurnOptions>({});
 const [profiles,setProfiles]=useState<ProjectProfiles>();
 const [profileId,setProfileId]=useState("");
 const customized=useRef(false);
 const acting=useRef(false),generation=useRef(0),reading=useRef(false);
 const runs=(monitor.snapshot?.runs??[]).filter(r=>r.projectId===projectId&&r.taskId===taskId).sort((a,b)=>b.createdAt.localeCompare(a.createdAt));
 const run=runs.find(r=>r.id===selected)??runs.find(isActive)??runs[0];
 async function refresh(){
  const seq=++generation.current;
  reading.current=true;
  try{
   const [tasks,context]=await Promise.all([listTasks(projectId),projectContext(projectId)]);
   if(seq!==generation.current)return;
   onTasksLoaded?.(tasks);
   const current=tasks.find(t=>t.id===taskId);
   if(!current)throw new Error("Ticket unavailable");
   setTask(current);setProject(context);setReadError("");
   if(!customized.current&&(context.aiProvider==="claude"||context.aiProvider==="codex")){
    setProvider(context.aiProvider);
   }
  }catch{
   if(seq===generation.current)setReadError("Unable to read ticket workflow. Refresh before launching an action.");
  }finally{
   if(seq===generation.current)reading.current=false;
  }
 }
 useEffect(()=>{void refresh();return()=>{generation.current++;};},[projectId,taskId]);
 useEffect(()=>{
  let current=true;
  customized.current=false;setProfiles(undefined);setProfileId("");setTurnOptions({});
  void Promise.resolve().then(()=>conversationProfiles(projectId)).then(data=>{
   if(!current||!data||!Array.isArray(data.profiles))return;
   setProfiles(data);
   const profile=data.profiles.find(p=>p.id===data.defaultId);
   if(profile&&!customized.current){
    setProfileId(profile.id);
    setProvider(profile.provider==="codex"?"codex":"claude");
    setTurnOptions({model:profile.model||undefined,mode:profile.mode||undefined});
   }
  }).catch(()=>{});
  return()=>{current=false;};
 },[projectId]);
 const effectiveProvider=customized.current?provider:(project?.aiProvider==="codex"?"codex":"claude");
 useEffect(()=>{if(!project?.aiProvider&&!customized.current)return;let current=true;void providerReadiness(effectiveProvider).then(value=>{if(current)setReadiness(value);}).catch(()=>{if(current)setReadiness(undefined);});return()=>{current=false;};},[project?.aiProvider,effectiveProvider]);
 useEffect(()=>{if(!run)return;const timer=setInterval(()=>{if(!acting.current&&!reading.current)void refresh();},4000);return()=>clearInterval(timer);},[run?.id,run?.status]);
 // The selected run's skill status, read through the reader the conversation
 // strip and the sidebar share, so the three surfaces never disagree (#908).
 const conversation=useConversation(run?.id??"");
 const asking=!!run?.conversation&&conversation.loaded&&conversation.snapshot.id===run.id&&!!conversation.snapshot.approvals?.length;
 const skill=useSkillStatus(run,{task,project,asking,runIds:monitor.snapshot?.runs.map(r=>r.id)});
 useEffect(()=>{if(!submitted)return;const next=runs.find(r=>!submitted.ids.includes(r.id));if(next){setSelected(next.id);setSubmitted(undefined);setNotice("");if(next.conversation&&onOpenConversation)onOpenConversation(next.id);}},[monitor.snapshot,submitted,onOpenConversation]);
 const selectedProfile=profiles?.profiles.find(p=>p.id===profileId);
 const formatProfileLabel=(profile:{provider:string;name:string})=>{
  const providerLabel=profile.provider==="codex"?"Codex":"Claude Code";
  const name=profile.name.trim();
  if(!name||name.toLowerCase()===providerLabel.toLowerCase()||name.toLowerCase()===profile.provider.toLowerCase()){
   return providerLabel;
  }
  return `${providerLabel} · ${name}`;
 };
 const engineChoices=[
  ...(profiles?.profiles.map(profile=>({
   key:`profile:${profile.id}`,
   label:formatProfileLabel(profile),
   provider:(profile.provider==="codex"?"codex":"claude") as "claude"|"codex",
  }))??[]),
  {key:"engine:claude",label:"Claude Code",provider:"claude" as const},
  {key:"engine:codex",label:"Codex",provider:"codex" as const},
 ];
 function selectEngine(val:string){
  customized.current=true;
  if(val.startsWith("profile:")){
   const profile=profiles?.profiles.find(item=>item.id===val.slice(8));
   if(profile){
    setProfileId(profile.id);
    setProvider(profile.provider==="codex"?"codex":"claude");
    setTurnOptions({model:profile.model||undefined,mode:profile.mode||undefined});
   }
  }else{
   setProfileId("");
   setProvider(val.slice(7)==="codex"?"codex":"claude");
   setTurnOptions({});
  }
 }
 const step=task&&project?nextSkill(task,project):undefined;
 const finished=step?.stage==="finished";
 const chain=task&&project?chainProgress(task,project):undefined;
 const active=runs.find(isActive);
 const connected=monitor.fresh&&monitor.snapshot?.status.connected&&!monitor.snapshot.status.contractError;
 const mcpConflict=readiness?.steps?.mcp?.configuration?.present===true&&readiness.steps.mcp.configuration.valid===false;
 const blocked=mcpConflict||pending||!!submitted||!!active||!connected||!project?.configured||!task||!!error||!!readError;
 async function launch(kind:"next"|"chain"|"custom"){
  if(acting.current||blocked||!task||!project||kind==="chain"&&chain?.reached)return;acting.current=true;++generation.current;setPending(true);setError("");setNotice("");
  try{
   const [tasks,context,state]=await Promise.all([listTasks(projectId),projectContext(projectId),executionSnapshot()]);
   const freshTask=tasks.find(t=>t.id===taskId);if(!freshTask)throw new Error("Ticket unavailable");
   setTask(freshTask);setProject(context);
   if(kind==="chain"){
    const progress=chainProgress(freshTask,context);
    if(progress.reached)return;
   }
   if(!state.status.connected||state.status.contractError||state.runs.some(r=>r.taskId===taskId&&r.projectId===projectId&&isActive(r)))throw new Error("Task busy");
   const checkProvider=customized.current?provider:context.aiProvider;
   if(checkProvider==="claude"||checkProvider==="codex"){
    const currentPreparation=await providerReadiness(checkProvider);setReadiness(currentPreparation);
    const mcp=currentPreparation.steps?.mcp?.configuration;
    if(mcp?.present&&!mcp.valid)throw new Error("MCP configuration does not match this companion");
   }
   const freshStep=nextSkill(freshTask,context);
   const skill=kind==="next"?freshStep.skillId:kind==="chain"?"pickup":customSkill;
   if(!context.configured||taskStage(freshTask,context.board)==="finished"||!skill||!context.skills.some(s=>s.id===skill)||kind==="next"&&skill!==step?.skillId)throw new Error("Workflow changed");
   const options:LaunchOptions={};
   if(launchView==="terminal")options.view="terminal";
   if(customized.current){
    if(profileId)options.engineId=profileId;
    else options.provider=provider;
    if(turnOptions.model)options.model=turnOptions.model;
    if(turnOptions.mode)options.conversationMode=turnOptions.mode;
   }
   await launchTask(projectId,launchInput(taskId,skill,draft,kind==="chain"?"autonomous":"",options));
   setSubmitted({ids:state.runs.map(r=>r.id),skill});setNotice("Launch accepted · waiting for execution. No automatic retry.");
   await monitor.refresh();await refresh();
  }catch{setError("Launch not confirmed: the ticket or its executions may have changed. Refresh status before trying again. Your instructions are kept.");}
  finally{acting.current=false;setPending(false);}
 }
 const readinessUnknown=readiness?.steps?.skills?.status!=="success"||readiness?.steps?.mcp?.status!=="success";
 return <section className="workflow-task">
  <div className="workflow-task-top">
   <button onClick={onBack}>← Project tickets</button>
   <div className="workflow-task-top-actions">
    <button disabled={pending} onClick={()=>{setError("");void monitor.refresh();void refresh();}}>Refresh workflow</button>
    {task&&onEdit&&<button onClick={onEdit}>Edit ticket</button>}
   </div>
  </div>
  <header className="workflow-header">
   <div className="workflow-header-eyebrow">
    <span className="eyebrow">TICKET WORKFLOW · {task?.key||taskId}</span>
    {!!taskPullRequests(task).length&&<div className="workflow-pr-links"><strong>PR/MR</strong>{taskPullRequests(task).map(link=><button type="button" key={link.url} onClick={()=>void openExternalLink(link.url).catch(()=>setError("Unable to open PR / MR link."))}>{pullRequestLabel(link)} · {pullRequestStateLabel(link)}</button>)}</div>}
   </div>
   <h2>{task?.title||"Loading ticket…"}</h2>
   <p className="workflow-meta">Tracker status: {task?.trackerStatus||"Unavailable"} · Assignee: {task?.assignee||"Unassigned"}{task?.sprint?` · Sprint: ${task.sprint}`:""}</p>
   {task?.description&&<details className="workflow-description"><summary>Ticket description</summary><Markdown text={task.description}/></details>}
   <ol className="workflow-progress" aria-label="Workflow stages">{stages.map(stage=><li key={stage} aria-current={step?.stage===stage?"step":undefined}>{stageLabels[stage]}</li>)}</ol>
  </header>
  {(error||readError)&&<p role="alert" className="error">{error||readError}</p>}
  {notice&&<p role="status" className="workflow-notice">{notice}</p>}
  {task&&project&&!finished&&mcpConflict&&<p role="alert" className="error">The existing MCP entry does not match this companion. Check its target in AI engines before launching the workflow.</p>}
  {step?.reason&&<p role="status" className="workflow-reason">{step.reason}</p>}
  {active&&<p role="status" className="workflow-active-banner">{skillLabels[active.skill]||active.skill} · {statusLabel(active)}. An execution is already open for this ticket.</p>}
  {task&&project&&!finished&&(
   <div className="workflow-launch-panel">
    <div className="workflow-launch-main">
     <div className="workflow-actions">
      <button className="primary" disabled={blocked||!step?.skillId} onClick={()=>void launch("next")}>{pending?"Sending…":step?.skillId?`Next step: ${step.label}`:step?.label||"Step unavailable"}</button>
      {project?.skills.some(s=>s.id==="pickup")&&(chain?.reached?<p role="status" className="workflow-chain-reached">Chain target reached: {stageLabels[chain.target]}.</p>:<button disabled={blocked} onClick={()=>void launch("chain")}>Run chain · up to {project.policy.fullChainStopStage==="reviewed"?"Reviewed":"Implemented"}</button>)}
     </div>
     <div className="workflow-launch-bar">
      <div className="workflow-view-toggle" role="group" aria-label="Launch view">
       <button type="button" className={launchView==="conversation"?"is-active":""} aria-pressed={launchView==="conversation"} disabled={pending} onClick={()=>{setLaunchView("conversation");saveTaskLaunchView("conversation");}}>Conversation</button>
       <button type="button" className={launchView==="terminal"?"is-active":""} aria-pressed={launchView==="terminal"} disabled={pending} onClick={()=>{setLaunchView("terminal");saveTaskLaunchView("terminal");}}>Terminal</button>
      </div>
      <TurnSettings compact showProviderMark provider={effectiveProvider} defaultModel={selectedProfile?.model||project?.aiModel} value={turnOptions} onChange={next=>{customized.current=true;setTurnOptions(next);}} disabled={pending} engineChoices={engineChoices} selectedEngineKey={profileId?`profile:${profileId}`:`engine:${effectiveProvider}`} onSelectEngine={selectEngine}/>
     </div>
    </div>
    <div className="workflow-launch-disclosures">
     <details className="workflow-advanced"><summary>Instructions and other skill</summary><label>Additional instructions<textarea value={draft} maxLength={5000} disabled={pending} onChange={e=>setDraft(e.target.value)}/></label><label>Skill<select aria-label="Other skill" value={customSkill} onChange={e=>setCustomSkill(e.target.value)}><option value="">Choose a skill…</option>{project?.skills.map(s=><option key={s.id} value={s.id}>{skillLabels[s.id]||s.id}{s.command?` · /${s.command}`:""}</option>)}</select></label><button disabled={blocked||!customSkill||step?.stage==="finished"} onClick={()=>void launch("custom")}>Launch selected skill</button></details>
     {readinessUnknown&&<details className="workflow-prerequisite"><summary>Engine preparation unverified</summary><p>Skills/MCP preparation is unconfirmed for this engine. A plugin installation may already be ready; CLI connection alone does not guarantee it.</p><button onClick={onSetup}>Check AI engines</button></details>}
    </div>
   </div>
  )}
  {step?.stage==="implemented"&&task?.prUrl&&<details className="manual-review"><summary>Declare review validated</summary><p>This action explicitly records the "Reviewed" stage on the server. It does not launch any skill and does not merge the PR.</p><label>Justification<textarea value={reviewNote} onChange={e=>setReviewNote(e.target.value)} maxLength={1000}/></label><button disabled={blocked||!reviewNote.trim()} onClick={()=>{if(acting.current)return;acting.current=true;setPending(true);void declareReviewed(projectId,taskId,reviewNote).then(()=>refresh()).catch(()=>setError("Validation not confirmed. Refresh the ticket before trying again.")).finally(()=>{acting.current=false;setPending(false);});}}>Save validated review</button></details>}
  {run&&(
   <section className="workflow-run">
    <div className="run-heading">
     <div className="run-heading-main">
      <h3>{skillLabels[run.skill]||run.skill} · {statusLabel(run)}</h3>
      {skill&&<p role="status" className="workflow-run-status" title={skillStatusTitle(skill)}><span className={`skill-status-mark skill-${skill.kind}`} aria-hidden="true"/>{skillStatusText(skill)}</p>}
     </div>
     <div className="run-heading-actions">
      <label className="workflow-run-select">
       <span className="sr-only">Execution</span>
       <select aria-label="Ticket execution" value={run?.id??""} onChange={e=>setSelected(e.target.value)}>
        {runs.map(r=><option key={r.id} value={r.id}>{skillLabels[r.skill]||r.skill} · {statusLabel(r)} · {new Date(r.createdAt).toLocaleString("en")}</option>)}
       </select>
      </label>
      {run.conversation&&onOpenConversation&&<button type="button" onClick={()=>onOpenConversation(run.id)}>Open conversation</button>}
      {isActive(run)&&<button disabled={pending||!monitor.fresh||run.cancelRequested} onClick={()=>{void stopRun(run.id).then(()=>monitor.refresh()).catch(()=>setError("Stop not confirmed. Refresh status."));}}>{run.cancelRequested?"Stop requested…":"Stop execution"}</button>}
     </div>
    </div>
    <RunReview key={run.id} run={run}/>
    <TaskOutput projectId={projectId} run={run} task={task} board={project?.board} project={project} onExecution={onExecution} onArchive={onArchiveConversation?()=>onArchiveConversation(run.id):undefined} onChanged={()=>void monitor.refresh()}/>
   </section>
  )}
 </section>;
}
function TaskOutput({projectId,run,task,board,project,onExecution,onArchive,onChanged}:{projectId:string;run:Run;task?:Task;board?:ProjectContext["board"];project?:ProjectContext;onExecution?:(runId:string)=>void;onArchive?:()=>void;onChanged:()=>void}){
 return run.conversation?<Suspense fallback={<p>Opening conversation…</p>}><ProjectPrompt projectId={projectId} runId={run.id} run={run} task={task} board={board} projectContext={project} directory={run.directory} folders={run.folders} onCreated={()=>{}} onOpenQueue={onExecution?()=>onExecution(""):undefined} onArchive={onArchive} onStopped={onChanged} onFoldersChanged={onChanged}/></Suspense>:run.sessionId||run.trace||run.restored?<Suspense fallback={<p>Opening console…</p>}><RunConsole runId={run.id} sessionId={run.sessionId}/></Suspense>:<p>The console will be available once picked up by Go.</p>;
}
