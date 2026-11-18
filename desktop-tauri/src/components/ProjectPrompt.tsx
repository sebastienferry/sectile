import { useEffect, useMemo, useRef, useState } from "react";
import { conversationFor, useConversation } from "../lib/conversation-controller";
import { createConversation, renameConversation, type ConversationAction, type ConversationSnapshot, type TurnOptions } from "../lib/conversation";

import { conversationProfiles, type ProjectProfiles } from "../lib/profiles";
import { PromptFolders, type FolderEntry } from "./FolderContext";
import { ContextTray, type ContextTrayItem } from "./ContextTray";
import { ContextUsageMeter } from "./ContextUsageMeter";
import type { Run } from "../domain/execution";
import { statusLabel } from "../domain/execution";
import { isActive } from "../domain/execution";
import { stopRun } from "../lib/agent";
import type { Task } from "../domain/board";
import { nextSkill, skillLabels, stageLabels, stages, taskStage } from "../domain/workflow";
import type { BoardMapping, ProjectContext } from "../lib/workspace";
import { Icon } from "./Icon";
import { Markdown } from "./Markdown";
import { RunReview } from "./RunReview";
import { TaskEditor } from "./TaskEditor";
import { TurnSettings } from "./TurnSettings";
import { QuestionForm } from "./QuestionForm";
import { ConversationTranscript } from "./ConversationTranscript";
import { QueuedMessages, QueueStrip } from "./QueuedMessages";
import { QUEUE_LIMIT, mergeDraft } from "../lib/conversation-queue";
import { invoke } from "@tauri-apps/api/core";
import { openExternalLink } from "../lib/workspace";
import { pullRequestLabel, pullRequestStateLabel, taskPullRequests } from "../domain/pull-requests";
import { skillStatusText, skillStatusTitle } from "../domain/skill-status";
import { useSkillStatus } from "../lib/skill-results";
import { conversationTitle } from "../domain/conversation-navigation";
import { userPrompts } from "../domain/prompt-history";
import { isShellDraft } from "../domain/command-completion";
import { PromptInput, type MentionSources, type PromptInputHandle } from "./PromptInput";
import { decodeEvents } from "../lib/conversation";

/** The context tray shows folders only when there is more than the working folder (#914). */
function folderTrayItems(entries: FolderEntry[], onCopy: (path: string) => void): ContextTrayItem[] {
  if (entries.length < 2) return [];
  return entries.map(entry => ({ key: entry.path, label: entry.name, title: `${entry.label} · ${entry.path} · click to copy`, icon: "folder", onClick: () => onCopy(entry.path) }));
}
const copyPath = (path: string) => void navigator.clipboard?.writeText(path).catch(() => {});
interface ProjectPromptProps {
  projectId:string; projectName?:string; runId?:string; directory?:string; folders?:Run["folders"];
  agentAvailable?:boolean;
  run?:Run; task?:Task; board?:BoardMapping;
  projectContext?:ProjectContext;
  taskRuns?:Run[];
  canEditTask?:boolean;
  knownSprints?:string[];
  knownAssignees?:string[];
  onTaskSaved?:(task:Task)=>void;
  onSelectRun?:(runId:string)=>void;
  onLaunchSkill?:(skillId:string)=>void;
  onEditTask?:()=>void;
  onCreated:(id:string)=>void; onTitleChanged?:()=>void; onFoldersChanged?:()=>void;
  /** What `@` can mention besides files: the project's conversations and tickets (#917). */
  mentionSources?:MentionSources;
  onOpenQueue?:()=>void; onOpenTask?:()=>void;
  onArchive?:()=>void; onStopped?:()=>void;
  /** The conversation is in the archived list: it offers Start over (#906). */
  archived?:boolean; onStartOver?:()=>void;
  /** Opens the archived conversation a run started over from, when still listed. */
  onOpenSource?:(runId:string)=>void;
}
export default function ProjectPrompt({ projectId, projectName, runId, agentAvailable=true, onCreated, onTitleChanged, directory, folders, onFoldersChanged, run, task, board, projectContext, taskRuns, canEditTask, knownSprints, knownAssignees, onTaskSaved, onSelectRun, onLaunchSkill, onEditTask, onOpenQueue, onOpenTask, onArchive, onStopped, archived, onStartOver, onOpenSource, mentionSources }: ProjectPromptProps) {
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
  const promptForm = useRef<HTMLFormElement>(null);
  const mentions = mentionSources ?? { conversations: [], tickets: [] };
  const [folderEntries, setFolderEntries] = useState<FolderEntry[]>([]);
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
  if (runId) return <Conversation id={runId} projectId={projectId} projectName={projectName} agentAvailable={agentAvailable} directory={directory} folders={folders} run={run} task={task} board={board} projectContext={projectContext} taskRuns={taskRuns} canEditTask={canEditTask} knownSprints={knownSprints} knownAssignees={knownAssignees} onTaskSaved={onTaskSaved} onSelectRun={onSelectRun} onLaunchSkill={onLaunchSkill} onEditTask={onEditTask} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} onArchive={onArchive} onStopped={onStopped} onFoldersChanged={onFoldersChanged} onTitleChanged={onTitleChanged} archived={archived} onStartOver={onStartOver} onOpenSource={onOpenSource} mentionSources={mentions} />;
  return <section className="project-prompt prompt-start prompt-new">
    <div className="prompt-intro"><h2>New prompt</h2></div>
    <form ref={promptForm} className="composer" onSubmit={async e => {
      e.preventDefault(); if (!agentAvailable || busy || attaching || !draft.trim()) return;
      setBusy(true); setError("");
      try {
        const created = profileId ? await createConversation(projectId, provider, profileId) : await createConversation(projectId, provider);
        void conversationFor(created.id).send(draft, true, options);
        if (mounted.current) onCreated(created.id);
      } catch (cause) { setError(String(cause)); }
      finally { setBusy(false); }
    }}>
      <div className="composer-box">
        <ContextTray items={folderTrayItems(folderEntries, copyPath)} />
        <PromptInput ariaLabel="Free prompt" viewSelector=".project-prompt" projectId={projectId} value={draft} onChange={setDraft} mentions={mentions} placeholder="Ask something about this project…" readOnly={busy} autoFocus onSubmit={() => promptForm.current?.requestSubmit()} />
        <div className="composer-footer"><div className="composer-options">
          <label className="sr-only"><span className="sr-only">Profile or engine</span><select tabIndex={-1} aria-label="Profile or engine" title={selectedProfile?`${selectedProfile.provider==="codex"?"Codex":"Claude Code"} · ${selectedProfile.name}`:"Choose a profile or engine"} value={profileId?`profile:${profileId}`:`engine:${provider}`} disabled={busy} onChange={e => selectEngine(e.target.value)}>
            {profiles?.profiles.length?<optgroup label="Project profiles">{profiles.profiles.map(profile=><option key={profile.id} value={`profile:${profile.id}`}>{profile.provider==="codex"?"Codex":"Claude Code"} · {profile.name}</option>)}</optgroup>:null}
            <optgroup label="Without profile"><option value="engine:claude">Claude Code</option><option value="engine:codex">Codex</option></optgroup>
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
          <div className="composer-inline-tools">
            <PromptFolders compact projectId={projectId} directory={directory} onChanged={onFoldersChanged} onPendingChange={setAttaching} onEntriesChange={setFolderEntries}/>
            <button type="button" className="composer-nav-btn" aria-label="Terminal" disabled={!directory} title="Open terminal in working folder" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setError("Unable to open terminal in this folder."))}><Icon name="terminal"/><span className="sr-only">Terminal</span></button>
          </div>
        </div><button className="send-button" aria-label="Send" title="Send · Enter" disabled={!agentAvailable || busy || attaching || !draft.trim()}><Icon name="send" /></button></div>
      </div>
    </form>
    {!agentAvailable&&<p role="status" className="composer-feedback">Agent unavailable. Your message can be sent after reconnecting.</p>}
    <div className="prompt-suggestions" aria-label="Prompt ideas">{[
     ["Understand the project", "Explain the structure of this project and key points to get started."],
     ["Investigate an issue", "Help me understand and fix an issue in this project: "],
     ["Prepare a plan", "Propose a plan to improve this project without modifying files for now."],
    ].map(([label,text])=><button key={label} onClick={()=>{customized.current=true;setDraft(text);if(label==="Prepare a plan")setOptions(previous=>({...previous,mode:"plan"}));}}>{label}<Icon name="chevron" /></button>)}</div>
    <p className="composer-shortcut">Enter to send · Shift + Enter for a new line</p>
    {error && <p role="alert" className="error">{error}</p>}
  </section>;
}
function Conversation({ id, projectId, projectName, agentAvailable, directory, folders, run, task, board, projectContext, taskRuns, canEditTask, knownSprints, knownAssignees, onTaskSaved, onSelectRun, onLaunchSkill, onOpenQueue, onOpenTask, onArchive, onStopped, onFoldersChanged, onTitleChanged, archived, onStartOver, onOpenSource, mentionSources }: { mentionSources:MentionSources; id:string;projectId:string;projectName?:string;agentAvailable:boolean;directory?:string;folders?:Run["folders"];run?:Run;task?:Task;board?:BoardMapping;projectContext?:ProjectContext;taskRuns?:Run[];canEditTask?:boolean;knownSprints?:string[];knownAssignees?:string[];onTaskSaved?:(task:Task)=>void;onSelectRun?:(runId:string)=>void;onLaunchSkill?:(skillId:string)=>void;onEditTask?:()=>void;onOpenQueue?:()=>void;onOpenTask?:()=>void;onArchive?:()=>void;onStopped?:()=>void;onFoldersChanged?:()=>void;onTitleChanged?:()=>void;archived?:boolean;onStartOver?:()=>void;onOpenSource?:(runId:string)=>void }) {
  const { controller, snapshot, loaded, pending, error, queue } = useConversation(id);
  const [draft, setDraft] = useState("");
  const [queueFeedback, setQueueFeedback] = useState("");
  const [actionError, setActionError] = useState("");
  const [mcpFeedback,setMcpFeedback]=useState("");
  const [acting, setActing] = useState(false);
  const [attaching,setAttaching]=useState(false);
  const [followRequest, setFollowRequest] = useState(0);
  const [options, setOptions] = useState<TurnOptions>({});
  const [workflowOpen, setWorkflowOpen] = useState(false);
  const [sidePane, setSidePane] = useState<"ticket" | "changes" | null>(null);
  const [sidePaneExpanded, setSidePaneExpanded] = useState(true);
  const [editingTicketInline, setEditingTicketInline] = useState(false);
  const overflowMenuRef = useRef<HTMLDetailsElement | null>(null);
  const availableBefore = useRef(agentAvailable);
  const messageInput = useRef<PromptInputHandle>(null);
  const sentPrompts = useMemo(() => userPrompts(decodeEvents(snapshot)), [snapshot.events]);
  const [folderEntries, setFolderEntries] = useState<FolderEntry[]>([]);
  /** Edit (#914): the queued message leaves the queue for the composer, with its turn options. */
  function editQueued(itemId: string) {
    const item = controller.withdrawQueued(itemId);
    if (!item) return;
    setDraft(current => mergeDraft(current, item.text));
    setOptions({ ...item.options });
    setQueueFeedback("");
    messageInput.current?.focus();
  }
  useEffect(() => { void controller.refresh(); }, [controller]);
  useEffect(() => {
    if(agentAvailable && !availableBefore.current) void controller.refresh();
    availableBefore.current=agentAvailable;
  }, [agentAvailable,controller]);
  useEffect(() => {
    const onPointerDown = (e: MouseEvent) => {
      if (overflowMenuRef.current && !overflowMenuRef.current.contains(e.target as Node)) {
        overflowMenuRef.current.removeAttribute("open");
      }
    };
    window.addEventListener("mousedown", onPointerDown);
    return () => window.removeEventListener("mousedown", onPointerDown);
  }, []);
  const [renaming,setRenaming] = useState(false);
  const [title,setTitle] = useState("");
  const [savingTitle,setSavingTitle] = useState(false);
  const [stopping,setStopping] = useState(false);
  const [linkError,setLinkError] = useState("");
  const [pathCopyState,setPathCopyState] = useState("");
  useEffect(()=>setPathCopyState(""),[directory]);
  const busy = pending || snapshot.busy;
  // While a turn runs, or while messages already wait, a submission joins the queue instead of being sent.
  const queueing = busy || queue.items.length > 0;
  const queueFull = queue.items.length >= QUEUE_LIMIT;
  useEffect(() => { if (!queueFull) setQueueFeedback(""); }, [queueFull]);
  function submitDraft() {
    if (!loaded || snapshot.readOnly || attaching || !draft.trim()) return;
    if (!queueing && (!agentAvailable || error)) return;
    const result = controller.submit(draft, options);
    if (result === "sent" || result === "queued") {
      setDraft(""); setQueueFeedback(""); messageInput.current?.resetHistory();
      setFollowRequest(value => value + 1);
    } else if (result === "full") setQueueFeedback(`Queue full (${QUEUE_LIMIT} messages). Remove one to add another.`);
  }
  async function control(input: Exclude<ConversationAction, { action: "message" }>) {
    if (acting) throw new Error("An action is already in progress.");
    setActing(true); setActionError("");
    try { await controller.control(input); }
    catch (error) { if (input.action !== "answer") setActionError(String(error)); throw error; }
    finally { setActing(false); }
  }
  async function stopConversation() {
    if(stopping || !run || !isActive(run))return;
    setStopping(true);setActionError("");
    try {await stopRun(id);await controller.refresh();onStopped?.();}
    catch {setActionError("Stop not confirmed. Refresh status before trying again.");}
    finally {setStopping(false);}
  }
  const displayTitle = conversationTitle(snapshot, run, task);
  const taskKey = run?.taskKey || task?.key;
  const sortedTaskRuns = taskRuns && taskRuns.length > 0
    ? [...taskRuns].sort((a, b) => (Date.parse(b.createdAt) || 0) - (Date.parse(a.createdAt) || 0))
    : [];
  const step = task && projectContext ? nextSkill(task, projectContext) : undefined;
  const anyTaskRunActive = sortedTaskRuns.some(isActive) || (run ? isActive(run) : false);
  const currentStage = task ? taskStage(task, board ?? projectContext?.board) : undefined;
  return <section className="project-prompt conversation-prompt">
    <header className="workspace-toolbar prompt-heading">
      <div className="workspace-toolbar-main">
        <h2 title={displayTitle} style={{cursor:snapshot.canRename?"pointer":"default"}} onClick={()=>{if(snapshot.canRename){setTitle(snapshot.title||displayTitle||"");setRenaming(true);}}}>
          <Icon name="chat" />
          {taskKey && <span role={onOpenTask ? "button" : undefined} tabIndex={onOpenTask ? 0 : undefined} className="sidebar-task-key" title={onOpenTask ? `Open ticket ${taskKey}` : undefined} aria-label={onOpenTask ? `Open ticket ${taskKey}` : undefined} onClick={e=>{if(onOpenTask){e.stopPropagation();onOpenTask();}}} onKeyDown={e=>{if(onOpenTask && (e.key==="Enter"||e.key===" ")){e.preventDefault();e.stopPropagation();onOpenTask();}}}>{taskKey}</span>}
          <span>{displayTitle}</span>
        </h2>
        {run?.contextRunId && (
          <p className="conversation-started-from">
            {onOpenSource
              ? <>Started from <button type="button" className="link-button" title="Open the archived conversation" onClick={()=>onOpenSource(run.contextRunId!)}>{run.contextTitle?.trim() || "an archived conversation"}</button></>
              : <>Started from {run.contextTitle?.trim() ? `“${run.contextTitle.trim()}”` : "an archived conversation"}</>}
            {(run.contextOmitted ?? 0) > 0 && <span className="muted"> · {run.contextOmitted} earlier message{run.contextOmitted === 1 ? "" : "s"} left out</span>}
          </p>
        )}
        <div className="workspace-path">
          <p title={directory}>{directory || "Conversation workspace"}</p>
          {directory && <button type="button" className="copy-path" title="Copy working folder path" aria-label="Copy working folder path" onClick={()=>void navigator.clipboard.writeText(directory).then(()=>setPathCopyState("Copied"),()=>setPathCopyState("Copy failed"))}>{pathCopyState||"Copy"}</button>}
        </div>
      </div>
      <div className="toolbar-actions">
        {run?.taskId && (
          <div className="board-view-switch conversation-view-tabs" role="tablist" aria-label="Ticket views">
            <button
              type="button"
              role="tab"
              aria-label="Conversation"
              title="Conversation"
              aria-selected={sidePane === null}
              aria-pressed={sidePane === null}
              onClick={() => {
                setSidePane(null);
                setEditingTicketInline(false);
              }}
            >
              <Icon name="chat" size={16} />
            </button>
            <button
              type="button"
              role="tab"
              aria-label="Ticket"
              title="Ticket"
              aria-selected={sidePane === "ticket"}
              aria-pressed={sidePane === "ticket"}
              onClick={() => {
                setEditingTicketInline(false);
                setSidePaneExpanded(true);
                setSidePane("ticket");
              }}
            >
              <Icon name="ticket" size={16} />
            </button>
            <button
              type="button"
              role="tab"
              aria-label="Changes"
              title="Changes"
              aria-selected={sidePane === "changes"}
              aria-pressed={sidePane === "changes"}
              onClick={() => {
                setSidePaneExpanded(true);
                setSidePane("changes");
              }}
            >
              <Icon name="branch" size={16} />
            </button>
          </div>
        )}
        {run?.taskId && sortedTaskRuns.length > 0 && onSelectRun && (
          <label className="conversation-run-history-label" title="Ticket execution history">
            <span className="sr-only">Ticket execution history</span>
            <select
              aria-label="Ticket execution history"
              className="conversation-run-history"
              value={id}
              onChange={e => onSelectRun(e.target.value)}
            >
              {sortedTaskRuns.map((item, index) => {
                const runNumber = sortedTaskRuns.length - index;
                const skillName = skillLabels[item.skill] || item.skill || "Conversation";
                return (
                  <option key={item.id} value={item.id}>
                    #{runNumber} · {skillName} · {statusLabel(item)}
                  </option>
                );
              })}
            </select>
          </label>
        )}
        {run?.taskId && step?.skillId && onLaunchSkill && !anyTaskRunActive && (
          <button
            type="button"
            className="icon-button"
            disabled={!agentAvailable}
            aria-label={`Next step: ${step.label}`}
            title={`Next step: ${step.label}`}
            onClick={() => onLaunchSkill(step.skillId!)}
          >
            <Icon name="chevron" />
          </button>
        )}
        {snapshot.canAdoptConnection && <button disabled={!agentAvailable || acting || !!error} title="This history is not linked to the selected connection yet." onClick={() => void control({ action: "adoptConnection" }).catch(() => {})}>Link to this connection</button>}
        {snapshot.canRename && <button type="button" className="icon-button" aria-label="Rename conversation" title="Rename conversation" onClick={()=>{setTitle(snapshot.title||displayTitle||"");setRenaming(true);}}><Icon name="edit" /></button>}
        {archived && onStartOver && (!run || !isActive(run)) && (
          <button
            type="button"
            className="conversation-start-over"
            disabled={!agentAvailable || !loaded}
            title={!agentAvailable ? "Agent unavailable. Reconnect to start over." : !loaded ? "Loading this conversation's history…" : "Start a new conversation or skill with this conversation as context"}
            onClick={onStartOver}
          >
            <Icon name="refresh" /><span>Start over</span>
          </button>
        )}
        {run && isActive(run) ? (
          <button type="button" className="icon-button stop-button" aria-label="Stop discussion" title={stopping ? "Stopping…" : "Stop discussion"} disabled={!agentAvailable || stopping || !!run.cancelRequested} onClick={() => void stopConversation()}><Icon name="stop" /></button>
        ) : onArchive ? (
          <button type="button" className="icon-button" aria-label="Archive discussion" title="Archive discussion" onClick={onArchive}><Icon name="archive" /></button>
        ) : null}
        {run?.taskId && onLaunchSkill && projectContext?.skills && projectContext.skills.length > 0 && (
          <details className="project-action-menu conversation-overflow-menu" ref={overflowMenuRef}>
            <summary
              className="icon-button"
              aria-label="Ticket actions"
              title="Ticket actions and related views"
            >
              <Icon name="more" />
            </summary>
            <div className="project-action-panel conversation-overflow-panel">
              {step?.skillId && (
                <button
                  type="button"
                  disabled={!agentAvailable || anyTaskRunActive}
                  onClick={e => {
                    e.currentTarget.closest("details")?.removeAttribute("open");
                    onLaunchSkill(step.skillId!);
                  }}
                >
                  <Icon name="sparkles" />
                  <span>Run next step: {step.label}</span>
                </button>
              )}
              {projectContext.skills.some(s => s.id === "pickup") && currentStage !== "finished" && (
                <button
                  type="button"
                  disabled={!agentAvailable || anyTaskRunActive}
                  onClick={e => {
                    e.currentTarget.closest("details")?.removeAttribute("open");
                    onLaunchSkill("pickup");
                  }}
                >
                  <Icon name="agent" />
                  <span>Run chain (Pickup)</span>
                </button>
              )}
              {projectContext.skills
                .filter(s => s.id !== step?.skillId && s.id !== "pickup" && s.id !== "pickup_issues")
                .map(skill => (
                  <button
                    key={skill.id}
                    type="button"
                    disabled={!agentAvailable || anyTaskRunActive || currentStage === "finished"}
                    onClick={e => {
                      e.currentTarget.closest("details")?.removeAttribute("open");
                      onLaunchSkill(skill.id);
                    }}
                  >
                    <Icon name="terminal" />
                    <span>Launch: {skillLabels[skill.id] || skill.id}</span>
                  </button>
                ))}
            </div>
          </details>
        )}
      </div>
    </header>
    <div className={`conversation-body-split ${sidePane ? "has-side-pane" : ""} ${sidePane && sidePaneExpanded ? "side-pane-expanded" : ""}`}>
      <div className="conversation-main-column">
        {renaming&&<form className="conversation-rename" onSubmit={async e=>{e.preventDefault();if(savingTitle||!title.trim())return;setSavingTitle(true);setActionError("");try{await renameConversation(id,title.trim());await controller.refresh();onTitleChanged?.();setRenaming(false);}catch(cause){setActionError(String(cause));}finally{setSavingTitle(false);}}}><label>Conversation name<input autoFocus required maxLength={80} value={title} onChange={e=>setTitle(e.target.value)}/></label><button type="button" disabled={savingTitle} onClick={()=>setRenaming(false)}>Cancel</button><button disabled={savingTitle||!title.trim()}>Save name</button></form>}
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
              ? "Allow once"
              : choice.decision === "always"
              ? "Always allow"
              : choice.decision === "deny"
              ? "Deny"
              : choice.label,
          }))
        : [
            { decision: "allow", label: "Allow once" },
            ...(hasSuggestions ? [{ decision: "always" as const, label: "Always allow" }] : []),
            { decision: "deny", label: "Deny" },
          ];
      return <section className="prompt-approval" key={approval.id}>
        <strong>Permission required · {approval.tool}</strong><p>{approval.description || approval.reason}</p>
        <pre>{JSON.stringify(approval.input, null, 2)}</pre>
        {hasSuggestions && (
          <details className="prompt-approval-suggestions">
            <summary>Always allow: proposed rule for this project</summary>
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
    {agentAvailable && (error || actionError) && <p role="alert" className="error">{error || actionError} Check status before resending a message. <button type="button" aria-label="Refresh status" onClick={() => void controller.refresh()}>Retry</button></p>}
    </div>
    {workflowOpen && (
      <section className="conversation-workflow-box" aria-label="Ticket workflow">
        <div className="conversation-workflow-box-header">
          <div>
            <span className="eyebrow">TICKET WORKFLOW · {taskKey || run?.taskId}</span>
            <p className="workflow-meta">
              Tracker status: {task?.trackerStatus || "Unavailable"} · Assignee: {task?.assignee || "Unassigned"}
              {task?.sprint ? ` · Sprint: ${task.sprint}` : ""}
            </p>
          </div>
          <button
            type="button"
            className="icon-button"
            aria-label="Close workflow"
            title="Close workflow"
            onClick={() => setWorkflowOpen(false)}
          >
            <Icon name="close" />
          </button>
        </div>
        <ol className="workflow-progress" aria-label="Workflow stages">
          {stages.map(s => (
            <li key={s} aria-current={(step?.stage ?? currentStage) === s ? "step" : undefined}>
              {stageLabels[s]}
            </li>
          ))}
        </ol>
      </section>
    )}
    {snapshot.readOnly && <QueuedMessages queue={queue} onRemove={itemId => controller.removeQueued(itemId)} />}
    {snapshot.readOnly ? <>
      <ConversationStatus snapshot={snapshot} run={run} task={task} board={board} projectContext={projectContext} busy={busy} error={!!error} agentAvailable={agentAvailable} workflowOpen={workflowOpen} onToggleWorkflow={() => setWorkflowOpen(prev => !prev)} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} />
      <div className="conversation-readonly-footer" aria-label="History options">
        <PromptFolders compact projectId={projectId} runId={id} directory={directory} folders={folders} readOnly onChanged={onFoldersChanged}/>
        <button type="button" className="composer-action" disabled={!directory} title="Open terminal in working folder" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setActionError("Unable to open terminal in this folder."))}><Icon name="terminal"/>Terminal</button>
        {run?.taskId && <TaskPrStatus task={task} onOpenPr={url=>void openExternalLink(url).catch(()=>setLinkError("Unable to open PR/MR."))} />}
      </div>
    </> : <form className="composer" onSubmit={e => { e.preventDefault(); submitDraft(); }}>
      <div className="composer-nav-row">
        <ConversationStatus snapshot={snapshot} run={run} task={task} board={board} projectContext={projectContext} busy={busy} error={!!error} agentAvailable={agentAvailable} workflowOpen={workflowOpen} onToggleWorkflow={() => setWorkflowOpen(prev => !prev)} onOpenQueue={onOpenQueue} onOpenTask={onOpenTask} />
      </div>
      <div className={`composer-box${isShellDraft(draft) ? " shell-mode" : ""}`}>
        <QueueStrip queue={queue} onRemove={itemId => controller.removeQueued(itemId)} onResume={() => controller.resumeQueue()} onEdit={editQueued} />
        <ContextTray items={folderTrayItems(folderEntries, copyPath)} />
        <PromptInput
          ref={messageInput}
          ariaLabel="Next message"
          viewSelector=".conversation-main-column"
          projectId={projectId}
          conversationId={id}
          mentions={mentionSources}
          value={draft}
          onChange={setDraft}
          commands={snapshot.commands}
          prompts={sentPrompts}
          readOnly={!!snapshot.readOnly}
          placeholder={snapshot.busy ? "Queue a follow-up…" : "Ask a question or describe a change…"}
          onSubmit={submitDraft}
          onEscape={() => {
            if (!busy || snapshot.readOnly || !agentAvailable || acting) return false;
            void control({ action: "interrupt" }).catch(() => {});
            return true;
          }}
        />
        <div className="composer-footer"><TurnSettings compact showProviderMark provider={snapshot.provider} snapshot={snapshot} value={options} onChange={setOptions} disabled={!loaded || busy || !!snapshot.readOnly || !!error || acting} />
        <ContextUsageMeter context={snapshot.context} />
        {isShellDraft(draft) && <span className="composer-shell-badge" title="Runs as a shell command in this conversation's folder"><Icon name="terminal" />Shell</span>}
        <div className="composer-inline-tools">
          <PromptFolders compact projectId={projectId} runId={id} directory={directory} folders={folders} readOnly={!loaded||snapshot.readOnly} onChanged={onFoldersChanged} onPendingChange={setAttaching} onEntriesChange={setFolderEntries}/>
          <button type="button" className="composer-nav-btn" aria-label="Terminal" disabled={!directory} title="Open terminal in working folder" onClick={()=>void invoke("open_terminal_at",{path:directory}).catch(()=>setActionError("Unable to open terminal in this folder."))}><Icon name="terminal"/><span className="sr-only">Terminal</span></button>
          <button type="button" className={`composer-nav-btn mcp-action mcp-${snapshot.sectileMcp?.status??"unknown"}`} disabled={!agentAvailable||!loaded||acting||snapshot.checkingMcp||snapshot.readOnly} aria-label="Check MCP for this conversation" title={`${snapshot.sectileMcp?.source==="transport"?"Companion MCP transport":"Engine MCP"} · ${snapshot.checkingMcp?"Checking…":({connected:"Connected",missing:"Not registered",failed:"Connection failed","needs-auth":"Sign-in required",pending:"Connecting",unknown:"Status unverified"} as Record<string,string>)[snapshot.sectileMcp?.status??"unknown"]||"Status unverified"}`} onClick={()=>void control({action:"checkMcp"}).then(()=>{const current=controller.getSnapshot().snapshot;setMcpFeedback(current.provider==="codex"&&!current.checkingMcp&&current.sectileMcp?.status==="unknown"?"Codex will check MCP on the next message, after its session reconnects.":"");}).catch(()=>{})}><span className="mcp-dot"/>MCP</button>
          {run?.taskId && <TaskPrStatus task={task} onOpenPr={url=>void openExternalLink(url).catch(()=>setLinkError("Unable to open PR/MR."))} />}
        </div>
        {busy && !snapshot.readOnly && (
          <button
            type="button"
            className="send-button interrupt-button"
            aria-label="Interrupt"
            title="Cancel current execution · Esc"
            disabled={!agentAvailable || acting}
            onClick={() => void control({ action: "interrupt" }).catch(() => {})}
          >
            <Icon name="stopSquare" />
          </button>
        )}
        {queueing
          ? <button className="send-button queue-button" aria-label="Queue message" title="Queue · Enter" disabled={!loaded || attaching || !!snapshot.readOnly || queueFull || !draft.trim()}><Icon name="plus" /></button>
          : <button className="send-button" aria-label="Send" title="Send · Enter" disabled={!agentAvailable || !loaded || busy || attaching || !!snapshot.readOnly || !!error || !draft.trim()}><Icon name="send" /></button>}</div>
      </div>
      {snapshot.busy && <p className="composer-feedback composer-shortcuts">Enter to queue · Esc to interrupt</p>}
      {queueFeedback && <p className="composer-feedback" role="status">{queueFeedback}</p>}
      {mcpFeedback&&snapshot.sectileMcp?.status==="unknown"&&<p className="composer-feedback" role="status">{mcpFeedback}</p>}
    </form>}
    </div>
      </div>
      {sidePane === "ticket" && (
        <aside className="conversation-side-pane" aria-label="Ticket panel">
          <div className="side-pane-toolbar">
            <div className="side-pane-toolbar-left">
              <span className="eyebrow">TICKET · {taskKey || run?.taskId}</span>
              {onOpenTask && (
                <button
                  type="button"
                  className="changes-toolbar-btn"
                  title="Open ticket in external tracker"
                  onClick={onOpenTask}
                >
                  <Icon name="external" size={14} />
                  <span>Open in tracker</span>
                </button>
              )}
            </div>
            <div className="side-pane-toolbar-right">
              <button
                type="button"
                className="changes-toolbar-btn"
                aria-pressed={editingTicketInline}
                onClick={() => setEditingTicketInline(prev => !prev)}
              >
                <Icon name="edit" />
                <span>{editingTicketInline ? "Ticket preview" : "Edit ticket"}</span>
              </button>
              <button
                type="button"
                className="icon-button"
                aria-label={sidePaneExpanded ? "Collapse to split view" : "Expand to full screen"}
                title={sidePaneExpanded ? "Collapse to split view" : "Expand to full screen"}
                onClick={() => setSidePaneExpanded(prev => !prev)}
              >
                <Icon name={sidePaneExpanded ? "collapse" : "expand"} />
              </button>
              <button
                type="button"
                className="icon-button"
                aria-label="Close ticket panel"
                title="Close ticket panel"
                onClick={() => {
                  setSidePane(null);
                  setSidePaneExpanded(false);
                  setEditingTicketInline(false);
                }}
              >
                <Icon name="close" />
              </button>
            </div>
          </div>
          <div className="side-pane-body">
            {editingTicketInline && run?.taskId ? (
              <TaskEditor
                projectId={projectId}
                projectName={projectName || "Project"}
                taskId={run.taskId}
                available={canEditTask ?? true}
                knownSprints={knownSprints}
                knownAssignees={knownAssignees}
                onClose={() => setEditingTicketInline(false)}
                onSaved={saved => {
                  onTaskSaved?.(saved);
                  setEditingTicketInline(false);
                }}
              />
            ) : (
              <div className="ticket-split-detail">
                <div className="ticket-split-detail-header">
                  <h3>{task?.title || displayTitle}</h3>
                  <div className="ticket-split-meta-pills">
                    <span>Status: <strong>{task?.trackerStatus || "Unavailable"}</strong></span>
                    {task?.issueType && <span>Type: <strong>{task.issueType}</strong></span>}
                    {task?.priority && <span>Priority: <strong>{task.priority}</strong></span>}
                    <span>Assignee: <strong>{task?.assignee || "Unassigned"}</strong></span>
                    {task?.sprint && <span>Sprint: <strong>{task.sprint}</strong></span>}
                  </div>
                </div>
                <div className="ticket-split-detail-description">
                  {task?.description ? (
                    <Markdown text={task.description} />
                  ) : (
                    <p className="muted">No description provided for this ticket.</p>
                  )}
                </div>
              </div>
            )}
          </div>
        </aside>
      )}
      {sidePane === "changes" && run && (
        <aside className="conversation-side-pane conversation-side-pane-changes" aria-label="Changes review panel">
          <RunReview
            key={run.id}
            run={run}
            fullPage
            expanded={sidePaneExpanded}
            onToggleExpand={() => setSidePaneExpanded(prev => !prev)}
            onClose={() => {
              setSidePane(null);
              setSidePaneExpanded(false);
            }}
          />
        </aside>
      )}
    </div>
  </section>;
}
function ConversationStatus({snapshot,run,task,board,projectContext,busy,error,agentAvailable,workflowOpen,onToggleWorkflow,onOpenQueue,onOpenTask}:{snapshot:ConversationSnapshot;run?:Run;task?:Task;board?:BoardMapping;projectContext?:ProjectContext;busy:boolean;error:boolean;agentAvailable:boolean;workflowOpen?:boolean;onToggleWorkflow?:()=>void;onOpenQueue?:()=>void;onOpenTask?:()=>void}) {
  const stage=task?taskStage(task,board):undefined;
  const activity=!agentAvailable?"Agent unavailable":error?"Status interrupted":snapshot.approvals?.length?"Your answer needed":busy?"Responding":snapshot.readOnly?"Read-only history":"Ready to respond";
  const execution=!agentAvailable?"Status unavailable":run?busy&&run.conversation?"Responding":statusLabel(run):"Loading";
  // A ticket skill shows how the skill stands in place of the run state (#908);
  // the run state, the full reason and the diagnostics move into the panel.
  const skill=useSkillStatus(run,{task,project:projectContext,asking:!!snapshot.approvals?.length,turnBusy:snapshot.busy});
  const status=agentAvailable?skill:null;
  return <div className="conversation-status-strip" aria-label="Conversation status">
    <span className={`conversation-status-activity ${agentAvailable&&busy?"live":""}`}><span className="conversation-status-dot"/>{activity}</span>
    <details className={`conversation-execution-menu${status?` skill-status-${status.kind}`:""}`}>{status
      ?<summary title={skillStatusTitle(status)}><span className={`skill-status-mark skill-${status.kind}`} aria-hidden="true"/>{skillStatusText(status)}</summary>
      :<summary>Execution: {execution}</summary>}<div className="conversation-execution-panel">
      {status&&<div className="conversation-execution-details">
        <p>Execution: {execution}</p>
        {status.reason&&<p className="conversation-execution-reason">{status.reason}</p>}
        <p className="conversation-execution-diagnostics">{status.diagnostics}</p>
      </div>}
      {onOpenQueue&&<button type="button" onClick={onOpenQueue}>View execution queue</button>}
      {onOpenTask&&<button type="button" onClick={onOpenTask}>Open ticket in tracker</button>}
    </div></details>
    {run?.taskId && onToggleWorkflow && (
      <button
        type="button"
        className="conversation-workflow-toggle"
        aria-pressed={workflowOpen}
        onClick={onToggleWorkflow}
      >
        <Icon name="layers" size={13} />
        <span>Workflow{stage ? ` · ${stageLabels[stage]}` : ""}</span>
      </button>
    )}
  </div>;
}

function TaskPrStatus({ task, onOpenPr }: { task?: Task; onOpenPr: (url: string) => void }) {
  const links = taskPullRequests(task);
  if (links.length > 1) {
    return (
      <details className="conversation-pr-menu">
        <summary>PR/MR: {links.length} linked</summary>
        <div className="conversation-pr-panel">
          {links.map(link => (
            <button type="button" key={link.url} onClick={() => onOpenPr(link.url)}>
              <Icon name="branch" />
              {pullRequestLabel(link)} · {pullRequestStateLabel(link)}
            </button>
          ))}
        </div>
      </details>
    );
  }
  return (
    <span className="composer-pr-status">
      PR/MR:{" "}
      {links.length ? (
        <button type="button" className="conversation-pr-link" onClick={() => onOpenPr(links[0].url)}>
          <Icon name="branch" />
          {pullRequestLabel(links[0])} · {pullRequestStateLabel(links[0])}
        </button>
      ) : (
        "none"
      )}
    </span>
  );
}
