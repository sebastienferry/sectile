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
  const [credentialProvider, setCredentialProvider] = useState("github");
  const [credentialSite, setCredentialSite] = useState("");
  const [trackerEmail, setTrackerEmail] = useState("");
  const [token, setToken] = useState("");
  const [sourceProvider, setSourceProvider] = useState("github");
  const [sourceSite, setSourceSite] = useState("");
  const [trackerName, setTrackerName] = useState("");
  const [scope, setScope] = useState("");
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
    }).catch(cause => { if (mounted.current) setError(String(cause)); });
    return () => { mounted.current = false; };
  }, []);
  async function perform(
    action: () => Promise<unknown>,
    message: string,
    isCancelled?: (result: unknown) => boolean,
    hydrateProfile = false,
  ) {
    if (busy.current) return;
    busy.current = true; setPending(true); setError(""); setNotice("");
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
  function submit(event: FormEvent, action: () => Promise<unknown>, message: string, hydrateProfile = false) {
    event.preventDefault(); void perform(action, message, undefined, hydrateProfile);
  }
  const ready = setup?.ready && setup.signedIn && setup.apiSetupAvailable !== false;
  const fleetMode = setup?.mode === "fleet";
  const sectionLabel = (item: typeof sections[number]) => item.id === "services" && fleetMode ? "Connexion et services" : item.label;
  const activeSection = sections.find(item => item.id === section);
  return <section className="settings local-settings">
    <header className="settings-heading"><span className="eyebrow">{fleetMode ? "VOTRE ESPACE FLEET" : setup?.mode === "local" ? "VOTRE ESPACE LOCAL" : "CONNEXION À SECTILE"}</span><h1>{fleetMode ? "Configuration Fleet" : "Configuration du poste"}</h1>
    <p>{fleetMode ? "Gérez votre profil et vos comptes sur le backend connecté. Les services d’exécution restent sur ce poste." : setup?.mode === "local" ? "Connectez vos outils et configurez vos projets sur ce poste." : "Choisissez le mode Local ou associez ce poste à un backend Fleet."}</p></header>
    {error && <p role="alert" className="error">{error}</p>}
    {notice && <p role="status" className="notice">{notice}</p>}
    {pending && <p role="status">Enregistrement en cours…</p>}
    {!setup && !error && <p>Lecture de la configuration…</p>}
    <div className="settings-layout">
    <nav className="settings-navigation" aria-label="Sections des paramètres">
     {sections.map(item => <button key={item.id} aria-current={section === item.id ? "page" : undefined} disabled={(item.id === "profile" && (!setup?.ready || setup.mode === "fleet" && setup.apiSetupAvailable === false)) || (["connections","projects"].includes(item.id) && !ready)} onClick={() => {navigated.current=true;setSection(item.id);}}><Icon name={item.icon} />{sectionLabel(item)}{section === item.id && <Icon name="chevron" />}</button>)}
     <p>{setup?.mode === "fleet" ? "Les comptes et projets sont conservés sur le backend Fleet." : "Configuration conservée sur ce poste."}</p>
    </nav>
    <div className="settings-content">
    <header className="settings-section-heading"><h2>{activeSection ? sectionLabel(activeSection) : ""}</h2><p>{section === "services" && fleetMode ? "État de la connexion et de l’agent Fleet associé à ce poste." : activeSection?.description}</p></header>
    <div hidden={section !== "engines"}><ProviderSettings projects={setup?.projects??[]} /></div>
    <div hidden={section !== "services"}><article>
      <h2>{fleetMode ? "Agent Fleet" : "Services locaux"}</h2>
      <p>{setup?.mode === "fleet" ? `Mode Fleet · ${setup.agentReady ? "agent connecté" : "agent déconnecté"}${setup.fleetServer ? ` · ${setup.fleetServer}` : ""}` : !setup?.enabled ? "Choisissez un mode pour connecter ce desktop à Sectile." : setup.ready ? `Mode Local · compagnon prêt · ${setup.agentReady ? "exécution prête" : "en attente du profil"}` : "Le compagnon local ne répond pas."}</p>
      {setup?.mode === "fleet" && setup.apiSetupAvailable === false && <p className="muted">Cet agent a été attaché sans identifiant API du poste. Configurez les comptes et projets sur le backend Fleet.</p>}
      {setup?.dataDirectory && <p className="muted runtime-path">{setup.dataDirectory}</p>}
      <p className="muted">{setup?.mode === "fleet" ? "L’agent Fleet reste géré par son installation habituelle." : "Le compagnon démarre automatiquement. Il continue en arrière-plan lorsque vous fermez le desktop, pour laisser les tâches se terminer."}</p>
      {setup?.mode !== "fleet" && <button disabled={pending} onClick={() => void perform(api.enableLocalRuntime, "Compagnon local démarré et mode Local sélectionné.")}>{setup?.enabled ? "Vérifier / démarrer le compagnon local" : "Démarrer en mode Local"}</button>}
      {(!setup?.mode || setup.mode === "unconfigured") && <article><h3>Mode Fleet</h3>
        <p className="muted">Associez ce poste avec un code à usage unique depuis votre profil sur le backend Fleet. Le jeton durable reste dans le stockage privé natif et n’est jamais affiché.</p>
        <form onSubmit={e => submit(e, async () => { try { await api.pairFleetAgent(fleetUrl, fleetCode); } finally { setFleetCode(""); } }, "Poste associé au backend Fleet.", true)}>
          <fieldset disabled={pending}>
            <label>Adresse du backend Fleet<input type="url" required placeholder="https://sectile.example.com" value={fleetUrl} onChange={e => setFleetUrl(e.target.value)} autoComplete="url" /></label>
            <label>Code de jumelage<input type="password" required autoComplete="off" value={fleetCode} onChange={e => setFleetCode(e.target.value)} /></label>
            <button className="primary" type="submit">Associer ce poste à Fleet</button>
          </fieldset>
        </form>
        <details><summary>J’ai déjà associé un agent sur ce poste</summary><p className="muted">Tauri peut reprendre explicitement l’agent existant sans modifier son installation partagée.</p><button disabled={pending} onClick={() => void perform(api.attachFleetAgent, "Agent Fleet associé à ce desktop.", undefined, true)}>Associer l’agent Fleet existant</button></details>
      </article>}
      {setup?.mode === "fleet" && <p className="muted">Cette page configure votre profil et vos projets sur le backend Fleet sélectionné. L’agent d’exécution reste sur ce poste.</p>}
      <button disabled={pending} onClick={() => void perform(async () => {}, "Configuration actualisée.")}>Actualiser</button>
    </article>{setup && setup.mode !== "unconfigured" && <ServiceLogPanel active={section === "services"} fleet={fleetMode} />}</div>
    <div hidden={section !== "profile"}>{setup?.ready && setup.apiSetupAvailable !== false && (setup.mode !== "fleet" || setup.signedIn) && <article>
      <h2>{fleetMode ? "Profil Fleet" : "Profil local"}</h2>
      <form onSubmit={e => submit(e, () => api.saveLocalProfile(email, displayName), "Profil enregistré et agent associé.")}>
        <fieldset disabled={pending}>
          <label>Adresse e-mail<input type="email" required value={email} readOnly={setup.signedIn} onChange={e => setEmail(e.target.value)} autoComplete="email" /></label>
          <label>Nom affiché<input required value={displayName} onChange={e => setDisplayName(e.target.value)} autoComplete="name" /></label>
          <p className="muted">{fleetMode ? "Ce profil identifie vos actions avec votre compte sur le backend Fleet." : "Ce profil identifie vos actions sur ce poste. Il ne crée pas de compte chez un fournisseur."}</p>
          <button className="primary" type="submit">Enregistrer le profil</button>
        </fieldset>
      </form>
    </article>}</div>
    {ready && <>
      <div hidden={section !== "connections"}><OAuthConnections serverManaged={setup.mode === "fleet"} credentials={setup.credentials?.credentials} onConnected={() => { void refresh(); onChanged(); }} />
      <article>
        <details><summary>Accès manuel par jeton · option avancée</summary>
        <form onSubmit={e => submit(e, async () => {
          try { await api.saveLocalCredential({ tracker: credentialProvider, siteUrl: credentialSite, email: trackerEmail, token }); }
          finally { setToken(""); }
        }, "Accès personnel enregistré. Lancez une synchronisation pour vérifier l’accès aux tickets.")}>
          <fieldset disabled={pending}>
            <label>Fournisseur<select value={credentialProvider} onChange={e => { setCredentialProvider(e.target.value); setCredentialSite(""); setToken(""); }}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="jira">Jira</option></select></label>
            <label>URL du site ou de l’API<input type="url" required={credentialProvider !== "github"} placeholder={credentialProvider === "jira" ? "https://votre-site.atlassian.net" : credentialProvider === "gitlab" ? "https://gitlab.com" : "https://api.github.com"} value={credentialSite} onChange={e => setCredentialSite(e.target.value)} /></label>
            <label>E-mail du compte<input type="email" required={credentialProvider === "jira"} value={trackerEmail} onChange={e => setTrackerEmail(e.target.value)} /></label>
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
        {setup.mode !== "fleet" || setup.profile?.role === "admin" ? <form onSubmit={e => submit(e, async () => { await api.createLocalTracker({ name: trackerName, provider: sourceProvider, site: sourceSite, scope }); setTrackerName(""); setScope(""); }, "Tracker ajouté.")}>
          <fieldset disabled={pending}>
            <label>Fournisseur de la source<select value={sourceProvider} onChange={e => {setSourceProvider(e.target.value);setSourceSite("");}}><option value="github">GitHub</option><option value="gitlab">GitLab</option><option value="jira">Jira</option></select></label>
            <label>Adresse de la source<input type="url" required={sourceProvider==="jira"} placeholder={sourceProvider==="jira"?"https://votre-site.atlassian.net":"URL facultative pour l’instance publique"} value={sourceSite} onChange={e=>setSourceSite(e.target.value)} /></label>
            <label>Nom du tracker<input required value={trackerName} onChange={e => setTrackerName(e.target.value)} /></label>
            <label>Dépôt ou clé du projet<input required placeholder={sourceProvider === "jira" ? "PROJET" : "organisation/depot"} value={scope} onChange={e => setScope(e.target.value)} /></label>
            <button type="submit">Ajouter le tracker</button>
          </fieldset>
        </form> : <p className="muted">Les trackers sont partagés dans Fleet. Demandez à un administrateur du serveur d’enregistrer une nouvelle source.</p>}
        <ul className="setup-list">{setup.trackers?.map(t => <li key={t.id}><span>{t.name} <small>{t.provider} · {t.scope}</small>{trackerSyncs[t.id]&&<TrackerSyncResult activityId={trackerSyncs[t.id].id} onFinished={()=>{setTrackerSyncs(previous=>({...previous,[t.id]:{...previous[t.id],active:false}}));void refresh();onChanged();}}/>}</span><button disabled={pending||trackerSyncs[t.id]?.active} onClick={() => void perform(async()=>{const result=await api.syncLocalTracker(t.id);if(!result.activity?.id)throw new Error("Le backend n’a pas fourni de suivi pour cette synchronisation.");setTrackerSyncs(previous=>({...previous,[t.id]:{id:result.activity.id,active:true}}));}, "Synchronisation demandée. Son résultat apparaîtra sous le tracker.")}>Synchroniser</button></li>)}</ul>
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
        <div className="existing-project-folder">
          <h3>Associer un dossier à un projet existant</h3>
          <fieldset disabled={pending || !setup.agentReady}>
            <label>Projet à associer<select required value={projectId} onChange={e => setProjectId(e.target.value)}><option value="">Choisir un projet</option>{setup.projects?.map(p => <option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
            <button type="button" disabled={!projectId} onClick={() => void perform(() => api.chooseProjectFolder(projectId), "Dossier local associé au projet.", result => result === null)}>Choisir un dossier</button>
          </fieldset>
        </div>
      </article></div>
    </>}
    </div></div>
  </section>;
}

function ServiceLogPanel({active,fleet}:{active:boolean;fleet:boolean}) {
 const [snapshot,setSnapshot]=useState<api.ServiceLog>();
 const [error,setError]=useState("");
 const [loading,setLoading]=useState(false);
 const [refresh,setRefresh]=useState(0);
 useEffect(()=>{
  if(!active)return;
  let current=true;
  setSnapshot(undefined);setLoading(true);setError("");
  void api.localServiceLog().then(value=>{if(current)setSnapshot(value);}).catch(cause=>{if(current)setError(String(cause));}).finally(()=>{if(current)setLoading(false);});
  return()=>{current=false;};
 },[active,fleet,refresh]);
 return <article className="service-log">
  <div className="service-log-heading"><h2>{fleet?"Journal de l’agent Fleet":"Journal du compagnon local"}</h2><button type="button" disabled={loading} onClick={()=>setRefresh(value=>value+1)}>Actualiser le journal</button></div>
  <p className="muted">{fleet?"L’agent Fleet associé peut écrire dans le journal de son installation d’origine.":"Diagnostic du compagnon démarré par ce desktop."}</p>
  {snapshot && <p className="service-log-path" title={snapshot.path}><code>{snapshot.path}</code></p>}
  {loading && <p role="status">Lecture du journal…</p>}
  {error && <p role="alert" className="error">Impossible de lire le journal : {error}</p>}
  {!loading && !error && snapshot && <>
   <p role="status">{snapshot.missing?"Aucun journal disponible pour ce service.":!snapshot.text?"Le journal est vide.":snapshot.truncated?"Affichage des 256 derniers Kio du journal.":"Journal actuel du service."}</p>
   {!snapshot.missing && !!snapshot.text && <pre className="service-log-output" tabIndex={0} aria-label="Contenu du journal du service">{snapshot.text}</pre>}
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
 if(readError)return <small className="sync-error" role="alert">Suivi de synchronisation indisponible. <button type="button" onClick={()=>setRetry(value=>value+1)}>Réessayer le suivi</button></small>;
 if(activity?.status==="failed")return <small className="sync-error" role="alert">Synchronisation échouée{activity.error?` : ${activity.error}`:"."}</small>;
 if(activity?.status==="canceled")return <small role="status">Synchronisation annulée.</small>;
 if(activity?.status==="completed"){
  const count=/^(\d+) .* issues synchronized successfully$/.exec(activity.summary??"");
  return <small role="status">{count?`${count[1]} ${count[1]==="1"?"ticket synchronisé":"tickets synchronisés"}.`:"Synchronisation terminée."}</small>;
 }
 return <small role="status">Synchronisation en cours…</small>;
}
