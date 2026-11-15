import { useEffect, useState, type FormEvent } from "react";
import { attachProjectTracker, localSetupStatus, type LocalSetup } from "../lib/local";

export function ProjectTrackerSetup({
  projectId,
  setup: suppliedSetup,
  onChanged,
  onManageTrackers,
}: {
  projectId?: string;
  setup?: LocalSetup;
  onChanged: () => void;
  onManageTrackers?: () => void;
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
      .catch(() => { if (current) setError("Unable to load ticket sources. Check your connection and try again."); });
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
      setNotice("Tracker attached to project.");
      if (!suppliedSetup) setReload(value => value + 1);
      onChanged();
    } catch { setError("Unable to attach this source to the project. Check your connection and try again."); }
    finally { setPending(false); }
  }
  return <section className="project-tracker-setup" aria-label="Project ticket source">
    <div className="project-tracker-heading">
      <h2>Ticket source</h2>
      {onManageTrackers && <button type="button" className="project-tracker-manage" onClick={onManageTrackers}>Manage sources</button>}
    </div>
    {setup && projectId && <p className="muted project-tracker-summary">{linked.length ? `Attached trackers: ${linked.map(tracker => tracker.name).join(", ")}.` : remoteTrackers.length ? "No tracker attached to this project." : "No remote source configured. Prompts remain available without a tracker."}</p>}
    {!setup && !error && <p>Loading trackers…</p>}
    {error && <p role="alert" className="error">{error} {!suppliedSetup && <button type="button" onClick={() => {setError("");setReload(value => value + 1);}}>Retry</button>}</p>}
    {notice && <p role="status" className="notice">{notice}</p>}
    {setup && !projectId && !remoteTrackers.length && <p className="muted">No remote source available for this project.</p>}
    {setup && !!remoteTrackers.length && (!selected?.id || !!available.length) && <form onSubmit={submit}>
      <fieldset disabled={pending}>
        {!projectId && <label>Existing project<select required value={selectedProjectId} onChange={event => {setSelectedProjectId(event.target.value);setTrackerId("");}}><option value="">Choose a project</option>{setup.projects?.map(project => <option key={project.id} value={project.id}>{project.name}</option>)}</select></label>}
        <label>Tracker to add<select required value={trackerId} onChange={event => setTrackerId(event.target.value)}><option value="">Choose a tracker</option>{available.map(tracker => <option key={tracker.id} value={tracker.id}>{tracker.name}</option>)}</select></label>
        <button type="submit" disabled={!selected?.id || !available.length}>Attach tracker</button>
      </fieldset>
    </form>}
  </section>;
}
