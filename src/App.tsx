import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listProjects, listTasks } from "./lib/agent";
import { trackerColumns, type Project, type Task } from "./domain/board";
import { executionConnectionLabel, isActive, projectRuns, statusLabel, type Run } from "./domain/execution";
import {
  ExecutionProvider,
  useExecutionMonitor,
} from "./components/ExecutionMonitor";
import { Supervisor } from "./components/Supervisor";
import { ProjectFolderSetup } from "./components/ProjectFolderSetup";
import { ProjectTrackerSetup } from "./components/ProjectTrackerSetup";
import { AdvancedFolders } from "./components/AdvancedFolders";
import { WorkflowSettings } from "./components/WorkflowSettings";
import { OrchestrationSettings } from "./components/OrchestrationSettings";

import { projectContext, type ProjectContext } from "./lib/workspace";
import { skillLabels, stages, stageLabels, taskStage } from "./domain/workflow";
import { AttachedFolderSettings } from "./components/FolderContext";
import { ProjectProfiles } from "./components/ProjectProfiles";
import { LocalSettings } from "./components/LocalSettings";
import { Icon } from "./components/Icon";
import { readPreference, savePreference, useAppearance } from "./lib/appearance";
import {
  createWorkspaceDestination,
  resolveLaunchDestination,
  serializeWorkspaceDestination,
  type WorkspaceDestination,
  type WorkspaceViewName,
} from "./lib/workspace-preference";
import { useConversation } from "./lib/conversation-controller";
import { CommandPalette } from "./components/CommandPalette";
import { shortcutModifier, workspaceShortcut } from "./lib/workspace-shortcuts";
import "./App.css";

const WorkflowTask = lazy(()=>import("./components/WorkflowTask").then(module=>({default:module.WorkflowTask})));
const ProjectPrompt = lazy(() => import("./components/ProjectPrompt"));

type View = WorkspaceViewName | "settings";
interface WorkspaceView {
  id: string;
  projectId: string;
  view: View;
  runId?: string;
  taskId?: string;
}
export default function App() {
  return (
    <ExecutionProvider>
      <Workspace />
    </ExecutionProvider>
  );
}
function Workspace() {
  const mac = useMemo(() => /Mac/i.test(navigator.platform || navigator.userAgent), []);
  const modifier = shortcutModifier(mac);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [sidebarWidth, setSidebarWidth] = useState(() => {
    const saved = Number(readPreference("sidebarWidth", "268"));
    return Number.isFinite(saved) ? Math.max(200, Math.min(480, saved)) : 268;
  });
  const sidebarDrag = useRef<{ pointerId: number; startX: number; startWidth: number } | null>(null);
  const [openingFolder, setOpeningFolder] = useState(false);
  const [folderError, setFolderError] = useState("");
  const [unmapped, setUnmapped] = useState<(Project & { mappingError?: string }) | null>(null);
  const folderBusy = useRef(false);
  async function openFolder() {
    if (folderBusy.current) return;
    folderBusy.current = true; setOpeningFolder(true); setFolderError("");
    try {
      const project = await invoke<(Project & { mappingError?: string }) | null>("open_folder_project");
      if (project?.mappingError) { setUnmapped(project); setFolderError(`Le projet a été créé, mais le dossier n’a pas été associé : ${project.mappingError}`); await refreshProjects(); }
      else if (project) { await refreshProjects(); navigate(project, "prompt"); }
    } catch (cause) { setFolderError(String(cause)); }
    finally { folderBusy.current = false; setOpeningFolder(false); }
  }
  const [projects, setProjects] = useState<Project[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [current, setCurrent] = useState<WorkspaceView | null>(null);
  const [sidebar, setSidebar] = useState(true);
  const currentRef = useRef(current);
  currentRef.current = current;
  const workspaceBeforeSettings = useRef<WorkspaceView | null>(null);
  const [filter, setFilter] = useState("");
  const [appearance, setAppearance] = useAppearance();
  const { snapshot, fresh, refresh: refreshExecutions } = useExecutionMonitor();
  const request = useRef(0);
  const initialized = useRef(false);
  const pendingRunRestore = useRef<WorkspaceDestination | null>(null);
  const navigate = useCallback(
    (project: Project, view: View, runId?: string, taskId?: string) => {
      if (view === "settings" && currentRef.current?.view !== "settings") {
        workspaceBeforeSettings.current = currentRef.current;
      }
      const destination: WorkspaceView = {
        id: `${project.id}:${view}:${runId ?? ""}:${taskId ?? ""}`,
        projectId: project.id,
        view,
        runId,
        taskId,
      };
      if (view !== "settings") {
        savePreference("workspace", serializeWorkspaceDestination(createWorkspaceDestination(project.id, view, runId, taskId)));
      }
      setCurrent(destination);
    }, []);
  const returnToWorkspace = useCallback(() => {
    const previous = workspaceBeforeSettings.current;
    if (previous) {
      workspaceBeforeSettings.current = null;
      setCurrent(previous);
    } else {
      setSidebar(value => !value);
    }
  }, []);
  async function refreshProjects() {
    const seq = ++request.current;
    setBusy(true);
    setError("");
    try {
      const items = await listProjects();
      if (seq !== request.current) return;
      setProjects(items);
      if (!initialized.current && items.length) {
        initialized.current = true;
        const destination = resolveLaunchDestination(readPreference("workspace", ""), items);
        if (destination?.runId) pendingRunRestore.current = destination;
        setCurrent((previous) => previous ?? destination);
      }
      if (!items.length) setCurrent(previous => previous ?? { id: "settings", projectId: "settings", view: "settings" });
    } catch (cause) {
      if (seq === request.current) {
        setError(String(cause));
        setCurrent(previous => previous ?? { id: "settings", projectId: "settings", view: "settings" });
      }
    } finally {
      if (seq === request.current) setBusy(false);
    }
  }
  useEffect(() => {
    void refreshProjects();
    return () => {
      request.current++;
    };
  }, []);
  useEffect(() => {
    const pending = pendingRunRestore.current;
    if (!pending || !fresh) return;
    if (current?.id !== pending.id) {
      pendingRunRestore.current = null;
      return;
    }
    const restoredRun = snapshot?.runs.find((run) => run.id === pending.runId);
    if (restoredRun && (pending.view !== "prompt" || restoredRun.conversation)) {
      pendingRunRestore.current = null;
      return;
    }
    const project = projects.find((item) => item.id === pending.projectId);
    pendingRunRestore.current = null;
    if (project) navigate(project, "board");
  }, [current?.id, fresh, navigate, projects, snapshot?.runs]);
  const connected = fresh && snapshot?.status.connected;
  const settingsHasWorkspace = current?.view === "settings" && Boolean(workspaceBeforeSettings.current);
  const showProjectSidebar = sidebar && (current?.view !== "settings" || !settingsHasWorkspace);
  useEffect(() => {
    function onShortcut(event: KeyboardEvent) {
      const target = event.target instanceof Element ? event.target : null;
      const selected = projects.find(project => project.id === currentRef.current?.projectId)
        ?? projects.find(project => project.id === workspaceBeforeSettings.current?.projectId);
      const action = workspaceShortcut(event, {
        mac,
        modalOpen: paletteOpen || Boolean(document.querySelector("dialog[open]")),
        terminalFocused: Boolean(target?.closest(".terminal,.xterm")),
        editableFocused: Boolean(target?.closest("input,textarea,select,[contenteditable='true']")),
        settingsOpen: currentRef.current?.view === "settings",
        hasProject: Boolean(selected),
      });
      if (action === "none") return;
      event.preventDefault();
      if (action === "settings") {
        if (currentRef.current?.view !== "settings") navigate({ id: "settings", name: "Paramètres" }, "settings");
      } else if (action === "sidebar") {
        setSidebar(value => !value);
      } else if (action === "prompt" && selected) {
        navigate(selected, "prompt");
      } else if (action === "folder") {
        void openFolder();
      }
    }
    window.addEventListener("keydown", onShortcut);
    return () => window.removeEventListener("keydown", onShortcut);
  }, [mac, navigate, paletteOpen, projects]);
  return (
    <div className="desktop-shell">
      <header className="app-header">
        <div className="header-brand">
          <button
            className="icon-button"
            title={settingsHasWorkspace ? "Retourner à l’espace de travail" : `Afficher ou masquer les projets · ${modifier}B`}
            aria-label={settingsHasWorkspace ? "Retourner à l’espace de travail" : "Afficher ou masquer les projets"}
            aria-keyshortcuts={settingsHasWorkspace ? undefined : mac ? "Meta+B" : "Control+B"}
            aria-expanded={showProjectSidebar}
            onClick={returnToWorkspace}
          >
            <Icon name="panel" />
          </button>
          <span className="brand-mark"><Icon name="terminal" /></span>
          <strong>
            Sectile <span>Desktop</span>
          </strong>

        </div>
        <div className="header-actions">
          <div className="theme-toggle" aria-label="Apparence">
            {(["system", "light", "dark"] as const).map((value, i) => (
              <button
                key={value}
                className="icon-button"
                aria-label={["Thème système", "Thème clair", "Thème sombre"][i]}
                title={["Système", "Clair", "Sombre"][i]}
                aria-pressed={appearance === value}
                onClick={() => setAppearance(value)}
              >
                <Icon name={(["monitor", "sun", "moon"] as const)[i]} />
              </button>
            ))}
          </div>
          <button className="icon-button" aria-label="Navigation rapide" title="Navigation rapide · Cmd/Ctrl+K" onClick={() => setPaletteOpen(true)}><Icon name="search" /></button>
          <button className="icon-button" title={`Paramètres · ${modifier},`} aria-label="Paramètres" aria-keyshortcuts={mac ? "Meta+Comma" : "Control+Comma"} onClick={()=>navigate({id:"settings",name:"Paramètres"},"settings")}><Icon name="settings" /></button>
        </div>
      </header>
      <div className="desktop-body">
        {showProjectSidebar && (
          <>
          <aside className="sidebar" style={{ width: sidebarWidth }}>
            <div className="sidebar-scroll">
              <div className="sidebar-search">
                <Icon name="search" />
                <input
                  aria-label="Filtrer les projets"
                  placeholder="Rechercher un projet"
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                />
              </div>
              <button className="sidebar-link" disabled={openingFolder} onClick={() => void openFolder()}><Icon name="folder" />{openingFolder ? "Ouverture…" : "Ouvrir un dossier"}</button>
              <div className="section-label">
                PROJETS <span>{projects.length}</span>
              </div>
              {projects
                .filter((p) =>
                  p.name.toLowerCase().includes(filter.toLowerCase()),
                )
                .map((project) => {
                  const runs = projectRuns(snapshot?.runs ?? [], project.id);
                  const selected = current?.projectId === project.id;
                  return (
                    <section className="project-group" key={project.id}>
                      <div
                        className={`project-row ${selected ? "selected" : ""}`}
                      >
                        <button
                          className="project-heading"
                          onClick={() => navigate(project, "board")}
                        >
                          <Icon name="folder" />
                          <span>{project.name}</span>
                          {runs.some(run => !run.conversation && isActive(run)) && <span className="dot live" />}
                        </button>
                        <button
                          className="icon-button"
                          aria-label={`Nouveau prompt dans ${project.name}`}
                          title="Nouveau prompt"
                          onClick={() => navigate(project, "prompt")}
                        >
                          <Icon name="plus" />
                        </button>
                      </div>
                      {selected && (
                        <div className="project-children">
                          <button
                            className="sidebar-link"
                            onClick={() => navigate(project, "orchestration")}
                          >
                            <Icon name="agent" />
                            Orchestration
                          </button>
                          {runs.filter(run => run.conversation).slice(0,8).map(run => <RunNavigation key={run.id} run={run} selected={current?.runId===run.id} onClick={()=>navigate(project,"prompt",run.id)} />)}
                          <button
                            className="sidebar-link"
                            onClick={() => navigate(project, "executions")}
                          >
                            <Icon name="terminal" />
                            Toutes les exécutions <span>{runs.length}</span>
                          </button>
                        </div>
                      )}
                    </section>
                  );
                })}
              {!projects.length && (
                <p className="sidebar-hint">
                  {busy
                    ? "Connexion à l’agent…"
                    : "Vos projets apparaîtront ici."}
                </p>
              )}
            </div>
            <footer className="sidebar-footer">
              <div>
                <span className={`dot ${connected ? "live" : ""}`} />
                {connected ? "Agent connecté" : "Agent indisponible"}
                <small>
                  {fresh ? executionConnectionLabel(snapshot?.status.runtimeMode) : "Connexion en cours"}
                </small>
              </div>

            </footer>
          </aside>
          <div
            className="sidebar-resizer"
            role="separator"
            aria-label="Redimensionner la barre latérale"
            title="Glisser pour redimensionner · ← → au clavier"
            aria-orientation="vertical"
            aria-valuemin={200}
            aria-valuemax={480}
            aria-valuenow={sidebarWidth}
            tabIndex={0}
            onPointerDown={event => {
              if (event.button !== 0) return;
              sidebarDrag.current = { pointerId: event.pointerId, startX: event.clientX, startWidth: sidebarWidth };
              event.currentTarget.setPointerCapture(event.pointerId);
              document.body.classList.add("resizing-sidebar");
            }}
            onPointerMove={event => {
              const drag = sidebarDrag.current;
              if (drag?.pointerId === event.pointerId) setSidebarWidth(Math.max(200, Math.min(480, drag.startWidth + event.clientX - drag.startX)));
            }}
            onPointerUp={event => {
              const drag = sidebarDrag.current;
              if (drag?.pointerId !== event.pointerId) return;
              const next = Math.max(200, Math.min(480, drag.startWidth + event.clientX - drag.startX));
              setSidebarWidth(next);
              savePreference("sidebarWidth", String(next));
              sidebarDrag.current = null;
              document.body.classList.remove("resizing-sidebar");
              event.currentTarget.releasePointerCapture(event.pointerId);
            }}
            onPointerCancel={() => { sidebarDrag.current = null; document.body.classList.remove("resizing-sidebar"); }}
            onLostPointerCapture={() => { sidebarDrag.current = null; document.body.classList.remove("resizing-sidebar"); }}
            onKeyDown={event => {
              if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;
              event.preventDefault();
              const next = event.key === "Home" ? 200 : event.key === "End" ? 480 : Math.max(200, Math.min(480, sidebarWidth + (event.key === "ArrowRight" ? 20 : -20)));
              setSidebarWidth(next);
              savePreference("sidebarWidth", String(next));
            }}
          />
          </>
        )}
        <main className="workspace">
          {folderError && <div role="alert" className="error">{folderError}
            {unmapped && <button onClick={async () => {
              try { await invoke("retry_folder_project", { projectId: unmapped.id, path: unmapped.path }); setFolderError(""); setUnmapped(null); await refreshProjects(); navigate(unmapped, "prompt"); }
              catch (cause) { setFolderError(String(cause)); }
            }}>Réessayer l’association</button>}
          </div>}
          {error && current?.view !== "settings" && (
            <div role="alert" className="error">
              {error}
              <button onClick={() => void refreshProjects()}>Réessayer</button>
            </div>
          )}
          {!current && (
            <section className="welcome">
              <span className="brand-mark">S</span>
              <h1>Votre espace de travail</h1>
              <p>
                Choisissez un projet pour ouvrir son tableau, ou lancez un
                nouveau prompt.
              </p>
            </section>
          )}
          {current && (
            <section className={`workspace-pane ${current.view === "prompt" && current.runId ? "workspace-pane-conversation" : ""}`}>
              {current.view === "settings" ? (
                <LocalSettings onConfigureProject={id=>{const project=projects.find(p=>p.id===id);if(project)navigate(project,"project-settings");}} onOpenFolder={() => void openFolder()} onChanged={() => { setProjects([]); void refreshProjects(); void refreshExecutions(); }} />
              ) : (
                <ProjectPane
                  key={current.id}
                  selection={current}
                  project={projects.find((p) => p.id === current.projectId)}
                  navigate={navigate}
                  onFolderMapped={(id,path)=>setProjects(items=>items.map(item=>item.id===id?{...item,path}:item))}
                  onProjectsChanged={() => void refreshProjects()}
                />
              )}
            </section>
          )}
        </main>
      </div>
      <CommandPalette
        open={paletteOpen}
        onOpenChange={setPaletteOpen}
        projects={projects}
        runs={snapshot?.runs ?? []}
        selectedProjectId={current?.projectId}
        onNavigate={navigate}
        onOpenFolder={() => void openFolder()}
        onOpenSettings={() => navigate({ id: "settings", name: "Paramètres" }, "settings")}
        onToggleSidebar={returnToWorkspace}
        sidebarActionLabel={settingsHasWorkspace ? "Retourner à l’espace de travail" : sidebar ? "Masquer les projets" : "Afficher les projets"}
        shortcutModifier={modifier}
      />
    </div>
  );
}
function ProjectPane({
  selection,
  project,
  navigate,
  onFolderMapped,
  onProjectsChanged,
}: {
  selection: WorkspaceView;
  project?: Project;
  onFolderMapped: (projectId:string,path:string) => void;
  onProjectsChanged: () => void;
  navigate: (
    project: Project,
    view: View,
    runId?: string,
    taskId?: string,
  ) => void;
}) {
  const monitor = useExecutionMonitor();
  const currentRun = monitor.snapshot?.runs.find(run=>run.id===selection.runId);
  const activeTaskRuns = useMemo(() => {
    const latest = new Map<string, Run>();
    for (const run of projectRuns(monitor.snapshot?.runs ?? [], selection.projectId)) {
      if (!run.conversation && run.taskId && !latest.has(run.taskId)) latest.set(run.taskId, run);
    }
    return latest;
  }, [monitor.snapshot?.runs, selection.projectId]);
  const workingDirectory = selection.runId ? currentRun?.directory : project?.path;
  const [taskState, setTaskState] = useState<{projectId:string;items:Task[]}>();
  const tasks = taskState?.projectId === selection.projectId ? taskState.items : [];
  const [errorState, setErrorState] = useState<{projectId:string;message:string}>();
  const error = errorState?.projectId === selection.projectId ? errorState.message : "";
  const [busy, setBusy] = useState(false);
  const [queryState, setQueryState] = useState<{projectId:string;value:string}>();
  const query = queryState?.projectId === selection.projectId ? queryState.value : "";
  const [grouping,setGrouping] = useState("tracker");
  const [compactCards,setCompactCards] = useState(()=>readPreference("boardCards","compact")!=="full");
  const [visibleByColumn,setVisibleByColumn] = useState<Record<string,number>>({});
  useEffect(()=>setVisibleByColumn({}),[selection.projectId,query,grouping]);
  const [contextState,setContextState] = useState<{projectId:string;value:ProjectContext}>();
  const [contextError,setContextError] = useState<{projectId:string;message:string}>();
  const [contextRefresh,setContextRefresh] = useState(0);
  const context = contextState?.projectId === selection.projectId ? contextState.value : undefined;
  const currentContextError = contextError?.projectId === selection.projectId ? contextError.message : "";
  useEffect(()=>{let current=true;setContextError(undefined);void projectContext(selection.projectId).then(data=>{if(current){setContextState({projectId:selection.projectId,value:data});setContextError(undefined);}}).catch(cause=>{if(current)setContextError({projectId:selection.projectId,message:String(cause)});});return()=>{current=false;};},[selection.projectId,contextRefresh]);
  const [selectedRun, setSelectedRun] = useState(selection.runId ?? "");
  const request = useRef(0);
  async function refresh() {
    const seq = ++request.current;
    setBusy(true);
    setErrorState(undefined);
    try {
      const items = await listTasks(selection.projectId);
      if (seq === request.current) setTaskState({projectId:selection.projectId,items});
    } catch (cause) {
      if (seq === request.current) setErrorState({projectId:selection.projectId,message:String(cause)});
    } finally {
      if (seq === request.current) setBusy(false);
    }
  }
  useEffect(() => {
    if(selection.view !== "prompt" && selection.view !== "project-settings" && selection.view !== "task") void refresh();
    return () => { request.current++; };
  }, [selection.projectId, selection.view]);
  const remoteColumns = trackerColumns(tasks, query);
  const hasRemoteTasks = trackerColumns(tasks).some(column => column.tasks.length > 0);
  const trackers = context?.board?.trackers ?? [];
  const hasNoExternalTrackers = Boolean(context?.board && (
    trackers.length === 0 ||
    trackers.every(tracker=>tracker.provider === "local") ||
    (trackers.length === 1 && !trackers[0].provider && context.issueTracker === "local")
  ));
  const boardContextPending = !context && !currentContextError;
  const localOnlyProject = Boolean(
    hasNoExternalTrackers &&
    !hasRemoteTasks,
  );
  const visibleTasks = remoteColumns.flatMap(column=>column.tasks);
  const columns = grouping === "stage" ? [...stages.map(stage=>({name:stageLabels[stage],tasks:visibleTasks.filter(task=>taskStage(task,context?.board)===stage)})),...(visibleTasks.some(task=>!taskStage(task,context?.board))?[{name:"Étape inconnue",tasks:visibleTasks.filter(task=>!taskStage(task,context?.board))}]:[])] : remoteColumns;
  const count = columns.reduce((n, c) => n + c.tasks.length, 0);
  return (
    <>
      <div className="workspace-toolbar">
        <div>
          <h1>{project?.name ?? "Projet"}</h1>
          <p title={workingDirectory}>
            {workingDirectory || (selection.runId ? "Dossier de l’exécution en cours de chargement…" : "Espace de travail du projet")}
          </p>
        </div>
        <div className="toolbar-actions">
          {project && (
            <>
              {selection.view !== "project-settings" && <button onClick={() => navigate(project, "project-settings")}><Icon name="settings" />Configurer le projet</button>}
              {selection.view !== "board" && <button onClick={() => navigate(project, "board")}>
                <Icon name="board" />
                Tableau
              </button>}
              {(selection.view !== "prompt" || selection.runId) && <button
                className={selection.view === "board" ? "primary" : "subtle"}
                onClick={() => navigate(project, "prompt")}
              >
                <Icon name="plus" />
                Nouveau prompt
              </button>}
            </>
          )}
          {selection.view === "board" && <button
            className="icon-button"
            title="Actualiser"
            aria-label="Actualiser le tableau"
            disabled={busy}
            onClick={() => void refresh()}
          >
            <Icon name="refresh" />
          </button>}
        </div>
      </div>
      {selection.view === "board" && error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {selection.view === "project-settings" ? <ProjectProfiles key={selection.projectId} projectId={selection.projectId} after={<section className="project-advanced" aria-label="Options avancées du projet"><h2>Options avancées</h2><details className="additional-folder-settings"><summary>Dossiers supplémentaires<small>Contexte partagé entre les conversations du projet</small></summary><AttachedFolderSettings projectId={selection.projectId} onChanged={()=>void monitor.refresh()}/></details><AdvancedFolders projectId={selection.projectId} onChanged={()=>void monitor.refresh()}/><WorkflowSettings projectId={selection.projectId}/><OrchestrationSettings projectId={selection.projectId} onSupervise={()=>{if(project)navigate(project,"orchestration");}}/></section>}><>{project && <ProjectFolderSetup variant="settings" project={project} onMapped={path=>onFolderMapped(project.id,path)} />}<ProjectTrackerSetup projectId={selection.projectId} onChanged={() => {setContextRefresh(value=>value+1);onProjectsChanged();}} /></></ProjectProfiles> : selection.view === "task" && selection.taskId ? <Suspense fallback={<p>Ouverture du workflow…</p>}><WorkflowTask projectId={selection.projectId} taskId={selection.taskId} onBack={()=>{if(project)navigate(project,"board");}} onSetup={()=>{if(project)navigate(project,"settings");}}/></Suspense> : selection.view === "board" ? (
        <div className="board-workspace">
          {error && tasks.length > 0 && <p className="hint" role="status">La dernière actualisation a échoué. Les tickets affichés proviennent de la lecture précédente.</p>}
          {!boardContextPending && !currentContextError && !localOnlyProject && <div className="board-toolbar">
            <label className="search">
              <Icon name="search" />
              <input
                aria-label="Rechercher des tickets"
                placeholder="Rechercher un ticket…"
                value={query}
                onChange={(e) => setQueryState({projectId:selection.projectId,value:e.target.value})}
              />
            </label>
            <label>Regrouper<select aria-label="Regrouper les tickets" value={grouping} onChange={e=>setGrouping(e.target.value)}><option value="tracker">Par statut tracker</option><option value="stage">Par étape du workflow</option></select></label>
            <button type="button" className="board-density" aria-pressed={compactCards} onClick={()=>setCompactCards(value=>{const next=!value;savePreference("boardCards",next?"compact":"full");return next;})}>Cartes compactes</button>
            <span className="badge">{count} tickets</span>
            <span className="muted">Statuts du tracker · lecture seule</span>
          </div>}
          {busy && (
            <p className="hint" role="status">
              Actualisation des tickets…
            </p>
          )}
          {boardContextPending && !busy && <p className="hint" role="status">Chargement de la configuration du projet…</p>}
          {currentContextError && <div className="error" role="alert">Configuration du tableau indisponible : {currentContextError} <button onClick={()=>setContextRefresh(value=>value+1)}>Réessayer</button></div>}
          <div className="board" role="region" aria-label="Colonnes de tickets, défilement horizontal" tabIndex={0}>
            {columns.map((column) => (
              <section className="column" key={column.name}>
                <h2>
                  <span className="dot" />
                  {column.name}
                  <span>{column.tasks.length}</span>
                </h2>
                {visibleBoardTasks(column.tasks,activeTaskRuns,visibleByColumn[column.name] ?? 40).map((task) => {
                  const taskRun = activeTaskRuns.get(task.id);
                  return <article className={`task ${compactCards?"compact":""}`} key={task.id}>
                    <div className="task-meta">
                      <span>{task.key || task.id}</span>
                      <span>{task.source}</span>
                    </div>
                    <h3><button className="task-open" title={task.title} onClick={()=>{if(project)navigate(project,"task",undefined,task.id);}}>{task.title}</button></h3>
                    <p className="task-stage">{taskStage(task,context?.board)?stageLabels[taskStage(task,context?.board)!]:"Étape non déterminée"}</p>
                    {taskRun && isActive(taskRun) && <p className="task-run-state" role="status">{skillLabels[taskRun.skill] || taskRun.skill} · {statusLabel(taskRun)}</p>}
                    <footer>
                      <span>{task.assignee || "Non assigné"}</span>
                      {project && (
                        <button
                          title="Ouvrir le workflow du ticket"
                          aria-label={`Ouvrir le workflow de ${task.key || task.title}`}
                          className="icon-button"
                          onClick={() =>
                            navigate(project, "task", undefined, task.id)
                          }
                        >
                          <Icon name="chevron" />
                        </button>
                      )}
                    </footer>
                  </article>
                })}
                {column.tasks.length > (visibleByColumn[column.name] ?? 40) && <button
                  className="board-show-more"
                  aria-label={`Afficher les tickets suivants dans ${column.name}`}
                  onClick={()=>setVisibleByColumn(previous=>({...previous,[column.name]:(previous[column.name]??40)+40}))}
                >Afficher {Math.min(40,column.tasks.length-(visibleByColumn[column.name]??40))} suivants · {Math.min(visibleByColumn[column.name]??40,column.tasks.length)} sur {column.tasks.length}</button>}
              </section>
            ))}
          </div>
          {!busy && !error && !currentContextError && !boardContextPending && !count && (
            <div className="empty">
              <Icon name="board" />
              <h2>{localOnlyProject ? project?.path ? "Votre projet local est prêt" : "Choisissez un dossier pour commencer" : "Aucun ticket distant"}</h2>
              <p>{localOnlyProject ? project?.path ? "Aucun tracker n’est associé. Lancez un prompt pour travailler dans votre dossier local." : "Lancez un prompt libre dans ce projet, sans configurer de tracker." : query ? "Essayez une autre recherche ou vérifiez la synchronisation du projet." : project?.path ? "Le tracker ne remonte aucun ticket. Vous pouvez lancer un prompt dans le dossier local associé." : "Vérifiez la synchronisation et le périmètre du tracker."}</p>
              {localOnlyProject && project && !project.path && <ProjectFolderSetup variant="board" project={project} onMapped={path=>onFolderMapped(project.id,path)} />}
            </div>
          )}
        </div>
      ) : selection.view === "prompt" ? (
        selection.runId || project?.path?.trim() ? <Suspense fallback={<p>Ouverture du prompt…</p>}>
          <ProjectPrompt key={`${selection.projectId}:${selection.runId??"new"}`} directory={workingDirectory} folders={currentRun?.folders} onFoldersChanged={()=>void monitor.refresh()} onTitleChanged={()=>void monitor.refresh()} projectId={selection.projectId} runId={selection.runId} onCreated={id => { if (project) navigate(project, "prompt", id); }} />
        </Suspense> : project ? <ProjectFolderSetup project={project} onMapped={path=>onFolderMapped(project.id,path)} /> : <p>Chargement du projet…</p>
      ) : (
        <Supervisor
          projectId={selection.projectId}
          tasks={tasks}
          onChanged={() => void refresh()}
          view={selection.view as "launch" | "executions" | "orchestration"}
          selectedRun={selectedRun}
          onSelectRun={(id) => {
            if (project && id) navigate(project, "executions", id);
            else setSelectedRun(id);
          }}
          initialTaskId={selection.taskId}
          active
        />
      )}
    </>
  );
}

function visibleBoardTasks(tasks:Task[],activeRuns:Map<string,Run>,limit:number):Task[]{
 const active:Task[]=[];
 const others:Task[]=[];
 for(const task of tasks){
  const run=activeRuns.get(task.id);
  (run&&isActive(run)?active:others).push(task);
 }
 return active.concat(others).slice(0,limit);
}

function RunNavigation({run,selected,onClick}:{run:Run;selected:boolean;onClick:()=>void}){
 if(run.conversation)return <ConversationNavigation run={run} selected={selected} onClick={onClick} />;
 return <button className={`sidebar-run ${selected ? "selected" : ""}`} title={statusLabel(run)} onClick={onClick}><span className={`dot ${isActive(run) ? "live" : ""}`} /><span>{run.taskKey || run.skill || "Exécution"}</span><small>{run.engineName || statusLabel(run)}</small></button>;
}
function ConversationNavigation({run,selected,onClick}:{run:Run;selected:boolean;onClick:()=>void}){
 // Subscribe to already loaded transcripts; navigation never fetches histories.
 const {snapshot,loaded,error}=useConversation(run.id);
 const title=useMemo(()=>{
  if(run.title || snapshot.title)return run.title || snapshot.title;
  for(const line of snapshot.events ?? []){try{const event=JSON.parse(line);if(event.kind==="user"&&typeof event.text==="string")return event.text.trim().split("\n")[0].slice(0,100);}catch{/* Keep provider metadata as the fallback. */}}
  return run.prompt?.trim().split("\n")[0] || "Conversation";
 },[snapshot.events,snapshot.title,run.prompt,run.title]);
 const provider=run.provider==="claude"?"Claude Code":run.provider==="codex"?"Codex":run.engineName||"Assistant";
 const activity=error?"Suivi interrompu":loaded&&snapshot.id===run.id?snapshot.approvals?.length?"Votre intervention est requise":snapshot.busy?"En cours":snapshot.readOnly?"Lecture seule":"Prête":statusLabel(run);
 return <button className={`sidebar-run conversation-navigation ${selected ? "selected" : ""}`} onClick={onClick} title={`${provider}${run.model ? ` · ${run.model}` : ""} · ${activity}`}><Icon name="chat" /><span className="conversation-navigation-text"><span>{title}</span><small>{provider}{run.model && ` · ${run.model}`} · {activity}</small></span></button>;
}
