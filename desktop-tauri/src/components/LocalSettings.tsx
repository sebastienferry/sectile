import { useEffect, useRef, useState, type FormEvent } from "react";
import { ProviderSettings } from "./ProviderSettings";
import { HarnessSettings } from "./HarnessSettings";
import { OAuthConnections } from "./OAuthConnections";
import { Icon } from "./Icon";
import { ProjectTrackerSetup } from "./ProjectTrackerSetup";
import * as api from "../lib/local";
import { trackerApiSite } from "../tracker-site";
import { useAppearance, useFontSize, type Appearance, type FontSize } from "../lib/appearance";

type SettingsSection = "engines" | "harness" | "profile" | "connections" | "projects" | "services" | "appearance";
const sections = [
 {id:"engines", label:"AI engines", icon:"agent", description:"Connect Claude Code or Codex to work in your projects."},
 {id:"harness", label:"Harness", icon:"shield", description:"Workstation permission mode, sandbox network, and shared writable directories across all AI engines."},
 {id:"profile", label:"Profile", icon:"user", description:"Your identity for actions performed from this workstation."},
 {id:"connections", label:"Accounts and trackers", icon:"link", description:"Connect your accounts and choose your ticket sources."},
 {id:"projects", label:"Projects", icon:"folder", description:"Start from a local folder or link a project to a tracker."},
 {id:"appearance", label:"Appearance", icon:"sun", description:"Color theme and font size for the application."},
 {id:"services", label:"Local services", icon:"monitor", description:"Installation status and diagnostic tools."},
] as const;
export function LocalSettings({ onChanged, onClose, onOpenFolder, onConfigureProject, initialSection, hasActiveExecutions }: { onChanged: () => void; onClose?: () => void; onOpenFolder?: () => void; onConfigureProject?: (id:string) => void; initialSection?: SettingsSection; hasActiveExecutions?: boolean }) {
 const [section, setSection] = useState<SettingsSection>(initialSection??"engines");
 const [appearance, setAppearance] = useAppearance();
 const [fontSize, setFontSize] = useFontSize();
 const navigated = useRef(!!initialSection);
 const trackerManagement = useRef<HTMLElement>(null);
 const trackerList = useRef<HTMLUListElement>(null);
  const [setup, setSetup] = useState<api.LocalSetup | null>(null);
  const [pending, setPending] = useState(false);
  const [pendingMessage, setPendingMessage] = useState("Saving…");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [fleetUrl, setFleetUrl] = useState("");
  const [fleetCode, setFleetCode] = useState("");
  const [credentialProvider, setCredentialProvider] = useState("github");
  const [credentialSite, setCredentialSite] = useState("");
  const [trackerEmail, setTrackerEmail] = useState("");
  const [token, setToken] = useState("");
  const [connectionsTab, setConnectionsTab] = useState<"trackers" | "pat">("trackers");
  const [sourceProvider, setSourceProvider] = useState("github");
  const [sourceSite, setSourceSite] = useState("https://api.github.com");
  const [trackerName, setTrackerName] = useState("");
  const [scope, setScope] = useState("");
  const [editingTracker, setEditingTracker] = useState<{ id: string; name: string; provider: string; site: string; scope: string } | null>(null);
  const [editingIssueTypes, setEditingIssueTypes] = useState<string[]>([]);
  const [availableIssueTypes, setAvailableIssueTypes] = useState<string[]>([]);
  const [loadingIssueTypes, setLoadingIssueTypes] = useState(false);
  const [deletingTrackerId, setDeletingTrackerId] = useState<string | null>(null);
  const [deletingProjectId, setDeletingProjectId] = useState<string | null>(null);
  const [projectName, setProjectName] = useState("");
  const [trackerId, setTrackerId] = useState("");
  const [projectId, setProjectId] = useState("");
  const [trackerSyncs, setTrackerSyncs] = useState<Record<string,{id:string;active:boolean}>>({});
  const busy = useRef(false);
  const mounted = useRef(true);
  async function refresh() {
    const next = await api.localSetupStatus();
    if (mounted.current) setSetup(next);
    return next;
  }
  useEffect(() => {
    mounted.current = true;
    void refresh().then(next => {
      if (!mounted.current) return;
      if (!navigated.current) setSection(!next.enabled || !next.ready ? "services" : next.mode === "fleet" && next.apiSetupAvailable === false ? "engines" : !next.signedIn ? "profile" : "engines");
      setEmail(next.profile?.email ?? "");
      setDisplayName(next.profile?.displayName ?? "");
    }).catch(cause => { if (mounted.current) { setError(String(cause)); setSection("services"); } });
    return () => { mounted.current = false; };
  }, []);
  useEffect(() => {
    if (initialSection === "connections" && section === "connections" && setup?.ready) {
      (setup.trackers?.length ? trackerList.current : trackerManagement.current)?.scrollIntoView?.({block:"start"});
    }
  }, [initialSection, section, setup?.ready, setup?.trackers?.length]);
  useEffect(() => {
    if (!editingTracker || editingTracker.provider !== "jira" || typeof api.trackerIssueTypes !== "function") {
      setEditingIssueTypes([]);
      setAvailableIssueTypes([]);
      setLoadingIssueTypes(false);
      return;
    }
    let cancelled = false;
    setLoadingIssueTypes(true);
    api.trackerIssueTypes(editingTracker.id)
      .then(result => {
        if (cancelled) return;
        setEditingIssueTypes(Array.isArray(result?.selected) ? result.selected : []);
        setAvailableIssueTypes(Array.isArray(result?.available) ? result.available : []);
      })
      .catch(() => {
        if (cancelled) return;
        setEditingIssueTypes([]);
        setAvailableIssueTypes([]);
      })
      .finally(() => {
        if (!cancelled) setLoadingIssueTypes(false);
      });
    return () => { cancelled = true; };
  }, [editingTracker?.id, editingTracker?.provider]);
  async function perform(
    action: () => Promise<unknown>,
    message: string,
    isCancelled?: (result: unknown) => boolean,
    hydrateProfile = false,
  ) {
    if (busy.current) return;
    busy.current = true; setPendingMessage("Saving…"); setPending(true); setError(""); setNotice("");
    try {
      const result = await action();
      if (isCancelled?.(result)) return;
      if (mounted.current) setNotice(message);
      const next = await refresh();
      if (hydrateProfile) {
        setEmail(next.profile?.email ?? "");
        setDisplayName(next.profile?.displayName ?? "");
      }
      setSection(previous => previous === "services" && next.ready && !next.signedIn ? "profile" : previous === "profile" && !setup?.signedIn && next.signedIn ? "engines" : previous);
      onChanged();
    } catch (cause) {
      if (mounted.current) setError(String(cause));
      // A mutation may have succeeded before a later step failed. Refresh the
      // saved objects so the user can recover without creating duplicates.
      try { await refresh(); onChanged(); } catch { /* Keep the original error. */ }
    } finally { busy.current = false; if (mounted.current) setPending(false); }
  }
  async function prepareUpdate() {
    if (busy.current) return;
    busy.current = true; setPending(true); setError(""); setNotice("");
    setPendingMessage("Stopping companion and saving data before closing…");
    try {
      // Native code fences automatic startup and exits after store release.
      // Do not refresh status here: that could undo an acknowledged shutdown.
      await api.prepareLocalUpdate();
      if (mounted.current) setNotice("Services stopped. Closing desktop for update…");
    } catch (cause) {
      busy.current = false;
      if (mounted.current) { setError(String(cause)); setPending(false); }
    }
  }
  function submit(event: FormEvent, action: () => Promise<unknown>, message: string, hydrateProfile = false) {
    event.preventDefault(); void perform(action, message, undefined, hydrateProfile);
  }
  async function refreshStatus() {
    if (busy.current) return;
    busy.current = true; setPendingMessage("Refreshing…"); setPending(true); setError(""); setNotice("");
    try {
      const next = await refresh();
      if (mounted.current) {
        if (!setup?.ready && next.ready) {
          setEmail(next.profile?.email ?? "");
          setDisplayName(next.profile?.displayName ?? "");
        }
        setNotice("Status refreshed.");
      }
      onChanged();
    }
    catch (cause) { if (mounted.current) setError(String(cause)); }
    finally { busy.current = false; if (mounted.current) setPending(false); }
  }
  const ready = setup?.ready && setup.signedIn && setup.apiSetupAvailable !== false;
  useEffect(() => {
    if (!ready || section !== "connections") return;
    let current = true;
    let timer: ReturnType<typeof setTimeout> | undefined;
    async function read() {
      try {
        const active = await api.activeTrackerSyncs();
        if (current) setTrackerSyncs(previous => {
          const next = { ...previous };
          let changed = false;
          for (const activity of active) {
            if (activity.trackerId && !next[activity.trackerId]?.active) {
              next[activity.trackerId] = { id: activity.id, active: true };
              changed = true;
            }
          }
          return changed ? next : previous;
        });
      } catch { /* Older Fleet backends can still use explicit sync. */ }
      if (current) timer = setTimeout(() => void read(), 3000);
    }
    void read();
    return () => { current = false; if (timer) clearTimeout(timer); };
  }, [ready, section]);
  const fleetMode = setup?.mode === "fleet";
  const visibleSections = sections.filter(item => !(item.id === "profile" && setup?.mode === "local" && setup.signedIn));
  const sectionLabel = (item: typeof sections[number]) => item.id === "services" && fleetMode ? "Connection and services" : item.label;
  const activeSection = sections.find(item => item.id === section);
  const personalAccess = <>
    <form className="personal-access-form" onSubmit={e => submit(e, async () => {
      try { await api.saveLocalCredential({ tracker: credentialProvider, siteUrl: trackerApiSite(credentialProvider, credentialSite), email: trackerEmail, token }); }
      finally { setToken(""); }
    }, "Personal access saved. Run a sync to verify ticket access.")}>
      <fieldset disabled={pending}>
        <label>Provider<select value={credentialProvider} onChange={e => { setCredentialProvider(e.target.value); setCredentialSite(e.target.value === "gitlab" ? "https://gitlab.com" : ""); setToken(""); }}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="jira">Jira</option></select></label>
        {credentialProvider !== "github" && <label>Site or API URL<input type="url" required placeholder={credentialProvider === "jira" ? "https://your-site.atlassian.net" : "https://gitlab.com"} value={credentialSite} onChange={e => setCredentialSite(e.target.value)} /></label>}
        {credentialProvider === "jira" && <label>Account email<input type="email" required value={trackerEmail} onChange={e => setTrackerEmail(e.target.value)} /></label>}
        <label>Personal access token<input type="password" required autoComplete="off" value={token} onChange={e => setToken(e.target.value)} /></label>
        <p className="muted">The token is encrypted by {fleetMode ? "the Fleet backend" : "the local server"}. Saving replaces the personal access token for this provider.</p>
        <button type="submit">Save access</button>
      </fieldset>
    </form>
    <ul className="setup-list">{setup?.credentials?.credentials.map(c => <li key={c.tracker}><span>{c.tracker} <small>{c.kind === "oauth" ? "OAuth" : "Token"} · {c.disconnected ? "Connection interrupted" : c.account || "Access saved · account unconfirmed"}{c.siteUrl && ` · ${c.siteUrl}`}</small></span></li>)}</ul>
  </>;
  return <section className={`settings local-settings settings-${section}`}>
    <header className="settings-heading">
      <div className="settings-heading-top">
        <span className="eyebrow">{fleetMode ? "YOUR FLEET WORKSPACE" : setup?.mode === "local" ? "YOUR LOCAL WORKSPACE" : "CONNECT TO SECTILE"}</span>
        {onClose && (
          <button type="button" className="settings-close-btn" onClick={onClose}>
            <Icon name="close" size={14} />
            <span>Exit settings</span>
          </button>
        )}
      </div>
      <h1>{section === "engines" ? "AI engines" : fleetMode ? "Fleet configuration" : "Workstation configuration"}</h1>
      <p>{section === "engines" ? "Connect Claude Code and Codex to work in your projects." : fleetMode ? "Manage your profile and accounts on the connected backend. Execution services remain on this workstation." : setup?.mode === "local" ? "Connect your tools and configure your projects on this workstation." : "Choose Local mode or pair this workstation with a Fleet backend."}</p>
    </header>
    {error && <p role="alert" className="error">{error}</p>}
    {notice && <p role="status" className="notice">{notice}</p>}
    {pending && <p role="status">{pendingMessage}</p>}
    {!setup && !error && <p>Reading configuration…</p>}
    <div className="settings-layout">
    <nav className="settings-navigation" aria-label="Settings sections">
     {visibleSections.map(item => <button key={item.id} aria-current={section === item.id ? "page" : undefined} disabled={(item.id === "profile" && (!setup?.ready || setup.mode === "fleet" && setup.apiSetupAvailable === false)) || (["connections","projects"].includes(item.id) && !ready)} onClick={() => {navigated.current=true;setSection(item.id);}}><Icon name={item.icon} />{sectionLabel(item)}{section === item.id && <Icon name="chevron" />}</button>)}
     <p>{setup?.mode === "fleet" ? "Accounts and projects are stored on the Fleet backend." : "Configuration stored on this workstation."}</p>
    </nav>
    <div className="settings-content">
    {section !== "engines" && <header className="settings-section-heading"><h2>{activeSection ? sectionLabel(activeSection) : ""}</h2><p>{section === "services" && fleetMode ? "Connection status and Fleet agent attached to this workstation." : activeSection?.description}</p></header>}
    <div hidden={section !== "engines"}>{setup?.agentReady ? <ProviderSettings key={`${setup.mode}:${setup.fleetServer??""}`} projects={setup.projects??[]} showProfiles /> : <article><p>Connect a Local or Fleet agent to check your AI engines.</p><button onClick={()=>setSection("services")}>Configure connection</button></article>}</div>
    <div hidden={section !== "harness"}>{setup?.agentReady ? <HarnessSettings key={`${setup.mode}:${setup.fleetServer??""}`} /> : <article><p>Connect a Local or Fleet agent to configure the global harness.</p><button onClick={()=>setSection("services")}>Configure connection</button></article>}</div>
    <div hidden={section !== "appearance"}>
      <article>
        <h2>Theme and typography</h2>
        <fieldset>
          <label>Color theme
            <select aria-label="Color theme" value={appearance} onChange={e => setAppearance(e.target.value as Appearance)}>
              <option value="system">Automatic (System)</option>
              <option value="dark">Dark</option>
              <option value="light">Light</option>
            </select>
          </label>
          <label>Font size
            <select aria-label="Font size" value={fontSize} onChange={e => setFontSize(e.target.value as FontSize)}>
              <option value="compact">Compact (13 px)</option>
              <option value="default">Normal (14 px)</option>
              <option value="large">Large (15 px)</option>
              <option value="xlarge">Extra large (16 px)</option>
            </select>
          </label>
          <p className="muted">The interface uses a unified three-tier scale (secondary, body, and heading) that adapts immediately to your setting.</p>
        </fieldset>
      </article>
    </div>
    <div hidden={section !== "services"}><article>
      <h2>{fleetMode ? "Fleet Agent" : "Local services"}</h2>
      <div className="mode-badge">
        <Icon name={setup?.mode === "fleet" ? "link" : "monitor"} />
        <span>Active mode: {setup?.mode === "fleet" ? "Fleet Agent" : setup?.mode === "local" ? "Local Companion" : "Unconfigured"}</span>
      </div>
      <p>{setup?.mode === "fleet" ? `Fleet mode · ${setup.agentReady ? "agent connected" : "agent disconnected"}${setup.fleetServer ? ` · ${setup.fleetServer}` : ""}` : !setup ? "Reading workstation configuration…" : !setup.enabled ? "Choose a mode to connect this desktop to Sectile." : setup.ready ? `Local mode · companion ready · ${setup.agentReady ? "execution ready" : "waiting for profile"}` : "The local companion is not responding."}</p>
      {setup?.backendError && <p role="alert" className="error">{setup.backendError}</p>}
      {setup?.mode === "fleet" && setup.apiSetupAvailable === false && <p className="muted">This agent was attached without a workstation API credential. Configure accounts and projects on the Fleet backend.</p>}
      {setup?.dataDirectory && <p className="muted runtime-path">{setup.dataDirectory}</p>}
      <p className="muted">{setup?.mode === "fleet" ? setup.apiSetupAvailable === false ? "This Fleet agent is managed by its original installation." : "The Fleet agent starts automatically on this workstation and continues in the background when you close the desktop." : "The companion starts automatically and stays running in the background while tasks or open conversations (even idle ones) remain active in the sidebar."}</p>
      {setup && setup.mode !== "fleet" && <button disabled={pending} onClick={() => void perform(api.enableLocalRuntime, "Local companion started and Local mode selected.")}>{setup.enabled ? "Check / start local companion" : "Start in Local mode"}</button>}
      {setup?.mode === "local" && (setup.agentReady || setup.updatePreparing) && <details className="update-preparation"><summary>Update application</summary>
        <p>Stop active runs and open conversations in the sidebar, and close other Tauri instances. This stops the companion and closes this desktop before replacing the application. Your data is preserved. No download is started.</p>
        <button disabled={pending} onClick={() => void prepareUpdate()}>Stop and quit to update</button>
      </details>}
      {setup?.mode === "local" && (
        <details className="mode-switch-card">
          <summary>Switch to Fleet mode (Team Agent)</summary>
          <p className="muted">Pair this workstation with a one-time code from your profile on the Fleet backend. The long-lived token remains in private native storage and is never displayed.</p>
          <form onSubmit={e => submit(e, async () => {
            if (hasActiveExecutions && typeof window !== "undefined" && !window.confirm("Executions are currently running on this workstation. Do you really want to switch modes?")) return;
            try { await api.pairFleetAgent(fleetUrl, fleetCode); } finally { setFleetCode(""); }
          }, "Workstation paired with Fleet backend.", true)}>
            <fieldset disabled={pending}>
              <label>Fleet backend URL<input type="url" required placeholder="https://sectile.example.com" value={fleetUrl} onChange={e => setFleetUrl(e.target.value)} autoComplete="url" /></label>
              <label>Pairing code<input type="password" required autoComplete="off" value={fleetCode} onChange={e => setFleetCode(e.target.value)} /></label>
              <button className="primary" type="submit">Pair this workstation with Fleet</button>
            </fieldset>
          </form>
          <details><summary>I already paired an agent on this workstation</summary><p className="muted">Tauri can explicitly attach to the existing agent without modifying its shared installation.</p><button disabled={pending} onClick={() => {
            if (hasActiveExecutions && typeof window !== "undefined" && !window.confirm("Executions are currently running on this workstation. Do you really want to switch modes?")) return;
            void perform(api.attachFleetAgent, "Fleet agent attached to this desktop.", undefined, true);
          }}>Attach existing Fleet agent</button></details>
        </details>
      )}
      {setup?.mode === "fleet" && (
        <details className="mode-switch-card">
          <summary>Switch to Local Companion mode</summary>
          <p className="muted">Resume 100% standalone task execution and ticket management on this workstation with your local SQLite database.</p>
          <button disabled={pending} onClick={() => {
            if (hasActiveExecutions && typeof window !== "undefined" && !window.confirm("Executions are currently running on this workstation. Do you really want to switch modes?")) return;
            void perform(api.enableLocalRuntime, "Local companion started and Local mode selected.", undefined, true);
          }}>Enable Local Companion mode</button>
        </details>
      )}
      {setup && (!setup.mode || setup.mode === "unconfigured") && <article><h3>Fleet mode</h3>
        <p className="muted">Pair this workstation with a one-time code from your profile on the Fleet backend. The long-lived token remains in private native storage and is never displayed.</p>
        <form onSubmit={e => submit(e, async () => { try { await api.pairFleetAgent(fleetUrl, fleetCode); } finally { setFleetCode(""); } }, "Workstation paired with Fleet backend.", true)}>
          <fieldset disabled={pending}>
            <label>Fleet backend URL<input type="url" required placeholder="https://sectile.example.com" value={fleetUrl} onChange={e => setFleetUrl(e.target.value)} autoComplete="url" /></label>
            <label>Pairing code<input type="password" required autoComplete="off" value={fleetCode} onChange={e => setFleetCode(e.target.value)} /></label>
            <button className="primary" type="submit">Pair this workstation with Fleet</button>
          </fieldset>
        </form>
        <details><summary>I already paired an agent on this workstation</summary><p className="muted">Tauri can explicitly attach to the existing agent without modifying its shared installation.</p><button disabled={pending} onClick={() => void perform(api.attachFleetAgent, "Fleet agent attached to this desktop.", undefined, true)}>Attach existing Fleet agent</button></details>
      </article>}
      {setup?.mode === "fleet" && setup.apiSetupAvailable !== false && <p className="muted">This page configures your profile and projects on the selected Fleet backend. The execution agent remains on this workstation.</p>}
      <button disabled={pending || (!setup && !error)} onClick={() => void refreshStatus()}>Refresh</button>
    </article>
    {setup?.mode !== "fleet" && <ServiceLogPanel active={section === "services"} service="companion" />}
    {setup?.mode !== "local" && <ServiceLogPanel active={section === "services"} service="agent" />}
    </div>
    <div hidden={section !== "profile"}>{setup?.ready && setup.apiSetupAvailable !== false && (setup.mode !== "fleet" || setup.signedIn) && <article>
      <h2>{fleetMode ? "Fleet profile" : "Local profile"}</h2>
      <form onSubmit={e => submit(e, () => api.saveLocalProfile(email, displayName), "Profile saved and agent attached.")}>
        <fieldset disabled={pending}>
          <label>Email address<input type="email" required value={email} readOnly={setup.signedIn} onChange={e => setEmail(e.target.value)} autoComplete="email" /></label>
          <label>Display name<input required value={displayName} onChange={e => setDisplayName(e.target.value)} autoComplete="name" /></label>
          <p className="muted">{fleetMode ? "This profile identifies your actions with your account on the Fleet backend." : "This profile identifies your actions on this workstation. It does not create an account with any provider."}</p>
          <button className="primary" type="submit">Save profile</button>
        </fieldset>
      </form>
    </article>}</div>
    {ready && <>
      <div hidden={section !== "connections"}>
        <div className="settings-subtabs" role="tablist" aria-label="Trackers and access subsections">
          <button
            id="tab-trackers"
            type="button"
            role="tab"
            aria-selected={connectionsTab === "trackers"}
            aria-controls="tabpanel-trackers"
            className={`settings-subtab ${connectionsTab === "trackers" ? "active" : ""}`}
            onClick={() => setConnectionsTab("trackers")}
          >
            Trackers
          </button>
          <button
            id="tab-pat"
            type="button"
            role="tab"
            aria-selected={connectionsTab === "pat"}
            aria-controls="tabpanel-pat"
            className={`settings-subtab ${connectionsTab === "pat" ? "active" : ""}`}
            onClick={() => setConnectionsTab("pat")}
          >
            PAT access
          </button>
        </div>
        <div role="tabpanel" id="tabpanel-trackers" aria-labelledby="tab-trackers" className={`settings-tabpanel ${connectionsTab === "trackers" ? "active" : "is-hidden"}`}>
          <article ref={trackerManagement}>
            <h2>Configured trackers</h2>
            {(!setup.trackers || setup.trackers.length === 0) ? (
              <p className="muted">No trackers configured yet.</p>
            ) : (
              <ul ref={trackerList} className="setup-list tracker-list">{setup.trackers?.map(t => {
                const linkedProjects = (setup.projects ?? []).filter(p =>
                  p.defaultTrackerId === t.id || p.trackers?.some(link => link.trackerId === t.id),
                );
                return <li key={t.id}>
                <div className="tracker-list-row"><span><strong>{t.name}</strong><small>{t.provider} · {t.scope}{t.site ? ` · ${t.site}` : ""} · {linkedProjects.length > 0 ? `Used by: ${linkedProjects.map(p => p.name).join(", ")}` : "No linked project"}{t.ticketCount ? ` · ${t.ticketCount} cached ticket(s)` : ""}</small>{trackerSyncs[t.id]&&<TrackerSyncResult activityId={trackerSyncs[t.id].id} onFinished={()=>{setTrackerSyncs(previous=>({...previous,[t.id]:{...previous[t.id],active:false}}));void refresh();onChanged();}}/>}</span>
                  <div className="tracker-list-actions"><button disabled={pending||trackerSyncs[t.id]?.active} onClick={() => void perform(async()=>{const active=await api.activeTrackerSyncs().catch(()=>[]);const existing=active.find(activity=>activity.trackerId===t.id);const result=existing?{activity:existing}:await api.syncLocalTracker(t.id);if(!result.activity?.id)throw new Error("The backend did not provide tracking for this sync.");setTrackerSyncs(previous=>({...previous,[t.id]:{id:result.activity.id,active:true}}));}, "Sync tracked. Its result will appear below the tracker.")}>Sync</button>
                    {(setup.mode !== "fleet" || setup.profile?.role === "admin") && <><button type="button" disabled={pending} onClick={() => {setDeletingTrackerId(null);setEditingTracker(editingTracker?.id === t.id ? null : {...t,site:t.site??""});}}>Edit</button><button type="button" disabled={pending} onClick={() => {setEditingTracker(null);setDeletingTrackerId(deletingTrackerId === t.id ? null : t.id);}}>Delete</button></>}
                  </div></div>
                {editingTracker?.id === t.id && <form className="tracker-editor" onSubmit={e => submit(e, async () => {const payload = editingTracker.provider === "jira" && availableIssueTypes.length > 0 ? {name:editingTracker.name,provider:editingTracker.provider,site:trackerApiSite(editingTracker.provider,editingTracker.site),scope:editingTracker.scope,issueTypes:editingIssueTypes} : {name:editingTracker.name,provider:editingTracker.provider,site:trackerApiSite(editingTracker.provider,editingTracker.site),scope:editingTracker.scope};await api.updateLocalTracker(t.id,payload);setEditingTracker(null);}, "Tracker updated.")}>
                  <fieldset disabled={pending}><label>Tracker name to edit<input required value={editingTracker.name} onChange={e=>setEditingTracker({...editingTracker,name:e.target.value})}/></label>
                    <label>Tracker provider to edit<select value={editingTracker.provider} onChange={e=>setEditingTracker({...editingTracker,provider:e.target.value,site:e.target.value==="github"?"https://api.github.com":""})}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="jira">Jira</option></select></label>
                    <label>Tracker URL to edit<input type="url" required={editingTracker.provider==="jira"} value={editingTracker.site} onChange={e=>setEditingTracker({...editingTracker,site:e.target.value})}/></label>
                    <label>Repository or key to edit<input required value={editingTracker.scope} onChange={e=>setEditingTracker({...editingTracker,scope:e.target.value})}/></label>
                    {editingTracker.provider === "jira" && (
                      <div className="tracker-issue-types-field">
                        <span>Imported ticket types</span>
                        {loadingIssueTypes ? (
                          <p className="muted">Reading project types…</p>
                        ) : availableIssueTypes.length === 0 ? (
                          <p className="muted">Types unavailable: the tracker did not respond.</p>
                        ) : (
                          <div className="issue-type-pills" role="group" aria-label="Imported ticket types">
                            {availableIssueTypes.map(type => {
                              const isDefault = type === "Task" || type === "Story";
                              const isActive = editingIssueTypes.length === 0 ? isDefault : editingIssueTypes.includes(type);
                              return (
                                <button
                                  key={type}
                                  type="button"
                                  aria-pressed={isActive}
                                  className={`issue-type-pill ${isActive ? "active" : ""}`}
                                  onClick={() => {
                                    const selected = editingIssueTypes.length === 0
                                      ? availableIssueTypes.filter(item => item === "Task" || item === "Story")
                                      : editingIssueTypes;
                                    setEditingIssueTypes(
                                      selected.includes(type)
                                        ? selected.filter(item => item !== type)
                                        : [...selected, type],
                                    );
                                  }}
                                >
                                  {type}
                                </button>
                              );
                            })}
                          </div>
                        )}
                        <small className="muted">Nothing selected defaults to Task and Story. Check Bug (or your custom types) then run a sync to import them.</small>
                      </div>
                    )}
                    <p className="muted">If the tracker already contains tickets, only its name (and its imported types) can change.</p>
                    <div className="tracker-list-actions"><button className="primary" type="submit">Save changes</button><button type="button" onClick={()=>setEditingTracker(null)}>Cancel</button></div>
                  </fieldset>
                </form>}
                {deletingTrackerId === t.id && (
                  <div className="tracker-delete-confirm">
                    {linkedProjects.length > 0 ? (
                      <p>
                        Cannot delete “{t.name}” while it is linked to project “{linkedProjects.map(p => p.name).join("”, “")}”.
                        Unlink it or delete that project first in the Projects section.
                      </p>
                    ) : (
                      <p>
                        Delete “{t.name}”? No project uses this tracker
                        {t.ticketCount ? ` (${t.ticketCount} locally cached ticket(s) will also be removed)` : ""}.
                      </p>
                    )}
                    <div className="tracker-list-actions">
                      {linkedProjects.length > 0 && (
                        <button type="button" onClick={() => { setDeletingTrackerId(null); setSection("projects"); }}>
                          View linked projects
                        </button>
                      )}
                      <button type="button" disabled={pending} onClick={()=>void perform(async()=>{await api.deleteLocalTracker(t.id);setDeletingTrackerId(null);},"Tracker deleted.")}>Confirm deletion</button>
                      <button type="button" disabled={pending} onClick={()=>setDeletingTrackerId(null)}>Cancel</button>
                    </div>
                  </div>
                )}
              </li>;})}</ul>
            )}
          </article>
          <article>
            <h2>Add tracker</h2>
            {setup.mode !== "fleet" || setup.profile?.role === "admin" ? <form onSubmit={e => submit(e, async () => {
              await api.createLocalTracker({ name: trackerName, provider: sourceProvider, site: trackerApiSite(sourceProvider, sourceSite), scope });
              setTrackerName("");
              setScope("");
              if (sourceProvider === "github") setSourceSite("https://api.github.com");
            }, "Tracker added.")}>
              <fieldset disabled={pending}>
                <label>Source provider<select value={sourceProvider} onChange={e => {
                  const next = e.target.value;
                  setSourceProvider(next);
                  setSourceSite(next === "github" ? "https://api.github.com" : next === "gitlab" ? "https://gitlab.com" : "");
                }}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="jira">Jira</option></select></label>
                <label>Source URL<input type="url" required={sourceProvider==="jira"} placeholder={sourceProvider==="jira"?"https://your-site.atlassian.net":"https://api.github.com"} value={sourceSite} onChange={e=>setSourceSite(e.target.value)} /></label>
                <label>Tracker name<input required value={trackerName} onChange={e => setTrackerName(e.target.value)} /></label>
                <label>Repository or project key<input required placeholder={sourceProvider === "jira" ? "PROJECT" : "organization/repo"} value={scope} onChange={e => setScope(e.target.value)} /></label>
                <button type="submit">Add tracker</button>
              </fieldset>
            </form> : <p className="muted">Trackers are shared in Fleet. Ask a server administrator to register a new source.</p>}
          </article>
        </div>
        <div role="tabpanel" id="tabpanel-pat" aria-labelledby="tab-pat" className={`settings-tabpanel ${connectionsTab === "pat" ? "active" : "is-hidden"}`}>
          {fleetMode ? <>
            <OAuthConnections serverManaged canManageTrackers={setup.profile?.role === "admin"} credentials={setup.credentials?.credentials} onConnected={() => { void refresh(); onChanged(); }} />
            <article><details><summary>Manual token access · advanced option</summary>{personalAccess}</details></article>
          </> : <>
            <article><h2>Personal tracker access</h2><p className="muted">For a new Local installation, save a personal access token from the provider. Your tickets remain managed by the companion on this workstation.</p>{personalAccess}</article>
            <details><summary>Existing local OAuth connection</summary><OAuthConnections credentials={setup.credentials?.credentials} onConnected={() => { void refresh(); onChanged(); }} /></details>
          </>}
        </div>
      </div>
      <div hidden={section !== "projects"}>
      {onConfigureProject && !!setup.projects?.length && <article><h2>Your projects</h2><div className="profile-list">{setup.projects.map(project=><div className="profile-card" key={project.id}><strong>{project.name}</strong><button onClick={()=>onConfigureProject(project.id)}>Configure {project.name}</button>{deletingProjectId === project.id ? <><button type="button" disabled={pending} onClick={()=>void perform(async()=>{await api.deleteLocalProject(project.id);setDeletingProjectId(null);},"Project deleted.")}>Confirm deletion</button><button type="button" disabled={pending} onClick={()=>setDeletingProjectId(null)}>Cancel</button></> : <button type="button" disabled={pending} onClick={()=>setDeletingProjectId(project.id)}>Delete {project.name}</button>}</div>)}</div></article>}
      {onOpenFolder && <article className="folder-setup"><div className="settings-illustration"><Icon name="folder" /></div><h2>Start with a local folder</h2><p>Open a folder and launch your first prompt. You can connect a tracker later.</p><button className="primary" onClick={onOpenFolder}><Icon name="plus" />Choose a local folder</button></article>}
      {!!setup.projects?.length && <article><ProjectTrackerSetup setup={setup} onChanged={() => {void refresh();onChanged();}} /></article>}
      <article>
        <h2>Create a project with a tracker</h2>
        <form onSubmit={e => submit(e, async () => { await api.createLocalProject(projectName, trackerId); setProjectName(""); }, "Project created. Now map its local repository.")}>
          <fieldset disabled={pending}>
            <label>Project name<input required value={projectName} onChange={e => setProjectName(e.target.value)} /></label>
            <label>Tracker<select required value={trackerId} onChange={e => setTrackerId(e.target.value)}><option value="">Choose a tracker</option>{setup.trackers?.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}</select></label>
            <button type="submit">Create project</button>
          </fieldset>
        </form>
        <div className="existing-project-folder">
          <h3>Map a folder to an existing project</h3>
          <fieldset disabled={pending || !setup.agentReady}>
            <label>Project to map<select required value={projectId} onChange={e => setProjectId(e.target.value)}><option value="">Choose a project</option>{setup.projects?.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
            <button type="button" disabled={!projectId} onClick={() => void perform(() => api.chooseProjectFolder(projectId), "Local folder mapped to project.", result => result === null)}>Choose folder</button>
          </fieldset>
        </div>
      </article></div>
    </>}
    </div></div>
  </section>;
}

function ServiceLogPanel({active,service}:{active:boolean;service:"companion"|"agent"}) {
 const fleet=service==="agent";
 const [snapshot,setSnapshot]=useState<api.ServiceLog>();
 const [error,setError]=useState("");
 const [loading,setLoading]=useState(false);
 const [refresh,setRefresh]=useState(0);
 useEffect(()=>{
  if(!active)return;
  let current=true;
  setSnapshot(undefined);setLoading(true);setError("");
  void Promise.resolve(api.localServiceLog(service)).then(value=>{if(current && value)setSnapshot(value);}).catch(cause=>{if(current)setError(String(cause));}).finally(()=>{if(current)setLoading(false);});
  return()=>{current=false;};
 },[active,service,refresh]);
 return <article className="service-log">
  <div className="service-log-heading"><h2>{fleet?"Fleet agent log":"Local companion log"}</h2><button type="button" disabled={loading} onClick={()=>setRefresh(value=>value+1)}>Refresh log</button></div>
  <p className="muted">{fleet?"An attached Fleet agent may write to the log of its original installation.":"Diagnostics for the companion started by this desktop."}</p>
  {snapshot && <p className="service-log-path" title={snapshot.path}><code>{snapshot.path}</code></p>}
  {loading && <p role="status">Reading log…</p>}
  {error && <p role="alert" className="error">Unable to read log: {error}</p>}
  {!loading && !error && snapshot && <>
   <p role="status">{snapshot.missing?"No log available for this service.":!snapshot.text?"The log is empty.":snapshot.truncated?"Showing the last 256 KiB of the log.":"Current service log."}</p>
   {!snapshot.missing && !!snapshot.text && <pre className="service-log-output" tabIndex={0} aria-label="Service log content">{snapshot.text}</pre>}
  </>}
 </article>;
}

function TrackerSyncResult({activityId,onFinished}:{activityId:string;onFinished:()=>void}) {
 const [activity,setActivity]=useState<api.TrackerSyncActivity>();
 const [readError,setReadError]=useState(false);
 const [retry,setRetry]=useState(0);
 const finished=useRef(onFinished);
 finished.current=onFinished;
 useEffect(()=>{
  let current=true;
  let timer:ReturnType<typeof setTimeout>|undefined;
  async function read(){
   try{
    const next=await api.trackerSyncStatus(activityId);
    if(!current)return;
    setActivity(next);setReadError(false);
    if(["completed","failed","canceled"].includes(next.status)){finished.current();return;}
   }catch{if(current)setReadError(true);return;}
   timer=setTimeout(()=>void read(),2000);
  }
  void read();
  return()=>{current=false;if(timer)clearTimeout(timer);};
 },[activityId,retry]);
 if(readError)return <small className="sync-error" role="alert">Sync tracking unavailable. <button type="button" onClick={()=>setRetry(value=>value+1)}>Retry tracking</button></small>;
 if(activity?.status==="failed")return <small className="sync-error" role="alert">Sync failed{activity.error?`: ${activity.error}`:"."}</small>;
 if(activity?.status==="canceled")return <small role="status">Sync canceled.</small>;
 if(activity?.status==="completed"){
  const count=/^(\d+) .* issues synchronized successfully$/.exec(activity.summary??"");
  return <small role="status">{count?`${count[1]} ${count[1]==="1"?"ticket synced":"tickets synced"}.`:"Sync completed."}</small>;
 }
 return <small role="status">Syncing…</small>;
}
