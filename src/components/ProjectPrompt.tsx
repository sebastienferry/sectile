import { useEffect, useRef, useState } from "react";
import { conversationFor, useConversation } from "../lib/conversation-controller";
import { createConversation, renameConversation, type ConversationAction, type ConversationSnapshot, type TurnOptions } from "../lib/conversation";

import { conversationProfiles, type ProjectProfiles } from "../lib/profiles";
import { PromptFolders } from "./FolderContext";
import type { Run } from "../domain/execution";
import { statusLabel } from "../domain/execution";
import { isActive } from "../domain/execution";
import { stopRun } from "../lib/agent";
import type { Task } from "../domain/board";
import { skillLabels, stageLabels, taskStage } from "../domain/workflow";
import type { BoardMapping } from "../lib/workspace";
import { Icon } from "./Icon";
import { TurnSettings } from "./TurnSettings";
import { QuestionForm } from "./QuestionForm";
import { ConversationTranscript } from "./ConversationTranscript";
import { invoke } from "@tauri-apps/api/core";
import { openExternalLink } from "../lib/workspace";
import { pullRequestLabel, pullRequestStateLabel, taskPullRequests } from "../domain/pull-requests";

function sendOnEnter(event: React.KeyboardEvent<HTMLTextAreaElement>) {
 if(event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {event.preventDefault();event.currentTarget.form?.requestSubmit();}
}
interface ProjectPromptProps {
  projectId:string; runId?:string; directory?:string; folders?:Run["folders"];
  run?:Run; task?:Task; board?:BoardMapping;
  onCreated:(id:string)=>void; onTitleChanged?:()=>void; onFoldersChanged?:()=>void;
  onOpenExecution?:()=>void; onOpenQueue?:()=>void; onOpenTask?:()=>void;
  onArchive?:()=>void; onStopped?:()=>void;
}
export default function ProjectPrompt({ projectId, runId, onCreated, onTitleChanged, directory, folders, onFoldersChanged, run, task, board, onOpenExecution, onOpenQueue, onOpenTask, onArchive, onStopped }: ProjectPromptProps) {
  const [provider, setProvider] = useState("claude");
  const [options, setOptions] = useState<TurnOptions>({});
  const [draft, setDraft] = useState("");
  const [busy, setBusy] = useState(false);
  const [attaching,setAttaching]=useState(false);
  const [error, setError] = useState("");
  const [profiles, setProfiles] = useState<ProjectProfiles>();
  const [profileId, setProfileId] = useState("");
  const customized = useRef(false);
  const mounted = useRef(false);
  useEffect(() => { mounted.current=true; return () => { mounted.current=false; }; }, []);
  useEffect(() => {
    if(runId) return;
    let current=true;
    customized.current=false; setProfiles(undefined); setProfileId(""); setProvider("claude"); setOptions({});
    void conversationProfiles(projectId).then(data=>{
      if(!current || !data || !Array.isArray(data.profiles))return;
      setProfiles(data);
      const profile=data.profiles.find(p=>p.id===data.defaultId);
      if(profile&&!customized.current){setProfileId(profile.id);setProvider(profile.provider);setOptions({model:profile.model||undefined,mode:profile.mode||undefined});}
    }).catch(()=>{});
    return()=>{current=false;};
  },[projectId,runId]);
  if (runId) return <Conversation id={runId} projectId={projectId} directory={directory} folders={folders} run={run} task={task} board={board} onOpenExecution={onOpenExecution} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} onArchive={onArchive} onStopped={onStopped} onFoldersChanged={onFoldersChanged} onTitleChanged={onTitleChanged} />;
  return <section className="project-prompt prompt-start">
    <div className="prompt-intro"><span className="prompt-symbol"><Icon name="chat" /></span><span className="eyebrow">DANS CE PROJET</span>
    <h2>Que voulez-vous faire ?</h2><p>Une idée, une question, une tâche.<br />Commencez simplement par un message.</p></div>
    <form className="composer" onSubmit={async e => {
      e.preventDefault(); if (busy || attaching || !draft.trim()) return;
      setBusy(true); setError("");
      try {
        const created = profileId ? await createConversation(projectId, provider, profileId) : await createConversation(projectId, provider);
        void conversationFor(created.id).send(draft, true, options);
        if (mounted.current) onCreated(created.id);
      } catch (cause) { setError(String(cause)); }
      finally { setBusy(false); }
    }}>
      <textarea aria-label="Prompt libre" value={draft} disabled={busy} maxLength={32000} onChange={e => setDraft(e.target.value)} onKeyDown={sendOnEnter} placeholder="Demandez quelque chose à propos de ce projet…" rows={4} />
      <div className="composer-footer"><div className="composer-options">
      {profiles?.profiles.length ? <label><span className="composer-option-label">Profil</span><select aria-label="Profil" value={profileId} disabled={busy} onChange={e=>{customized.current=true;setProfileId(e.target.value);const profile=profiles.profiles.find(p=>p.id===e.target.value);if(profile){setProvider(profile.provider);setOptions({model:profile.model||undefined,mode:profile.mode||undefined});}}}><option value="">Configuration personnalisée</option>{profiles.profiles.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label> : null}
      <label className="provider-select"><span className="composer-option-label">Moteur</span><select aria-label="Moteur" value={provider} disabled={busy} onChange={e => { customized.current=true; setProfileId(""); setProvider(e.target.value); setOptions({}); }}><option value="claude">Claude Code</option><option value="codex">Codex</option></select></label>
      <TurnSettings compact provider={provider} value={options} onChange={value=>{customized.current=true;setOptions(value);}} disabled={busy} />
      <PromptFolders compact projectId={projectId} directory={directory} onChanged={onFoldersChanged} onPendingChange={setAttaching}/>
      <button type="button" className="composer-action" disabled={!directory} title="Ouvrir le terminal dans le dossier de travail" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setError("Impossible d’ouvrir le terminal dans ce dossier."))}><Icon name="terminal"/>Terminal</button>
      </div><button className="send-button" aria-label="Envoyer" title="Envoyer · Entrée" disabled={busy || attaching || !draft.trim()}><Icon name="send" /></button></div>
    </form>
    <div className="prompt-suggestions" aria-label="Idées de prompts">{[
     ["Comprendre le projet", "Explique la structure de ce projet et les points importants pour commencer."],
     ["Examiner un problème", "Aide-moi à comprendre et résoudre un problème dans ce projet : "],
     ["Préparer un plan", "Propose un plan pour améliorer ce projet, sans modifier de fichiers pour le moment."],
    ].map(([label,text])=><button key={label} onClick={()=>{customized.current=true;setDraft(text);if(label==="Préparer un plan")setOptions(previous=>({...previous,mode:"plan"}));}}>{label}<Icon name="chevron" /></button>)}</div>
    <p className="composer-shortcut">Entrée pour envoyer · Maj + Entrée pour une nouvelle ligne</p>
    {error && <p role="alert" className="error">{error}</p>}
  </section>;
}
function Conversation({ id, projectId, directory, folders, run, task, board, onOpenExecution, onOpenQueue, onOpenTask, onArchive, onStopped, onFoldersChanged, onTitleChanged }: { id:string;projectId:string;directory?:string;folders?:Run["folders"];run?:Run;task?:Task;board?:BoardMapping;onOpenExecution?:()=>void;onOpenQueue?:()=>void;onOpenTask?:()=>void;onArchive?:()=>void;onStopped?:()=>void;onFoldersChanged?:()=>void;onTitleChanged?:()=>void }) {
  const { controller, snapshot, loaded, pending, error } = useConversation(id);
  const [draft, setDraft] = useState("");
  const [actionError, setActionError] = useState("");
  const [mcpFeedback,setMcpFeedback]=useState("");
  const [acting, setActing] = useState(false);
  const [attaching,setAttaching]=useState(false);
  const [followRequest, setFollowRequest] = useState(0);
  const [options, setOptions] = useState<TurnOptions>({});
  useEffect(() => { void controller.refresh(); }, [controller]);
  const [renaming,setRenaming] = useState(false);
  const [title,setTitle] = useState("");
  const [savingTitle,setSavingTitle] = useState(false);
  const [stopping,setStopping] = useState(false);
  const [linkError,setLinkError] = useState("");
  const busy = pending || snapshot.busy;
  async function control(input: Exclude<ConversationAction, { action: "message" }>) {
    if (acting) throw new Error("Une action est déjà en cours.");
    setActing(true); setActionError("");
    try { await controller.control(input); }
    catch (error) { if (input.action !== "answer") setActionError(String(error)); throw error; }
    finally { setActing(false); }
  }
  async function stopConversation() {
    if(stopping || !run || !isActive(run))return;
    setStopping(true);setActionError("");
    try {await stopRun(id);await controller.refresh();onStopped?.();}
    catch {setActionError("Arrêt non confirmé. Actualisez le suivi avant de réessayer.");}
    finally {setStopping(false);}
  }
  return <section className="project-prompt conversation-prompt">
    <header className="prompt-heading"><Icon name="chat" /><h2>{snapshot.title || (snapshot.provider === "codex" ? "Codex" : snapshot.provider === "claude" ? "Claude Code" : "Conversation du projet")}</h2>
      <span role="status">{error ? "Suivi interrompu" : busy ? "En cours…" : snapshot.readOnly ? "Lecture seule" : "Prêt"}</span>
      {snapshot.canAdoptConnection && <button disabled={acting || !!error} title="Cet historique n’est pas encore lié à la connexion sélectionnée." onClick={() => void control({ action: "adoptConnection" }).catch(() => {})}>Associer à cette connexion</button>}
      {busy && !snapshot.readOnly && <button disabled={acting} onClick={() => void control({ action: "interrupt" }).catch(() => {})}>Interrompre</button>}
      <button className="icon-button" aria-label="Actualiser le suivi" title="Actualiser le suivi" disabled={pending || (!!snapshot.busy && !error)} onClick={() => void controller.refresh()}><Icon name="refresh" /></button>
      {(snapshot.canRename||onOpenExecution||run&&isActive(run)||onArchive)&&<details className="conversation-header-menu"><summary aria-label="Actions de la discussion" title="Actions de la discussion">•••</summary><div className="conversation-header-panel">
        {snapshot.canRename&&<button type="button" aria-label="Renommer la conversation" onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");setTitle(snapshot.title||"");setRenaming(true);}}>Renommer</button>}
        {onOpenExecution&&<button type="button" onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");onOpenExecution();}}>Voir l’exécution</button>}
        {run&&isActive(run)&&<button type="button" disabled={stopping||!!run.cancelRequested} onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");void stopConversation();}}>{stopping?"Arrêt…":"Arrêter la discussion"}</button>}
        {onArchive&&<button type="button" disabled={!!run&&isActive(run)} title={run&&isActive(run)?"Arrêtez la discussion avant de l’archiver":undefined} onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");onArchive();}}>Archiver la discussion</button>}
      </div></details>}
    </header>
    {renaming&&<form className="conversation-rename" onSubmit={async e=>{e.preventDefault();if(savingTitle||!title.trim())return;setSavingTitle(true);setActionError("");try{await renameConversation(id,title.trim());await controller.refresh();onTitleChanged?.();setRenaming(false);}catch(cause){setActionError(String(cause));}finally{setSavingTitle(false);}}}><label>Nom de la conversation<input autoFocus required maxLength={80} value={title} onChange={e=>setTitle(e.target.value)}/></label><button type="button" disabled={savingTitle} onClick={()=>setRenaming(false)}>Annuler</button><button disabled={savingTitle||!title.trim()}>Enregistrer le nom</button></form>}
    {snapshot.persistenceError && <p className="error" role="alert">{snapshot.persistenceError}</p>}
    {linkError&&<p role="alert" className="error">{linkError}</p>}
    <ConversationTranscript snapshot={snapshot} followRequest={followRequest} />
    <div className="conversation-controls">
    <div className="conversation-interactions">
    {snapshot.approvals?.map(approval => approval.tool === "AskUserQuestion" ? <QuestionForm key={approval.id + JSON.stringify(approval.input)} approval={approval} disabled={acting || !!error || !!snapshot.readOnly} onAnswer={answers => control({ action: "answer", id: approval.id, answers })} onSkip={() => control({ action: "approval", id: approval.id, decision: "deny" })} /> : <section className="prompt-approval" key={approval.id}>
      <strong>Autorisation requise · {approval.tool}</strong><p>{approval.description || approval.reason}</p>
      <pre>{JSON.stringify(approval.input, null, 2)}</pre>
      <button disabled={acting || !!error || !!snapshot.readOnly} onClick={() => void control({ action: "approval", id: approval.id, decision: "allow" }).catch(() => {})}>Autoriser une fois</button>
      <button disabled={acting || !!error} onClick={() => void control({ action: "approval", id: approval.id, decision: "deny" }).catch(() => {})}>Refuser</button>
    </section>)}
    {(error || actionError) && <p role="alert" className="error">{error || actionError} Vérifiez le suivi avant de renvoyer un message.</p>}
    </div>
    <ConversationStatus snapshot={snapshot} run={run} task={task} board={board} busy={busy} error={!!error} onOpenExecution={onOpenExecution} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} onOpenPr={url=>void openExternalLink(url).catch(()=>setLinkError("Impossible d’ouvrir la PR/MR."))}/>
    <form className="composer" onSubmit={e => { e.preventDefault(); if (loaded && !busy && !attaching && !snapshot.readOnly && !error && draft.trim()) { void controller.send(draft, false, options); setDraft(""); setFollowRequest(value => value + 1); } }}>
      <textarea aria-label="Message suivant" rows={3} maxLength={32000} value={draft} onChange={e => setDraft(e.target.value)} onKeyDown={sendOnEnter} disabled={!!snapshot.readOnly} placeholder="Posez une question ou décrivez une modification…" />
      <div className="composer-footer"><TurnSettings compact provider={snapshot.provider} snapshot={snapshot} value={options} onChange={setOptions} disabled={!loaded || busy || !!snapshot.readOnly || !!error || acting} />
      <PromptFolders compact projectId={projectId} runId={id} directory={directory} folders={folders} readOnly={!loaded||snapshot.readOnly} onChanged={onFoldersChanged} onPendingChange={setAttaching}/>
      <button type="button" className="composer-action" disabled={!directory} title="Ouvrir le terminal dans le dossier de travail" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setActionError("Impossible d’ouvrir le terminal dans ce dossier."))}><Icon name="terminal"/>Terminal</button>
      <button type="button" className={`composer-action mcp-action mcp-${snapshot.sectileMcp?.status??"unknown"}`} disabled={!loaded||acting||snapshot.checkingMcp||snapshot.readOnly} aria-label="Vérifier le MCP de cette conversation" title={`${snapshot.sectileMcp?.source==="transport"?"Transport MCP du compagnon":"MCP du moteur"} · ${snapshot.checkingMcp?"Vérification en cours…":({connected:"Connecté",missing:"Non enregistré",failed:"Échec de connexion","needs-auth":"Connexion requise",pending:"Connexion en cours",unknown:"État non vérifié"} as Record<string,string>)[snapshot.sectileMcp?.status??"unknown"]||"État non vérifié"}`} onClick={()=>void control({action:"checkMcp"}).then(()=>{const current=controller.getSnapshot().snapshot;setMcpFeedback(current.provider==="codex"&&!current.checkingMcp&&current.sectileMcp?.status==="unknown"?"Codex vérifiera le MCP au prochain message, après la reconnexion de sa session.":"");}).catch(()=>{})}><span className="mcp-dot"/>MCP</button>
      <button className="send-button" aria-label="Envoyer" title="Envoyer · Entrée" disabled={!loaded || busy || attaching || !!snapshot.readOnly || !!error || !draft.trim()}><Icon name="send" /></button></div>
      {mcpFeedback&&snapshot.sectileMcp?.status==="unknown"&&<p className="composer-feedback" role="status">{mcpFeedback}</p>}
    </form>
    </div>
  </section>;
}
function ConversationStatus({snapshot,run,task,board,busy,error,onOpenExecution,onOpenQueue,onOpenTask,onOpenPr}:{snapshot:ConversationSnapshot;run?:Run;task?:Task;board?:BoardMapping;busy:boolean;error:boolean;onOpenExecution?:()=>void;onOpenQueue?:()=>void;onOpenTask?:()=>void;onOpenPr:(url:string)=>void}) {
  const stage=task?taskStage(task,board):undefined;
  const links=taskPullRequests(task);
  const activity=error?"Suivi interrompu":snapshot.approvals?.length?"Votre réponse attendue":busy?"Réponse en cours":snapshot.readOnly?"Historique en lecture seule":"Prêt à répondre";
  const execution=run?busy&&run.conversation?"Réponse en cours":statusLabel(run):"Chargement";
  return <div className="conversation-status-strip" aria-label="Suivi de la conversation">
    <span className={`conversation-status-activity ${busy?"live":""}`}><span className="conversation-status-dot"/>{activity}</span>
    <details className="conversation-execution-menu"><summary>Exécution : {execution}</summary><div className="conversation-execution-panel">
      {onOpenExecution&&<button type="button" onClick={onOpenExecution}>Voir cette exécution</button>}
      {onOpenQueue&&<button type="button" onClick={onOpenQueue}>Voir la file des exécutions</button>}
      {onOpenTask&&<button type="button" onClick={onOpenTask}>Ouvrir le workflow du ticket</button>}
    </div></details>
    {run?.skill&&<span>Skill : {skillLabels[run.skill]||run.skill}</span>}
    {run?.taskId&&<span>Ticket : <strong>{task?.key||run.taskKey||run.taskId}</strong> · {task?.trackerStatus||"statut indisponible"}</span>}
    {run?.taskId&&(links.length>1?<details className="conversation-pr-menu"><summary>PR/MR : {links.length} liées</summary><div className="conversation-pr-panel">{links.map(link=><button type="button" key={link.url} onClick={()=>onOpenPr(link.url)}><Icon name="branch"/>{pullRequestLabel(link)} · {pullRequestStateLabel(link)}</button>)}</div></details>:<span>PR/MR : {links.length?<button type="button" className="conversation-pr-link" onClick={()=>onOpenPr(links[0].url)}><Icon name="branch"/>{pullRequestLabel(links[0])} · {pullRequestStateLabel(links[0])}</button>:"aucune"}</span>)}
    {run?.taskId&&<span>Étape : {stage?stageLabels[stage]:"inconnue"}</span>}
  </div>;
}
