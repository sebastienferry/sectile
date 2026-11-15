import {useEffect,useRef,useState} from "react";
import {getOrchestration} from "../lib/agent";
import {saveOrchestration,type OrchestrationSettings as Settings} from "../lib/workspace";
import type {Orchestration} from "../domain/execution";

export function OrchestrationSettings({projectId,defaultOpen,onSupervise,onChanged}:{projectId:string;defaultOpen?:boolean;onSupervise:()=>void;onChanged?:()=>void}){
 const [view,setView]=useState<Orchestration>();const [draft,setDraft]=useState<Settings>();
 const [dirty,setDirty]=useState(false),[pending,setPending]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState("");
 const mounted=useRef(false),acting=useRef(false);
 async function refresh(){const data=await getOrchestration(projectId);if(!mounted.current)return;setView(data);setDraft({orchestratorEnabled:data.orchestratorEnabled,orchestratorMaxConcurrent:data.orchestratorMaxConcurrent,agents:data.agents.map(({id,name,file})=>({id,name,file}))});setDirty(false);setError("");}
 useEffect(()=>{mounted.current=true;void refresh().catch(()=>{if(mounted.current)setError("Orchestration unavailable. Check the Git repository and project services.");});return()=>{mounted.current=false;};},[projectId]);
 function update(patch:Partial<Settings>){if(!draft)return;setDraft({...draft,...patch});setDirty(true);setNotice("");}
 const names=draft?.agents.map(a=>a.name.trim().toLowerCase())??[];
 const valid=!!draft&&draft.orchestratorMaxConcurrent>=1&&draft.orchestratorMaxConcurrent<=10&&(!draft.orchestratorEnabled||draft.agents.length>0)&&draft.agents.every(a=>!!a.name.trim()&&!!a.file.trim())&&new Set(names).size===names.length;
 return <details className="orchestrator-settings" open={defaultOpen||undefined}><summary>Orchestration agents<small>Markdown agents, activation, and shared capacity</small></summary><p>Named agents evaluate the backlog on demand from local Markdown files. They share the project capacity.</p>
 {error&&<p className="error" role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
 {draft&&<form onSubmit={async e=>{e.preventDefault();if(acting.current||!valid||!dirty)return;acting.current=true;setPending(true);setError("");try{await saveOrchestration(projectId,{...draft,agents:draft.agents.map(a=>({...a,name:a.name.trim(),file:a.file.trim()}))});await refresh();onChanged?.();if(mounted.current)setNotice("Settings saved. No agents were launched.");}catch{if(mounted.current)setError("Save not confirmed. Check the Markdown files and services, then reload before trying again.");}finally{acting.current=false;if(mounted.current)setPending(false);}}}>
 <label className="profile-checkbox"><input type="checkbox" checked={draft.orchestratorEnabled} disabled={pending} onChange={e=>update({orchestratorEnabled:e.target.checked})}/> Allow orchestration launches</label>
 <label>Concurrent runs<input type="number" min={1} max={10} required value={draft.orchestratorMaxConcurrent} disabled={pending} onChange={e=>update({orchestratorMaxConcurrent:Number(e.target.value)})}/></label>
 <p className="muted">Disabling prevents future launches; it does not stop tasks already running.</p>
 {draft.agents.map((agent,index)=>{const saved=view?.agents.find(a=>a.id===agent.id);return <fieldset key={agent.id}><legend>{agent.name||"New agent"}</legend><label>Agent name<input required maxLength={80} value={agent.name} disabled={pending} onChange={e=>update({agents:draft.agents.map((a,i)=>i===index?{...a,name:e.target.value}:a)})}/></label><label>Guidelines file<input required value={agent.file} disabled={pending} placeholder="AGENTS.md" onChange={e=>update({agents:draft.agents.map((a,i)=>i===index?{...a,file:e.target.value}:a)})}/></label>
 {saved?.guidelinesError&&<p>{saved.guidelinesError}</p>}
 <details><summary>Saved guidelines preview</summary><pre>{saved?.file===agent.file?saved.guidelines||"No preview available.":"Save to verify this file."}</pre></details>
 <button type="button" disabled={pending} onClick={()=>update({agents:draft.agents.filter(a=>a.id!==agent.id)})}>Remove this agent</button></fieldset>;})}
 <button type="button" disabled={pending||draft.agents.length>=20} onClick={()=>update({agents:[...draft.agents,{id:crypto.randomUUID(),name:`Agent ${draft.agents.length+1}`,file:"AGENTS.md"}]})}>Add agent</button>
 <button disabled={pending||!dirty||!valid}>Save orchestration</button>
 <button type="button" disabled={pending||dirty} onClick={()=>void refresh().catch(()=>setError("Unable to reload guidelines."))}>Reload files</button>
 </form>}
 {!draft&&<button disabled={pending} onClick={()=>{setPending(true);void refresh().catch(()=>setError("Unable to reload orchestration.")).finally(()=>setPending(false));}}>Retry configuration</button>}
 <button disabled={pending||dirty||!view} onClick={onSupervise}>Open supervision</button>
 </details>;
}
