import { RunReview } from "./RunReview";
import { useExecutionMonitor } from "./ExecutionMonitor";
import { lazy, Suspense, useEffect, useRef, useState } from "react";
import type { Task } from "../domain/board";
import type { Orchestration, OrchestrationReport } from "../domain/execution";
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
const ProjectPrompt = lazy(()=>import("./ProjectPrompt"));
const RunConsole = lazy(() =>
  import("./RunConsole").then((module) => ({ default: module.RunConsole })),
);

export function Supervisor({
  projectId,
  tasks,
  onChanged,
  view = "launch",
  selectedRun = "",
  onSelectRun = () => {},
  onArchiveConversation,
  initialTaskId = "",
  active = true,
}: {
  projectId: string;
  tasks: Task[];
  onChanged: () => void;
  view?: "launch" | "executions" | "orchestration";
  selectedRun?: string;
  onSelectRun?: (id: string) => void;
  onArchiveConversation?: (id: string) => void;
  initialTaskId?: string;
  active?: boolean;
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
    const read=async()=>{if(reading)return;reading=true;try{const latest=await getOrchestration(projectId);if(current){setOrchestration(latest);setOrchestrationError("");}}catch{if(current)setOrchestrationError("Capacité du projet non vérifiée. Actualisez le suivi avant de lancer un agent.");}finally{reading=false;}};
    const timer=setInterval(()=>void read(),15000);
    return()=>{current=false;clearInterval(timer);};
  },[projectId,view,supportsOrchestration]);
  const runs = projectRuns(snapshot?.runs ?? [], projectId);
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
            ? "Nouveau lancement"
            : view === "orchestration"
              ? "Orchestration"
              : "Exécutions"}
        </h2>
        <span className="muted">
          {fresh
            ? `Dernière lecture : ${lastRead} · toutes les 3 s`
            : "État non vérifié"}
        </span>
        <button
          onClick={() => {
            void refresh();
            setReload((v) => v + 1);
          }}
        >
          Actualiser le suivi
        </button>
      </div>
      {pollError && (
        <p role="alert" className="error">
          Suivi indisponible : {pollError}. Les données affichées sont
          anciennes.
        </p>
      )}
      {snapshot && !snapshot.status.connected && (
        <p role="alert">
          L’agent local est déconnecté du serveur. Les nouveaux lancements sont
          désactivés.
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
        <summary>Démarrer une tâche</summary>
        {metadataError && (
          <p role="alert">Compétences indisponibles : {metadataError}</p>
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
                "Demande acceptée. L’exécution apparaîtra après sa prise en charge par l’agent.",
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
              <option value="">Choisir un ticket</option>
              {launchable.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.key || t.id} — {t.title}
                </option>
              ))}
            </select>
          </label>
          <label>
            Compétence
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
              <option value="">Configuration du projet</option>
              <option value="autonomous">Autonome</option>
              <option value="interactive">
                Interactif (console locale)
              </option>
            </select>
          </label>
          <label className="wide">
            Instructions supplémentaires
            <textarea
              value={prompt}
              maxLength={5000}
              disabled={!!pending}
              onChange={(e) => setPrompt(e.target.value)}
              rows={2}
            />
          </label>
          <p className="muted wide">
            Le fournisseur et les permissions proviennent de la configuration
            Sectile. Les chaînes pickup sont toujours autonomes. Un mode
            interactif peut ouvrir un terminal externe.
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
            {pending === "launch" ? "Lancement…" : "Démarrer la tâche"}
          </button>
        </form>
      </details>
      <details
        className="orchestration-panel"
        open
        hidden={view !== "orchestration"}
      >
        <summary>Orchestration du backlog</summary>
        <p className="muted">{runs.filter(r=>!!r.taskId&&isActive(r)).length} exécutions actives sur ce poste. La capacité d’orchestration partagée est calculée par le serveur.</p>
        {view === "orchestration" && runs.filter(r=>!!r.taskId&&isActive(r)).map(r=><article className="orchestration-live-run" key={r.id}><strong>{tasks.find(t=>t.id===r.taskId)?.title||r.taskKey||r.taskId}</strong><span>{r.skill} · {statusLabel(r)}</span><button onClick={()=>onSelectRun(r.id)}>Voir l’exécution</button><button disabled={!!pending||!canStop(r,fresh)} onClick={()=>void act(`stop:${r.id}`,()=>stopRun(r.id),"Arrêt demandé pour cette exécution.")}>Arrêter cette exécution</button></article>)}
        {!supportsOrchestration ? (
          <p>
            Le suivi doit être connecté à un agent compatible avec les agents
            d’orchestration nommés.
          </p>
        ) : (
          <>
            {orchestrationError && <p role="alert">{orchestrationError}</p>}
            {orchestration && (
              <>
                <p>
                  Capacité d’orchestration partagée :{" "}
                  {orchestration.orchestratorEnabled ? "activée" : "désactivée"}{" "}
                  · limite du serveur : {orchestration.orchestratorMaxConcurrent}{" "}
                  exécutions. {orchestration.orchestratorActiveRuns===undefined?"Occupation du serveur non vérifiée.":`${orchestration.orchestratorActiveRuns} / ${orchestration.orchestratorMaxConcurrent} places utilisées.`}
                </p>
                <p className="muted">
                  Configurez les agents et l’activation dans « Configurer le projet ». Chaque lancement évalue le backlog une fois
                  ; il ne crée aucune boucle automatique.
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
                        <summary>Consignes enregistrées</summary>
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
                          "Évaluation du backlog terminée.",
                        )
                      }
                    >
                      {pending === agent.id
                        ? "Évaluation…"
                        : "Lancer cet agent"}
                    </button>
                  </article>
                ))}
                {!orchestration.agents.length && (
                  <p>Aucun agent enregistré dans ce projet.</p>
                )}
              </>
            )}
            {report && (
              <div role="status">
                <p>
                  {report.launched.length} lancements · {report.skipped.length}{" "}
                  tickets ignorés · capacité après évaluation :{" "}
                  {report.activeRunsAfter}/{report.maxConcurrent}
                </p>
                <ul>
                  {report.launched.map((t) => (
                    <li key={t.id}>Lancé : {t.key || t.title || t.id}</li>
                  ))}
                  {report.skipped.map((t) => (
                    <li key={t.id}>
                      {t.key || t.id} : {t.reason}
                    </li>
                  ))}
                </ul>
              </div>
            )}
          </>
        )}
      </details>
      <div hidden={view !== "executions"} className="execution-workspace">
        <h3>Exécutions de ce poste · {runs.filter(isActive).length} actives</h3>
        <div className="execution-list" hidden={!!run}>
          {runs.map((item) => (
            <article
              className={
                selectedRun === item.id ? "execution selected" : "execution"
              }
              key={item.id}
            >
              <button
                className="run-select"
                onClick={() => onSelectRun(item.id)}
              >
                <strong>
                  {item.taskKey ||
                    tasks.find((t) => t.id === item.taskId)?.title ||
                    item.taskId ||
                    "Console libre"}
                </strong>
                <span>
                  {item.skill || item.engineName} · {statusLabel(item)}
                </span>
                <small>
                  {new Date(item.createdAt).toLocaleString("fr-FR")}
                  {item.engineName ? ` · ${item.engineName}` : ""}
                </small>
              </button>
              <button
                disabled={!!pending || !canStop(item, fresh)}
                onClick={() =>
                  void act(
                    `stop:${item.id}`,
                    () => stopRun(item.id),
                    "Arrêt confirmé par l’agent. Le suivi est actualisé.",
                  )
                }
              >
                {pending === `stop:${item.id}` ? "Arrêt…" : "Arrêter"}
              </button>
            </article>
          ))}
        </div>
        {fresh && !runs.length && (
          <p className="muted">Aucune exécution locale pour ce projet.</p>
        )}
        {run && (
          <section className="run-detail">
            <button className="text-button" onClick={() => onSelectRun("")}>
              ← Toutes les exécutions
            </button>
            <div className="run-heading">
              <h3>
                {run.taskKey || run.id} · {statusLabel(run)}
              </h3>
              <button
                className="danger"
                disabled={!!pending || !canStop(run, fresh)}
                onClick={() =>
                  void act(
                    `stop:${run.id}`,
                    () => stopRun(run.id),
                    "Arrêt confirmé par l’agent.",
                  )
                }
              >
                {pending === `stop:${run.id}` ? "Arrêt…" : "Arrêter"}
              </button>
            </div>
            <p className="muted">
              {run.directory}
              {run.branch ? ` · ${run.branch}` : ""}
            </p>
            <RunReview key={run.id} run={run}/>
            {run.waitingSince && !run.conversation && (
              <p>
                Si une réponse ou une approbation est attendue, ouvrez la
                session dans Sectile Desktop ou le terminal du fournisseur.
              </p>
            )}
            {run.conversation ? <Suspense fallback={<p>Ouverture de la conversation…</p>}><ProjectPrompt projectId={projectId} runId={run.id} run={run} task={tasks.find(task=>task.id===run.taskId)} directory={run.directory} folders={run.folders} onCreated={()=>{}} onOpenQueue={()=>onSelectRun("")} onArchive={onArchiveConversation?()=>onArchiveConversation(run.id):undefined} onStopped={()=>void refresh()} onFoldersChanged={()=>void refresh()}/></Suspense> : run.sessionId || run.trace || run.restored ? (
              <Suspense fallback={<p>Chargement de la console…</p>}>
                {active && <RunConsole key={run.id} runId={run.id} />}
              </Suspense>
            ) : (
              <p>
                La console n’est pas encore disponible pour cette exécution.
              </p>
            )}
          </section>
        )}
      </div>
    </section>
  );
}
