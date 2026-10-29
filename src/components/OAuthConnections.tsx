import { useEffect, useRef, useState } from "react";
import * as api from "../lib/local";
const names: Record<string,string> = { github: "GitHub", gitlab: "GitLab.com", jira: "Jira Cloud" };
const outcome: Record<string,string> = {
  connected: "Compte connecté. Vos accès sont enregistrés sur ce poste.",
  cancelled: "Connexion annulée.", expired: "La demande a expiré. Vous pouvez recommencer.",
  invalid: "Réponse de connexion invalide ou déjà utilisée. Recommencez.",
  unreachable: "Le fournisseur n’a pas pu valider la connexion. Réessayez.",
  no_site: "Aucun site Jira configuré n’a été autorisé. Vérifiez vos sources de tickets et le consentement.",
  failed: "La connexion a échoué. Vérifiez la configuration de l’application OAuth.",
};
export function OAuthConnections({ onConnected, credentials = [], serverManaged = false, canManageTrackers = true }: { onConnected: () => void; credentials?: {tracker:string;account?:string;kind?:string;disconnected?:boolean}[]; serverManaged?: boolean; canManageTrackers?: boolean }) {
  const [apps,setApps] = useState<Record<string,api.OAuthApp>>({});
  const [error,setError] = useState("");
  async function reload() { try { setApps(await api.oauthApps()); setError(""); } catch(e) { setError(String(e)); } }
  useEffect(()=>{ let live=true; void api.oauthApps().then(v=>{if(live)setApps(v);}).catch(e=>{if(live)setError(String(e));}); return ()=>{live=false;}; },[]);
  const ownerGuidance = serverManaged ? "Les applications OAuth et leurs rappels sont gérés par le backend Fleet." : "Chaque fournisseur nécessite une application OAuth configurée une fois pour cette installation.";
  const jiraGuidance = serverManaged && !canManageTrackers ? "Pour Jira, demandez à un administrateur d’ajouter votre site aux sources de tickets." : "Pour Jira, ajoutez aussi votre site dans les sources de tickets ci-dessous.";
  return <article>
    <h2>Connexion aux comptes</h2>
    <p>Connectez-vous dans votre navigateur habituel. Les accès seront enregistrés automatiquement, sans copier de jeton personnel.</p>
    <p className="muted">{ownerGuidance} {jiraGuidance}</p>
    {error && <p role="alert" className="error">{error}<button onClick={()=>void reload()}>Réessayer</button></p>}
    {Object.entries(names).map(([provider,name])=>apps[provider] && <OAuthProvider key={provider} provider={provider} name={name} configuration={apps[provider]} serverManaged={serverManaged} connection={credentials.find(c=>c.tracker===provider && c.kind==="oauth")} onSaved={()=>void reload()} onConnected={onConnected} />)}
  </article>;
}
function OAuthProvider({provider,name,configuration,serverManaged,connection,onSaved,onConnected}:{provider:string;name:string;configuration:api.OAuthApp;serverManaged:boolean;connection?:{account?:string;disconnected?:boolean};onSaved:()=>void;onConnected:()=>void}) {
  const [clientId,setClientId]=useState("");
  const [secret,setSecret]=useState("");
  const [pending,setPending]=useState(false);
  const [flow,setFlow]=useState("");
  const [message,setMessage]=useState("");
  const [error,setError]=useState("");
  const mounted=useRef(true);
  const busy=useRef(false);
  const settings=useRef<HTMLDetailsElement>(null);
  const connected=useRef(onConnected); connected.current=onConnected;
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  useEffect(()=>{
    if(!flow)return;
    let disposed=false, terminal=false;
    let timer:ReturnType<typeof setTimeout>;
    const deadline=Date.now()+10*60*1000;
    async function poll(){
      if(Date.now()>=deadline){setFlow("");setMessage(outcome.expired);return;}
      try {
        const value=await api.oauthStatus(flow);
        if(disposed)return;
        if(value.status!=="waiting"&&value.status!=="completing") {
          terminal=true; setFlow(""); setMessage(value.status === "connected" && serverManaged ? "Compte connecté. Vos accès sont enregistrés sur le backend Fleet." : outcome[value.status]??outcome.failed);
          if(value.status==="connected")connected.current();
          return;
        }
        setError(""); timer=setTimeout(()=>void poll(),1500);
      } catch(e) {
        if(!disposed){setError("Suivi de connexion interrompu : "+String(e));timer=setTimeout(()=>void poll(),3000);}
      }
    }
    void poll();
    return()=>{disposed=true;clearTimeout(timer);if(!terminal)void api.cancelOAuth(flow).catch(()=>{});};
  },[flow]);
  async function begin(){
    if(busy.current||flow)return;
    busy.current=true;setPending(true);setError("");setMessage("");
    try {
      const id=await api.beginOAuth(provider);
      if(!mounted.current){await api.cancelOAuth(id);return;}
      setFlow(id);setMessage("Terminez la connexion dans le navigateur. Cette vue attend la confirmation du fournisseur.");
    }catch(e){if(mounted.current)setError(String(e));}
    finally{busy.current=false;if(mounted.current)setPending(false);}
  }
  const configured=configuration.configured && (serverManaged ? Boolean(configuration.redirectUrl) : configuration.redirectUrl===configuration.expectedRedirectUrl);
  return <section className="oauth-provider">
    <div className="oauth-heading"><h3>{name}</h3><button className="primary" disabled={pending||!!flow||!configured} onClick={()=>void begin()}>Se connecter à {name}</button></div>
    {connection?.disconnected ? <p role="alert" className="error">Connexion {name} interrompue{connection.account?` pour ${connection.account}`:""}. Reconnectez-vous pour accéder à vos tickets.</p> : connection?.account && <p>Compte connecté : {connection.account}</p>}
    {message&&<p role="status">{message}</p>}
    {error&&<p role="alert" className="error">{error}</p>}
    {flow&&<button onClick={()=>void api.cancelOAuth(flow).then(()=>{setFlow("");setMessage(outcome.cancelled);}).catch(e=>setError(String(e)))}>Annuler la connexion</button>}
    {serverManaged ? <p className="muted">L’application OAuth et l’URL de rappel sont administrées sur le backend Fleet.</p> : <details ref={settings} onToggle={event=>{if(!event.currentTarget.open){setClientId("");setSecret("");}}}>
      <summary>{configured?"Application OAuth configurée · modifier":"Configurer l’application OAuth"}</summary>
      <p>Enregistrez cette URL de rappel dans l’application {name} :</p>
      <input aria-label={`URL de rappel ${name}`} readOnly value={configuration.expectedRedirectUrl} onFocus={e=>e.target.select()} />
      <p className="muted">{provider==="github"?"Application GitHub OAuth · autorisations demandées : repo, read:project.":provider==="gitlab"?"Application GitLab.com confidentielle · autorisation demandée : api.":"Application Atlassian OAuth 2.0 (3LO) · ajoutez les autorisations Jira décrites dans la documentation du projet."}</p>
      <form onSubmit={event=>{
        event.preventDefault();if(busy.current)return;busy.current=true;setPending(true);setError("");
        void api.saveOAuthApp(provider,clientId,secret).then(()=>{setMessage("Application OAuth enregistrée. Vous pouvez vous connecter.");if(settings.current)settings.current.open=false;onSaved();}).catch(e=>setError(String(e))).finally(()=>{setClientId("");setSecret("");busy.current=false;setPending(false);});
      }}>
        <fieldset disabled={pending||!!flow}>
          <label>Identifiant client {name}<input type="password" required={!configuration.clientIdSet} value={clientId} onChange={e=>setClientId(e.target.value)} autoComplete="off" placeholder={configuration.clientIdSet?"Laisser vide pour conserver l’identifiant":""} /></label>
          <label>Secret client {name}<input type="password" required={!configuration.secretSet} value={secret} onChange={e=>setSecret(e.target.value)} autoComplete="off" placeholder={configuration.secretSet?"Laisser vide pour conserver le secret":""} /></label>
          <p className="muted">Les identifiants enregistrés restent dans le service local. Les champs vides conservent les valeurs existantes.</p>
          <button type="submit">Enregistrer l’application {name}</button>
        </fieldset>
      </form>
    </details>}
  </section>;
}
