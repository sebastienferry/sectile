import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listProjects, listTasks } from "./lib/agent";
import { trackerColumns, type Project, type Task } from "./domain/board";
import { isActive, projectRuns, statusLabel, type Run } from "./domain/execution";
import {
  ExecutionProvider,
  useExecutionMonitor,
} from "./components/ExecutionMonitor";
import { Supervisor } from "./components/Supervisor";
import { ProjectFolderSetup } from "./components/ProjectFolderSetup";
import { AdvancedFolders } from "./components/AdvancedFolders";
import { WorkflowSettings } from "./components/WorkflowSettings";
import { OrchestrationSettings } from "./components/OrchestrationSettings";

import { projectContext, type ProjectContext } from "./lib/workspace";
import { stages, stageLabels, taskStage } from "./domain/workflow";
import { AttachedFolderSettings } from "./components/FolderContext";
import { ProjectProfiles } from "./components/ProjectProfiles";
import { LocalSettings } from "./components/LocalSettings";
import { Icon } from "./components/Icon";
import { useAppearance } from "./lib/appearance";
import { useConversation } from "./lib/conversation-controller";
import "./App.css";
import "./workspace-polish.css";

const WorkflowTask = lazy(()=>import("./components/WorkflowTask").then(module=>({default:module.WorkflowTask})));
const ProjectPrompt = lazy(() => import("./components/ProjectPrompt"));

type View = "prompt" | "board" | "executions" | "orchestration" | "launch" | "settings" | "project-settings" | "task";
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
  const [filter, setFilter] = useState("");
  const [appearance, setAppearance] = useAppearance();
  const { snapshot, fresh, refresh: refreshExecutions } = useExecutionMonitor();
  const request = useRef(0);
  const initialized = useRef(false);
  const navigate = useCallback(
    (project: Project, view: View, runId?: string, taskId?: string) => {
      setCurrent({ id: `${project.id}:${view}:${runId ?? ""}:${taskId ?? ""}`, projectId: project.id, view, runId, taskId });
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
        const first = items[0];
        const id = `${first.id}:board:`;
        setCurrent((previous) => previous ?? { id, projectId: first.id, view: "board" });
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
  const connected = fresh && snapshot?.status.connected;
  return (
    <div className="desktop-shell">
      <header className="app-header">
        <div className="header-brand">
          <button
            className="icon-button"
            title="Afficher ou masquer les projets"
            aria-label="Afficher ou masquer les projets"
            aria-expanded={sidebar}
            onClick={() => setSidebar((v) => !v)}
          >
            <Icon name="panel" />
          </button>
          <span className="brand-mark"><Icon name="terminal" /></span>
          <strong>
            Sectile <span>Desktop</span>
          </strong>

        </div>
        <div className="header-actions"><span className="connection-pill"><span className={`dot ${connected ? "live" : ""}`} />{connected ? "Connecté" : "Hors ligne"}</span>
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
          <button
            className="icon-button"
            aria-label="Actualiser les projets"
            title="Actualiser les projets"
            disabled={busy}
            onClick={() => void refreshProjects()}
          >
            <Icon name="refresh" />
          </button>
          <button className="icon-button" title="Paramètres" aria-label="Paramètres" onClick={()=>navigate({id:"settings",name:"Paramètres"},"settings")}><Icon name="settings" /></button>
        </div>
      </header>
      <div className="desktop-body">
        {sidebar && (
          <aside className="sidebar">
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
                          {runs.slice(0,8).map(run => <RunNavigation key={run.id} run={run} selected={current?.runId===run.id} onClick={()=>navigate(project,run.conversation ? "prompt" : "executions",run.id)} />)}
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
                  {fresh ? "Exécution locale" : "Connexion en cours"}
                </small>
              </div>

            </footer>
          </aside>
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
            <section className="workspace-pane">
              {current.view === "settings" ? (
                <LocalSettings onConfigureProject={id=>{const project=projects.find(p=>p.id===id);if(project)navigate(project,"project-settings");}} onOpenFolder={() => void openFolder()} onChanged={() => { setProjects([]); void refreshProjects(); void refreshExecutions(); }} />
              ) : (
                <ProjectPane
                  key={current.id}
                  selection={current}
                  project={projects.find((p) => p.id === current.projectId)}
                  navigate={navigate}
                  onFolderMapped={(id,path)=>setProjects(items=>items.map(item=>item.id===id?{...item,path}:item))}
                />
              )}
            </section>
          )}
          <footer className="statusbar">
            <span>
              {connected ? "Connecté à l’agent local" : "Connexion à vérifier"}
            </span>
            <span>Données et exécution locales · 0.1.0</span>
          </footer>
        </main>
      </div>
    </div>
  );
}
function ProjectPane({
  selection,
  project,
  navigate,
  onFolderMapped,
}: {
  selection: WorkspaceView;
  project?: Project;
  onFolderMapped: (projectId:string,path:string) => void;
  navigate: (
    project: Project,
    view: View,
    runId?: string,
    taskId?: string,
  ) => void;
}) {
  const monitor = useExecutionMonitor();
  const currentRun = monitor.snapshot?.runs.find(run=>run.id===selection.runId);
  const workingDirectory = selection.runId ? currentRun?.directory : project?.path;
  const [tasks, setTasks] = useState<Task[]>([]);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [query, setQuery] = useState("");
  const [grouping,setGrouping] = useState("tracker");
  const [context,setContext] = useState<ProjectContext>();
  useEffect(()=>{let current=true;void projectContext(selection.projectId).then(data=>{if(current)setContext(data);}).catch(()=>{});return()=>{current=false;};},[selection.projectId]);
  const [selectedRun, setSelectedRun] = useState(selection.runId ?? "");
  const request = useRef(0);
  async function refresh() {
    const seq = ++request.current;
    setBusy(true);
    setError("");
    try {
      const items = await listTasks(selection.projectId);
      if (seq === request.current) setTasks(items);
    } catch (cause) {
      if (seq === request.current) setError(String(cause));
    } finally {
      if (seq === request.current) setBusy(false);
    }
  }
  useEffect(() => {
    if(selection.view !== "prompt" && selection.view !== "project-settings" && selection.view !== "task") void refresh();
    return () => { request.current++; };
  }, [selection.projectId, selection.view]);
  const remoteColumns = trackerColumns(tasks, query);
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
              <button onClick={() => navigate(project, "project-settings")}><Icon name="settings" />Configurer le projet</button>
              <button onClick={() => navigate(project, "board")}>
                <Icon name="board" />
                Tableau
              </button>
              <button
                className={selection.view === "board" ? "primary" : "subtle"}
                onClick={() => navigate(project, "prompt")}
              >
                <Icon name="plus" />
                Nouveau prompt
              </button>
            </>
          )}
          {selection.view !== "prompt" && selection.view !== "project-settings" && <button
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
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {selection.view === "project-settings" ? <ProjectProfiles key={selection.projectId} projectId={selection.projectId} after={<section className="project-advanced" aria-label="Options avancées du projet"><h2>Options avancées</h2><details className="additional-folder-settings"><summary>Dossiers supplémentaires<small>Contexte partagé entre les conversations du projet</small></summary><AttachedFolderSettings projectId={selection.projectId} onChanged={()=>void monitor.refresh()}/></details><AdvancedFolders projectId={selection.projectId} onChanged={()=>void monitor.refresh()}/><WorkflowSettings projectId={selection.projectId}/><OrchestrationSettings projectId={selection.projectId} onSupervise={()=>{if(project)navigate(project,"orchestration");}}/></section>}>{project && <ProjectFolderSetup variant="settings" project={project} onMapped={path=>onFolderMapped(project.id,path)} />}</ProjectProfiles> : selection.view === "task" && selection.taskId ? <Suspense fallback={<p>Ouverture du workflow…</p>}><WorkflowTask projectId={selection.projectId} taskId={selection.taskId} onBack={()=>{if(project)navigate(project,"board");}} onSetup={()=>{if(project)navigate(project,"settings");}}/></Suspense> : selection.view === "board" ? (
        <div className="board-workspace">
          <div className="board-toolbar">
            <label className="search">
              <Icon name="search" />
              <input
                aria-label="Rechercher des tickets"
                placeholder="Rechercher un ticket…"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </label>
            <label>Regrouper<select aria-label="Regrouper les tickets" value={grouping} onChange={e=>setGrouping(e.target.value)}><option value="tracker">Par statut tracker</option><option value="stage">Par étape du workflow</option></select></label>
            <span className="badge">{count} tickets</span>
            <span className="muted">Statuts du tracker · lecture seule</span>
          </div>
          {busy && (
            <p className="hint" role="status">
              Actualisation des tickets…
            </p>
          )}
          <div className="board">
            {columns.map((column) => (
              <section className="column" key={column.name}>
                <h2>
                  <span className="dot" />
                  {column.name}
                  <span>{column.tasks.length}</span>
                </h2>
                {column.tasks.map((task) => (
                  <article className="task" key={task.id}>
                    <div className="task-meta">
                      <span>{task.key || task.id}</span>
                      <span>{task.source}</span>
                    </div>
                    <h3><button className="task-open" onClick={()=>{if(project)navigate(project,"task",undefined,task.id);}}>{task.title}</button></h3>
                    <p className="task-stage">{taskStage(task,context?.board)?stageLabels[taskStage(task,context?.board)!]:"Étape non déterminée"}</p>
                    <footer>
                      <span>{task.assignee || "Non assigné"}</span>
                      {project && (
                        <button
                          title="Ouvrir le workflow du ticket"
                          aria-label={`Lancer ${task.key || task.title}`}
                          className="icon-button"
                          onClick={() =>
                            navigate(project, "task", undefined, task.id)
                          }
                        >
                          <Icon name="play" />
                        </button>
                      )}
                    </footer>
                  </article>
                ))}
              </section>
            ))}
          </div>
          {!busy && !error && !count && (
            <div className="empty">
              <Icon name="board" />
              <h2>{project?.path && !query ? "Votre projet, prêt à démarrer" : "Aucun ticket distant"}</h2>
              <p>{project?.path && !query ? "Posez une question, explorez le code ou lancez une tâche dans votre dossier local." : "Essayez une autre recherche ou vérifiez la synchronisation du projet."}</p>
              {project && !query && <button className="primary" onClick={()=>navigate(project,"prompt")}><Icon name="chat" />Commencer une conversation</button>}
            </div>
          )}
        </div>
      ) : selection.view === "prompt" ? (
        selection.runId || project?.path?.trim() ? <Suspense fallback={<p>Ouverture du prompt…</p>}>
          <ProjectPrompt directory={workingDirectory} folders={currentRun?.folders} onFoldersChanged={()=>void monitor.refresh()} onTitleChanged={()=>void monitor.refresh()} projectId={selection.projectId} runId={selection.runId} onCreated={id => { if (project) navigate(project, "prompt", id); }} />
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

function RunNavigation({run,selected,onClick}:{run:Run;selected:boolean;onClick:()=>void}){
 if(run.conversation)return <ConversationNavigation run={run} selected={selected} onClick={onClick} />;
 return <button className={`sidebar-run ${selected ? "selected" : ""}`} title={statusLabel(run)} onClick={onClick}><span className={`dot ${isActive(run) ? "live" : ""}`} /><span>{run.taskKey || run.skill || "Exécution"}</span><small>{run.engineName || statusLabel(run)}</small></button>;
}
function ConversationNavigation({run,selected,onClick}:{run:Run;selected:boolean;onClick:()=>void}){
 // Subscribe to already loaded transcripts; navigation never fetches histories.
 const {snapshot}=useConversation(run.id);
 const title=useMemo(()=>{
  if(run.title || snapshot.title)return run.title || snapshot.title;
  for(const line of snapshot.events ?? []){try{const event=JSON.parse(line);if(event.kind==="user"&&typeof event.text==="string")return event.text.trim().split("\n")[0].slice(0,100);}catch{/* Keep provider metadata as the fallback. */}}
  return run.prompt?.trim().split("\n")[0] || "Conversation";
 },[snapshot.events,snapshot.title,run.prompt,run.title]);
 const provider=run.provider==="claude"?"Claude Code":run.provider==="codex"?"Codex":run.engineName||"Assistant";
 const date=new Date(run.createdAt);
 const time=Number.isNaN(date.getTime())?"":new Intl.DateTimeFormat("fr",{hour:"2-digit",minute:"2-digit"}).format(date);
 return <button className={`sidebar-run conversation-navigation ${selected ? "selected" : ""}`} onClick={onClick} title={`${provider}${run.model ? ` · ${run.model}` : ""} · ${statusLabel(run)}`}><Icon name="chat" /><span className="conversation-navigation-text"><span>{title}</span><small>{provider}{run.model && ` · ${run.model}`}{time && ` · ${time}`}</small></span></button>;
}
