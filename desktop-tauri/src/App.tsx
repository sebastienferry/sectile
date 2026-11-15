import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState, type CSSProperties } from "react";
import { invoke } from "@tauri-apps/api/core";
import { listen } from "@tauri-apps/api/event";
import { executionSnapshot, getTask, launchTask, listProjects, listTasks, stopRun } from "./lib/agent";
import { trackerColumns, type Project, type Task } from "./domain/board";
import { epicColorHex } from "./domain/epic-color";
import { isActive, launchInput, projectRuns, readTaskLaunchView, saveTaskLaunchView, statusLabel, type Run, type TaskLaunchView } from "./domain/execution";
import packageJson from "../package.json";
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

import { openExternalLink, projectContext, providerReadiness, renameProject, type ProjectContext } from "./lib/workspace";
import { engineMark, nextSkill, skillLabels, stages, stageLabels, taskStage } from "./domain/workflow";
import { DEFAULT_SORT, nextSort, orderedTasks, type SortState } from "./domain/task-list-order";
import { taskFilterPredicate } from "./domain/task-filters";
import { AttachedFolderSettings } from "./components/FolderContext";
import { ProjectProfiles } from "./components/ProjectProfiles";
import { TaskEditor } from "./components/TaskEditor";
import { LocalSettings } from "./components/LocalSettings";
import { ModeOnboarding } from "./components/ModeOnboarding";
import { deleteLocalProject, enableLocalRuntime, localSetupStatus, prepareLocalUpdate } from "./lib/local";
import { Icon } from "./components/Icon";
import { readPreference, savePreference, useAppearance, useFontSize } from "./lib/appearance";
import {
  createWorkspaceDestination,
  resolveLaunchDestination,
  serializeWorkspaceDestination,
  type WorkspaceDestination,
  type WorkspaceViewName,
} from "./lib/workspace-preference";
import { useConversation } from "./lib/conversation-controller";
import { renameConversation } from "./lib/conversation";
import { conversationGroups, conversationTicketGroupKey, conversationTitle, deduplicateTicketConversations, type ConversationGrouping } from "./domain/conversation-navigation";
import { pullRequestLabel, pullRequestStateLabel, taskPullRequests } from "./domain/pull-requests";
import { CommandPalette } from "./components/CommandPalette";
import { shortcutModifier, workspaceShortcut } from "./lib/workspace-shortcuts";
import "./App.css";

const WorkflowTask = lazy(()=>import("./components/WorkflowTask").then(module=>({default:module.WorkflowTask})));
const ProjectPrompt = lazy(() => import("./components/ProjectPrompt"));
const StartOverDialog = lazy(() => import("./components/StartOverDialog"));

const OPTIONAL_LIST_COLUMNS = [
  ["parent", "Macro"],
  ["issueType", "Type"],
  ["stage", "Status"],
  ["priority", "Priority"],
  ["sprint", "Sprint"],
  ["assignee", "Assignee"],
  ["engine", "Engine"],
  ["pr", "PR"],
] as const;
type OptionalListColumn = (typeof OPTIONAL_LIST_COLUMNS)[number][0];
const DEFAULT_LIST_COLUMNS: OptionalListColumn[] = OPTIONAL_LIST_COLUMNS.map(([id]) => id);

function readListColumns(): OptionalListColumn[] {
  const raw = readPreference("ticketListColumns", "");
  if (!raw) return DEFAULT_LIST_COLUMNS;
  try {
    const parsed = JSON.parse(raw);
    if (Array.isArray(parsed)) {
      const valid = new Set<string>(DEFAULT_LIST_COLUMNS);
      return parsed.filter((col): col is OptionalListColumn => typeof col === "string" && valid.has(col));
    }
  } catch {
    /* ignore invalid saved column preference */
  }
  return DEFAULT_LIST_COLUMNS;
}

type View = WorkspaceViewName | "settings" | "ticket-editor";
interface WorkspaceView {
  id: string;
  projectId: string;
  view: View;
  runId?: string;
  taskId?: string;
  settingsSection?: "connections" | "engines";
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
  const [windowWidth, setWindowWidth] = useState(() => window.innerWidth);
  useEffect(() => {
    const onResize = () => setWindowWidth(window.innerWidth);
    window.addEventListener("resize", onResize);
    return () => window.removeEventListener("resize", onResize);
  }, []);
  const sidebarMaximum = Math.max(200, Math.min(480, windowWidth - 500));
  const visibleSidebarWidth = Math.min(sidebarWidth, sidebarMaximum);
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
      if (project?.mappingError) { setUnmapped(project); setFolderError(`The project was created, but the folder was not mapped: ${project.mappingError}`); await refreshProjects(); }
      else if (project) { await refreshProjects(); navigate(project, "prompt"); }
    } catch (cause) { setFolderError(String(cause)); }
    finally { folderBusy.current = false; setOpeningFolder(false); }
  }
  const [projects, setProjects] = useState<Project[]>([]);
  const [unconfigured, setUnconfigured] = useState(false);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [current, setCurrent] = useState<WorkspaceView | null>(null);
  const [sidebar, setSidebar] = useState(true);
  const currentRef = useRef(current);
  currentRef.current = current;
  const workspaceBeforeSettings = useRef<WorkspaceView | null>(null);
  const [filter, setFilter] = useState("");
  const [projectTaskSort, setProjectTaskSort] = useState<Record<string, ConversationGrouping>>(() => {
    try {
      const saved = JSON.parse(readPreference("projectTaskSort", "{}"));
      if (typeof saved === "object" && saved) return saved;
    } catch {}
    const legacy = readPreference("conversationGrouping", "recent");
    return legacy && legacy !== "recent" ? { default: legacy as ConversationGrouping } : {};
  });

  const getProjectSort = (projectId: string): ConversationGrouping => {
    return projectTaskSort[projectId] || projectTaskSort["default"] || "recent";
  };

  const toggleProjectSort = (projectId: string, sort: "created" | "id" | "stage") => {
    setProjectTaskSort(prev => {
      const current = prev[projectId] || prev["default"] || "recent";
      const next: ConversationGrouping = current === sort ? "recent" : sort;
      const updated: Record<string, ConversationGrouping> = { ...prev, [projectId]: next };
      savePreference("projectTaskSort", JSON.stringify(updated));
      savePreference("conversationGrouping", next);
      return updated;
    });
  };
  const [visibleConversationCount, setVisibleConversationCount] = useState(8);
  const [archivedConversationIds, setArchivedConversationIds] = useState<string[]>(() => {
    try { const saved = JSON.parse(readPreference("archivedConversations", "[]")); return Array.isArray(saved) ? saved.filter((id): id is string => typeof id === "string") : []; }
    catch { return []; }
  });
  const [showArchivedConversations, setShowArchivedConversations] = useState(false);
  const [renamingRunId, setRenamingRunId] = useState<string | null>(null);
  const [renamingDraft, setRenamingDraft] = useState("");
  const [conversationActionError, setConversationActionError] = useState("");
  const [stoppingConversationId, setStoppingConversationId] = useState("");
  const [renamingProjectId, setRenamingProjectId] = useState<string | null>(null);
  const [renamingProjectDraft, setRenamingProjectDraft] = useState("");
  const [projectActionError, setProjectActionError] = useState<{ projectId: string; message: string } | null>(null);
  const renamingProjectSubmitting = useRef(false);
  const [deletingProject, setDeletingProject] = useState<Project | null>(null);
  const [deletingProjectPending, setDeletingProjectPending] = useState(false);
  const [deletingProjectError, setDeletingProjectError] = useState("");
  const [navigationTasks, setNavigationTasks] = useState<{projectId:string;items:Task[]}>();
  const [navigationContext, setNavigationContext] = useState<{projectId:string;value:ProjectContext}>();
  const [appearance, setAppearance] = useAppearance();
  useFontSize();
  const { snapshot, fresh, refresh: refreshExecutions } = useExecutionMonitor();
  function setArchived(runId:string, archived:boolean) {
    const targetRun = snapshot?.runs.find(item => item.id === runId);
    const targetGroupKey = targetRun ? conversationTicketGroupKey(targetRun) : "";
    const matchingIds = targetRun && targetGroupKey
      ? (snapshot?.runs ?? [])
          .filter(item => item.conversation && conversationTicketGroupKey(item) === targetGroupKey)
          .map(item => item.id)
      : [runId];
    if (!matchingIds.includes(runId)) matchingIds.push(runId);
    setArchivedConversationIds(previous=>{
      const next=archived?[...new Set([...previous,...matchingIds])]:previous.filter(id=>!matchingIds.includes(id));
      savePreference("archivedConversations",JSON.stringify(next));
      return next;
    });
    setConversationActionError("");
    if(archived && currentRef.current?.runId && matchingIds.includes(currentRef.current.runId)){
      const project=projects.find(item=>item.id===currentRef.current?.projectId);
      if(project)navigate(project,"board");
    }
  }
  async function stopConversation(run:Run) {
    if(stoppingConversationId || !fresh || !isActive(run))return;
    setStoppingConversationId(run.id);setConversationActionError("");
    try {await stopRun(run.id);await refreshExecutions();}
    catch {setConversationActionError("Stop not confirmed. Refresh status before trying again.");}
    finally {setStoppingConversationId("");}
  }
  async function submitProjectRename(project: Project, draft: string) {
    if (renamingProjectSubmitting.current) return;
    const nextName = draft.trim();
    setRenamingProjectId(null);
    if (!nextName || nextName === project.name) return;
    renamingProjectSubmitting.current = true;
    setProjectActionError(null);
    const previousName = project.name;
    setProjects(previous => previous.map(item => item.id === project.id ? { ...item, name: nextName } : item));
    try {
      await renameProject(project.id, nextName);
      await refreshProjects();
    } catch {
      setProjects(previous => previous.map(item => item.id === project.id ? { ...item, name: previousName } : item));
      setProjectActionError({ projectId: project.id, message: "Unable to rename project." });
    } finally {
      renamingProjectSubmitting.current = false;
    }
  }
  async function confirmDeleteProject() {
    if (!deletingProject || deletingProjectPending) return;
    setDeletingProjectPending(true);
    setDeletingProjectError("");
    try {
      await deleteLocalProject(deletingProject.id);
      const removedId = deletingProject.id;
      setDeletingProject(null);
      const remaining = projects.filter(p => p.id !== removedId);
      setProjects(remaining);
      if (currentRef.current?.projectId === removedId) {
        if (remaining[0]) navigate(remaining[0], "board");
        else setCurrent({ id: "settings", projectId: "settings", view: "settings" });
      }
      await refreshProjects();
      await refreshExecutions();
    } catch (cause) {
      setDeletingProjectError(String(cause));
    } finally {
      setDeletingProjectPending(false);
    }
  }
  const request = useRef(0);
  const initialized = useRef(false);
  const pendingRunRestore = useRef<WorkspaceDestination | null>(null);
  const navigate = useCallback(
    (project: Project, view: View, runId?: string, taskId?: string, settingsSection?: "connections" | "engines") => {
      if (view === "settings" && currentRef.current?.view !== "settings") {
        workspaceBeforeSettings.current = currentRef.current;
      }
      const destination: WorkspaceView = {
        id: `${project.id}:${view}:${runId ?? ""}:${taskId ?? ""}:${settingsSection ?? ""}`,
        projectId: project.id,
        view,
        runId,
        taskId,
        settingsSection,
      };
      if (view !== "settings" && view !== "ticket-editor") {
        savePreference("workspace", serializeWorkspaceDestination(createWorkspaceDestination(project.id, view, runId, taskId)));
      }
      setCurrent(destination);
    }, []);
  const returnToWorkspace = useCallback(() => {
    if (currentRef.current?.view === "settings") {
      const previous = workspaceBeforeSettings.current;
      workspaceBeforeSettings.current = null;
      if (previous && projects.some(project => project.id === previous.projectId)) {
        setCurrent(previous);
      } else if (projects[0]) {
        navigate(projects[0], "board");
      }
      return;
    }
    setSidebar(value => !value);
  }, [navigate, projects]);
  async function handleConfigured() {
    setUnconfigured(false);
    await refreshProjects();
    await refreshExecutions();
  }
  async function refreshProjects() {
    const seq = ++request.current;
    setBusy(true);
    setError("");
    try {
      const items = await listProjects();
      if (seq !== request.current) return;
      setProjects(items);
      setUnconfigured(false);
      if (!initialized.current && items.length) {
        initialized.current = true;
        const destination = resolveLaunchDestination(readPreference("workspace", ""), items);
        if (destination?.runId) pendingRunRestore.current = destination;
        setCurrent((previous) => previous ?? destination);
      }
      if (!items.length) {
        const status = await localSetupStatus().catch(() => null);
        if (seq !== request.current) return;
        if (status && (status.mode === "unconfigured" || status.enabled === false)) {
          setUnconfigured(true);
          return;
        }
        setCurrent(previous => previous ?? { id: "settings", projectId: "settings", view: "settings" });
      }
    } catch (cause) {
      if (seq === request.current) {
        const status = await localSetupStatus().catch(() => null);
        if (seq !== request.current) return;
        if (status && (status.mode === "unconfigured" || status.enabled === false)) {
          setUnconfigured(true);
          return;
        }
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
  const runtimeMode = snapshot?.status.runtimeMode ?? "local";
  const isFleet = runtimeMode === "fleet";
  const serverUrl = isFleet ? snapshot?.status.server : undefined;
  const contractError = snapshot?.status.contractError;

  let connectionText: string;
  let connectionTitle: string;

  if (isFleet) {
    connectionText = connected
      ? "Connected"
      : contractError
      ? "Server incompatible"
      : "Not connected";
    connectionTitle = connected
      ? serverUrl
        ? `Open ${serverUrl} in the default browser`
        : "Connected"
      : contractError || snapshot?.status.text || "The local agent does not reach the Sectile server.";
  } else {
    connectionText = connected
      ? "Local"
      : "Companion unavailable";
    connectionTitle = connected
      ? "Local companion active · Ready for executions"
      : "The local companion is unavailable. Restart it from the menu or settings.";
  }
  const agentMenuRef = useRef<HTMLDetailsElement | null>(null);
  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (agentMenuRef.current && !agentMenuRef.current.contains(e.target as Node)) {
        agentMenuRef.current.removeAttribute("open");
      }
      document.querySelectorAll<HTMLDetailsElement>("details.project-action-menu[open]").forEach(menu => {
        if (!menu.contains(e.target as Node)) menu.removeAttribute("open");
      });
    };
    window.addEventListener("mousedown", handleClickOutside);
    return () => window.removeEventListener("mousedown", handleClickOutside);
  }, []);
  const settingsHasWorkspace = current?.view === "settings" && projects.length > 0;
  const showProjectSidebar = !unconfigured && sidebar && current?.view !== "settings";
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
        if (currentRef.current?.view === "settings") {
          returnToWorkspace();
        } else {
          navigate({ id: "settings", name: "Settings" }, "settings");
        }
      } else if (action === "sidebar") {
        setSidebar(value => !value);
      } else if (action === "prompt" && selected) {
        navigate(selected, "prompt");
      } else if (action === "folder") {
        void openFolder();
      } else if (action === "task" && selected) {
        navigate(selected, "ticket-editor");
      }
    }
    window.addEventListener("keydown", onShortcut);
    return () => window.removeEventListener("keydown", onShortcut);
  }, [mac, navigate, paletteOpen, projects, returnToWorkspace]);
  useEffect(() => {
    if (!("__TAURI_INTERNALS__" in window)) return;
    let disposed = false;
    let unlisten: (() => void) | undefined;
    void listen("sectile:open-settings", () => {
      if (currentRef.current?.view === "settings") {
        returnToWorkspace();
      } else {
        navigate({ id: "settings", name: "Settings" }, "settings");
      }
    }).then(stop => { if (disposed) stop(); else unlisten = stop; }).catch(() => {});
    return () => { disposed = true; unlisten?.(); };
  }, [navigate, returnToWorkspace]);
  return (
    <div className="desktop-shell">
      <header className={`app-header ${mac ? "is-macos" : ""}`} data-tauri-drag-region>
        <div className="header-brand">
          {(current?.view !== "settings" || settingsHasWorkspace) && <button
            id="toggle-sidebar"
            className="icon-button"
            title={settingsHasWorkspace ? "Return to workspace" : `Show or hide projects · ${modifier}B`}
            aria-label={settingsHasWorkspace ? "Return to workspace" : "Show or hide projects"}
            aria-keyshortcuts={settingsHasWorkspace ? undefined : mac ? "Meta+B" : "Control+B"}
            aria-expanded={showProjectSidebar}
            onClick={returnToWorkspace}
          >
            <Icon name="panel" />
          </button>}
          <strong id="app-title">
            Sectile Desktop
          </strong>
          <span className="preview-badge" title={`Version ${packageJson.version}`}>
            v{packageJson.version}
          </span>
        </div>
        <div className="header-actions">
          <div className="theme-toggle" aria-label="Appearance">
            {(["system", "light", "dark"] as const).map((value, i) => (
              <button
                key={value}
                className="icon-button"
                aria-label={["System theme", "Light theme", "Dark theme"][i]}
                title={["System", "Light", "Dark"][i]}
                aria-pressed={appearance === value}
                onClick={() => setAppearance(value)}
              >
                <Icon name={(["monitor", "sun", "moon"] as const)[i]} />
              </button>
            ))}
          </div>
          <button className="icon-button" aria-label="Quick navigation" title="Quick navigation · Cmd/Ctrl+K" onClick={() => setPaletteOpen(true)}><Icon name="search" /></button>
          <div id="connection" className="connection-status" data-state={connected ? "on" : "off"} data-mode={runtimeMode} title={connectionTitle}>
            {isFleet && connected && serverUrl ? (
              <a
                className="connection-body"
                href="#"
                onClick={(e) => {
                  e.preventDefault();
                  void Promise.resolve(invoke("open_external_link", { url: serverUrl })).catch(() => {});
                }}
              >
                <span className="connection-dot" aria-hidden="true" />
                <span className="connection-label">{connectionText}</span>
              </a>
            ) : (
              <span className="connection-body">
                <span className="connection-dot" aria-hidden="true" />
                <span className="connection-label">{connectionText}</span>
              </span>
            )}
          </div>
          <details className="agent-action-menu" ref={agentMenuRef}>
            <summary
              id="agent-actions-more"
              className="icon-button"
              title={isFleet ? "Fleet agent actions" : "Local companion actions"}
              aria-label={isFleet ? "Fleet agent actions" : "Local companion actions"}
            >
              <Icon name="more" />
            </summary>
            <div className="agent-action-panel">
              <button
                type="button"
                onClick={(e) => {
                  e.currentTarget.closest("details")?.removeAttribute("open");
                  void enableLocalRuntime().then(() => refreshExecutions()).catch(() => {});
                }}
              >
                <Icon name="refresh" />
                <span>{isFleet ? "Restart agent" : "Restart companion"}</span>
              </button>
              <button
                type="button"
                className="danger"
                onClick={(e) => {
                  e.currentTarget.closest("details")?.removeAttribute("open");
                  void prepareLocalUpdate().catch(() => {});
                }}
              >
                <Icon name="close" />
                <span>{isFleet ? "Stop agent" : "Stop companion"}</span>
              </button>
            </div>
          </details>
          <button
            id="settings"
            className="icon-button"
            title={`Settings · ${modifier},`}
            aria-label="Settings"
            aria-pressed={current?.view === "settings"}
            aria-keyshortcuts={mac ? "Meta+Comma" : "Control+Comma"}
            onClick={() => {
              if (current?.view === "settings") returnToWorkspace();
              else navigate({ id: "settings", name: "Settings" }, "settings");
            }}
          >
            <Icon name="settings" />
          </button>
        </div>
      </header>
      <div className="desktop-body">
        {showProjectSidebar && (
          <>
          <aside className="sidebar" style={{ width: visibleSidebarWidth }}>
            <div className="sidebar-scroll">
              <div className="sidebar-search">
                <Icon name="search" />
                <input
                  aria-label="Filter projects"
                  placeholder="Search projects"
                  value={filter}
                  onChange={(e) => setFilter(e.target.value)}
                />
              </div>
              <button className="sidebar-link" disabled={openingFolder} onClick={() => void openFolder()}><Icon name="folder" />{openingFolder ? "Opening…" : "Open folder"}</button>
              <div className="section-label">
                PROJECTS <span>{projects.length}</span>
              </div>
              {projects
                .filter((p) =>
                  p.name.toLowerCase().includes(filter.toLowerCase()),
                )
                .map((project) => {
                  const runs = projectRuns(snapshot?.runs ?? [], project.id);
                  const selected = current?.projectId === project.id;
                  const matchingConversations = runs.filter(run => run.conversation && archivedConversationIds.includes(run.id)===showArchivedConversations);
                  const conversations = deduplicateTicketConversations(matchingConversations);
                  const currentSelectedRun = current?.runId ? runs.find(r => r.id === current.runId) : undefined;
                  const projectTasks = navigationTasks?.projectId === project.id ? navigationTasks.items : [];
                  const projectBoard = navigationContext?.projectId === project.id ? navigationContext.value.board : undefined;
                  const orchestratorEnabled = navigationContext?.projectId === project.id && navigationContext.value.orchestratorEnabled !== undefined
                    ? navigationContext.value.orchestratorEnabled
                    : Boolean(project.orchestratorEnabled);
                  const visibleConversations = conversations.slice(0, visibleConversationCount);
                  const isRenamingProject = renamingProjectId === project.id;
                  return (
                    <section className="project-group" key={project.id}>
                      <div
                        className={`project-row ${selected ? "selected" : ""}`}
                      >
                        {isRenamingProject ? (
                          <input
                            className="project-rename"
                            autoFocus
                            maxLength={80}
                            value={renamingProjectDraft}
                            aria-label="Project name"
                            onChange={e => setRenamingProjectDraft(e.target.value)}
                            onKeyDown={e => {
                              if (e.key === "Enter") {
                                e.preventDefault();
                                void submitProjectRename(project, renamingProjectDraft);
                              } else if (e.key === "Escape") {
                                e.preventDefault();
                                e.stopPropagation();
                                setRenamingProjectId(null);
                              }
                            }}
                            onBlur={() => {
                              if (renamingProjectId === project.id) {
                                void submitProjectRename(project, renamingProjectDraft);
                              }
                            }}
                            onClick={e => e.stopPropagation()}
                          />
                        ) : (
                          <button
                            className="project-heading"
                            onClick={() => navigate(project, "board")}
                            onDoubleClick={() => {
                              setProjectActionError(null);
                              setRenamingProjectId(project.id);
                              setRenamingProjectDraft(project.name);
                            }}
                          >
                            <Icon name="folder" />
                            <span>{project.name}</span>
                            {runs.some(run => !run.conversation && isActive(run)) && <span className="dot live" />}
                          </button>
                        )}
                        <button
                          className="icon-button"
                          aria-label={`New prompt in ${project.name}`}
                          title="New prompt"
                          onClick={() => navigate(project, "prompt")}
                        >
                          <Icon name="plus" />
                        </button>
                        <details className="project-action-menu">
                          <summary aria-label={`Actions for ${project.name}`} title={`Actions for ${project.name}`}><Icon name="moreHorizontal" /></summary>
                          <div className="project-action-panel">
                            <button onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");navigate(project,"board");}}><Icon name="list" /><span>View tickets</span></button>
                            <button onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");navigate(project,"board");}}><Icon name="kanban" /><span>Open board</span></button>
                            <button onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");navigate(project,"executions");}}><Icon name="terminal" /><span>Execution queue</span></button>
                            <hr />
                            <button
                              type="button"
                              role="menuitemcheckbox"
                              aria-checked={getProjectSort(project.id) === "created"}
                              onClick={event => {
                                event.currentTarget.closest("details")?.removeAttribute("open");
                                toggleProjectSort(project.id, "created");
                              }}
                            >
                              <Icon name="clock" />
                              <span>Sort by created</span>
                              {getProjectSort(project.id) === "created" && <span className="project-menu-check" aria-hidden="true"><Icon name="check" /></span>}
                            </button>
                            <button
                              type="button"
                              role="menuitemcheckbox"
                              aria-checked={getProjectSort(project.id) === "id"}
                              onClick={event => {
                                event.currentTarget.closest("details")?.removeAttribute("open");
                                toggleProjectSort(project.id, "id");
                              }}
                            >
                              <Icon name="hash" />
                              <span>Sort by ID</span>
                              {getProjectSort(project.id) === "id" && <span className="project-menu-check" aria-hidden="true"><Icon name="check" /></span>}
                            </button>
                            <button
                              type="button"
                              role="menuitemcheckbox"
                              aria-checked={getProjectSort(project.id) === "stage"}
                              onClick={event => {
                                event.currentTarget.closest("details")?.removeAttribute("open");
                                toggleProjectSort(project.id, "stage");
                              }}
                            >
                              <Icon name="layers" />
                              <span>Group by stage</span>
                              {getProjectSort(project.id) === "stage" && <span className="project-menu-check" aria-hidden="true"><Icon name="check" /></span>}
                            </button>
                            <hr />
                            <button disabled={!fresh||!snapshot?.status.capabilities?.includes("task-page")} onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");navigate(project,"ticket-editor");}}><Icon name="plusSquare" /><span>New ticket</span></button>
                            <button onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");navigate(project,"prompt");}}><Icon name="chat" /><span>Start free prompt</span></button>
                            <button onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");setShowArchivedConversations(value=>!value);}}><Icon name="archive" /><span>{showArchivedConversations?"View active conversations":"View archived conversations"}</span></button>
                            <hr />
                            <button
                              type="button"
                              onClick={event => {
                                event.currentTarget.closest("details")?.removeAttribute("open");
                                setProjectActionError(null);
                                setRenamingProjectId(project.id);
                                setRenamingProjectDraft(project.name);
                              }}
                            >
                              <Icon name="edit" />
                              <span>Rename project</span>
                            </button>
                            <button onClick={event=>{event.currentTarget.closest("details")?.removeAttribute("open");navigate(project,"project-settings");}}><Icon name="settings" /><span>Project settings</span></button>
                            <button
                              type="button"
                              className="danger"
                              onClick={event => {
                                event.currentTarget.closest("details")?.removeAttribute("open");
                                setDeletingProjectError("");
                                setDeletingProject(project);
                              }}
                            >
                              <Icon name="trash" />
                              <span>Delete project</span>
                            </button>
                          </div>
                        </details>
                      </div>
                      {projectActionError?.projectId === project.id && <p className="sidebar-action-error" role="alert">{projectActionError.message}</p>}
                      {selected && (
                        <div className="project-children">
                          {orchestratorEnabled && (
                            <button
                              className="sidebar-link"
                              onClick={() => navigate(project, "orchestration")}
                            >
                              <Icon name="agent" />
                              Orchestration
                            </button>
                          )}
                          {showArchivedConversations&&<div className="sidebar-group-label">Archived <button type="button" onClick={()=>setShowArchivedConversations(false)}>Back</button></div>}
                          {conversationGroups(visibleConversations,projectTasks,getProjectSort(project.id),projectBoard).map(group=><div className="sidebar-conversation-group" key={group.label||"recent"}>{group.label&&<div className="sidebar-group-label">{group.label}<span>{group.runs.length}</span></div>}{group.runs.map(run=>{
                            const task=projectTasks.find(item=>item.id===run.taskId);
                            const isRenaming = renamingRunId === run.id;
                            const initialRenameTitle = conversationTitle(undefined, run, task);
                            return <div className="sidebar-conversation-row" key={run.id}>
                              {isRenaming ? (
                                <input
                                  className="task-rename"
                                  autoFocus
                                  value={renamingDraft}
                                  aria-label="Conversation name"
                                  onChange={e => setRenamingDraft(e.target.value)}
                                  onKeyDown={async e => {
                                    if (e.key === "Enter") {
                                      e.preventDefault();
                                      const val = renamingDraft.trim();
                                      setRenamingRunId(null);
                                      if (val) {
                                        try {
                                          await renameConversation(run.id, val);
                                          void refreshExecutions();
                                        } catch {
                                          setConversationActionError("Unable to rename conversation.");
                                        }
                                      }
                                    } else if (e.key === "Escape") {
                                      e.preventDefault();
                                      e.stopPropagation();
                                      setRenamingRunId(null);
                                    }
                                  }}
                                  onBlur={async () => {
                                    const val = renamingDraft.trim();
                                    setRenamingRunId(null);
                                    if (val && val !== (run.title || "")) {
                                      try {
                                        await renameConversation(run.id, val);
                                        void refreshExecutions();
                                      } catch {
                                        setConversationActionError("Unable to rename conversation.");
                                      }
                                    }
                                  }}
                                  onClick={e => e.stopPropagation()}
                                />
                              ) : (
                                <RunNavigation run={run} task={task} selected={current?.runId===run.id || (Boolean(run.taskId) && Boolean(currentSelectedRun) && conversationTicketGroupKey(currentSelectedRun!) === conversationTicketGroupKey(run))} onClick={()=>navigate(project,"prompt",run.id)} onOpenTask={run.taskId ? () => void openTaskInTracker(project.id, run.taskId, task, setConversationActionError) : undefined} />
                              )}
                              <ConversationPrAction task={task} taskKey={run.taskKey} onError={()=>setConversationActionError("Unable to open PR/MR.")}/>
                              {!showArchivedConversations && isActive(run) ? (
                                <button
                                  type="button"
                                  className="task-stop-button"
                                  aria-label="Stop conversation"
                                  title={stoppingConversationId === run.id ? "Stopping…" : "Stop conversation"}
                                  disabled={!fresh || !!stoppingConversationId}
                                  onClick={e => {
                                    e.stopPropagation();
                                    void stopConversation(run);
                                  }}
                                >
                                  <Icon name="stop" />
                                </button>
                              ) : (
                                <button
                                  type="button"
                                  className="task-archive"
                                  aria-label={showArchivedConversations ? "Restore conversation" : "Archive conversation"}
                                  title={showArchivedConversations ? "Restore conversation" : "Archive conversation"}
                                  onClick={e => {
                                    e.stopPropagation();
                                    setArchived(run.id, !showArchivedConversations);
                                  }}
                                >
                                  <Icon name="archive" />
                                </button>
                              )}
                              {!showArchivedConversations && !isRenaming && (
                                <button
                                  type="button"
                                  className="task-rename-button"
                                  aria-label="Rename conversation"
                                  title="Rename conversation"
                                  onClick={e => {
                                    e.stopPropagation();
                                    setRenamingRunId(run.id);
                                    setRenamingDraft(initialRenameTitle);
                                  }}
                                >
                                  <Icon name="edit" />
                                </button>
                              )}
                            </div>;
                          })}</div>)}
                          {conversationActionError&&<p className="sidebar-action-error" role="alert">{conversationActionError}</p>}
                          {conversations.length>visibleConversationCount&&<button className="sidebar-link" onClick={()=>setVisibleConversationCount(value=>value+8)}>Show more conversations · {visibleConversationCount}/{conversations.length}</button>}
                          <button
                            className="sidebar-link"
                            onClick={() => navigate(project, "executions")}
                          >
                            <Icon name="terminal" />
                            All executions <span>{runs.length}</span>
                          </button>
                        </div>
                      )}
                    </section>
                  );
                })}
              {!projects.length && (
                <p className="sidebar-hint">
                  {busy
                    ? "Connecting to agent…"
                    : "Your projects will appear here."}
                </p>
              )}
            </div>
          </aside>
          <div
            className="sidebar-resizer"
            role="separator"
            aria-label="Resize sidebar"
            title="Drag to resize · ← → with keyboard"
            aria-orientation="vertical"
            aria-valuemin={200}
            aria-valuemax={sidebarMaximum}
            aria-valuenow={visibleSidebarWidth}
            tabIndex={0}
            onPointerDown={event => {
              if (event.button !== 0) return;
              sidebarDrag.current = { pointerId: event.pointerId, startX: event.clientX, startWidth: visibleSidebarWidth };
              event.currentTarget.setPointerCapture(event.pointerId);
              document.body.classList.add("resizing-sidebar");
            }}
            onPointerMove={event => {
              const drag = sidebarDrag.current;
              if (drag?.pointerId === event.pointerId) setSidebarWidth(Math.max(200, Math.min(sidebarMaximum, drag.startWidth + event.clientX - drag.startX)));
            }}
            onPointerUp={event => {
              const drag = sidebarDrag.current;
              if (drag?.pointerId !== event.pointerId) return;
              const next = Math.max(200, Math.min(sidebarMaximum, drag.startWidth + event.clientX - drag.startX));
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
              const next = event.key === "Home" ? 200 : event.key === "End" ? sidebarMaximum : Math.max(200, Math.min(sidebarMaximum, visibleSidebarWidth + (event.key === "ArrowRight" ? 20 : -20)));
              setSidebarWidth(next);
              savePreference("sidebarWidth", String(next));
            }}
          />
          </>
        )}
        <main className="workspace">
          {unconfigured ? (
            <ModeOnboarding onConfigured={() => void handleConfigured()} />
          ) : (
            <>
              {folderError && <div role="alert" className="error">{folderError}
                {unmapped && <button onClick={async () => {
                  try { await invoke("retry_folder_project", { projectId: unmapped.id, path: unmapped.path }); setFolderError(""); setUnmapped(null); await refreshProjects(); navigate(unmapped, "prompt"); }
                  catch (cause) { setFolderError(String(cause)); }
                }}>Retry mapping</button>}
              </div>}
              {error && current?.view !== "settings" && (
                <div role="alert" className="error">
                  {error}
                  <button onClick={() => void refreshProjects()}>Retry</button>
                </div>
              )}
              {!current && (
                <section className="welcome">
                  <span className="brand-mark">S</span>
                  <h1>Your workspace</h1>
                  <p>
                    Choose a project to open its board, or start a
                    new prompt.
                  </p>
                </section>
              )}
              {current && (
                <section className={`workspace-pane ${current.view === "prompt" && current.runId ? "workspace-pane-conversation" : ""}`}>
                  {current.view === "settings" ? (
                    <LocalSettings
                      initialSection={current.settingsSection}
                      hasActiveExecutions={snapshot?.runs.some(isActive) ?? false}
                      onClose={settingsHasWorkspace ? returnToWorkspace : undefined}
                      onConfigureProject={id=>{const project=projects.find(p=>p.id===id);if(project)navigate(project,"project-settings");}}
                      onOpenFolder={() => void openFolder()}
                      onChanged={() => { setProjects([]); void refreshProjects(); void refreshExecutions(); }}
                    />
                  ) : (
                    <ProjectPane
                      key={current.id}
                      selection={current}
                      project={projects.find((p) => p.id === current.projectId)}
                      projects={projects}
                      navigate={navigate}
                      onFolderMapped={(id,path)=>setProjects(items=>items.map(item=>item.id===id?{...item,path}:item))}
                      onProjectsChanged={() => void refreshProjects()}
                      navigationTasks={navigationTasks?.projectId===current.projectId?navigationTasks.items:undefined}
                      onTasksChanged={items=>setNavigationTasks({projectId:current.projectId,items})}
                      onTaskChanged={task=>setNavigationTasks(previous=>{
                        const items=previous?.projectId===current.projectId?previous.items:[];
                        return {projectId:current.projectId,items:[...items.filter(item=>item.id!==task.id),task]};
                      })}
                      onContextChanged={value=>setNavigationContext({projectId:current.projectId,value})}
                      archivedConversationIds={archivedConversationIds}
                      onArchiveConversation={runId=>setArchived(runId,true)}
                    />
                  )}
                </section>
              )}
            </>
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
        onOpenSettings={() => navigate({ id: "settings", name: "Settings" }, "settings")}
        onToggleSidebar={returnToWorkspace}
        sidebarActionLabel={current?.view === "settings" ? settingsHasWorkspace ? "Return to workspace" : "" : sidebar ? "Hide projects" : "Show projects"}
        shortcutModifier={modifier}
      />
      {deletingProject && (
        <div
          className="project-delete-backdrop"
          onMouseDown={event => {
            if (event.target === event.currentTarget && !deletingProjectPending) {
              setDeletingProject(null);
            }
          }}
        >
          <dialog open className="project-delete-dialog" role="alertdialog" aria-label={`Delete ${deletingProject.name}`}>
            <h2>Delete “{deletingProject.name}”?</h2>
            <p className="muted">
              The project will be removed from Sectile on this workstation. Files in your local folder and remote tickets are preserved.
            </p>
            {deletingProjectError && <p role="alert" className="error">{deletingProjectError}</p>}
            <div className="project-delete-actions">
              <button
                type="button"
                disabled={deletingProjectPending}
                onClick={() => setDeletingProject(null)}
              >
                Cancel
              </button>
              <button
                type="button"
                className="danger-button"
                disabled={deletingProjectPending}
                onClick={() => void confirmDeleteProject()}
              >
                {deletingProjectPending ? "Deleting…" : "Confirm deletion"}
              </button>
            </div>
          </dialog>
        </div>
      )}
    </div>
  );
}
interface SavedBoardFilters {
  query?: string;
  priority?: string;
  status?: string;
  statuses?: string[];
  assignee?: string;
  parent?: string;
  sprint?: string;
  issueType?: string;
}

function readBoardFilters(projectId: string): SavedBoardFilters {
  if (!projectId) return {};
  try {
    const raw = readPreference(`boardFilters.${projectId}`, "");
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return typeof parsed === "object" && parsed ? parsed : {};
  } catch {
    return {};
  }
}

function saveBoardFilters(projectId: string, filters: SavedBoardFilters) {
  if (!projectId) return;
  savePreference(`boardFilters.${projectId}`, JSON.stringify(filters));
}

async function openTaskInTracker(
  projectId: string,
  taskId?: string,
  task?: Task,
  onError?: (message: string) => void,
) {
  try {
    let url = task?.externalUrl?.trim();
    if (!url && projectId && taskId) {
      const detail = await getTask(projectId, taskId);
      url = detail?.externalUrl?.trim();
    }
    if (!url) {
      onError?.("No external tracker link is available for this ticket.");
      return;
    }
    await openExternalLink(url);
  } catch {
    onError?.("Unable to open the ticket in the tracker.");
  }
}

function ProjectPane({
  selection,
  project,
  projects,
  navigate,
  onFolderMapped,
  onProjectsChanged,
  navigationTasks,
  onTasksChanged,
  onTaskChanged,
  onContextChanged,
  archivedConversationIds,
  onArchiveConversation,
}: {
  selection: WorkspaceView;
  project?: Project;
  projects: Project[];
  onFolderMapped: (projectId:string,path:string) => void;
  onProjectsChanged: () => void;
  navigationTasks?: Task[];
  onTasksChanged: (tasks:Task[]) => void;
  onTaskChanged: (task:Task) => void;
  onContextChanged: (context:ProjectContext) => void;
  archivedConversationIds: string[];
  onArchiveConversation: (runId:string) => void;
  navigate: (
    project: Project,
    view: View,
    runId?: string,
    taskId?: string,
    settingsSection?: "connections" | "engines",
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
  const [pathCopyState,setPathCopyState]=useState("");
  useEffect(()=>setPathCopyState(""),[workingDirectory]);
  const [taskState, setTaskState] = useState<{projectId:string;items:Task[]}>();
  const tasks = taskState?.projectId === selection.projectId ? taskState.items : navigationTasks ?? [];
  const [errorState, setErrorState] = useState<{projectId:string;message:string}>();
  const error = errorState?.projectId === selection.projectId ? errorState.message : "";
  const [busy, setBusy] = useState(false);
  const [filtersState, setFiltersState] = useState<{projectId:string;filters:SavedBoardFilters}>(() => ({
    projectId: selection.projectId,
    filters: readBoardFilters(selection.projectId),
  }));
  const currentFilters = useMemo(
    () => (filtersState.projectId === selection.projectId ? filtersState.filters : readBoardFilters(selection.projectId)),
    [filtersState, selection.projectId],
  );
  const query = currentFilters.query ?? "";
  const priorityFilter = currentFilters.priority ?? "";
  const statusFilters = useMemo(() => {
    if (Array.isArray(currentFilters.statuses)) {
      return currentFilters.statuses.map(s => s.trim().toLowerCase()).filter(Boolean);
    }
    if (currentFilters.status?.trim()) {
      return currentFilters.status.split(",").map(s => s.trim().toLowerCase()).filter(Boolean);
    }
    return [];
  }, [currentFilters.statuses, currentFilters.status]);
  const statusFilter = statusFilters.join(",");
  const assigneeFilter = currentFilters.assignee ?? "";
  const parentFilter = currentFilters.parent ?? "";
  const sprintFilter = currentFilters.sprint ?? "";
  const issueTypeFilter = currentFilters.issueType ?? "";
  const updateFilters = useCallback((patch: Partial<SavedBoardFilters>) => {
    setFiltersState(prev => {
      const base = prev.projectId === selection.projectId ? prev.filters : readBoardFilters(selection.projectId);
      const next = { ...base, ...patch };
      saveBoardFilters(selection.projectId, next);
      return { projectId: selection.projectId, filters: next };
    });
  }, [selection.projectId]);
  const setPriorityFilter = (value: string) => updateFilters({ priority: value });
  const setStatusFilters = (values: string[]) => {
    const normalized = Array.from(new Set(values.map(v => v.trim().toLowerCase()).filter(Boolean)));
    updateFilters({ statuses: normalized, status: normalized.join(",") });
  };
  const setStatusFilter = (value: string) => setStatusFilters(value ? value.split(",") : []);
  const toggleStatusFilter = (value: string) => {
    const target = value.trim().toLowerCase();
    if (!target) return;
    setStatusFilters(
      statusFilters.includes(target)
        ? statusFilters.filter(item => item !== target)
        : [...statusFilters, target],
    );
  };
  const setAssigneeFilter = (value: string) => updateFilters({ assignee: value });
  const setParentFilter = (value: string) => updateFilters({ parent: value });
  const setSprintFilter = (value: string) => updateFilters({ sprint: value });
  const setIssueTypeFilter = (value: string) => updateFilters({ issueType: value });
  const [sprintSuggestionsOpen, setSprintSuggestionsOpen] = useState(false);
  const [grouping,setGrouping] = useState<"stage"|"tracker">(()=>readPreference("boardGrouping","stage")==="tracker"?"tracker":"stage");
  const [compactCards,setCompactCards] = useState(()=>readPreference("boardCards","compact")!=="full");
  const [ticketView,setTicketView]=useState<"board"|"list">(()=>readPreference("ticketView","board")==="list"?"list":"board");
  const [tableSort,setTableSort]=useState<SortState>(DEFAULT_SORT);
  const [listColumns,setListColumns]=useState<OptionalListColumn[]>(()=>readListColumns());
  const visibleColumns=useMemo(()=>new Set<OptionalListColumn>(listColumns),[listColumns]);
  const toggleListColumn=(col:OptionalListColumn)=>{
    setListColumns(prev=>{
      const next=prev.includes(col)?prev.filter(item=>item!==col):DEFAULT_LIST_COLUMNS.filter(item=>prev.includes(item)||item===col);
      savePreference("ticketListColumns",JSON.stringify(next));
      return next;
    });
  };
  const resetListColumns=()=>{
    setListColumns(DEFAULT_LIST_COLUMNS);
    savePreference("ticketListColumns",JSON.stringify(DEFAULT_LIST_COLUMNS));
  };
  const [filterMenuOpen,setFilterMenuOpen]=useState(false);
  const filterMenuRef=useRef<HTMLDivElement>(null);
  const [columnMenuOpen,setColumnMenuOpen]=useState(false);
  const columnMenuRef=useRef<HTMLDivElement>(null);
  useEffect(()=>{
    if(!filterMenuOpen&&!columnMenuOpen)return;
    const onPointerDown=(event:MouseEvent)=>{
      if(filterMenuRef.current&&!filterMenuRef.current.contains(event.target as Node)){
        setFilterMenuOpen(false);
      }
      if(columnMenuRef.current&&!columnMenuRef.current.contains(event.target as Node)){
        setColumnMenuOpen(false);
      }
    };
    const onKeyDown=(event:KeyboardEvent)=>{if(event.key==="Escape"){setFilterMenuOpen(false);setColumnMenuOpen(false);}};
    window.addEventListener("pointerdown",onPointerDown);
    window.addEventListener("keydown",onKeyDown);
    return ()=>{window.removeEventListener("pointerdown",onPointerDown);window.removeEventListener("keydown",onKeyDown);};
  },[filterMenuOpen,columnMenuOpen]);
  const [selectedTaskIds,setSelectedTaskIds]=useState<string[]>([]);
  const [batchConfirm,setBatchConfirm]=useState(false);
  const [launching,setLaunching]=useState(false);
  const [startOverSource,setStartOverSource]=useState<Run|null>(null);
  const [launchError,setLaunchError]=useState("");
  const [visibleByColumn,setVisibleByColumn] = useState<Record<string,number>>({});
  const [visibleListCount,setVisibleListCount]=useState(80);
  useEffect(()=>setVisibleByColumn({}),[selection.projectId,query,grouping,priorityFilter,statusFilter,assigneeFilter,parentFilter,sprintFilter,issueTypeFilter]);
  useEffect(()=>setVisibleListCount(80),[selection.projectId,query,priorityFilter,statusFilter,assigneeFilter,parentFilter,sprintFilter,issueTypeFilter]);
  useEffect(()=>{setSelectedTaskIds([]);setBatchConfirm(false);setLaunchError("");},[selection.projectId]);
  const [contextState,setContextState] = useState<{projectId:string;value:ProjectContext}>();
  const [contextError,setContextError] = useState<{projectId:string;message:string}>();
  const [contextRefresh,setContextRefresh] = useState(0);
  const context = contextState?.projectId === selection.projectId ? contextState.value : undefined;
  const currentContextError = contextError?.projectId === selection.projectId ? contextError.message : "";
  useEffect(()=>{let current=true;setContextError(undefined);void projectContext(selection.projectId).then(data=>{if(current){setContextState({projectId:selection.projectId,value:data});onContextChanged(data);setContextError(undefined);}}).catch(cause=>{if(current)setContextError({projectId:selection.projectId,message:String(cause)});});return()=>{current=false;};},[selection.projectId,contextRefresh]);
  const [selectedRun, setSelectedRun] = useState(selection.runId ?? "");
  const [projectSettingsTab, setProjectSettingsTab] = useState("folder");
  useEffect(() => setProjectSettingsTab("folder"), [selection.projectId]);
  const request = useRef(0);
  async function refresh() {
    const seq = ++request.current;
    setBusy(true);
    setErrorState(undefined);
    try {
      const items = await listTasks(selection.projectId);
      if (seq === request.current) {setTaskState({projectId:selection.projectId,items});onTasksChanged(items);}
    } catch (cause) {
      if (seq === request.current) setErrorState({projectId:selection.projectId,message:String(cause)});
    } finally {
      if (seq === request.current) setBusy(false);
    }
  }
  useEffect(() => {
    if(selection.view !== "project-settings" && selection.view !== "task" && (selection.view !== "prompt" || projectRuns(monitor.snapshot?.runs ?? [],selection.projectId).some(run=>!!run.taskId))) void refresh();
    return () => { request.current++; };
  }, [selection.projectId, selection.view, monitor.snapshot?.runs.some(run=>run.projectId===selection.projectId&&!!run.taskId)]);
  useEffect(()=>{
    if(selection.view!=="prompt"||!currentRun?.taskId)return;
    let current=true,reading=false;
    const taskId=currentRun.taskId;
    const read=async()=>{
      if(reading)return;
      reading=true;
      try{
        const task=await getTask(selection.projectId,taskId);
        if(current&&task?.id===taskId){
          setTaskState(previous=>{
            const items=previous?.projectId===selection.projectId?previous.items:navigationTasks??[];
            return {projectId:selection.projectId,items:[...items.filter(item=>item.id!==task.id),task]};
          });
          onTaskChanged(task);
        }
      }catch{/* Keep the last known ticket state while its tracker is unavailable. */}
      finally{reading=false;}
    };
    void read();
    const timer=setInterval(()=>void read(),15000);
    return()=>{current=false;clearInterval(timer);};
  },[selection.projectId,selection.view,currentRun?.taskId]);
  const seenTaskRunIds = useRef<{ taskId: string; ids: Set<string> } | null>(null);
  useEffect(() => {
    if (selection.view !== "prompt" || !currentRun?.taskId || !project) {
      seenTaskRunIds.current = null;
      return;
    }
    const taskId = currentRun.taskId;
    const runsForTask = projectRuns(monitor.snapshot?.runs ?? [], selection.projectId).filter(r => r.taskId === taskId && r.conversation);
    if (!seenTaskRunIds.current || seenTaskRunIds.current.taskId !== taskId) {
      seenTaskRunIds.current = { taskId, ids: new Set(runsForTask.map(r => r.id)) };
      return;
    }
    const known = seenTaskRunIds.current.ids;
    const newlyStarted = runsForTask.find(r => !known.has(r.id));
    for (const r of runsForTask) known.add(r.id);
    if (newlyStarted && newlyStarted.id !== selection.runId) {
      navigate(project, "prompt", newlyStarted.id);
    }
  }, [selection.view, selection.projectId, selection.runId, currentRun?.taskId, monitor.snapshot?.runs, project, navigate]);
  useEffect(() => {
    if (selection.view === "executions" && selection.runId && currentRun?.conversation && project) {
      navigate(project, "prompt", selection.runId);
    }
  }, [selection.view, selection.runId, currentRun?.conversation, project]);
  const allRemoteTasks = useMemo(() => trackerColumns(tasks).flatMap(column => column.tasks), [tasks]);
  const taskFilter = useMemo(
    () => taskFilterPredicate({
      query,
      priority: priorityFilter,
      statuses: statusFilters,
      assignee: assigneeFilter,
      parent: parentFilter,
      sprint: sprintFilter,
      issueType: issueTypeFilter,
    }, context?.board),
    [query, priorityFilter, statusFilters, assigneeFilter, parentFilter, sprintFilter, issueTypeFilter, context?.board]
  );
  const remoteColumns = trackerColumns(tasks, taskFilter);
  const hasRemoteTasks = allRemoteTasks.length > 0;
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
  const sortedTasks = useMemo(() => orderedTasks(visibleTasks, tableSort, context?.board), [visibleTasks, tableSort, context?.board]);
  const availableStatuses = useMemo(() => {
    const values = new Map<string, string>();
    for (const t of allRemoteTasks) {
      if (t.trackerStatus?.trim()) {
        values.set(t.trackerStatus.trim().toLowerCase(), t.trackerStatus.trim());
      }
      const stage = taskStage(t, context?.board);
      if (stage) {
        values.set(stage.toLowerCase(), stageLabels[stage] || stage);
      }
    }
    return Array.from(values.entries()).map(([value, label]) => ({ value, label }));
  }, [allRemoteTasks, context?.board]);
  const statusCounts = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const s of availableStatuses) {
      counts[s.value] = allRemoteTasks.filter(t => {
        const stage = taskStage(t, context?.board);
        return (
          (t.trackerStatus ?? "").toLowerCase() === s.value ||
          (stage ?? "").toLowerCase() === s.value ||
          (t.status ?? "").toLowerCase() === s.value
        );
      }).length;
    }
    return counts;
  }, [availableStatuses, allRemoteTasks, context?.board]);
  const availableAssignees = useMemo(() => {
    const assignees = new Set<string>();
    let hasUnassigned = false;
    for (const t of allRemoteTasks) {
      if (t.assignee?.trim()) {
        assignees.add(t.assignee.trim());
      } else {
        hasUnassigned = true;
      }
    }
    return {
      people: Array.from(assignees).sort((a, b) => a.localeCompare(b)),
      hasUnassigned,
    };
  }, [allRemoteTasks]);
  const availableParents = useMemo(() => {
    const parents = new Map<string, string>();
    let hasNone = false;
    for (const t of allRemoteTasks) {
      if (t.parentKey?.trim()) {
        parents.set(t.parentKey.trim(), t.parentTitle?.trim() || "");
      } else {
        hasNone = true;
      }
    }
    return {
      parents: Array.from(parents.entries()).map(([key, title]) => ({ key, title })),
      hasNone,
    };
  }, [allRemoteTasks]);
  const availableSprints = useMemo(() => {
    const sprints = new Set<string>();
    let hasNone = false;
    for (const t of allRemoteTasks) {
      if (t.sprint?.trim()) {
        sprints.add(t.sprint.trim());
      } else {
        hasNone = true;
      }
    }
    return {
      sprints: Array.from(sprints).sort((a, b) => a.localeCompare(b, undefined, { numeric: true, sensitivity: "base" })),
      hasNone,
    };
  }, [allRemoteTasks]);
  const availableIssueTypes = useMemo(() => {
    const types = new Set<string>();
    let hasNone = false;
    for (const t of allRemoteTasks) {
      if (t.issueType?.trim()) {
        types.add(t.issueType.trim());
      } else {
        hasNone = true;
      }
    }
    return {
      types: Array.from(types).sort((a, b) => a.localeCompare(b, undefined, { sensitivity: "base" })),
      hasNone,
    };
  }, [allRemoteTasks]);
  const hasActiveFilters = Boolean(priorityFilter || statusFilters.length > 0 || assigneeFilter || parentFilter || sprintFilter || issueTypeFilter);
  const activeFilterCount = [priorityFilter, statusFilters.length > 0 ? "status" : "", assigneeFilter, parentFilter, sprintFilter, issueTypeFilter].filter(Boolean).length;
  const resetFilters = () => {
    updateFilters({
      priority: "",
      status: "",
      statuses: [],
      assignee: "",
      parent: "",
      sprint: "",
      issueType: "",
    });
  };
  const columns = grouping === "stage" ? [...stages.map(stage=>({name:stageLabels[stage],tasks:visibleTasks.filter(task=>taskStage(task,context?.board)===stage),stage})),...(visibleTasks.some(task=>!taskStage(task,context?.board))?[{name:"Unknown stage",tasks:visibleTasks.filter(task=>!taskStage(task,context?.board)),stage:undefined}]:[])] : remoteColumns.map(column=>({...column,stage:undefined}));
  const count = columns.reduce((n, c) => n + c.tasks.length, 0);
  const selectedTasks=selectedTaskIds.map(id=>allRemoteTasks.find(task=>task.id===id)).filter((task):task is Task=>Boolean(task));
  function toggleTask(id:string){setSelectedTaskIds(previous=>previous.includes(id)?previous.filter(item=>item!==id):[...previous,id]);setBatchConfirm(false);}
  // startOver launches the skill in the conversation view with an archived
  // conversation as former context (#906), and rethrows a failure for the
  // dialog to show instead of the toolbar's generic notice.
  async function startSkill(task:Task,skillId:string,viewOverride?:TaskLaunchView,startOver?:{prompt:string;contextRunId:string}){
    if(launching||!project||!context){if(startOver)throw new Error("Another launch is in progress.");return;}
    setLaunching(true);setLaunchError("");
    try{
      const [freshTasks,freshContext,state]=await Promise.all([listTasks(project.id),projectContext(project.id),executionSnapshot()]);
      const freshTask=freshTasks.find(item=>item.id===task.id);
      if(!freshTask||!freshContext.configured||!state.status.connected||state.status.contractError||state.runs.some(run=>run.projectId===project.id&&run.taskId===task.id&&isActive(run))||taskStage(freshTask,freshContext.board)==="finished"||!freshContext.skills.some(skill=>skill.id===skillId))throw new Error("Workflow unavailable");
      if(freshContext.aiProvider==="claude"||freshContext.aiProvider==="codex"){
        const readiness=await providerReadiness(freshContext.aiProvider);
        if(readiness.steps?.mcp?.configuration?.present&&!readiness.steps.mcp.configuration.valid)throw new Error("MCP unavailable");
      }
      if(startOver){
        // Interactive explicitly: a project defaulting to autonomous would
        // otherwise run the skill headless, where no former context fits.
        await launchTask(project.id,launchInput(task.id,skillId,startOver.prompt,"interactive",{view:"conversation",contextRunId:startOver.contextRunId}));
        await monitor.refresh();
        navigate(project,"task",undefined,task.id);
        return;
      }
      const chosenView=viewOverride??readTaskLaunchView();
      if(viewOverride)saveTaskLaunchView(viewOverride);
      await launchTask(project.id,launchInput(task.id,skillId,"","",chosenView==="terminal"?{view:"terminal"}:undefined));
      await monitor.refresh();
      if (selection.view !== "prompt" || currentRun?.taskId !== task.id) {
        navigate(project,"task",undefined,task.id);
      }
    }catch(cause){
      if(startOver)throw new Error(cause instanceof Error&&cause.message!=="Workflow unavailable"&&cause.message!=="MCP unavailable"?cause.message:typeof cause==="string"?cause:"This skill cannot start on this ticket now. Check its stage, its running executions and the project configuration.");
      setLaunchError("Launch not confirmed. Check configuration and status before trying again.");
    }
    finally{setLaunching(false);}
  }
  // Skills offered by Start over: interactive ones only, on the archived
  // conversation's own ticket (#906).
  function startOverSkills(source:Run){
    const task=tasks.find(item=>item.id===source.taskId);
    if(!source.taskId)return {skills:[],reason:"Skills need a conversation linked to a ticket."};
    if(!task||!context)return {skills:[],reason:"This conversation's ticket is not loaded."};
    if(taskStage(task,context.board)==="finished")return {skills:[],reason:"This ticket is finished."};
    if((monitor.snapshot?.runs??[]).some(item=>item.taskId===task.id&&isActive(item)))return {skills:[],reason:"This ticket already has a running execution."};
    const skills=context.skills.filter(skill=>!["pickup","pickup_issue","pickup_issues"].includes(skill.id)).map(skill=>({id:skill.id,label:skillLabels[skill.id]||skill.id}));
    return {skills,reason:skills.length?undefined:"No interactive skill is available for this project."};
  }
  async function startBatch(){
    if(!project||launching||selectedTasks.length<2)return;
    setLaunching(true);setLaunchError("");
    try{
      const [freshTasks,freshContext,state]=await Promise.all([listTasks(project.id),projectContext(project.id),executionSnapshot()]);
      if(!freshContext.configured||!freshContext.skills.some(skill=>skill.id==="pickup_issues")||!state.status.connected||state.status.contractError||selectedTasks.some(task=>!freshTasks.some(current=>current.id===task.id&&taskStage(current,freshContext.board)!=="finished")||state.runs.some(run=>run.projectId===project.id&&run.taskId===task.id&&isActive(run))))throw new Error("Batch unavailable");
      if(freshContext.aiProvider==="claude"||freshContext.aiProvider==="codex"){
        const readiness=await providerReadiness(freshContext.aiProvider);
        if(readiness.steps?.mcp?.configuration?.present&&!readiness.steps.mcp.configuration.valid)throw new Error("MCP unavailable");
      }
      await launchTask(project.id,{...launchInput(selectedTasks[0].id,"pickup_issues","","autonomous"),batchTaskIds:selectedTasks.map(task=>task.id)});
      setBatchConfirm(false);setSelectedTaskIds([]);await monitor.refresh();
    }catch{setLaunchError("Batch not launched. Check the tickets, skill, and executions before trying again.");}
    finally{setLaunching(false);}
  }
  function taskMenu(task:Task){
    const next=context?nextSkill(task,context):undefined;
    const launchDisabled=launching||!monitor.fresh||taskStage(task,context?.board)==="finished";
    const defaultView=readTaskLaunchView();
    return <select className="task-actions" aria-label={`Actions for ${task.key||task.title}`} title="Ticket actions" defaultValue="" onChange={event=>{const value=event.currentTarget.value;event.currentTarget.value="";if(value==="open"){if(project)navigate(project,"task",undefined,task.id);}else if(value==="edit"){if(project)navigate(project,"ticket-editor",undefined,task.id);}else if(value.startsWith("terminal:")){void startSkill(task,value.slice(9),"terminal");}else if(value.startsWith("conversation:")){void startSkill(task,value.slice(13),"conversation");}else if(value)void startSkill(task,value);}}><option value="">•••</option><option value="open">View workflow</option><option value="edit" disabled={!monitor.fresh||!monitor.snapshot?.status.capabilities?.includes("task-page")}>Edit ticket</option>{next?.skillId&&<option value={next.skillId} disabled={launchDisabled}>Launch: {next.label}</option>}{next?.skillId&&next.skillId!=="pickup"&&(defaultView==="terminal"?<option value={`conversation:${next.skillId}`} disabled={launchDisabled}>Launch in conversation: {next.label}</option>:<option value={`terminal:${next.skillId}`} disabled={launchDisabled}>Launch in terminal: {next.label}</option>)}{context?.skills.filter(skill=>skill.id!==next?.skillId&&skill.id!=="pickup_issues").map(skill=><option key={skill.id} value={skill.id} disabled={launchDisabled}>Launch: {skillLabels[skill.id]||skill.id}</option>)}</select>;
  }
  function epicStyle(task:Task):CSSProperties|undefined {
    const color=context?.board?.epicColors ? epicColorHex(task.parentKey) : undefined;
    return color ? {"--epic":color} as CSSProperties : undefined;
  }
  return (
    <>
      {!(selection.view === "prompt" && selection.runId) && (
        <div className="workspace-toolbar">
          <div>
            <h1>{project?.name ?? "Project"}</h1>
            <div className="workspace-path"><p title={workingDirectory}>{workingDirectory || (selection.runId ? "Loading execution folder…" : "Project workspace")}</p>{workingDirectory&&<button type="button" className="copy-path" title="Copy working folder path" aria-label="Copy working folder path" onClick={()=>void navigator.clipboard.writeText(workingDirectory).then(()=>setPathCopyState("Copied"),()=>setPathCopyState("Copy failed"))}>{pathCopyState||"Copy"}</button>}</div>
          </div>
          <div className="toolbar-actions">
            {project && selection.view !== "task" && selection.view !== "ticket-editor" && !(selection.view === "executions" && currentRun?.conversation) && (
              <>
                {selection.view !== "board" && selection.view !== "project-settings" && <button onClick={() => navigate(project, "board")}>
                  <Icon name="board" />
                  Board
                </button>}
                {selection.view !== "prompt" && <button
                  className={selection.view === "board" ? "primary" : "subtle"}
                  onClick={() => navigate(project, "prompt")}
                >
                  <Icon name="plus" />
                  New prompt
                </button>}
                {selection.view !== "project-settings" && <button className={selection.view === "board" ? "icon-button" : undefined} title="Configure project" aria-label="Configure project" onClick={() => navigate(project, "project-settings")}><Icon name="settings" />{selection.view !== "board" && "Configure project"}</button>}
              </>
            )}
            {selection.view === "board" && <button
              className="icon-button"
              title="Refresh"
              aria-label="Refresh board"
              disabled={busy}
              onClick={() => void refresh()}
            >
              <Icon name="refresh" />
            </button>}
          </div>
        </div>
      )}
      {selection.view === "board" && error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {selection.view === "project-settings" ? <ProjectProfiles key={selection.projectId} projectId={selection.projectId} activeTab={projectSettingsTab} onTabChange={setProjectSettingsTab} onManageProfiles={() => navigate({id:"settings",name:"Settings"},"settings",undefined,undefined,"engines")} tabs={[{id:"folder",label:"Folders",content:<>{project && <ProjectFolderSetup variant="settings" project={project} onMapped={path=>onFolderMapped(project.id,path)} />}<AttachedFolderSettings projectId={selection.projectId} onChanged={()=>void monitor.refresh()}/></>},{id:"conversations",label:"Conversations"},{id:"tracker",label:"Tickets",content:<ProjectTrackerSetup projectId={selection.projectId} onChanged={() => {setContextRefresh(value=>value+1);onProjectsChanged();}} onManageTrackers={() => navigate({id:"settings",name:"Settings"},"settings",undefined,undefined,"connections")}/>},{id:"specs",label:"Specifications (SDD)",content:<AdvancedFolders projectId={selection.projectId} active={projectSettingsTab==="specs"} defaultOpen={projectSettingsTab==="specs"} onChanged={()=>void monitor.refresh()}/>},{id:"workflow",label:"Workflow",content:<><WorkflowSettings projectId={selection.projectId} defaultOpen={projectSettingsTab==="workflow"}/><OrchestrationSettings projectId={selection.projectId} defaultOpen={projectSettingsTab==="workflow"} onChanged={onProjectsChanged} onSupervise={()=>{if(project)navigate(project,"orchestration");}}/></>}]} /> : selection.view === "ticket-editor" ? <TaskEditor projectId={selection.projectId} projectName={project?.name||"Project"} projects={projects} onSelectProject={next=>{const found=projects.find(item=>item.id===next.id);if(found)navigate(found,"ticket-editor");}} taskId={selection.taskId} available={!!monitor.fresh&&!!monitor.snapshot?.status.capabilities?.includes("task-page")} knownSprints={availableSprints.sprints} knownAssignees={availableAssignees.people} onClose={()=>{if(project)navigate(project,selection.taskId?"task":"board",undefined,selection.taskId);}} onSaved={(task,options)=>{onTaskChanged(task);void refresh();if(project){if(options?.launch){const skillId=context?.skills.find(s=>s.id==="clarify")?.id||context?.skills[0]?.id||"clarify";void launchTask(project.id,launchInput(task.id,skillId,"","")).then(()=>monitor.refresh()).catch(()=>{}).finally(()=>navigate(project,"task",undefined,task.id));}else{navigate(project,"task",undefined,task.id);}}}}/> : selection.view === "task" && selection.taskId ? <Suspense fallback={<p>Opening workflow…</p>}><WorkflowTask projectId={selection.projectId} taskId={selection.taskId} onBack={()=>{if(project)navigate(project,"board");}} onSetup={()=>{if(project)navigate(project,"settings");}} onEdit={()=>{if(project)navigate(project,"ticket-editor",undefined,selection.taskId);}} onExecution={runId=>{if(project)navigate(project,"executions",runId||undefined);}} onOpenConversation={runId=>{if(project)navigate(project,"prompt",runId);}} onArchiveConversation={onArchiveConversation} onTasksLoaded={items=>{setTaskState({projectId:selection.projectId,items});onTasksChanged(items);}}/></Suspense> : selection.view === "board" ? (
        <div className="board-workspace">
          {error && tasks.length > 0 && <p className="hint" role="status">The last refresh failed. Displayed tickets come from the previous read.</p>}
          {!boardContextPending && !currentContextError && !localOnlyProject && <div className="board-toolbar">
            <div className="board-view-switch" role="group" aria-label="Ticket view"><button type="button" aria-label="Board" title="Board" aria-pressed={ticketView==="board"} onClick={()=>{setTicketView("board");savePreference("ticketView","board");}}><Icon name="kanban" size={14} /></button><button type="button" aria-label="List" title="List" aria-pressed={ticketView==="list"} onClick={()=>{setTicketView("list");savePreference("ticketView","list");}}><Icon name="list" size={14} /></button></div>
            <label className="search">
              <Icon name="search" />
              <input
                aria-label="Search tickets"
                placeholder="Search tickets…"
                value={query}
                onChange={(e) => updateFilters({ query: e.target.value })}
              />
            </label>
            <div className="board-filter-dropdown" ref={filterMenuRef}>
              <button
                type="button"
                className={`board-filter-toggle ${hasActiveFilters ? "active" : ""}`}
                aria-label="Ticket filters"
                title={hasActiveFilters ? `${activeFilterCount} active filter${activeFilterCount > 1 ? "s" : ""}` : "Filter tickets"}
                aria-expanded={filterMenuOpen}
                onClick={() => setFilterMenuOpen(open => !open)}
              >
                <span>Filters</span>
                {activeFilterCount > 0 && <span className="filter-count-badge">{activeFilterCount}</span>}
              </button>
              {filterMenuOpen && (
                <div className="board-filter-popover" role="dialog" aria-label="Ticket filters">
                  <div className="board-filter-popover-header">
                    <span className="filter-title">Filters</span>
                    {hasActiveFilters && (
                      <button
                        type="button"
                        className="filter-reset-button"
                        onClick={() => { resetFilters(); setFilterMenuOpen(false); }}
                      >
                        Reset
                      </button>
                    )}
                  </div>
                  <div className="board-filter-popover-body">
                    <div className="board-filter-row">
                      <span className="board-filter-label">Sprint</span>
                      <div className="board-filter-sprint-field">
                        <input
                          type="text"
                          list="board-filter-sprint-options"
                          aria-label="Filter by sprint"
                          className={sprintFilter ? "active" : ""}
                          placeholder="All sprints…"
                          value={sprintFilter === "__none__" ? "No sprint" : sprintFilter}
                          onFocus={() => setSprintSuggestionsOpen(true)}
                          onBlur={() => setTimeout(() => setSprintSuggestionsOpen(false), 150)}
                          onChange={e => {
                            const val = e.target.value;
                            setSprintFilter(val === "No sprint" ? "__none__" : val);
                            setSprintSuggestionsOpen(true);
                          }}
                        />
                        {sprintFilter && (
                          <button
                            type="button"
                            className="board-filter-input-clear"
                            aria-label="Clear sprint filter"
                            title="All sprints"
                            onClick={() => setSprintFilter("")}
                          >
                            ×
                          </button>
                        )}
                        <datalist id="board-filter-sprint-options">
                          {availableSprints.hasNone && <option value="No sprint" />}
                          {availableSprints.sprints.map(s => (
                            <option key={s} value={s} />
                          ))}
                        </datalist>
                        {sprintSuggestionsOpen && (availableSprints.hasNone || availableSprints.sprints.length > 0) && (
                          <div className="board-filter-suggestions" role="listbox" aria-label="Sprint suggestions">
                            <button
                              type="button"
                              role="option"
                              aria-selected={!sprintFilter}
                              className={`board-filter-suggestion-item ${!sprintFilter ? "active" : ""}`}
                              onMouseDown={e => { e.preventDefault(); setSprintFilter(""); setSprintSuggestionsOpen(false); }}
                            >
                              All sprints
                            </button>
                            {availableSprints.hasNone && (
                              <button
                                type="button"
                                role="option"
                                aria-selected={sprintFilter === "__none__"}
                                className={`board-filter-suggestion-item ${sprintFilter === "__none__" ? "active" : ""}`}
                                onMouseDown={e => { e.preventDefault(); setSprintFilter("__none__"); setSprintSuggestionsOpen(false); }}
                              >
                                No sprint
                              </button>
                            )}
                            {availableSprints.sprints
                              .filter(s => !sprintFilter || sprintFilter === "__none__" || s.toLowerCase().includes(sprintFilter.toLowerCase()))
                              .map(s => (
                                <button
                                  key={s}
                                  type="button"
                                  role="option"
                                  aria-selected={sprintFilter === s}
                                  className={`board-filter-suggestion-item ${sprintFilter === s ? "active" : ""}`}
                                  onMouseDown={e => { e.preventDefault(); setSprintFilter(s); setSprintSuggestionsOpen(false); }}
                                >
                                  {s}
                                </button>
                              ))}
                          </div>
                        )}
                      </div>
                    </div>
                    <div className="board-filter-row">
                      <span className="board-filter-label">Macro</span>
                      <select
                        aria-label="Filter by parent"
                        className={parentFilter ? "active" : ""}
                        value={parentFilter}
                        onChange={e => setParentFilter(e.target.value)}
                      >
                        <option value="">All macros…</option>
                        {availableParents.hasNone && <option value="__none__">No macro</option>}
                        {availableParents.parents.map(p => (
                          <option key={p.key} value={p.key}>
                            {p.key}{p.title ? ` · ${p.title}` : ""}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div className="board-filter-row">
                      <span className="board-filter-label">Assignee</span>
                      <select
                        aria-label="Filter by assignee"
                        className={assigneeFilter ? "active" : ""}
                        value={assigneeFilter}
                        onChange={e => setAssigneeFilter(e.target.value)}
                      >
                        <option value="">All assignees</option>
                        {availableAssignees.hasUnassigned && <option value="__unassigned__">Unassigned</option>}
                        {availableAssignees.people.map(person => (
                          <option key={person} value={person}>{person}</option>
                        ))}
                      </select>
                    </div>
                    <div className="board-filter-row">
                      <span className="board-filter-label">Type</span>
                      <select
                        aria-label="Filter by type"
                        className={issueTypeFilter ? "active" : ""}
                        value={issueTypeFilter}
                        onChange={e => setIssueTypeFilter(e.target.value)}
                      >
                        <option value="">All types</option>
                        {availableIssueTypes.hasNone && <option value="__none__">No type</option>}
                        {availableIssueTypes.types.map(type => (
                          <option key={type} value={type}>{type}</option>
                        ))}
                      </select>
                    </div>
                    <div className="board-filter-row">
                      <span className="board-filter-label">Priority</span>
                      <select
                        aria-label="Filter by priority"
                        className={priorityFilter ? "active" : ""}
                        value={priorityFilter}
                        onChange={e => setPriorityFilter(e.target.value)}
                      >
                        <option value="">All priorities</option>
                        <option value="urgent">Urgent</option>
                        <option value="high">High</option>
                        <option value="medium">Medium</option>
                        <option value="low">Low</option>
                      </select>
                    </div>
                    {availableStatuses.length > 0 && (
                      <div className="board-filter-section-divider">
                        <div className="board-filter-section-header">
                          <span className="board-filter-section-title">Status</span>
                          {statusFilters.length > 0 && (
                            <button
                              type="button"
                              className="filter-reset-button"
                              onClick={() => setStatusFilters([])}
                            >
                              All
                            </button>
                          )}
                        </div>
                        <select
                          aria-label="Filter by status"
                          className="sr-only"
                          value={statusFilters[0] ?? ""}
                          onChange={e => setStatusFilter(e.target.value)}
                        >
                          <option value="">All statuses</option>
                          {availableStatuses.map(s => (
                            <option key={s.value} value={s.value}>{s.label}</option>
                          ))}
                        </select>
                        <div className="board-filter-status-list" role="menu" aria-label="Status">
                          {availableStatuses.map(s => {
                            const isActive = statusFilters.includes(s.value);
                            const count = statusCounts[s.value] ?? 0;
                            return (
                              <button
                                key={s.value}
                                type="button"
                                role="menuitemcheckbox"
                                aria-checked={isActive}
                                className={`board-filter-status-item ${isActive ? "active" : ""}`}
                                onClick={() => toggleStatusFilter(s.value)}
                              >
                                <span className={`board-filter-checkbox ${isActive ? "checked" : ""}`}>
                                  {isActive && <Icon name="check" />}
                                </span>
                                <span className="board-filter-status-name">{s.label}</span>
                                <span className="board-filter-status-count">{count}</span>
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
            {ticketView === "list" && (
              <div className="board-filter-dropdown" ref={columnMenuRef}>
                <button
                  type="button"
                  className={`board-filter-toggle ${listColumns.length < DEFAULT_LIST_COLUMNS.length ? "active" : ""}`}
                  aria-label="List columns"
                  title="Choose visible columns"
                  aria-expanded={columnMenuOpen}
                  onClick={() => setColumnMenuOpen(open => !open)}
                >
                  <span>Columns</span>
                  {listColumns.length < DEFAULT_LIST_COLUMNS.length && (
                    <span className="filter-count-badge">{listColumns.length}/{DEFAULT_LIST_COLUMNS.length}</span>
                  )}
                </button>
                {columnMenuOpen && (
                  <div className="board-filter-popover" role="dialog" aria-label="List columns">
                    <div className="board-filter-popover-header">
                      <span className="filter-title">Columns</span>
                      {listColumns.length < DEFAULT_LIST_COLUMNS.length && (
                        <button
                          type="button"
                          className="filter-reset-button"
                          onClick={resetListColumns}
                        >
                          Show all
                        </button>
                      )}
                    </div>
                    <div className="board-filter-status-list" role="menu" aria-label="Columns">
                      {OPTIONAL_LIST_COLUMNS.map(([colId, colLabel]) => {
                        const isChecked = visibleColumns.has(colId);
                        return (
                          <button
                            key={colId}
                            type="button"
                            role="menuitemcheckbox"
                            aria-checked={isChecked}
                            className={`board-filter-status-item ${isChecked ? "active" : ""}`}
                            onClick={() => toggleListColumn(colId)}
                          >
                            <span className={`board-filter-checkbox ${isChecked ? "checked" : ""}`}>
                              {isChecked && <Icon name="check" />}
                            </span>
                            <span className="board-filter-status-name">{colLabel}</span>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                )}
              </div>
            )}
            {ticketView === "board" && (
              <div className="board-grouping-switch" role="group" aria-label="Ticket grouping">
                <button
                  type="button"
                  className="grouping-toggle"
                  aria-pressed={grouping === "stage"}
                  title="By workflow stage"
                  aria-label="By workflow stage"
                  onClick={() => {
                    setGrouping("stage");
                    savePreference("boardGrouping", "stage");
                  }}
                >
                  <Icon name="sparkles" />
                </button>
                <button
                  type="button"
                  className="grouping-toggle"
                  aria-pressed={grouping === "tracker"}
                  title="By tracker status"
                  aria-label="By tracker status"
                  onClick={() => {
                    setGrouping("tracker");
                    savePreference("boardGrouping", "tracker");
                  }}
                >
                  <Icon name="kanban" />
                </button>
                <select
                  aria-label="Group tickets"
                  className="sr-only"
                  tabIndex={-1}
                  value={grouping}
                  onChange={e => {
                    const value = e.target.value === "tracker" ? "tracker" : "stage";
                    setGrouping(value);
                    savePreference("boardGrouping", value);
                  }}
                >
                  <option value="stage">By workflow stage</option>
                  <option value="tracker">By tracker status</option>
                </select>
              </div>
            )}
            {ticketView==="board"&&<button type="button" className="board-density" aria-label="Compact cards" title="Toggle card density" aria-pressed={compactCards} onClick={()=>setCompactCards(value=>{const next=!value;savePreference("boardCards",next?"compact":"full");return next;})}>{compactCards?"Compact":"Detailed"}</button>}
            <span className="badge" title="Tracker statuses · read-only">{count} ticket{count>1?"s":""}</span><span className="sr-only">Tracker statuses · read-only</span>
            <button type="button" className="board-new-ticket" aria-label="New ticket" title="New ticket" disabled={!monitor.fresh||!monitor.snapshot?.status.capabilities?.includes("task-page")} onClick={()=>{if(project)navigate(project,"ticket-editor");}}><Icon name="plus"/></button>
            {selectedTasks.length>0&&<div className="board-batch-controls"><span className="board-selection-count">{selectedTasks.length} selected</span><button type="button" onClick={()=>{setSelectedTaskIds([]);setBatchConfirm(false);}}>Clear selection</button><button type="button" disabled={selectedTasks.length<2||launching||selectedTasks.some(task=>taskStage(task,context?.board)==="finished")||!context?.skills.some(skill=>skill.id==="pickup_issues")} onClick={()=>setBatchConfirm(true)}>Prepare batch</button></div>}
          </div>}
          {launchError&&<p className="error" role="alert">{launchError}</p>}
          {batchConfirm&&<section className="batch-confirm" aria-label="Prepare batch"><strong>Run chain on {selectedTasks.length} tickets</strong><p>Selected tickets will be processed together in a single workspace. Check their order before launching.</p><ol>{selectedTasks.map(task=><li key={task.id}>{task.key||task.id} · {task.title}</li>)}</ol><button type="button" disabled={launching||selectedTasks.length<2} onClick={()=>void startBatch()}>{launching?"Launching…":"Launch batch"}</button><button type="button" onClick={()=>setBatchConfirm(false)}>Cancel</button></section>}
          {busy && (
            <p className="hint" role="status">
              Refreshing tickets…
            </p>
          )}
          {boardContextPending && !busy && <p className="hint" role="status">Loading project configuration…</p>}
          {currentContextError && <div className="error" role="alert">Board configuration unavailable: {currentContextError} <button onClick={()=>setContextRefresh(value=>value+1)}>Retry</button></div>}
          {ticketView==="list"&&!localOnlyProject?<div className="ticket-list" role="region" aria-label="Ticket list" tabIndex={0}><table><thead><tr><th data-col="select" aria-label="Selection"/>{([["key","Key"],["title","Title"],["parent","Macro"],["issueType","Type"],["stage","Status"],["priority","Priority"],["sprint","Sprint"],["assignee","Assignee"]] as const).filter(([field])=>field==="key"||field==="title"||visibleColumns.has(field)).map(([field,label])=>{const active=tableSort.field===field;return <th key={field} data-col={field} scope="col" aria-sort={active?(tableSort.ascending?"ascending":"descending"):"none"}><button type="button" className="sort-header" onClick={()=>setTableSort(current=>nextSort(current,field))}>{label}{active&&<span className="sort-arrow" aria-hidden="true">{tableSort.ascending?"▲":"▼"}</span>}</button></th>;})}{visibleColumns.has("engine")&&<th data-col="engine">Engine</th>}{visibleColumns.has("pr")&&<th data-col="pr">PR</th>}<th data-col="actions">Actions</th></tr></thead><tbody>{sortedTasks.slice(0,visibleListCount).map(task=>{
            const next=context?nextSkill(task,context):undefined;
            const taskRun=activeTaskRuns.get(task.id);
            const active=taskRun&&isActive(taskRun);
            const isFinished=taskStage(task,context?.board)==="finished";
            const runDisabled=launching||!monitor.fresh||isFinished||!!active||!next?.skillId||!context?.configured;
            const actionLabel=active?"Running":isFinished?"Finished":next?.skillId?next.label:"Launch";
            const runTitle=active?"An execution is running on this ticket":isFinished?"This ticket is finished":!context?.configured?"Configure the project's local repository to launch a skill":next?.skillId?`Launch: ${next.label}`:(next?.reason||"No action available");
            return <tr key={task.id}>
              <td data-col="select"><input type="checkbox" aria-label={`Select ${task.key||task.title}`} checked={selectedTaskIds.includes(task.id)} onChange={()=>toggleTask(task.id)}/></td>
              <td data-col="key"><button type="button" className="task-key-link" title={`Open ticket ${task.key||task.id}`} aria-label={`Open ticket ${task.key||task.id}`} onClick={()=>{if(project)navigate(project,"task",undefined,task.id);}}>{task.key||task.id}</button></td>
              <td data-col="title"><button type="button" className="task-open" onClick={()=>{if(project)navigate(project,"task",undefined,task.id);}}>{task.title}</button></td>
              {visibleColumns.has("parent")&&<td data-col="parent">{task.parentKey?<small className="epic-chip" style={epicStyle(task)} title={task.parentTitle}>{task.parentKey}</small>:"—"}</td>}
              {visibleColumns.has("issueType")&&<td data-col="issueType">{task.issueType||"—"}</td>}
              {visibleColumns.has("stage")&&<td data-col="stage">{task.trackerStatus||"—"}</td>}
              {visibleColumns.has("priority")&&<td data-col="priority"><span className={`priority priority-${task.priority||"none"}`}>{task.priority||"—"}</span></td>}
              {visibleColumns.has("sprint")&&<td data-col="sprint">{task.sprint||"—"}</td>}
              {visibleColumns.has("assignee")&&<td data-col="assignee">{task.assignee||"Unassigned"}</td>}
              {visibleColumns.has("engine")&&<td data-col="engine">{context?.aiProvider?<span className="engine-mark" title={context.aiProvider==="codex"?"Codex":context.aiProvider==="claude"?"Claude Code":context.aiProvider}>{engineMark(context.aiProvider)}</span>:"—"}</td>}
              {visibleColumns.has("pr")&&<td data-col="pr" className="ticket-pr-cell">{(()=>{
                const prLinks = taskPullRequests(task);
                const primaryPr = prLinks[0] || (task.prUrl && /^https?:\/\//.test(task.prUrl) ? { url: task.prUrl } : undefined);
                if (!primaryPr) return "—";
                return (
                  <button
                    type="button"
                    className="task-pr-link"
                    title={`Open ${pullRequestLabel(primaryPr)} · ${pullRequestStateLabel(primaryPr)}`}
                    aria-label={`Open PR for ${task.key||task.title}`}
                    onClick={()=>void openExternalLink(primaryPr.url).catch(()=>setLaunchError("Unable to open PR."))}
                  >
                    <Icon name="pr" size={14}/>
                    {prLinks.length > 1 && <small className="pr-count">{prLinks.length}</small>}
                  </button>
                );
              })()}</td>}
              <td data-col="actions" className="ticket-actions-cell">
                <div className="ticket-actions">
                  <button
                    type="button"
                    className="ticket-run"
                    disabled={runDisabled}
                    title={runTitle}
                    aria-label={`Launch ${next?.label||"action"} for ${task.key||task.title}`}
                    onClick={()=>{if(next?.skillId)void startSkill(task,next.skillId);}}
                  >
                    {actionLabel}
                  </button>
                  {taskMenu(task)}
                </div>
              </td>
            </tr>;
          })}</tbody></table>{sortedTasks.length>visibleListCount&&<button type="button" className="board-show-more" onClick={()=>setVisibleListCount(value=>value+80)}>Show next 80 · {visibleListCount} of {sortedTasks.length}</button>}</div>:<div className="board" role="region" aria-label="Ticket columns, horizontal scroll" tabIndex={0}>
            {columns.map((column) => (
              <section className="column" data-stage={column.stage} key={column.name}>
                <h2>
                  <span className="dot" />
                  {column.name}
                  <span>{column.tasks.length}</span>
                </h2>
                {visibleBoardTasks(column.tasks,activeTaskRuns,visibleByColumn[column.name] ?? 40).map((task) => {
                  const taskRun = activeTaskRuns.get(task.id);
                  return <article className={`task ${compactCards?"compact":""} ${epicStyle(task)?"has-epic-color":""}`} style={epicStyle(task)} key={task.id}>
                    <div className="task-meta">
                      <label><input type="checkbox" aria-label={`Select ${task.key||task.title}`} checked={selectedTaskIds.includes(task.id)} onChange={()=>toggleTask(task.id)}/>{task.key || task.id}</label>
                      {taskMenu(task)}
                    </div>
                    <h3><button className="task-open" title={task.title} onClick={()=>{if(project)navigate(project,"task",undefined,task.id);}}>{task.title}</button></h3>
                    {task.parentKey&&<span className="epic-chip" style={epicStyle(task)} title={task.parentTitle}>{task.parentKey}</span>}
                    {task.issueType&&<span className="issue-type-chip" title={`Type: ${task.issueType}`}>{task.issueType}</span>}
                    {task.sprint&&<span className="sprint-chip" title={`Sprint: ${task.sprint}`}>{task.sprint}</span>}
                    {task.priority&&<span className={`priority priority-${task.priority}`}>{task.priority}</span>}
                    {taskRun && isActive(taskRun) && <p className="task-run-state" role="status">{skillLabels[taskRun.skill] || taskRun.skill} · {statusLabel(taskRun)}</p>}
                    <footer>
                      <span>{task.assignee || "Unassigned"}</span>
                      {project && (
                        <button
                          title="Open ticket workflow"
                          aria-label={`Open workflow for ${task.key || task.title}`}
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
                  aria-label={`Show more tickets in ${column.name}`}
                  onClick={()=>setVisibleByColumn(previous=>({...previous,[column.name]:(previous[column.name]??40)+40}))}
                >Show next {Math.min(40,column.tasks.length-(visibleByColumn[column.name]??40))} · {Math.min(visibleByColumn[column.name]??40,column.tasks.length)} of {column.tasks.length}</button>}
              </section>
            ))}
          </div>}
          {!busy && !error && !currentContextError && !boardContextPending && !count && (
            <div className="empty">
              <Icon name="board" />
              <h2>{localOnlyProject ? project?.path ? "Your local project is ready" : "Choose a folder to get started" : "No remote tickets"}</h2>
              <p>{localOnlyProject ? project?.path ? "No tracker is attached. Start a prompt to work in your local folder." : "Start a free prompt in this project without configuring a tracker." : query ? "Try another search or check project sync." : project?.path ? "The tracker returned no tickets. You can start a prompt in the mapped local folder." : "Check the tracker sync and scope."}</p>
              {localOnlyProject && project && !project.path && <ProjectFolderSetup variant="board" project={project} onMapped={path=>onFolderMapped(project.id,path)} />}
            </div>
          )}
        </div>
      ) : selection.view === "prompt" ? (
        selection.runId || project?.path?.trim() ? <Suspense fallback={<p>Opening prompt…</p>}>
          <ProjectPrompt
            key={`${selection.projectId}:${selection.runId??"new"}`}
            projectName={project?.name || "Project"}
            agentAvailable={monitor.fresh && !!monitor.snapshot?.status.connected}
            directory={workingDirectory}
            folders={currentRun?.folders}
            run={currentRun}
            task={tasks.find(task=>task.id===currentRun?.taskId)}
            board={context?.board}
            projectContext={context}
            taskRuns={currentRun?.taskId ? projectRuns(monitor.snapshot?.runs ?? [], selection.projectId).filter(r => r.taskId === currentRun.taskId) : undefined}
            canEditTask={Boolean(monitor.fresh && monitor.snapshot?.status.capabilities?.includes("task-page"))}
            knownSprints={availableSprints.sprints}
            knownAssignees={availableAssignees.people}
            onTaskSaved={saved => {
              onTaskChanged(saved);
              void refresh();
            }}
            onSelectRun={nextRunId => {
              if (!project || !nextRunId) return;
              const target = monitor.snapshot?.runs.find(item => item.id === nextRunId);
              if (target?.conversation) navigate(project, "prompt", target.id);
              else navigate(project, "executions", nextRunId);
            }}
            onLaunchSkill={currentRun?.taskId ? skillId => {
              const targetTask = tasks.find(t => t.id === currentRun.taskId);
              if (targetTask) void startSkill(targetTask, skillId);
            } : undefined}
            onEditTask={currentRun?.taskId && project && monitor.snapshot?.status.capabilities?.includes("task-page") ? () => navigate(project, "ticket-editor", undefined, currentRun.taskId) : undefined}
            onOpenQueue={project?()=>navigate(project,"executions"):undefined}
            onOpenTask={currentRun?.taskId&&project?()=>void openTaskInTracker(project.id,currentRun.taskId,tasks.find(task=>task.id===currentRun?.taskId),setLaunchError):undefined}
            onArchive={currentRun?()=>onArchiveConversation(currentRun.id):undefined}
            archived={!!currentRun&&archivedConversationIds.includes(currentRun.id)}
            onStartOver={currentRun?()=>setStartOverSource(currentRun):undefined}
            onOpenSource={project&&currentRun?.contextRunId&&monitor.snapshot?.runs.some(item=>item.id===currentRun.contextRunId)?sourceId=>navigate(project,"prompt",sourceId):undefined}
            onStopped={()=>void monitor.refresh()}
            onFoldersChanged={()=>void monitor.refresh()}
            onTitleChanged={()=>void monitor.refresh()}
            projectId={selection.projectId}
            runId={selection.runId}
            onCreated={id => { if (project) navigate(project, "prompt", id); }}
          />
          {startOverSource && project && (()=>{
            const offer=startOverSkills(startOverSource);
            const sourceTask=tasks.find(item=>item.id===startOverSource.taskId);
            return <StartOverDialog
              projectId={selection.projectId}
              source={startOverSource}
              agentAvailable={monitor.fresh && !!monitor.snapshot?.status.connected}
              skills={offer.skills}
              skillUnavailable={offer.reason}
              onStartSkill={sourceTask?(skillId,instructions)=>startSkill(sourceTask,skillId,"conversation",{prompt:instructions,contextRunId:startOverSource.id}):undefined}
              onConversationCreated={id=>{void monitor.refresh();navigate(project,"prompt",id);}}
              onClose={()=>setStartOverSource(null)}
            />;
          })()}
        </Suspense> : project ? <ProjectFolderSetup project={project} onMapped={path=>onFolderMapped(project.id,path)} /> : <p>Loading project…</p>
      ) : (
        <Supervisor
          projectId={selection.projectId}
          tasks={tasks}
          onChanged={() => void refresh()}
          onOpenConversation={id => { if (project) navigate(project, "prompt", id); }}
          view={selection.view as "launch" | "executions" | "orchestration"}
          selectedRun={selectedRun}
          onSelectRun={(id) => {
            if (project && id) {
              const r = monitor.snapshot?.runs.find(item => item.id === id);
              if (r?.conversation) {
                navigate(project, "prompt", id);
                return;
              }
              navigate(project, "executions", id);
            } else setSelectedRun(id);
          }}
          initialTaskId={selection.taskId}
          archivedConversationIds={archivedConversationIds}
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

function RunNavigation({run,task,selected,onClick,onOpenTask}:{run:Run;task?:Task;selected:boolean;onClick:()=>void;onOpenTask?:()=>void}){
 if(run.conversation)return <ConversationNavigation run={run} task={task} selected={selected} onClick={onClick} onOpenTask={onOpenTask} />;
 return <button className={`sidebar-run ${selected ? "selected" : ""}`} title={statusLabel(run)} onClick={onClick}>
  <span className={`dot ${isActive(run) ? "live" : ""}`} />
  {run.taskKey && <span role="button" tabIndex={0} className="sidebar-task-key" title={`Open ticket ${run.taskKey}`} aria-label={`Open ticket ${run.taskKey}`} onClick={e=>{e.stopPropagation();onOpenTask?.();}} onKeyDown={e=>{if(e.key==="Enter"||e.key===" "){e.preventDefault();e.stopPropagation();onOpenTask?.();}}}>{run.taskKey}</span>}
  <span>{run.taskKey || run.skill || "Execution"}</span>
 </button>;
}
function ConversationPrAction({task,taskKey,onError}:{task?:Task;taskKey?:string;onError:()=>void}){
 const links=taskPullRequests(task);
 if(!links.length)return null;
 const current=links[links.length-1];
 return <button className="conversation-pr-action" type="button" aria-label={`Open PR/MR for ${task?.key||taskKey||task?.title||"this ticket"}`} title={`${links.length>1?`${links.length} PR/MR · `:""}${pullRequestLabel(current)} · ${pullRequestStateLabel(current)}`} onClick={()=>void openExternalLink(current.url).catch(onError)}><Icon name="branch"/>{links.length>1&&<small>{links.length}</small>}</button>;
}
function ConversationNavigation({run,task,selected,onClick,onOpenTask}:{run:Run;task?:Task;selected:boolean;onClick:()=>void;onOpenTask?:()=>void}){
 // Subscribe to already loaded transcripts; navigation never fetches histories.
 const {snapshot,loaded,error}=useConversation(run.id);
 const title=useMemo(()=>conversationTitle(snapshot,run,task),[snapshot.events,snapshot.title,run,task]);
 const provider=run.provider==="claude"?"Claude Code":run.provider==="codex"?"Codex":run.engineName||"Assistant";
 const needsInput=isActive(run)&&((loaded&&snapshot.id===run.id&&!!snapshot.approvals?.length)||(!!run.waitingSince&&!run.waitingSince.startsWith("0001"))||run.status==="waiting");
 const activity=!isActive(run)?statusLabel(run):error?"Tracking interrupted":needsInput?"Action required":loaded&&snapshot.id===run.id?snapshot.busy?"Running":snapshot.readOnly?"Read-only":"Ready":statusLabel(run);
 return <button className={`sidebar-run conversation-navigation ${selected ? "selected" : ""}`} onClick={onClick} title={`${run.taskKey?`${run.taskKey} · `:""}${title} · ${provider}${run.model ? ` · ${run.model}` : ""} · ${task?.trackerStatus?`${task.trackerStatus} · `:""}${activity}`}>
  <span className={`conversation-indicator ${needsInput?"needs-input":loaded&&snapshot.busy?"live":isActive(run)?"session":"idle"}`} />
  {run.taskKey && <span role="button" tabIndex={0} className="sidebar-task-key" title={`Open ticket ${run.taskKey}`} aria-label={`Open ticket ${run.taskKey}`} onClick={e=>{e.stopPropagation();onOpenTask?.();}} onKeyDown={e=>{if(e.key==="Enter"||e.key===" "){e.preventDefault();e.stopPropagation();onOpenTask?.();}}}>{run.taskKey}</span>}
  <span className="conversation-navigation-text"><span>{title}</span></span>
 </button>;
}
