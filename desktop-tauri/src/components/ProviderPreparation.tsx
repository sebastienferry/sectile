import {ProviderPermissions} from "./ProviderPermissions";
import {useEffect,useRef,useState} from "react";
import {configureProviderMcp,installProviderSkills,providerReadiness,type Readiness} from "../lib/workspace";

const statusLabel=(status?:string)=>status==="success"?"Configuration verified":status==="skipped"?"Explicitly skipped":status==="failed"?"Failed":status==="unknown"?"Verification unavailable":"Unverified";
export function ProviderPreparation({provider,projects}:{provider:string;projects:{id:string;name:string}[]}){
 const [view,setView]=useState<Readiness>();
 const [source,setSource]=useState("");
 const [pending,setPending]=useState(false);
 const [replace,setReplace]=useState(false);
 const [error,setError]=useState("");
 const [notice,setNotice]=useState("");
 const mounted=useRef(false),acting=useRef(false);
 async function refresh(){const data=await providerReadiness(provider);if(mounted.current)setView(data);}
 useEffect(()=>{mounted.current=true;void refresh().catch(()=>{if(mounted.current)setError("Sectile preparation unavailable. Check local services.");});return()=>{mounted.current=false;};},[provider]);
 async function act(action:()=>Promise<unknown>,message:string){if(acting.current)return;acting.current=true;setPending(true);setError("");setNotice("");try{await action();await refresh();if(mounted.current){setNotice(message);setReplace(false);}}catch{if(mounted.current)setError("Operation could not be confirmed. Check the source project and open runs, then refresh before trying again.");}finally{acting.current=false;if(mounted.current)setPending(false);}}
 const isolated=view?.steps?.mcp?.scope==="session";
 const conflict=!!view?.steps?.mcp?.configuration?.present&&!view.steps.mcp.configuration.valid;
 return <details className="provider-preparation"><summary>Sectile skills and MCP connection</summary>
 <p>Connecting to the CLI and preparing it for Sectile workflows are separate steps. {isolated?"MCP is provided only to this companion's sessions without modifying Electron's configuration.":"MCP actions configure your user account for this engine."} These actions do not start any task.</p>
 {error&&<p role="alert" className="error">{error}</p>}{notice&&<p role="status">{notice}</p>}
 <dl><dt>Skills</dt><dd>{statusLabel(view?.steps?.skills?.status)}</dd><dt>MCP</dt><dd>{statusLabel(view?.steps?.mcp?.status)}</dd><dt>Permissions</dt><dd>Current settings kept · {view?.permissions?.mode ? String(view.permissions.mode):"CLI permissions"}</dd></dl>
 <p className="muted">"Unverified" does not mean missing: a plugin or manual installation may already work. A registered MCP configuration still needs to be tested in a conversation.</p>
 <label>Skills source project<select aria-label={`Skills source project ${provider}`} disabled={pending} value={source} onChange={e=>setSource(e.target.value)}><option value="">Choose a configured project…</option>{projects.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
 <button disabled={pending||!source} onClick={()=>void act(async()=>{const result=await installProviderSkills(provider,source);if(!result?.success||result.skills?.status!=="success")throw new Error("Skill installation not confirmed");},"Skills installed and verified. MCP settings remain independent.")}>Install skills</button>
 <p className="muted">Installation requires a configured repository and stopped runs. It does not change CLI permissions.</p>
 {conflict&&<label className="profile-checkbox"><input type="checkbox" checked={replace} disabled={pending} onChange={e=>setReplace(e.target.checked)}/> Replace the existing Sectile MCP entry with the local companion's entry. The other desktop app may use this entry.</label>}
 {isolated?<p className="muted">Isolated MCP connection active for future Claude Code and Codex launches. Open Codex conversations keep their configuration until their process restarts.</p>:<button disabled={pending||!view||(conflict&&!replace)} onClick={()=>void act(()=>configureProviderMcp(provider,replace),"MCP registered to the local agent. Verify its access in a conversation next.")}>Configure local MCP</button>}
 <button disabled={pending} onClick={()=>void act(refresh,"Preparation status refreshed.")}>Check preparation</button>
 <ProviderPermissions provider={provider} view={view} onSaved={refresh}/>
 </details>;
}
