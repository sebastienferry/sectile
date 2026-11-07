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
import { conversationTitle } from "../domain/conversation-navigation";

function sendOnEnter(event: React.KeyboardEvent<HTMLTextAreaElement>) {
 if(event.key === "Enter" && !event.shiftKey && !event.nativeEvent.isComposing) {event.preventDefault();event.currentTarget.form?.requestSubmit();}
}
interface ProjectPromptProps {
  projectId:string; runId?:string; directory?:string; folders?:Run["folders"];
  agentAvailable?:boolean;
  run?:Run; task?:Task; board?:BoardMapping;
  onCreated:(id:string)=>void; onTitleChanged?:()=>void; onFoldersChanged?:()=>void;
  onOpenQueue?:()=>void; onOpenTask?:()=>void;
  onArchive?:()=>void; onStopped?:()=>void;
}
export default function ProjectPrompt({ projectId, runId, agentAvailable=true, onCreated, onTitleChanged, directory, folders, onFoldersChanged, run, task, board, onOpenQueue, onOpenTask, onArchive, onStopped }: ProjectPromptProps) {
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
  const selectedProfile=profiles?.profiles.find(profile=>profile.id===profileId);
  const formatProfileLabel = (profile: { provider: string; name: string }) => {
    const providerLabel = profile.provider === "codex" ? "Codex" : "Claude Code";
    const name = profile.name.trim();
    if (!name || name.toLowerCase() === providerLabel.toLowerCase() || name.toLowerCase() === profile.provider.toLowerCase()) {
      return providerLabel;
    }
    return `${providerLabel} · ${name}`;
  };
  const engineChoices = [
    ...(profiles?.profiles.map(profile => ({
      key: `profile:${profile.id}`,
      label: formatProfileLabel(profile),
      provider: (profile.provider === "codex" ? "codex" : "claude") as "claude" | "codex",
    })) ?? []),
    { key: "engine:claude", label: "Claude Code", provider: "claude" as const },
    { key: "engine:codex", label: "Codex", provider: "codex" as const },
  ];
  function selectEngine(val: string) {
    customized.current = true;
    if (val.startsWith("profile:")) {
      const profile = profiles?.profiles.find(item => item.id === val.slice(8));
      if (profile) {
        setProfileId(profile.id);
        setProvider(profile.provider);
        setOptions({ model: profile.model || undefined, mode: profile.mode || undefined });
      }
    } else {
      setProfileId("");
      setProvider(val.slice(7));
      setOptions({});
    }
  }
  if (runId) return <Conversation id={runId} projectId={projectId} agentAvailable={agentAvailable} directory={directory} folders={folders} run={run} task={task} board={board} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} onArchive={onArchive} onStopped={onStopped} onFoldersChanged={onFoldersChanged} onTitleChanged={onTitleChanged} />;
  return <section className="project-prompt prompt-start prompt-new">
    <div className="prompt-intro"><h2>Nouveau prompt</h2></div>
    <form className="composer" onSubmit={async e => {
      e.preventDefault(); if (!agentAvailable || busy || attaching || !draft.trim()) return;
      setBusy(true); setError("");
      try {
        const created = profileId ? await createConversation(projectId, provider, profileId) : await createConversation(projectId, provider);
        void conversationFor(created.id).send(draft, true, options);
        if (mounted.current) onCreated(created.id);
      } catch (cause) { setError(String(cause)); }
      finally { setBusy(false); }
    }}>
      <div className="composer-nav-row">
        <div className="composer-nav-actions">
          <PromptFolders compact projectId={projectId} directory={directory} onChanged={onFoldersChanged} onPendingChange={setAttaching}/>
          <button type="button" className="composer-nav-btn" aria-label="Terminal" disabled={!directory} title="Ouvrir le terminal dans le dossier de travail" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setError("Impossible d’ouvrir le terminal dans ce dossier."))}><Icon name="terminal"/><span className="sr-only">Terminal</span></button>
        </div>
      </div>
      <div className="composer-box">
        <textarea autoFocus aria-label="Prompt libre" value={draft} disabled={busy} maxLength={32000} onChange={e => setDraft(e.target.value)} onKeyDown={sendOnEnter} placeholder="Demandez quelque chose à propos de ce projet…" rows={2} />
        <div className="composer-footer"><div className="composer-options">
          <label className="sr-only"><span className="sr-only">Profil ou moteur</span><select tabIndex={-1} aria-label="Profil ou moteur" title={selectedProfile?`${selectedProfile.provider==="codex"?"Codex":"Claude Code"} · ${selectedProfile.name}`:"Choisir un profil ou un moteur"} value={profileId?`profile:${profileId}`:`engine:${provider}`} disabled={busy} onChange={e => selectEngine(e.target.value)}>
            {profiles?.profiles.length?<optgroup label="Profils du projet">{profiles.profiles.map(profile=><option key={profile.id} value={`profile:${profile.id}`}>{profile.provider==="codex"?"Codex":"Claude Code"} · {profile.name}</option>)}</optgroup>:null}
            <optgroup label="Sans profil"><option value="engine:claude">Claude Code</option><option value="engine:codex">Codex</option></optgroup>
          </select></label>
          <TurnSettings
            compact
            showProviderMark
            provider={provider}
            defaultModel={selectedProfile?.model}
            value={options}
            onChange={value => { customized.current = true; setOptions(value); }}
            disabled={busy}
            engineChoices={engineChoices}
            selectedEngineKey={profileId ? `profile:${profileId}` : `engine:${provider}`}
            onSelectEngine={selectEngine}
          />
        </div><button className="send-button" aria-label="Envoyer" title="Envoyer · Entrée" disabled={!agentAvailable || busy || attaching || !draft.trim()}><Icon name="send" /></button></div>
      </div>
    </form>
    {!agentAvailable&&<p role="status" className="composer-feedback">Agent indisponible. Le message pourra être envoyé après la reconnexion.</p>}
    <div className="prompt-suggestions" aria-label="Idées de prompts">{[
     ["Comprendre le projet", "Explique la structure de ce projet et les points importants pour commencer."],
     ["Examiner un problème", "Aide-moi à comprendre et résoudre un problème dans ce projet : "],
     ["Préparer un plan", "Propose un plan pour améliorer ce projet, sans modifier de fichiers pour le moment."],
    ].map(([label,text])=><button key={label} onClick={()=>{customized.current=true;setDraft(text);if(label==="Préparer un plan")setOptions(previous=>({...previous,mode:"plan"}));}}>{label}<Icon name="chevron" /></button>)}</div>
    <p className="composer-shortcut">Entrée pour envoyer · Maj + Entrée pour une nouvelle ligne</p>
    {error && <p role="alert" className="error">{error}</p>}
  </section>;
}
function Conversation({ id, projectId, agentAvailable, directory, folders, run, task, board, onOpenQueue, onOpenTask, onArchive, onStopped, onFoldersChanged, onTitleChanged }: { id:string;projectId:string;agentAvailable:boolean;directory?:string;folders?:Run["folders"];run?:Run;task?:Task;board?:BoardMapping;onOpenQueue?:()=>void;onOpenTask?:()=>void;onArchive?:()=>void;onStopped?:()=>void;onFoldersChanged?:()=>void;onTitleChanged?:()=>void }) {
  const { controller, snapshot, loaded, pending, error } = useConversation(id);
  const [draft, setDraft] = useState("");
  const [actionError, setActionError] = useState("");
  const [mcpFeedback,setMcpFeedback]=useState("");
  const [acting, setActing] = useState(false);
  const [attaching,setAttaching]=useState(false);
  const [followRequest, setFollowRequest] = useState(0);
  const [options, setOptions] = useState<TurnOptions>({});
  const availableBefore = useRef(agentAvailable);
  useEffect(() => { void controller.refresh(); }, [controller]);
  useEffect(() => {
    if(agentAvailable && !availableBefore.current) void controller.refresh();
    availableBefore.current=agentAvailable;
  }, [agentAvailable,controller]);
  const [renaming,setRenaming] = useState(false);
  const [title,setTitle] = useState("");
  const [savingTitle,setSavingTitle] = useState(false);
  const [stopping,setStopping] = useState(false);
  const [linkError,setLinkError] = useState("");
  const [pathCopyState,setPathCopyState] = useState("");
  useEffect(()=>setPathCopyState(""),[directory]);
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
  const displayTitle = conversationTitle(snapshot, run, task);
  const taskKey = run?.taskKey || task?.key;
  return <section className="project-prompt conversation-prompt">
    <header className="workspace-toolbar prompt-heading">
      <div className="workspace-toolbar-main">
        <h2 title={displayTitle} style={{cursor:snapshot.canRename?"pointer":"default"}} onClick={()=>{if(snapshot.canRename){setTitle(snapshot.title||displayTitle||"");setRenaming(true);}}}>
          <Icon name="chat" />
          {taskKey && <span role={onOpenTask ? "button" : undefined} tabIndex={onOpenTask ? 0 : undefined} className="sidebar-task-key" title={onOpenTask ? `Ouvrir le ticket ${taskKey}` : undefined} aria-label={onOpenTask ? `Ouvrir le ticket ${taskKey}` : undefined} onClick={e=>{if(onOpenTask){e.stopPropagation();onOpenTask();}}} onKeyDown={e=>{if(onOpenTask && (e.key==="Enter"||e.key===" ")){e.preventDefault();e.stopPropagation();onOpenTask();}}}>{taskKey}</span>}
          <span>{displayTitle}</span>
        </h2>
        <div className="workspace-path">
          <p title={directory}>{directory || "Espace de travail de la conversation"}</p>
          {directory && <button type="button" className="copy-path" title="Copier le chemin du dossier de travail" aria-label="Copier le chemin du dossier de travail" onClick={()=>void navigator.clipboard.writeText(directory).then(()=>setPathCopyState("Copié"),()=>setPathCopyState("Copie impossible"))}>{pathCopyState||"Copier"}</button>}
        </div>
      </div>
      <div className="toolbar-actions">
        {snapshot.canAdoptConnection && <button disabled={!agentAvailable || acting || !!error} title="Cet historique n’est pas encore lié à la connexion sélectionnée." onClick={() => void control({ action: "adoptConnection" }).catch(() => {})}>Associer à cette connexion</button>}
        {snapshot.canRename && <button type="button" className="icon-button" aria-label="Renommer la conversation" title="Renommer la conversation" onClick={()=>{setTitle(snapshot.title||displayTitle||"");setRenaming(true);}}><Icon name="edit" /></button>}
        {run && isActive(run) ? (
          <button type="button" className="icon-button stop-button" aria-label="Arrêter la discussion" title={stopping ? "Arrêt en cours…" : "Arrêter la discussion"} disabled={!agentAvailable || stopping || !!run.cancelRequested} onClick={() => void stopConversation()}><Icon name="stop" /></button>
        ) : onArchive ? (
          <button type="button" className="icon-button" aria-label="Archiver la discussion" title="Archiver la discussion" onClick={onArchive}><Icon name="archive" /></button>
        ) : null}
      </div>
    </header>
    {renaming&&<form className="conversation-rename" onSubmit={async e=>{e.preventDefault();if(savingTitle||!title.trim())return;setSavingTitle(true);setActionError("");try{await renameConversation(id,title.trim());await controller.refresh();onTitleChanged?.();setRenaming(false);}catch(cause){setActionError(String(cause));}finally{setSavingTitle(false);}}}><label>Nom de la conversation<input autoFocus required maxLength={80} value={title} onChange={e=>setTitle(e.target.value)}/></label><button type="button" disabled={savingTitle} onClick={()=>setRenaming(false)}>Annuler</button><button disabled={savingTitle||!title.trim()}>Enregistrer le nom</button></form>}
    {snapshot.persistenceError && <p className="error" role="alert">{snapshot.persistenceError}</p>}
    {linkError&&<p role="alert" className="error">{linkError}</p>}
    <ConversationTranscript
      snapshot={snapshot}
      followRequest={followRequest}
      onInterrupt={busy && !snapshot.readOnly && agentAvailable && !acting ? () => void control({ action: "interrupt" }).catch(() => {}) : undefined}
    />
    <div className="conversation-controls">
    <div className="conversation-interactions">
    {snapshot.approvals?.map(approval => {
      if (approval.tool === "AskUserQuestion") {
        return <QuestionForm key={approval.id + JSON.stringify(approval.input)} approval={approval} disabled={!agentAvailable || acting || !!error || !!snapshot.readOnly} onAnswer={answers => control({ action: "answer", id: approval.id, answers })} onSkip={() => control({ action: "approval", id: approval.id, decision: "deny" })} />;
      }
      const hasSuggestions = Array.isArray(approval.suggestions) && approval.suggestions.length > 0;
      const choices: { decision: "allow" | "always" | "deny"; label: string }[] = Array.isArray(approval.choices) && approval.choices.length > 0
        ? approval.choices.map(choice => ({
            decision: choice.decision,
            label: choice.decision === "allow"
              ? "Autoriser une fois"
              : choice.decision === "always"
              ? "Toujours autoriser"
              : choice.decision === "deny"
              ? "Refuser"
              : choice.label,
          }))
        : [
            { decision: "allow", label: "Autoriser une fois" },
            ...(hasSuggestions ? [{ decision: "always" as const, label: "Toujours autoriser" }] : []),
            { decision: "deny", label: "Refuser" },
          ];
      return <section className="prompt-approval" key={approval.id}>
        <strong>Autorisation requise · {approval.tool}</strong><p>{approval.description || approval.reason}</p>
        <pre>{JSON.stringify(approval.input, null, 2)}</pre>
        {hasSuggestions && (
          <details className="prompt-approval-suggestions">
            <summary>Toujours autoriser : règle proposée pour ce projet</summary>
            <pre>{JSON.stringify(approval.suggestions?.map(update => ({
              ...update,
              destination: (update?.type === "addRules" && update?.behavior === "allow") || update?.type === "addDirectories"
                ? "project (this workstation)"
                : "session",
            })), null, 2)}</pre>
          </details>
        )}
        <div className="prompt-approval-actions">
          {choices.map(choice => (
            <button
              key={choice.decision}
              disabled={!agentAvailable || acting || !!error || (choice.decision !== "deny" && !!snapshot.readOnly)}
              onClick={() => void control({ action: "approval", id: approval.id, decision: choice.decision }).catch(() => {})}
            >
              {choice.label}
            </button>
          ))}
        </div>
      </section>;
    })}
    {agentAvailable && (error || actionError) && <p role="alert" className="error">{error || actionError} Vérifiez le suivi avant de renvoyer un message. <button type="button" aria-label="Actualiser le suivi" onClick={() => void controller.refresh()}>Réessayer</button></p>}
    </div>
    {snapshot.readOnly ? <>
      <ConversationStatus snapshot={snapshot} run={run} task={task} board={board} busy={busy} error={!!error} agentAvailable={agentAvailable} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} onOpenPr={url=>void openExternalLink(url).catch(()=>setLinkError("Impossible d’ouvrir la PR/MR."))}/>
      <div className="conversation-readonly-footer" aria-label="Options de l’historique">
        <PromptFolders compact projectId={projectId} runId={id} directory={directory} folders={folders} readOnly onChanged={onFoldersChanged}/>
        <button type="button" className="composer-action" disabled={!directory} title="Ouvrir le terminal dans le dossier de travail" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setActionError("Impossible d’ouvrir le terminal dans ce dossier."))}><Icon name="terminal"/>Terminal</button>
      </div>
    </> : <form className="composer" onSubmit={e => { e.preventDefault(); if (agentAvailable && loaded && !busy && !attaching && !snapshot.readOnly && !error && draft.trim()) { void controller.send(draft, false, options); setDraft(""); setFollowRequest(value => value + 1); } }}>
      <div className="composer-nav-row">
        <ConversationStatus snapshot={snapshot} run={run} task={task} board={board} busy={busy} error={!!error} agentAvailable={agentAvailable} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} onOpenPr={url=>void openExternalLink(url).catch(()=>setLinkError("Impossible d’ouvrir la PR/MR."))}/>
        <div className="composer-nav-actions">
          <PromptFolders compact projectId={projectId} runId={id} directory={directory} folders={folders} readOnly={!loaded||snapshot.readOnly} onChanged={onFoldersChanged} onPendingChange={setAttaching}/>
          <button type="button" className="composer-nav-btn" aria-label="Terminal" disabled={!directory} title="Ouvrir le terminal dans le dossier de travail" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setActionError("Impossible d’ouvrir le terminal dans ce dossier."))}><Icon name="terminal"/><span className="sr-only">Terminal</span></button>
          <button type="button" className={`composer-nav-btn mcp-action mcp-${snapshot.sectileMcp?.status??"unknown"}`} disabled={!agentAvailable||!loaded||acting||snapshot.checkingMcp||snapshot.readOnly} aria-label="Vérifier le MCP de cette conversation" title={`${snapshot.sectileMcp?.source==="transport"?"Transport MCP du compagnon":"MCP du moteur"} · ${snapshot.checkingMcp?"Vérification en cours…":({connected:"Connecté",missing:"Non enregistré",failed:"Échec de connexion","needs-auth":"Connexion requise",pending:"Connexion en cours",unknown:"État non vérifié"} as Record<string,string>)[snapshot.sectileMcp?.status??"unknown"]||"État non vérifié"}`} onClick={()=>void control({action:"checkMcp"}).then(()=>{const current=controller.getSnapshot().snapshot;setMcpFeedback(current.provider==="codex"&&!current.checkingMcp&&current.sectileMcp?.status==="unknown"?"Codex vérifiera le MCP au prochain message, après la reconnexion de sa session.":"");}).catch(()=>{})}><span className="mcp-dot"/>MCP</button>
        </div>
      </div>
      <div className="composer-box">
        <textarea
          aria-label="Message suivant"
          rows={2}
          maxLength={32000}
          value={draft}
          onChange={e => setDraft(e.target.value)}
          onKeyDown={e => {
            if (e.key === "Escape" && busy && !snapshot.readOnly && agentAvailable && !acting) {
              e.preventDefault();
              void control({ action: "interrupt" }).catch(() => {});
              return;
            }
            sendOnEnter(e);
          }}
          disabled={!!snapshot.readOnly}
          placeholder="Posez une question ou décrivez une modification…"
        />
        <div className="composer-footer"><TurnSettings compact showProviderMark provider={snapshot.provider} snapshot={snapshot} value={options} onChange={setOptions} disabled={!loaded || busy || !!snapshot.readOnly || !!error || acting} />
        {busy && !snapshot.readOnly && (
          <button
            type="button"
            className="send-button interrupt-button"
            aria-label="Interrompre"
            title="Annuler l’exécution en cours · Échap"
            disabled={!agentAvailable || acting}
            onClick={() => void control({ action: "interrupt" }).catch(() => {})}
          >
            <Icon name="stopSquare" />
          </button>
        )}
        <button className={`send-button ${busy && !snapshot.readOnly ? "sr-only" : ""}`} aria-label="Envoyer" title="Envoyer · Entrée" disabled={!agentAvailable || !loaded || busy || attaching || !!snapshot.readOnly || !!error || !draft.trim()}><Icon name="send" /></button></div>
      </div>
      {mcpFeedback&&snapshot.sectileMcp?.status==="unknown"&&<p className="composer-feedback" role="status">{mcpFeedback}</p>}
    </form>}
    </div>
  </section>;
}
function ConversationStatus({snapshot,run,task,board,busy,error,agentAvailable,onOpenQueue,onOpenTask,onOpenPr}:{snapshot:ConversationSnapshot;run?:Run;task?:Task;board?:BoardMapping;busy:boolean;error:boolean;agentAvailable:boolean;onOpenQueue?:()=>void;onOpenTask?:()=>void;onOpenPr:(url:string)=>void}) {
  const stage=task?taskStage(task,board):undefined;
  const links=taskPullRequests(task);
  const activity=!agentAvailable?"Agent indisponible":error?"Suivi interrompu":snapshot.approvals?.length?"Votre réponse attendue":busy?"Réponse en cours":snapshot.readOnly?"Historique en lecture seule":"Prêt à répondre";
  const execution=!agentAvailable?"Suivi indisponible":run?busy&&run.conversation?"Réponse en cours":statusLabel(run):"Chargement";
  return <div className="conversation-status-strip" aria-label="Suivi de la conversation">
    <span className={`conversation-status-activity ${agentAvailable&&busy?"live":""}`}><span className="conversation-status-dot"/>{activity}</span>
    <details className="conversation-execution-menu"><summary>Exécution : {execution}</summary><div className="conversation-execution-panel">
      {onOpenQueue&&<button type="button" onClick={onOpenQueue}>Voir la file des exécutions</button>}
      {onOpenTask&&<button type="button" onClick={onOpenTask}>Ouvrir le workflow du ticket</button>}
    </div></details>
    {run?.skill&&<span>Skill : {skillLabels[run.skill]||run.skill}</span>}
    {run?.taskId&&<span>Ticket : <strong>{task?.key||run.taskKey||run.taskId}</strong> · {task?.trackerStatus||"statut indisponible"}</span>}
    {run?.taskId&&(links.length>1?<details className="conversation-pr-menu"><summary>PR/MR : {links.length} liées</summary><div className="conversation-pr-panel">{links.map(link=><button type="button" key={link.url} onClick={()=>onOpenPr(link.url)}><Icon name="branch"/>{pullRequestLabel(link)} · {pullRequestStateLabel(link)}</button>)}</div></details>:<span>PR/MR : {links.length?<button type="button" className="conversation-pr-link" onClick={()=>onOpenPr(links[0].url)}><Icon name="branch"/>{pullRequestLabel(links[0])} · {pullRequestStateLabel(links[0])}</button>:"aucune"}</span>)}
    {run?.taskId&&<span>Étape : {stage?stageLabels[stage]:"inconnue"}</span>}
  </div>;
}
