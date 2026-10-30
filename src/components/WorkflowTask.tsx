import {lazy,Suspense,useEffect,useRef,useState} from "react";
import type {Task} from "../domain/board";
import {pullRequestLabel,pullRequestStateLabel,taskPullRequests} from "../domain/pull-requests";
import {isActive,launchInput,statusLabel,type Run} from "../domain/execution";
import {chainProgress,nextSkill,skillLabels,skillResultLabel,stageLabels,stages,taskStage,type RunResult} from "../domain/workflow";
import {executionSnapshot,launchTask,listTasks,stopRun} from "../lib/agent";
import {declareReviewed,openExternalLink,projectContext,providerReadiness,runResult,type ProjectContext,type Readiness} from "../lib/workspace";
import {useExecutionMonitor} from "./ExecutionMonitor";
import {Markdown} from "./Markdown";
import {RunReview} from "./RunReview";
const RunConsole=lazy(()=>import("./RunConsole").then(module=>({default:module.RunConsole})));
const ProjectPrompt=lazy(()=>import("./ProjectPrompt"));

type WorkflowTaskProps={projectId:string;taskId:string;onBack:()=>void;onSetup:()=>void;onExecution?:(runId:string)=>void;onArchiveConversation?:(runId:string)=>void};
export function WorkflowTask(props:WorkflowTaskProps){
 return <WorkflowTaskView key={JSON.stringify([props.projectId,props.taskId])} {...props}/>;
}
function WorkflowTaskView({projectId,taskId,onBack,onSetup,onExecution,onArchiveConversation}:WorkflowTaskProps){
 const monitor=useExecutionMonitor();
 const [task,setTask]=useState<Task>();const [project,setProject]=useState<ProjectContext>();
 const [selected,setSelected]=useState("");const [result,setResult]=useState<RunResult>();
 const [readiness,setReadiness]=useState<Readiness>();
 const [error,setError]=useState("");const [readError,setReadError]=useState("");const [notice,setNotice]=useState("");const [pending,setPending]=useState(false);
 const [draft,setDraft]=useState("");const [customSkill,setCustomSkill]=useState("");
 const [reviewNote,setReviewNote]=useState("");
 const [submitted,setSubmitted]=useState<{ids:string[];skill:string}>();
 const acting=useRef(false),generation=useRef(0),reading=useRef(false);
 const runs=(monitor.snapshot?.runs??[]).filter(r=>r.projectId===projectId&&r.taskId===taskId).sort((a,b)=>b.createdAt.localeCompare(a.createdAt));
 const run=runs.find(r=>r.id===selected)??runs.find(isActive)??runs[0];
 async function refresh(){
  const seq=++generation.current;
  reading.current=true;
  try{
   const [tasks,context]=await Promise.all([listTasks(projectId),projectContext(projectId)]);
   if(seq!==generation.current)return;
   const current=tasks.find(t=>t.id===taskId);
   if(!current)throw new Error("Ticket unavailable");
   setTask(current);setProject(context);setReadError("");
  }catch{
   if(seq===generation.current)setReadError("Impossible de lire le workflow du ticket. Actualisez avant de lancer une action.");
  }finally{
   if(seq===generation.current)reading.current=false;
  }
 }
 useEffect(()=>{void refresh();return()=>{generation.current++;};},[projectId,taskId]);
 useEffect(()=>{if(!project?.aiProvider)return;let current=true;void providerReadiness(project.aiProvider).then(value=>{if(current)setReadiness(value);}).catch(()=>{if(current)setReadiness(undefined);});return()=>{current=false;};},[project?.aiProvider]);
 useEffect(()=>{if(!run)return;let current=true;setResult(undefined);let resultReading=false;const read=()=>{if(resultReading)return;resultReading=true;void runResult(run.id).then(value=>{if(current)setResult(value);}).catch(()=>{if(current)setResult(undefined);}).finally(()=>{resultReading=false;});};read();const timer=setInterval(()=>{read();if(!acting.current&&!reading.current)void refresh();},4000);return()=>{current=false;clearInterval(timer);};},[run?.id,run?.status]);
 useEffect(()=>{if(!submitted)return;const next=runs.find(r=>!submitted.ids.includes(r.id));if(next){setSelected(next.id);setSubmitted(undefined);setNotice("");}},[monitor.snapshot,submitted]);
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
   if(context.aiProvider==="claude"||context.aiProvider==="codex"){
    const currentPreparation=await providerReadiness(context.aiProvider);setReadiness(currentPreparation);
    const mcp=currentPreparation.steps?.mcp?.configuration;
    if(mcp?.present&&!mcp.valid)throw new Error("MCP configuration does not match this companion");
   }
   const freshStep=nextSkill(freshTask,context);
   const skill=kind==="next"?freshStep.skillId:kind==="chain"?"pickup":customSkill;
   if(!context.configured||taskStage(freshTask,context.board)==="finished"||!skill||!context.skills.some(s=>s.id===skill)||kind==="next"&&skill!==step?.skillId)throw new Error("Workflow changed");
   await launchTask(projectId,launchInput(taskId,skill,draft,kind==="chain"?"autonomous":""));
   setSubmitted({ids:state.runs.map(r=>r.id),skill});setNotice("Lancement accepté · en attente de l’exécution. Aucun renvoi automatique.");
   await monitor.refresh();await refresh();
  }catch{setError("Lancement non confirmé : le ticket ou ses exécutions ont pu changer. Actualisez le suivi avant de réessayer. Vos instructions sont conservées.");}
  finally{acting.current=false;setPending(false);}
 }
 const readinessUnknown=readiness?.steps?.skills?.status!=="success"||readiness?.steps?.mcp?.status!=="success";
 return <section className="workflow-task"><button onClick={onBack}>← Tickets du projet</button>
 <header><span className="eyebrow">WORKFLOW DU TICKET · {task?.key||taskId}</span><h2>{task?.title||"Chargement du ticket…"}</h2><p>Statut tracker : {task?.trackerStatus||"Indisponible"}</p><p className="muted">Moteur du projet : {project?.aiProvider==="claude"?"Claude Code":project?.aiProvider==="codex"?"Codex":project?.aiProvider||"Non renseigné"}{project?.aiModel&&` · ${project.aiModel}`}</p></header>
 {task?.description&&<details><summary>Description du ticket</summary><Markdown text={task.description}/></details>}
 <ol className="workflow-progress" aria-label="Étapes du workflow">{stages.map(stage=><li key={stage} aria-current={step?.stage===stage?"step":undefined}>{stageLabels[stage]}</li>)}</ol>
 {(error||readError)&&<p role="alert" className="error">{error||readError}</p>}{notice&&<p role="status">{notice}</p>}
 <button disabled={pending} onClick={()=>{setError("");void monitor.refresh();void refresh();}}>Actualiser le workflow</button>
 {task&&project&&!finished&&readinessUnknown&&<details className="workflow-prerequisite"><summary>Préparation du moteur non vérifiée</summary><p>Préparation skills/MCP non confirmée pour ce moteur. Une installation par plugin peut déjà être prête ; la connexion CLI seule ne le garantit pas.</p><button onClick={onSetup}>Vérifier les moteurs IA</button></details>}
 {task&&project&&!finished&&mcpConflict&&<p role="alert" className="error">L’entrée MCP existante ne correspond pas à ce compagnon. Vérifiez sa cible dans les moteurs IA avant de lancer le workflow.</p>}
 {step?.reason&&<p role="status">{step.reason}</p>}
 {active&&<p role="status">{skillLabels[active.skill]||active.skill} · {statusLabel(active)}. Une exécution est déjà ouverte pour ce ticket.</p>}
 {task&&project&&!finished&&<div className="workflow-actions"><button className="primary" disabled={blocked||!step?.skillId} onClick={()=>void launch("next")}>{pending?"Envoi…":step?.skillId?`Étape suivante : ${step.label}`:step?.label||"Étape indisponible"}</button>
 {project?.skills.some(s=>s.id==="pickup")&&(chain?.reached?<p role="status">Objectif de la chaîne atteint : {stageLabels[chain.target]}.</p>:<button disabled={blocked} onClick={()=>void launch("chain")}>Lancer la chaîne · jusqu’à {project.policy.fullChainStopStage==="reviewed"?"Revu":"Implémenté"}</button>)}</div>}
 {task&&project&&!finished&&<details className="workflow-advanced"><summary>Instructions et autre skill</summary><label>Instructions supplémentaires<textarea value={draft} maxLength={5000} disabled={pending} onChange={e=>setDraft(e.target.value)}/></label><label>Skill<select aria-label="Autre skill" value={customSkill} onChange={e=>setCustomSkill(e.target.value)}><option value="">Choisir une skill…</option>{project?.skills.map(s=><option key={s.id} value={s.id}>{skillLabels[s.id]||s.id}{s.command?` · /${s.command}`:""}</option>)}</select></label><button disabled={blocked||!customSkill||step?.stage==="finished"} onClick={()=>void launch("custom")}>Lancer la skill choisie</button></details>}
 {!!taskPullRequests(task).length&&<div className="workflow-pr-links"><strong>PR/MR</strong>{taskPullRequests(task).map(link=><button type="button" key={link.url} onClick={()=>void openExternalLink(link.url).catch(()=>setError("Impossible d’ouvrir le lien de PR / MR."))}>{pullRequestLabel(link)} · {pullRequestStateLabel(link)}</button>)}</div>}
 {step?.stage==="implemented"&&task?.prUrl&&<details className="manual-review"><summary>Déclarer la revue validée</summary><p>Cette action enregistre explicitement l’étape « Revu » sur le serveur. Elle ne lance aucune skill et ne fusionne pas la PR.</p><label>Justification<textarea value={reviewNote} onChange={e=>setReviewNote(e.target.value)} maxLength={1000}/></label><button disabled={blocked||!reviewNote.trim()} onClick={()=>{if(acting.current)return;acting.current=true;setPending(true);void declareReviewed(projectId,taskId,reviewNote).then(()=>refresh()).catch(()=>setError("Validation non confirmée. Actualisez le ticket avant de réessayer.")).finally(()=>{acting.current=false;setPending(false);});}}>Enregistrer la revue validée</button></details>}
 {!!runs.length&&<label>Exécution<select aria-label="Exécution du ticket" value={run?.id??""} onChange={e=>setSelected(e.target.value)}>{runs.map(r=><option key={r.id} value={r.id}>{skillLabels[r.skill]||r.skill} · {statusLabel(r)} · {new Date(r.createdAt).toLocaleString("fr")}</option>)}</select></label>}
 {run&&<section className="workflow-run"><div className="run-heading"><h3>{skillLabels[run.skill]||run.skill} · {statusLabel(run)}</h3>{isActive(run)&&<button disabled={pending||!monitor.fresh||run.cancelRequested} onClick={()=>{void stopRun(run.id).then(()=>monitor.refresh()).catch(()=>setError("Arrêt non confirmé. Actualisez le suivi."));}}>{run.cancelRequested?"Arrêt demandé…":"Arrêter l’exécution"}</button>}</div><p role="status">{skillResultLabel(run,result)}</p><p className="muted">Une fin de processus ne valide pas une étape : les rapports du serveur font foi.</p><RunReview key={run.id} run={run}/><TaskOutput projectId={projectId} run={run} task={task} board={project?.board} onExecution={onExecution} onArchive={onArchiveConversation?()=>onArchiveConversation(run.id):undefined} onChanged={()=>void monitor.refresh()}/></section>}
 </section>;
}
function TaskOutput({projectId,run,task,board,onExecution,onArchive,onChanged}:{projectId:string;run:Run;task?:Task;board?:ProjectContext["board"];onExecution?:(runId:string)=>void;onArchive?:()=>void;onChanged:()=>void}){
 return run.conversation?<Suspense fallback={<p>Ouverture de la conversation…</p>}><ProjectPrompt projectId={projectId} runId={run.id} run={run} task={task} board={board} directory={run.directory} folders={run.folders} onCreated={()=>{}} onOpenExecution={onExecution?()=>onExecution(run.id):undefined} onOpenQueue={onExecution?()=>onExecution(""):undefined} onArchive={onArchive} onStopped={onChanged} onFoldersChanged={onChanged}/></Suspense>:run.sessionId||run.trace||run.restored?<Suspense fallback={<p>Ouverture de la console…</p>}><RunConsole runId={run.id}/></Suspense>:<p>La console sera disponible après la prise en charge par Go.</p>;
}
