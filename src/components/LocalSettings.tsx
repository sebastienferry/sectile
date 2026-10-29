import { useEffect, useRef, useState, type FormEvent } from "react";
import { ProviderSettings } from "./ProviderSettings";
import { OAuthConnections } from "./OAuthConnections";
import { Icon } from "./Icon";
import * as api from "../lib/local";

type SettingsSection = "engines" | "profile" | "connections" | "projects" | "services";
const sections = [
 {id:"engines", label:"Moteurs IA", icon:"agent", description:"Connectez Claude Code ou Codex pour travailler dans vos projets."},
 {id:"profile", label:"Profil", icon:"user", description:"Votre identité pour les actions effectuées depuis ce poste."},
 {id:"connections", label:"Comptes et trackers", icon:"link", description:"Connectez vos comptes et choisissez les sources de vos tickets."},
 {id:"projects", label:"Projets", icon:"folder", description:"Partez d’un dossier local ou associez un projet à un tracker."},
 {id:"services", label:"Services locaux", icon:"monitor", description:"État de votre installation et outils de diagnostic."},
] as const;
export function LocalSettings({ onChanged, onOpenFolder, onConfigureProject }: { onChanged: () => void; onOpenFolder?: () => void; onConfigureProject?: (id:string) => void }) {
 const [section, setSection] = useState<SettingsSection>("engines");
 const navigated = useRef(false);
  const [setup, setSetup] = useState<api.LocalSetup | null>(null);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [fleetUrl, setFleetUrl] = useState("");
  const [fleetCode, setFleetCode] = useState("");
  const [provider, setProvider] = useState("github");
  const [site, setSite] = useState("");
  const [trackerEmail, setTrackerEmail] = useState("");
  const [token, setToken] = useState("");
  const [trackerName, setTrackerName] = useState("");
  const [scope, setScope] = useState("");
  const [projectName, setProjectName] = useState("");
  const [trackerId, setTrackerId] = useState("");
  const [projectId, setProjectId] = useState("");
  const [path, setPath] = useState("");
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
    }).catch(cause => { if (mounted.current) setError(String(cause)); });
    return () => { mounted.current = false; };
  }, []);
  async function perform(action: () => Promise<unknown>, message: string) {
    if (busy.current) return;
    busy.current = true; setPending(true); setError(""); setNotice("");
    try {
      await action();
      if (mounted.current) setNotice(message);
      const next = await refresh();
      setSection(previous => previous === "services" && next.ready && !next.signedIn ? "profile" : previous === "profile" && !setup?.signedIn && next.signedIn ? "engines" : previous);
      onChanged();
    } catch (cause) {
      if (mounted.current) setError(String(cause));
      // A mutation may have succeeded before a later step failed. Refresh the
      // saved objects so the user can recover without creating duplicates.
      try { await refresh(); onChanged(); } catch { /* Keep the original error. */ }
    } finally { busy.current = false; if (mounted.current) setPending(false); }
  }
  function submit(event: FormEvent, action: () => Promise<unknown>, message: string) {
    event.preventDefault(); void perform(action, message);
  }
  const ready = setup?.ready && setup.signedIn && setup.apiSetupAvailable !== false;
  return <section className="settings local-settings">
    <header className="settings-heading"><span className="eyebrow">VOTRE ESPACE LOCAL</span><h1>Configuration du poste</h1>
    <p>Connectez vos outils. Choisissez vos projets. Tout se configure ici.</p></header>
    {error && <p role="alert" className="error">{error}</p>}
    {notice && <p role="status" className="notice">{notice}</p>}
    {pending && <p role="status">Enregistrement en cours…</p>}
    {!setup && !error && <p>Lecture de la configuration…</p>}
    <div className="settings-layout">
    <nav className="settings-navigation" aria-label="Sections des paramètres">
     {sections.map(item => <button key={item.id} aria-current={section === item.id ? "page" : undefined} disabled={(item.id === "profile" && (!setup?.ready || setup.mode === "fleet" && setup.apiSetupAvailable === false)) || (["connections","projects"].includes(item.id) && !ready)} onClick={() => {navigated.current=true;setSection(item.id);}}><Icon name={item.icon} />{item.label}{section === item.id && <Icon name="chevron" />}</button>)}
     <p>{setup?.mode === "fleet" ? "Les comptes et projets sont conservés sur le backend Fleet." : "Configuration conservée sur ce poste."}</p>
    </nav>
    <div className="settings-content">
    <header className="settings-section-heading"><h2>{sections.find(item=>item.id===section)?.label}</h2><p>{sections.find(item=>item.id===section)?.description}</p></header>
    <div hidden={section !== "engines"}><ProviderSettings projects={setup?.projects??[]} /></div>
    <div hidden={section !== "services"}><article>
      <h2>Services locaux</h2>
      <p>{setup?.mode === "fleet" ? `Mode Fleet · ${setup.agentReady ? "agent connecté" : "agent déconnecté"}${setup.fleetServer ? ` · ${setup.fleetServer}` : ""}` : !setup?.enabled ? "Choisissez un mode pour connecter ce desktop à Sectile." : setup.ready ? `Mode Local · compagnon prêt · ${setup.agentReady ? "exécution prête" : "en attente du profil"}` : "Le compagnon local ne répond pas."}</p>
      {setup?.mode === "fleet" && setup.apiSetupAvailable === false && <p className="muted">Cet agent a été attaché sans identifiant API du poste. Configurez les comptes et projets sur le backend Fleet.</p>}
      {setup?.dataDirectory && <p className="muted runtime-path">{setup.dataDirectory}</p>}
      <p className="muted">{setup?.mode === "fleet" ? "L’agent Fleet reste géré par son installation habituelle." : "Le compagnon démarre automatiquement. Il continue en arrière-plan lorsque vous fermez le desktop, pour laisser les tâches se terminer."}</p>
      {setup?.mode !== "fleet" && <button disabled={pending} onClick={() => void perform(api.enableLocalRuntime, "Compagnon local démarré et mode Local sélectionné.")}>{setup?.enabled ? "Vérifier / démarrer le compagnon local" : "Démarrer en mode Local"}</button>}
      {(!setup?.mode || setup.mode === "unconfigured") && <article><h3>Mode Fleet</h3>
        <p className="muted">Associez ce poste avec un code à usage unique depuis votre profil sur le backend Fleet. Le jeton durable reste dans le stockage privé natif et n’est jamais affiché.</p>
        <form onSubmit={e => submit(e, async () => { try { await api.pairFleetAgent(fleetUrl, fleetCode); } finally { setFleetCode(""); } }, "Poste associé au backend Fleet.")}>
          <fieldset disabled={pending}>
            <label>Adresse du backend Fleet<input type="url" required placeholder="https://sectile.example.com" value={fleetUrl} onChange={e => setFleetUrl(e.target.value)} autoComplete="url" /></label>
            <label>Code de jumelage<input type="password" required autoComplete="off" value={fleetCode} onChange={e => setFleetCode(e.target.value)} /></label>
            <button className="primary" type="submit">Associer ce poste à Fleet</button>
          </fieldset>
        </form>
        <details><summary>J’ai déjà associé un agent sur ce poste</summary><p className="muted">Tauri peut reprendre explicitement l’agent existant sans modifier son installation partagée.</p><button disabled={pending} onClick={() => void perform(api.attachFleetAgent, "Agent Fleet associé à ce desktop.")}>Associer l’agent Fleet existant</button></details>
      </article>}
      {setup?.mode === "fleet" && <p className="muted">Les profils, trackers et projets Fleet se configurent sur le backend Sectile. Cette page ne modifie que le compagnon Local.</p>}
      <button disabled={pending} onClick={() => void perform(async () => {}, "Configuration actualisée.")}>Actualiser</button>
    </article></div>
    <div hidden={section !== "profile"}>{setup?.ready && setup.apiSetupAvailable !== false && (setup.mode !== "fleet" || setup.signedIn) && <article>
      <h2>Profil local</h2>
      <form onSubmit={e => submit(e, () => api.saveLocalProfile(email, displayName), "Profil enregistré et agent associé.")}>
        <fieldset disabled={pending}>
          <label>Adresse e-mail<input type="email" required value={email} readOnly={setup.signedIn} onChange={e => setEmail(e.target.value)} autoComplete="email" /></label>
          <label>Nom affiché<input required value={displayName} onChange={e => setDisplayName(e.target.value)} autoComplete="name" /></label>
          <p className="muted">Ce profil identifie vos actions locales. Il ne crée pas de compte chez un fournisseur.</p>
          <button className="primary" type="submit">Enregistrer le profil</button>
        </fieldset>
      </form>
    </article>}</div>
    {ready && <>
      <div hidden={section !== "connections"}><OAuthConnections serverManaged={setup.mode === "fleet"} credentials={setup.credentials?.credentials} onConnected={() => { void refresh(); onChanged(); }} />
      <article>
        <details><summary>Accès manuel par jeton · option avancée</summary>
        <form onSubmit={e => submit(e, async () => {
          try { await api.saveLocalCredential({ tracker: provider, siteUrl: site, email: trackerEmail, token }); }
          finally { setToken(""); }
        }, "Accès personnel enregistré. Lancez une synchronisation pour vérifier l’accès aux tickets.")}>
          <fieldset disabled={pending}>
            <label>Fournisseur<select value={provider} onChange={e => { setProvider(e.target.value); setSite(""); setToken(""); }}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="jira">Jira</option></select></label>
            <label>URL du site ou de l’API<input type="url" required={provider !== "github"} placeholder={provider === "jira" ? "https://votre-site.atlassian.net" : provider === "gitlab" ? "https://gitlab.com" : "https://api.github.com"} value={site} onChange={e => setSite(e.target.value)} /></label>
            <label>E-mail du compte<input type="email" required={provider === "jira"} value={trackerEmail} onChange={e => setTrackerEmail(e.target.value)} /></label>
            <label>Jeton personnel<input type="password" required autoComplete="off" value={token} onChange={e => setToken(e.target.value)} /></label>
            <p className="muted">Le jeton est chiffré par {setup.mode === "fleet" ? "le backend Fleet" : "le serveur local"}. L’enregistrement remplace l’accès personnel de ce fournisseur.</p>
            <button type="submit">Enregistrer l’accès</button>
          </fieldset>
        </form>
        <ul className="setup-list">{setup.credentials?.credentials.map(c => <li key={c.tracker}><span>{c.tracker} <small>{c.account || "Accès enregistré · compte non confirmé"}{c.siteUrl && ` · ${c.siteUrl}`}</small></span></li>)}</ul>
        </details>
      </article>
      <article>
        <h2>Sources de tickets</h2>
        {setup.mode !== "fleet" || setup.profile?.role === "admin" ? <form onSubmit={e => submit(e, async () => { await api.createLocalTracker({ name: trackerName, provider, site, scope }); setTrackerName(""); setScope(""); }, "Tracker ajouté.")}>
          <fieldset disabled={pending}>
            <label>Fournisseur de la source<select value={provider} onChange={e => {setProvider(e.target.value);setSite("");}}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="jira">Jira</option></select></label>
            <label>Adresse de la source<input type="url" required={provider==="jira"} placeholder={provider==="jira"?"https://votre-site.atlassian.net":"URL facultative pour l’instance publique"} value={site} onChange={e=>setSite(e.target.value)} /></label>
            <label>Nom du tracker<input required value={trackerName} onChange={e => setTrackerName(e.target.value)} /></label>
            <label>Dépôt ou clé du projet<input required placeholder={provider === "jira" ? "PROJET" : "organisation/depot"} value={scope} onChange={e => setScope(e.target.value)} /></label>
            <button type="submit">Ajouter le tracker</button>
          </fieldset>
        </form> : <p className="muted">Les trackers sont partagés dans Fleet. Demandez à un administrateur du serveur d’enregistrer une nouvelle source.</p>}
        <ul className="setup-list">{setup.trackers?.map(t => <li key={t.id}><span>{t.name} <small>{t.provider} · {t.scope}</small></span><button disabled={pending} onClick={() => void perform(() => api.syncLocalTracker(t.id), "Synchronisation demandée. Les tickets apparaîtront après traitement.")}>Synchroniser</button></li>)}</ul>
      </article></div>
      <div hidden={section !== "projects"}>
      {onConfigureProject && !!setup.projects?.length && <article><h2>Vos projets</h2><div className="profile-list">{setup.projects.map(project=><div className="profile-card" key={project.id}><strong>{project.name}</strong><button onClick={()=>onConfigureProject(project.id)}>Configurer {project.name}</button></div>)}</div></article>}
      {onOpenFolder && <article className="folder-setup"><div className="settings-illustration"><Icon name="folder" /></div><h2>Commencer avec un dossier local</h2><p>Ouvrez un dossier et lancez votre premier prompt. Vous pourrez connecter un tracker plus tard.</p><button className="primary" onClick={onOpenFolder}><Icon name="plus" />Choisir un dossier local</button></article>}
      <article>
        <h2>Relier un projet à un tracker</h2>
        <form onSubmit={e => submit(e, async () => { await api.createLocalProject(projectName, trackerId); setProjectName(""); }, "Projet créé. Associez maintenant son dépôt local.")}>
          <fieldset disabled={pending}>
            <label>Nom du projet<input required value={projectName} onChange={e => setProjectName(e.target.value)} /></label>
            <label>Tracker<select required value={trackerId} onChange={e => setTrackerId(e.target.value)}><option value="">Choisir un tracker</option>{setup.trackers?.map(t => <option key={t.id} value={t.id}>{t.name}</option>)}</select></label>
            <button type="submit">Créer le projet</button>
          </fieldset>
        </form>
        <form onSubmit={e => submit(e, () => api.mapLocalProject(projectId, path), "Dépôt local associé au projet.")}>
          <fieldset disabled={pending || !setup.agentReady}>
            <label>Projet à associer<select required value={projectId} onChange={e => setProjectId(e.target.value)}><option value="">Choisir un projet</option>{setup.projects?.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
            <label>Chemin absolu du dépôt Git<input required placeholder="/chemin/vers/depot" value={path} onChange={e => setPath(e.target.value)} /></label>
            <button type="submit">Associer le dépôt</button>
          </fieldset>
        </form>
      </article></div>
    </>}
    </div></div>
  </section>;
}
