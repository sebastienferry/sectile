import { useEffect, useRef, useState } from "react";
import * as api from "../lib/local";
const names: Record<string,string> = { github: "GitHub", gitlab: "GitLab.com", jira: "Jira Cloud" };
const outcome: Record<string,string> = {
  connected: "Account connected. Your credentials are saved on this workstation.",
  cancelled: "Sign-in canceled.", expired: "The request expired. You can try again.",
  invalid: "Invalid or already used sign-in response. Try again.",
  unreachable: "The provider could not validate the sign-in. Try again.",
  no_site: "No configured Jira site was authorized. Check your ticket sources and consent.",
  failed: "Sign-in failed. Check the OAuth application configuration.",
};
export function OAuthConnections({ onConnected, credentials = [], serverManaged = false, canManageTrackers = true }: { onConnected: () => void; credentials?: {tracker:string;account?:string;kind?:string;disconnected?:boolean}[]; serverManaged?: boolean; canManageTrackers?: boolean }) {
  const [apps,setApps] = useState<Record<string,api.OAuthApp>>({});
  const [error,setError] = useState("");
  async function reload() { try { setApps(await api.oauthApps()); setError(""); } catch(e) { setError(String(e)); } }
  useEffect(()=>{ let live=true; void api.oauthApps().then(v=>{if(live)setApps(v);}).catch(e=>{if(live)setError(String(e));}); return ()=>{live=false;}; },[]);
  const ownerGuidance = serverManaged ? "OAuth applications and their callbacks are managed by the Fleet backend." : "Each provider requires an OAuth application configured once for this installation.";
  const jiraGuidance = serverManaged && !canManageTrackers ? "For Jira, ask an administrator to add your site to the ticket sources." : "For Jira, also add your site in the ticket sources below.";
  return <article>
    <h2>Account connections</h2>
    <p>Sign in with your default browser. Credentials are saved automatically without copying a personal access token.</p>
    <p className="muted">{ownerGuidance} {jiraGuidance}</p>
    {error && <p role="alert" className="error">{error}<button onClick={()=>void reload()}>Retry</button></p>}
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
  const cancelling=useRef(false);
  const cancelled=useRef(false);
  const settings=useRef<HTMLDetailsElement>(null);
  const connected=useRef(onConnected); connected.current=onConnected;
  useEffect(()=>{mounted.current=true;return()=>{mounted.current=false;};},[]);
  useEffect(()=>{
    if(!flow)return;
    let disposed=false, terminal=false;
    let timer:ReturnType<typeof setTimeout>;
    const deadline=Date.now()+10*60*1000;
    async function poll(){
      if(cancelling.current){timer=setTimeout(()=>void poll(),1500);return;}
      if(Date.now()>=deadline){setFlow("");setMessage(outcome.expired);return;}
      try {
        const value=await api.oauthStatus(flow);
        if(disposed)return;
        if(cancelling.current){timer=setTimeout(()=>void poll(),1500);return;}
        if(value.status!=="waiting"&&value.status!=="completing") {
          terminal=true; setFlow(""); setMessage(value.status === "connected" && serverManaged ? "Account connected. Your credentials are saved on the Fleet backend." : outcome[value.status]??outcome.failed);
          if(value.status==="connected")connected.current();
          return;
        }
        setError(""); timer=setTimeout(()=>void poll(),1500);
      } catch(e) {
        if(!disposed){if(!cancelling.current)setError("Sign-in status check interrupted: "+String(e));timer=setTimeout(()=>void poll(),3000);}
      }
    }
    void poll();
    return()=>{disposed=true;clearTimeout(timer);if(!terminal&&!cancelled.current)void api.cancelOAuth(flow).catch(()=>{});};
  },[flow]);
  async function begin(){
    if(busy.current||flow)return;
    busy.current=true;cancelled.current=false;setPending(true);setError("");setMessage("");
    try {
      const id=await api.beginOAuth(provider);
      if(!mounted.current){await api.cancelOAuth(id);return;}
      setFlow(id);setMessage("Complete sign-in in your browser. This view is waiting for provider confirmation.");
    }catch(e){if(mounted.current)setError(String(e));}
    finally{busy.current=false;if(mounted.current)setPending(false);}
  }
  async function cancel(){
    if(!flow||cancelling.current)return;
    cancelling.current=true;setPending(true);setError("");
    try {
      await api.cancelOAuth(flow);
      if(mounted.current){cancelled.current=true;setFlow("");setMessage(outcome.cancelled);}
    }catch(e){if(mounted.current)setError(String(e));}
    finally{cancelling.current=false;if(mounted.current)setPending(false);}
  }
  const configured=configuration.configured && (serverManaged ? Boolean(configuration.redirectUrl) : configuration.redirectUrl===configuration.expectedRedirectUrl);
  return <section className="oauth-provider">
    <div className="oauth-heading"><h3>{name}</h3><button className="primary" disabled={pending||!!flow||!configured} onClick={()=>void begin()}>Connect to {name}</button></div>
    {connection?.disconnected ? <p role="alert" className="error">{name} connection interrupted{connection.account?` for ${connection.account}`:""}. Reconnect to access your tickets.</p> : connection?.account && <p>Connected account: {connection.account}</p>}
    {message&&<p role="status">{message}</p>}
    {error&&<p role="alert" className="error">{error}</p>}
    {flow&&<button disabled={pending} onClick={()=>void cancel()}>Cancel sign-in</button>}
    {serverManaged ? <p className="muted">The OAuth application and callback URL are managed on the Fleet backend.</p> : <details ref={settings} onToggle={event=>{if(!event.currentTarget.open){setClientId("");setSecret("");}}}>
      <summary>{configured?"OAuth application configured · edit":"Configure OAuth application"}</summary>
      <p>Register this callback URL in your {name} application:</p>
      <input aria-label={`${name} callback URL`} readOnly value={configuration.expectedRedirectUrl} onFocus={e=>e.target.select()} />
      <p className="muted">{provider==="github"?"GitHub OAuth application · requested scopes: repo, read:project.":provider==="gitlab"?"Confidential GitLab.com application · requested scope: api.":"Atlassian OAuth 2.0 (3LO) application · add the Jira scopes described in the project documentation."}</p>
      <form onSubmit={event=>{
        event.preventDefault();if(busy.current)return;busy.current=true;setPending(true);setError("");
        void api.saveOAuthApp(provider,clientId,secret).then(()=>{setMessage("OAuth application saved. You can now sign in.");if(settings.current)settings.current.open=false;onSaved();}).catch(e=>setError(String(e))).finally(()=>{setClientId("");setSecret("");busy.current=false;setPending(false);});
      }}>
        <fieldset disabled={pending||!!flow}>
          <label>{name} client ID<input type="password" required={!configuration.clientIdSet} value={clientId} onChange={e=>setClientId(e.target.value)} autoComplete="off" placeholder={configuration.clientIdSet?"Leave blank to keep existing client ID":""} /></label>
          <label>{name} client secret<input type="password" required={!configuration.secretSet} value={secret} onChange={e=>setSecret(e.target.value)} autoComplete="off" placeholder={configuration.secretSet?"Leave blank to keep existing secret":""} /></label>
          <p className="muted">Saved credentials remain in the local service. Blank fields keep existing values.</p>
          <button type="submit">Save {name} application</button>
        </fieldset>
      </form>
    </details>}
  </section>;
}
