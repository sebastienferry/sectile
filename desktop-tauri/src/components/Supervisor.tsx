import { RunReview } from "./RunReview";
import { useExecutionMonitor } from "./ExecutionMonitor";
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from "react";
import type { Task } from "../domain/board";
import type { Orchestration, OrchestrationReport } from "../domain/execution";
import type { Run } from "../domain/execution";
import {
  canLaunchTask,
  canStop,
  isActive,
  launchInput,
  projectRuns,
  statusLabel,
} from "../domain/execution";
import {
  getOrchestration,
  launchTask,
  projectSkills,
  runOrchestration,
  stopRun,
} from "../lib/agent";
import { Icon } from "./Icon";
const RunConsole = lazy(() =>
  import("./RunConsole").then((module) => ({ default: module.RunConsole })),
);

function runTitle(run: Run, tasks: Task[]): string {
  const task = tasks.find(item => item.id === run.taskId);
  if (run.conversation) {
    if (task?.title) return [run.taskKey, task.title].filter(Boolean).join(" · ");
    return run.title?.trim() || run.prompt?.trim().split("\n")[0] || run.taskKey || "Conversation";
  }
  return run.taskKey || task?.title || run.taskId || "Free console";
}

function runSummary(run: Run, archived = false): string {
  const provider = run.provider === "claude" ? "Claude Code" : run.provider === "codex" ? "Codex" : run.engineName || "Assistant";
  return [run.conversation ? provider : run.skill || run.engineName, run.conversation ? run.skill : "", statusLabel(run), archived ? "Archived" : ""].filter(Boolean).join(" · ");
}

export function Supervisor({
  projectId,
  tasks,
  onChanged,
  view = "launch",
  selectedRun = "",
  onSelectRun = () => {},
  onOpenConversation,
  initialTaskId = "",
  active = true,
  archivedConversationIds = [],
  onToggleArchive,
}: {
  projectId: string;
  tasks: Task[];
  onChanged: () => void;
  view?: "launch" | "executions" | "orchestration";
  selectedRun?: string;
  onSelectRun?: (id: string) => void;
  onOpenConversation?: (id: string) => void;
  initialTaskId?: string;
  active?: boolean;
  archivedConversationIds?: string[];
  onToggleArchive?: (runId: string, archived: boolean) => void;
}) {
  const [actionError, setActionError] = useState("");
  const [notice, setNotice] = useState("");
  const [pending, setPending] = useState("");
  const pendingRef = useRef(false);
  const mounted = useRef(true);
  const { snapshot, fresh, pollError, lastRead, refresh } =
    useExecutionMonitor();
  const [taskId, setTaskId] = useState(initialTaskId);
  const [skills, setSkills] = useState<{ id: string }[]>([]);
  const [skillId, setSkillId] = useState("");
  const [prompt, setPrompt] = useState("");
  const [mode, setMode] = useState("");
  const [orchestration, setOrchestration] = useState<Orchestration | null>(
    null,
  );
  const [metadataError, setMetadataError] = useState("");
  const [orchestrationError, setOrchestrationError] = useState("");
  const [report, setReport] = useState<OrchestrationReport | null>(null);
  const [reload, setReload] = useState(0);
  useEffect(() => {
    mounted.current = true;
    return () => {
      mounted.current = false;
    };
  }, []);
  const supportsOrchestration =
    snapshot?.status.capabilities?.includes("orchestration-agents") === true;
  useEffect(() => {
    let disposed = false;
    setSkills([]);
    setSkillId("");
    setMetadataError("");
    void projectSkills(projectId)
      .then((list) => {
        if (!disposed) {
          setSkills(list);
          setSkillId(list[0]?.id ?? "");
        }
      })
      .catch((error) => {
        if (!disposed) setMetadataError(String(error));
      });
    return () => {
      disposed = true;
    };
  }, [projectId, reload]);
  useEffect(() => {
    let disposed = false;
    setOrchestration(null);
    setOrchestrationError("");
    if (supportsOrchestration)
      void getOrchestration(projectId)
        .then((value) => {
          if (!disposed) setOrchestration(value);
        })
        .catch((error) => {
          if (!disposed) setOrchestrationError(String(error));
        });
    return () => {
      disposed = true;
    };
  }, [projectId, reload, supportsOrchestration]);
  useEffect(()=>{
    if(view!=="orchestration"||!supportsOrchestration)return;
    let current=true,reading=false;
    const read=async()=>{if(reading)return;reading=true;try{const latest=await getOrchestration(projectId);if(current){setOrchestration(latest);setOrchestrationError("");}}catch{if(current)setOrchestrationError("Project capacity unverified. Refresh status before launching an agent.");}finally{reading=false;}};
    const timer=setInterval(()=>void read(),15000);
    return()=>{current=false;clearInterval(timer);};
  },[projectId,view,supportsOrchestration]);
  const runs = projectRuns(snapshot?.runs ?? [], projectId);
  const [executionQuery, setExecutionQuery] = useState("");
  const [archiveFilter, setArchiveFilter] = useState<"all" | "active" | "archived">("all");
  const [statusFilter, setStatusFilter] = useState<"all" | "active" | "failed" | "completed">("all");
  useEffect(() => {
    setExecutionQuery("");
    setArchiveFilter("all");
    setStatusFilter("all");
  }, [projectId]);
  const archivedSet = useMemo(() => new Set(archivedConversationIds), [archivedConversationIds]);
  const sortedRuns = useMemo(
    () =>
      [...runs].sort(
        (a, b) =>
          b.createdAt.localeCompare(a.createdAt) ||
          a.id.localeCompare(b.id),
      ),
    [runs],
  );
  const filteredRuns = useMemo(() => {
    const words = executionQuery.trim().toLocaleLowerCase().split(/\s+/).filter(Boolean);
    return sortedRuns.filter(item => {
      const isArchived = archivedSet.has(item.id);
      if (archiveFilter === "active" && isArchived) return false;
      if (archiveFilter === "archived" && !isArchived) return false;
      if (statusFilter === "active" && !isActive(item)) return false;
      if (statusFilter === "failed" && item.status !== "failed") return false;
      if (statusFilter === "completed" && item.status !== "completed" && item.status !== "finished") return false;
      if (!words.length) return true;
      const haystack = [
        runTitle(item, tasks),
        runSummary(item, isArchived),
        item.taskKey,
        item.skill,
        item.branch,
        item.provider,
        item.engineName,
      ]
        .filter(Boolean)
        .join(" ")
        .toLocaleLowerCase();
      return words.every(word => haystack.includes(word));
    });
  }, [archiveFilter, archivedSet, executionQuery, sortedRuns, statusFilter, tasks]);
  const activeExecutionCount = runs.filter(run => isActive(run) && (!run.conversation || !!run.taskId)).length;
  const openFreeConversationCount = runs.filter(run => isActive(run) && run.conversation && !run.taskId).length;
  const connected =
    fresh &&
    snapshot?.status.connected === true &&
    !snapshot.status.contractError;
  const launchable = tasks.filter(
    (t) =>
      ["github", "gitlab", "jira"].includes(t.source ?? "") &&
      canLaunchTask(t.status, t.id, snapshot?.runs ?? []),
  );
  const selectedTask = launchable.find((t) => t.id === taskId);
  const run = runs.find((r) => r.id === selectedRun);
  async function act(
    key: string,
    action: () => Promise<unknown>,
    success: string,
  ) {
    if (pendingRef.current) return;
    pendingRef.current = true;
    setPending(key);
    setActionError("");
    setNotice("");
    try {
      await action();
      if (mounted.current) setNotice(success);
    } catch (error) {
      if (mounted.current) setActionError(String(error));
    } finally {
      if (mounted.current) {
        await refresh();
        if (mounted.current) {
          setReload((v) => v + 1);
          onChanged();
          setPending("");
        }
      }
      pendingRef.current = false;
    }
  }
  return (
    <section className="supervisor" data-view={view}>
      <div className="supervisor-heading">
        <h2>
          {view === "launch"
            ? "New launch"
            : view === "orchestration"
              ? "Orchestration"
              : run ? "Execution details" : "Executions"}
        </h2>
        <span className="muted" title="Auto-refresh every 3 s">
          {fresh
            ? `Up to date · ${lastRead}`
            : "Status unverified"}
        </span>
        <button
          aria-label="Refresh status"
          onClick={() => {
            void refresh();
            setReload((v) => v + 1);
          }}
        >
          Refresh
        </button>
      </div>
      {pollError && (
        <p role="alert" className="error">
          Status unavailable: {pollError}. Displayed data is stale.
        </p>
      )}
      {snapshot && !snapshot.status.connected && (
        <p role="alert">
          The local agent is disconnected from the server. New launches are
          disabled.
        </p>
      )}
      {snapshot?.status.contractError && (
        <p role="alert">{snapshot.status.contractError}</p>
      )}
      {actionError && (
        <p className="error" role="alert">
          {actionError}
        </p>
      )}
      {notice && (
        <p role="status" className="success">
          {notice}
        </p>
      )}
      <details className="launch-panel" open hidden={view !== "launch"}>
        <summary>Start a task</summary>
        {metadataError && (
          <p role="alert">Skills unavailable: {metadataError}</p>
        )}
        <form
          onSubmit={(event) => {
            event.preventDefault();
            if (selectedTask && connected && skillId)
              void act(
                "launch",
                () =>
                  launchTask(
                    projectId,
                    launchInput(taskId, skillId, prompt, mode),
                  ),
                "Request accepted. The execution will appear once picked up by the agent.",
              );
          }}
        >
          <label>
            Ticket
            <select
              required
              value={taskId}
              disabled={!!pending}
              onChange={(e) => setTaskId(e.target.value)}
            >
              <option value="">Choose a ticket</option>
              {launchable.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.key || t.id} — {t.title}
                </option>
              ))}
            </select>
          </label>
          <label>
            Skill
            <select
              required
              value={skillId}
              disabled={!!pending}
              onChange={(e) => setSkillId(e.target.value)}
            >
              {skills.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.id}
                </option>
              ))}
            </select>
          </label>
          <label>
            Mode
            <select
              value={mode}
              disabled={!!pending}
              onChange={(e) => setMode(e.target.value)}
            >
              <option value="">Project configuration</option>
              <option value="autonomous">Autonomous</option>
              <option value="interactive">
                Interactive (local console)
              </option>
            </select>
          </label>
          <label className="wide">
            Additional instructions
            <textarea
              value={prompt}
              maxLength={5000}
              disabled={!!pending}
              onChange={(e) => setPrompt(e.target.value)}
              rows={2}
            />
          </label>
          <p className="muted wide">
            Provider and permissions come from the Sectile configuration. Pickup
            chains are always autonomous. Interactive mode may open an external
            terminal.
          </p>
          <button
            type="submit"
            disabled={
              !connected ||
              !!pending ||
              !selectedTask ||
              !skillId ||
              !skills.some((s) => s.id === skillId)
            }
          >
            {pending === "launch" ? "Launching…" : "Start task"}
          </button>
        </form>
      </details>
      <details
        className="orchestration-panel"
        open
        hidden={view !== "orchestration"}
      >
        <summary>Backlog orchestration</summary>
        <p className="muted">{runs.filter(r=>!!r.taskId&&isActive(r)).length} active executions on this workstation. Shared orchestration capacity is calculated by the server.</p>
        {view === "orchestration" && runs.filter(r=>!!r.taskId&&isActive(r)).map(r=><article className="orchestration-live-run" key={r.id}><strong>{tasks.find(t=>t.id===r.taskId)?.title||r.taskKey||r.taskId}</strong><span>{r.skill} · {statusLabel(r)}</span><button onClick={()=>onSelectRun(r.id)}>View execution</button><button disabled={!!pending||!canStop(r,fresh)} onClick={()=>void act(`stop:${r.id}`,()=>stopRun(r.id),"Stop requested for this execution.")}>Stop this execution</button></article>)}
        {!supportsOrchestration ? (
          <p>
            Status monitoring must be connected to an agent that supports named
            orchestration agents.
          </p>
        ) : (
          <>
            {orchestrationError && <p role="alert">{orchestrationError}</p>}
            {orchestration && (
              <>
                <p>
                  Shared orchestration capacity:{" "}
                  {orchestration.orchestratorEnabled ? "enabled" : "disabled"}{" "}
                  · server limit: {orchestration.orchestratorMaxConcurrent}{" "}
                  executions. {orchestration.orchestratorActiveRuns===undefined?"Server usage unverified.":`${orchestration.orchestratorActiveRuns} / ${orchestration.orchestratorMaxConcurrent} slots in use.`}
                </p>
                <p className="muted">
                  Configure agents and activation in "Configure project". Each launch evaluates the backlog once;
                  it does not create an automatic loop.
                </p>
                {orchestration.agents.map((agent) => (
                  <article className="profile" key={agent.id}>
                    <div>
                      <strong>{agent.name}</strong>
                      <p>{agent.file}</p>
                      {agent.guidelinesError && (
                        <p role="alert">{agent.guidelinesError}</p>
                      )}
                      <details>
                        <summary>Saved guidelines</summary>
                        <pre>{agent.guidelines}</pre>
                      </details>
                    </div>
                    <button
                      disabled={
                        !connected ||
                        !!pending ||
                        !orchestration.orchestratorEnabled || !!orchestrationError ||
                        !!agent.guidelinesError
                      }
                      onClick={() =>
                        void act(
                          agent.id,
                          async () => {
                            const result = await runOrchestration(
                              projectId,
                              agent.id,
                            );
                            if (mounted.current) setReport(result);
                          },
                          "Backlog evaluation completed.",
                        )
                      }
                    >
                      {pending === agent.id
                        ? "Evaluating…"
                        : "Launch this agent"}
                    </button>
                  </article>
                ))}
                {!orchestration.agents.length && (
                  <p>No agents saved in this project.</p>
                )}
              </>
            )}
            {report && (
              <div role="status">
                <p>
                  {report.launched.length} launched · {report.skipped.length}{" "}
                  tickets skipped · capacity after evaluation:{" "}
                  {report.activeRunsAfter}/{report.maxConcurrent}
                </p>
                <ul>
                  {report.launched.map((t) => (
                    <li key={t.id}>Launched: {t.key || t.title || t.id}</li>
                  ))}
                  {report.skipped.map((t) => (
                    <li key={t.id}>
                      {t.key || t.id}: {t.reason}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </details>
      <div hidden={view !== "executions"} className="execution-workspace">
        {!run && <h3>Workstation executions · {activeExecutionCount} active execution{activeExecutionCount > 1 ? "s" : ""}{openFreeConversationCount > 0 && ` · ${openFreeConversationCount} open session${openFreeConversationCount > 1 ? "s" : ""}`}</h3>}
        {!run && runs.length > 0 && (
          <div className="execution-toolbar">
            <label className="search execution-search">
              <Icon name="search" />
              <input
                aria-label="Filter executions"
                placeholder="Filter by title, ticket, skill, or branch…"
                value={executionQuery}
                onChange={event => setExecutionQuery(event.target.value)}
              />
            </label>
            <div className="execution-archive-switch" role="group" aria-label="Filter by status">
              {([
                ["all", "All"],
                ["active", "Active"],
                ["failed", "Failed"],
                ["completed", "Completed"],
              ] as const).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={statusFilter === value}
                  onClick={() => setStatusFilter(value)}
                >
                  {label}
                </button>
              ))}
            </div>
            <div className="execution-archive-switch" role="group" aria-label="Filter by archive status">
              {([
                ["all", "All"],
                ["active", "Unarchived"],
                ["archived", "Archived"],
              ] as const).map(([value, label]) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={archiveFilter === value}
                  onClick={() => setArchiveFilter(value)}
                >
                  {label}
                </button>
              ))}
            </div>
          </div>
        )}
        <div className="execution-list" hidden={!!run}>
          {filteredRuns.map((item) => (
            <article
              className={
                selectedRun === item.id ? "execution selected" : "execution"
              }
              key={item.id}
            >
              <button
                className="run-select"
                title={runTitle(item, tasks)}
                onClick={() => (item.conversation || (item.headless && item.trace)) && onOpenConversation ? onOpenConversation(item.id) : onSelectRun(item.id)}
              >
                <strong>{runTitle(item, tasks)}</strong>
                <span>{runSummary(item, archivedSet.has(item.id))}</span>
                <small>
                  {new Date(item.createdAt).toLocaleString("en-US")}
                  {item.engineName ? ` · ${item.engineName}` : ""}
                </small>
              </button>
              {onToggleArchive && !isActive(item) && (
                <button
                  type="button"
                  className="icon-button"
                  aria-label={archivedSet.has(item.id) ? "Restore execution" : "Archive execution"}
                  title={archivedSet.has(item.id) ? "Restore execution" : "Archive execution"}
                  onClick={() => onToggleArchive(item.id, !archivedSet.has(item.id))}
                >
                  <Icon name="archive" />
                </button>
              )}
              <button
                disabled={!!pending || !canStop(item, fresh)}
                onClick={() =>
                  void act(
                    `stop:${item.id}`,
                    () => stopRun(item.id),
                    "Stop confirmed by the agent. Status is refreshed.",
                  )
                }
              >
                {pending === `stop:${item.id}` ? "Stopping…" : "Stop"}
              </button>
            </article>
          ))}
        </div>
        {fresh && !runs.length && (
          <p className="muted">No local executions for this project.</p>
        )}
        {fresh && runs.length > 0 && !filteredRuns.length && !run && (
          <p className="muted">No executions match the selected filters.</p>
        )}
        {run && (
          <section className="run-detail">
            <div className="run-detail-nav">
              <button className="text-button" onClick={() => onSelectRun("")}>
                ← All executions
              </button>
              {(run.conversation || (run.headless && run.trace)) && onOpenConversation && <button type="button" onClick={() => onOpenConversation(run.id)}>Open conversation</button>}
            </div>
            <div className="run-heading">
              <h3 title={runTitle(run, tasks)}>{runTitle(run, tasks)}</h3>
              <span className="run-status">{statusLabel(run)}</span>
              {onToggleArchive && !isActive(run) && (
                <button
                  type="button"
                  className="icon-button"
                  aria-label={archivedSet.has(run.id) ? "Restore execution" : "Archive execution"}
                  title={archivedSet.has(run.id) ? "Restore execution" : "Archive execution"}
                  onClick={() => onToggleArchive(run.id, !archivedSet.has(run.id))}
                >
                  <Icon name="archive" />
                </button>
              )}
              <button
                className="danger"
                disabled={!!pending || !canStop(run, fresh)}
                onClick={() =>
                  void act(
                    `stop:${run.id}`,
                    () => stopRun(run.id),
                    "Stop confirmed by the agent.",
                  )
                }
              >
                {pending === `stop:${run.id}` ? "Stopping…" : "Stop"}
              </button>
            </div>
            <p className="muted">
              {run.directory}
              {run.branch ? ` · ${run.branch}` : ""}
            </p>
            <RunReview key={run.id} run={run}/>
            {run.waitingSince && !run.conversation && (
              <p>
                If a response or approval is needed, open the session in
                Sectile Desktop or the provider's terminal.
              </p>
            )}
            {!run.conversation && (run.sessionId || run.trace || run.restored ? (
              <Suspense fallback={<p>Loading console…</p>}>
                {active && <RunConsole key={run.id} runId={run.id} />}
              </Suspense>
            ) : (
              <p>
                The console is not available for this execution yet.
              </p>
            ))}
          </section>
        )}
      </div>
    </section>
  );
}
