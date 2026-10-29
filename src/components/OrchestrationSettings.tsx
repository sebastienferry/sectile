import {useEffect,useRef,useState} from "react";
import {getOrchestration} from "../lib/agent";
import {saveOrchestration,type OrchestrationSettings as Settings} from "../lib/workspace";
import type {Orchestration} from "../domain/execution";

export function OrchestrationSettings({projectId,onSupervise}:{projectId:string;onSupervise:()=>void}){
 const [view,setView]=useState<Orchestration>();const [draft,setDraft]=useState<Settings>();
 const [dirty,setDirty]=useState(false),[pending,setPending]=useState(false),[error,setError]=useState(""),[notice,setNotice]=useState("");
 const mounted=useRef(false),acting=useRef(false);
 async function refresh(){const data=await getOrchestration(projectId);if(!mounted.current)return;setView(data);setDraft({orchestratorEnabled:data.orchestratorEnabled,orchestratorMaxConcurrent:data.orchestratorMaxConcurrent,agents:data.agents.map(({id,name,file})=>({id,name,file}))});setDirty(false);setError("");}
 useEffect(()=>{mounted.current=true;void refresh().catch(()=>{if(mounted.current)setError("Orchestration indisponible. Vérifiez le dépôt Git et les services du projet.");});return()=>{mounted.current=false;};},[projectId]);
 function update(patch:Partial<Settings>){if(!draft)return;setDraft({...draft,...patch});setDirty(true);setNotice("");}
 const names=draft?.agents.map(a=>a.name.trim().toLowerCase())??[];
 const valid=!!draft&&draft.orchestratorMaxConcurrent>=1&&draft.orchestratorMaxConcurrent<=10&&(!draft.orchestratorEnabled||draft.agents.length>0)&&draft.agents.every(a=>!!a.name.trim()&&!!a.file.trim())&&new Set(names).size===names.length;
 return <details className="orchestrator-settings"><summary>Agents d’orchestration<small>Agents Markdown, activation et capacité partagée</small></summary><p>Des agents nommés évaluent le backlog à la demande, à partir de fichiers Markdown locaux. Ils partagent la capacité du projet.</p>
 {error&&<p className="error" role="alert">{error}</p>}{notice&&<p role="status">{notice}</p>}
 {draft&&<form onSubmit={async e=>{e.preventDefault();if(acting.current||!valid||!dirty)return;acting.current=true;setPending(true);setError("");try{await saveOrchestration(projectId,{...draft,agents:draft.agents.map(a=>({...a,name:a.name.trim(),file:a.file.trim()}))});await refresh();if(mounted.current)setNotice("Configuration enregistrée. Aucun agent n’a été lancé.");}catch{if(mounted.current)setError("Enregistrement non confirmé. Vérifiez les fichiers Markdown et les services, puis rechargez avant de réessayer.");}finally{acting.current=false;if(mounted.current)setPending(false);}}}>
 <label className="profile-checkbox"><input type="checkbox" checked={draft.orchestratorEnabled} disabled={pending} onChange={e=>update({orchestratorEnabled:e.target.checked})}/> Autoriser les lancements d’orchestration</label>
 <label>Exécutions simultanées<input type="number" min={1} max={10} required value={draft.orchestratorMaxConcurrent} disabled={pending} onChange={e=>update({orchestratorMaxConcurrent:Number(e.target.value)})}/></label>
 <p className="muted">Désactiver empêche les prochains lancements ; cela n’arrête pas les tâches déjà en cours.</p>
 {draft.agents.map((agent,index)=>{const saved=view?.agents.find(a=>a.id===agent.id);return <fieldset key={agent.id}><legend>{agent.name||"Nouvel agent"}</legend><label>Nom de l’agent<input required maxLength={80} value={agent.name} disabled={pending} onChange={e=>update({agents:draft.agents.map((a,i)=>i===index?{...a,name:e.target.value}:a)})}/></label><label>Fichier de consignes<input required value={agent.file} disabled={pending} placeholder="AGENTS.md" onChange={e=>update({agents:draft.agents.map((a,i)=>i===index?{...a,file:e.target.value}:a)})}/></label>
 {saved?.guidelinesError&&<p>{saved.guidelinesError}</p>}
 <details><summary>Aperçu des consignes enregistrées</summary><pre>{saved?.file===agent.file?saved.guidelines||"Aucun aperçu disponible.":"Enregistrez pour vérifier ce fichier."}</pre></details>
 <button type="button" disabled={pending} onClick={()=>update({agents:draft.agents.filter(a=>a.id!==agent.id)})}>Retirer cet agent</button></fieldset>;})}
 <button type="button" disabled={pending||draft.agents.length>=20} onClick={()=>update({agents:[...draft.agents,{id:crypto.randomUUID(),name:`Agent ${draft.agents.length+1}`,file:"AGENTS.md"}]})}>Ajouter un agent</button>
 <button disabled={pending||!dirty||!valid}>Enregistrer l’orchestration</button>
 <button type="button" disabled={pending||dirty} onClick={()=>void refresh().catch(()=>setError("Impossible de recharger les consignes."))}>Recharger les fichiers</button>
 </form>}
 {!draft&&<button disabled={pending} onClick={()=>{setPending(true);void refresh().catch(()=>setError("Impossible de recharger l’orchestration.")).finally(()=>setPending(false));}}>Réessayer la configuration</button>}
 <button disabled={pending||dirty||!view} onClick={onSupervise}>Ouvrir la supervision</button>
 </details>;
}
