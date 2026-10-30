import { useEffect, useState } from "react";
import { createTask, getTask, taskIssueTypes, taskTrackers, updateTask, type TaskFields, type TaskTrackers, type TaskUpdateFields } from "../lib/agent";
import type { Task, TaskPullRequest } from "../domain/board";

const emptyDraft: TaskFields = { title: "", description: "", priority: "medium", issueType: "" };
const priorityOptions = [["urgent", "Urgente"], ["high", "Haute"], ["medium", "Moyenne"], ["low", "Basse"]];
const taskLinks = (task?: Task): TaskPullRequest[] => task?.prLinks?.length ? task.prLinks : task?.prUrl ? [{ url: task.prUrl }] : [];
const sameLinks = (left: TaskPullRequest[], right: TaskPullRequest[]) => left.length === right.length && left.every((link, index) => link.url === right[index].url);
function validatedLink(value: string, existing: TaskPullRequest[]): { url?: string; error?: string } {
  const text = value.trim();
  let parsed: URL;
  try { parsed = new URL(text); } catch { return { error: "Saisissez une URL HTTP ou HTTPS complète." }; }
  if (!["http:", "https:"].includes(parsed.protocol) || !parsed.hostname || parsed.username || parsed.password || text.length > 2048)
    return { error: "Lien PR/MR invalide ou contenant des identifiants." };
  if (existing.some(link => link.url === text)) return { error: "Ce lien est déjà ajouté." };
  return { url: text };
}

export function TaskEditor({ projectId, projectName, taskId, available, onClose, onSaved }: {
  projectId: string; projectName: string; taskId?: string; available: boolean;
  onClose: () => void; onSaved: (task: Task) => void;
}) {
  const [trackers, setTrackers] = useState<TaskTrackers>();
  const [trackerId, setTrackerId] = useState("");
  const [original, setOriginal] = useState<Task>();
  const [draft, setDraft] = useState<TaskFields>(emptyDraft);
  const [types, setTypes] = useState<string[]>([]);
  const [typesError, setTypesError] = useState("");
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [createdId, setCreatedId] = useState("");
  const [links, setLinks] = useState<TaskPullRequest[]>([]);
  const [linkInput, setLinkInput] = useState("");
  const [linkError, setLinkError] = useState("");
  useEffect(() => {
    let current = true;
    setLoading(true); setError(""); setCreatedId(""); setOriginal(undefined); setDraft(emptyDraft); setLinks([]); setLinkInput(""); setLinkError("");
    void Promise.all([taskTrackers(projectId), taskId ? getTask(projectId, taskId) : Promise.resolve(undefined)]).then(([sources, task]) => {
      if (!current) return;
      setTrackers(sources);
      setOriginal(task);
      setDraft(task ? { title: task.title ?? "", description: task.description ?? "", priority: task.priority || "medium", issueType: task.issueType ?? "" } : emptyDraft);
      setLinks(taskLinks(task));
      const selected = task?.trackerId || sources.defaultTrackerId || sources.trackers[0]?.id || "";
      setTrackerId(selected);
    }).catch(cause => { if (current) setError(`Impossible de charger le ticket ou ses trackers : ${String(cause)}`); })
      .finally(() => { if (current) setLoading(false); });
    return () => { current = false; };
  }, [projectId, taskId]);
  useEffect(() => {
    if (!trackerId) { setTypes([]); setTypesError(""); return; }
    let current = true;
    setTypesError("");
    void taskIssueTypes(projectId, trackerId).then(result => { if (current) setTypes(Array.isArray(result.issueTypes) ? result.issueTypes : []); })
      .catch(() => { if (current) { setTypes([]); setTypesError("Types de ticket indisponibles. Le type actuel reste utilisable."); } });
    return () => { current = false; };
  }, [projectId, trackerId]);
  const dirty = original ? draft.title !== original.title || draft.description !== (original.description ?? "") || draft.priority !== (original.priority || "medium") || draft.issueType !== (original.issueType ?? "") || !sameLinks(links, taskLinks(original)) : !!draft.title.trim() || !!draft.description.trim();
  const canSave = available && !loading && !saving && !createdId && !!trackers?.trackers.length && !!trackerId && !!draft.title.trim() && draft.title.length <= 500 && draft.description.length <= 60000;
  const selectedTracker = trackers?.trackers.find(item => item.id === trackerId);
  async function save() {
    if (!canSave) return;
    const title = draft.title.trim();
    setSaving(true); setError("");
    try {
      if (!taskId) {
        const created = await createTask(projectId, { ...draft, title, trackerId });
        if (!created?.id) throw new Error("Création non confirmée. Vérifiez le tableau avant de réessayer.");
        setCreatedId(created.id);
        onSaved({ ...draft, ...created });
      } else {
        if (!original) throw new Error("Ticket non chargé.");
        const changes: TaskUpdateFields = {};
        if (title !== original.title) changes.title = title;
        if (draft.description !== (original.description ?? "")) changes.description = draft.description;
        if (draft.priority !== (original.priority || "medium")) changes.priority = draft.priority;
        if (draft.issueType !== (original.issueType ?? "")) changes.issueType = draft.issueType;
        if (!sameLinks(links, taskLinks(original))) changes.prLinks = links;
        if (!Object.keys(changes).length) { onClose(); return; }
        await updateTask(projectId, taskId, changes);
        const saved = await getTask(projectId, taskId);
        onSaved(saved);
      }
    } catch (cause) { setError(`Enregistrement non confirmé : ${String(cause)}. Vérifiez le tracker avant de réessayer.`); }
    finally { setSaving(false); }
  }
  function close() {
    if (dirty && !createdId && !window.confirm("Abandonner les modifications du ticket ?")) return;
    onClose();
  }
  function addLink() {
    const result = validatedLink(linkInput, links);
    if (!result.url) { setLinkError(result.error || "Lien PR/MR invalide."); return; }
    if (links.length >= 50) { setLinkError("La limite est de 50 liens."); return; }
    setLinks(current => [...current, { url: result.url! }]); setLinkInput(""); setLinkError("");
  }
  function moveLink(index: number, offset: number) {
    const target = index + offset;
    if (target < 0 || target >= links.length) return;
    setLinks(current => { const next = [...current]; [next[index], next[target]] = [next[target], next[index]]; return next; });
  }
  return <section className="task-editor" aria-label={taskId ? "Modifier le ticket" : "Nouveau ticket"}>
    <header className="task-editor-heading"><div><span className="eyebrow">{projectName} · {taskId ? original?.key || taskId : "NOUVEAU TICKET"}</span><h2>{taskId ? "Modifier le ticket" : "Nouveau ticket"}</h2></div><button type="button" onClick={close}>Fermer</button></header>
    {!available && <p role="alert" className="error">L’agent connecté ne permet pas encore l’édition des tickets. Reconnectez un agent compatible.</p>}
    {loading && <p role="status">Chargement du ticket et des trackers…</p>}
    {error && <p role="alert" className="error">{error}</p>}
    {createdId && <p role="status">Ticket créé. Chargement du workflow…</p>}
    {!loading && <form onSubmit={event => { event.preventDefault(); void save(); }}>
      {!trackers?.trackers.length && <p role="status">Ce projet n’a pas de tracker connecté. Ajoutez-en un dans la configuration du projet pour créer un ticket.</p>}
      <div className="task-editor-meta">
        {taskId ? <p><span>Tracker</span><strong>{selectedTracker?.name || selectedTracker?.scope || original?.source || "—"}</strong></p> : trackers && trackers.trackers.length > 1 && <label>Tracker<select value={trackerId} onChange={event => { setTrackerId(event.target.value); setDraft(value => ({ ...value, issueType: "" })); }}>{trackers.trackers.map(item => <option key={item.id} value={item.id}>{item.name || item.scope || item.provider}</option>)}</select></label>}
        <label>Type<select value={draft.issueType} onChange={event => setDraft(value => ({ ...value, issueType: event.target.value }))}><option value="">Par défaut</option>{[...new Set([...types, ...(draft.issueType ? [draft.issueType] : [])])].map(type => <option key={type} value={type}>{type}</option>)}</select></label>
        <label>Priorité<select value={draft.priority} onChange={event => setDraft(value => ({ ...value, priority: event.target.value }))}>{priorityOptions.map(([value, label]) => <option key={value} value={value}>{label}</option>)}</select></label>
      </div>
      {typesError && <p className="muted">{typesError}</p>}
      <label>Titre<input autoFocus required maxLength={500} value={draft.title} onChange={event => setDraft(value => ({ ...value, title: event.target.value }))} placeholder="Que faut-il faire ?" /></label>
      <label>Description <span className="muted">· Markdown</span><textarea rows={6} maxLength={60000} value={draft.description} onChange={event => setDraft(value => ({ ...value, description: event.target.value }))} placeholder="Décrivez le contexte et le résultat attendu…" /></label>
      {taskId && <details className="task-editor-links"><summary>Liens PR/MR · {links.length}</summary><p className="muted">Le dernier lien est le lien principal du ticket.</p><ol>{links.map((link, index) => <li key={`${link.url}-${index}`}><span title={link.url}>{link.url}</span>{link.state && <small>{link.state}</small>}{index === links.length - 1 && <small>Principal</small>}<button type="button" aria-label={`Monter ${link.url}`} disabled={index === 0} onClick={() => moveLink(index, -1)}>↑</button><button type="button" aria-label={`Descendre ${link.url}`} disabled={index === links.length - 1} onClick={() => moveLink(index, 1)}>↓</button><button type="button" aria-label={`Retirer ${link.url}`} onClick={() => setLinks(current => current.filter((_, position) => position !== index))}>×</button></li>)}</ol><div className="task-editor-link-add"><input aria-label="URL PR/MR" value={linkInput} onChange={event => { setLinkInput(event.target.value); setLinkError(""); }} onKeyDown={event => { if (event.key === "Enter") { event.preventDefault(); addLink(); } }} placeholder="https://…" /><button type="button" onClick={addLink}>Ajouter un lien</button></div>{linkError && <p role="alert" className="error">{linkError}</p>}</details>}
      <footer><span>{taskId ? "La mise à jour est envoyée au tracker du ticket." : "Le ticket sera créé dans le tracker sélectionné."}</span><button type="button" onClick={close}>Annuler</button><button className="primary" disabled={!canSave || !!taskId && !dirty}>{saving ? "Enregistrement…" : taskId ? "Enregistrer" : "Créer le ticket"}</button></footer>
    </form>}
  </section>;
}
