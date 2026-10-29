import { useEffect, useState, type FormEvent } from "react";
import { attachProjectTracker, localSetupStatus, type LocalSetup } from "../lib/local";

export function ProjectTrackerSetup({
  projectId,
  setup: suppliedSetup,
  onChanged,
}: {
  projectId?: string;
  setup?: LocalSetup;
  onChanged: () => void;
}) {
  const [loadedSetup, setLoadedSetup] = useState<LocalSetup>();
  const [selectedProjectId, setSelectedProjectId] = useState(projectId ?? "");
  const [trackerId, setTrackerId] = useState("");
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [reload, setReload] = useState(0);
  useEffect(() => {
    if (suppliedSetup) return;
    let current = true;
    void localSetupStatus().then(value => { if (current) setLoadedSetup(value); })
      .catch(cause => { if (current) setError(String(cause)); });
    return () => { current = false; };
  }, [suppliedSetup, reload]);
  const setup = suppliedSetup ?? loadedSetup;
  const selected = setup?.projects?.find(project => project.id === (projectId ?? selectedProjectId));
  const remoteTrackers = setup?.trackers?.filter(tracker => tracker.provider !== "local") ?? [];
  const linked = remoteTrackers.filter(tracker => selected?.trackers?.some(entry => entry.trackerId === tracker.id));
  const available = remoteTrackers.filter(tracker => !selected?.trackers?.some(entry => entry.trackerId === tracker.id));
  async function submit(event: FormEvent) {
    event.preventDefault();
    if (!trackerId || !selected?.id || pending) return;
    setPending(true); setError(""); setNotice("");
    try {
      await attachProjectTracker(selected.id, trackerId);
      setTrackerId("");
      setNotice("Tracker associé au projet.");
      if (!suppliedSetup) setReload(value => value + 1);
      onChanged();
    } catch (cause) { setError(String(cause)); }
    finally { setPending(false); }
  }
  return <section className="project-tracker-setup" aria-label="Source de tickets du projet">
    <h2>Source de tickets</h2>
    {projectId && <p className="muted">{linked.length ? `Trackers associés : ${linked.map(tracker => tracker.name).join(", ")}.` : "Aucun tracker distant associé."}</p>}
    {!setup && !error && <p>Chargement des trackers…</p>}
    {error && <p role="alert" className="error">{error} {!suppliedSetup && <button type="button" onClick={() => {setError("");setReload(value => value + 1);}}>Réessayer</button>}</p>}
    {notice && <p role="status" className="notice">{notice}</p>}
    {setup && !remoteTrackers.length && <p className="muted">Ajoutez d’abord une source dans Paramètres → Comptes et trackers.</p>}
    {setup && !!remoteTrackers.length && <form onSubmit={submit}>
      <fieldset disabled={pending}>
        {!projectId && <label>Projet existant<select required value={selectedProjectId} onChange={event => {setSelectedProjectId(event.target.value);setTrackerId("");}}><option value="">Choisir un projet</option>{setup.projects?.map(project => <option key={project.id} value={project.id}>{project.name}</option>)}</select></label>}
        <label>Tracker à ajouter<select required value={trackerId} onChange={event => setTrackerId(event.target.value)}><option value="">Choisir un tracker</option>{available.map(tracker => <option key={tracker.id} value={tracker.id}>{tracker.name}</option>)}</select></label>
        <button type="submit" disabled={!selected?.id || !available.length}>Associer le tracker</button>
      </fieldset>
    </form>}
  </section>;
}
