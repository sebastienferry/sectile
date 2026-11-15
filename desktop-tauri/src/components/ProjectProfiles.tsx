import { useEffect, useState, type ReactNode } from "react";
import { conversationProfiles, controlProfiles, type ConversationProfile, type ProjectProfiles as Profiles, type ProfileAction } from "../lib/profiles";
import type { ConversationMode } from "../lib/conversation";
import { TurnSettings } from "./TurnSettings";

const emptyProfile = () => ({id:"",name:"",provider:"claude",model:undefined as string | undefined,mode:"",makeDefault:true});
function profileModeLabel(mode?: string) {
  switch (mode) {
    case "plan":
      return "Plan";
    case "read-only":
      return "Read-only";
    case "acceptEdits":
      return "Accept edits";
    case "auto":
      return "Auto (YOLO)";
    case "execute":
    case "default":
    case "workspace-write":
      return "Execute";
    default:
      return "Workstation mode";
  }
}
export interface ProjectSettingsTab {
  id: string;
  label: string;
  content?: ReactNode;
}
export function WorkstationProfiles({projectId = ""}:{projectId?: string}) {
 const [data,setData]=useState<Profiles>();
 const [draft,setDraft]=useState<ReturnType<typeof emptyProfile>>();
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 useEffect(()=>{let current=true;setData(undefined);setDraft(undefined);setError("");void conversationProfiles("").catch(()=>projectId?conversationProfiles(projectId):Promise.reject<Profiles>(new Error("No profile available."))).then(value=>{if(current)setData(value);}).catch(cause=>{if(current)setError(String(cause));});return()=>{current=false;};},[projectId]);
 async function apply(input:ProfileAction){setBusy(true);setError("");try{setData(await controlProfiles("",input).catch(()=>projectId?controlProfiles(projectId,input):Promise.reject<Profiles>(new Error("Action unavailable."))));setDraft(undefined);}catch(cause){setError(String(cause));}finally{setBusy(false);}}
 function edit(profile:ConversationProfile){setDraft({id:profile.id,name:profile.name,provider:profile.provider,model:profile.model||undefined,mode:profile.mode??"",makeDefault:profile.id===data?.defaultId});}
 return <section className="workstation-profiles" aria-label="Conversation profiles">
  <h3>Conversation profiles</h3>
  <p className="muted">Profiles are shared on this workstation; each project can choose its own default profile or inherit the workstation default.</p>
  {error&&<p role="alert" className="error">{error}</p>}
  {!data&&!error&&<p>Loading profiles…</p>}
  {data&&<>
   <div className="profile-list">{data.profiles.map(p=><article key={p.id} className="profile-card"><div><strong>{p.name}</strong><p>{p.provider==="claude"?"Claude Code":"Codex"} · {p.model||"Engine model"} · {profileModeLabel(p.mode)}</p></div><button disabled={busy} onClick={()=>edit(p)}>Edit {p.name}</button><button disabled={busy} onClick={()=>void apply({action:"delete",id:p.id})}>Delete {p.name}</button></article>)}</div>
   {!draft&&<button className="primary" disabled={busy} onClick={()=>setDraft({...emptyProfile(),makeDefault:false})}>Create profile</button>}
   {draft&&<form className="profile-editor" onSubmit={e=>{e.preventDefault();void apply({action:"save",...draft,model:draft.model??""});}}><h3>{draft.id?"Edit profile":"New profile"}</h3>
    <label>Profile name<input required maxLength={64} value={draft.name} onChange={e=>setDraft({...draft,name:e.target.value})}/></label>
    <label>Engine<select aria-label="Profile engine" value={draft.provider} onChange={e=>setDraft({...draft,provider:e.target.value,model:undefined})}><option value="claude">Claude Code</option><option value="codex">Codex</option></select></label>
    <TurnSettings provider={draft.provider} value={{model:draft.model,mode:draft.mode?draft.mode as ConversationMode:undefined}} onChange={options=>setDraft({...draft,model:options.model,mode:options.mode??""})} disabled={busy}/>
    <div className="toolbar-actions"><button type="button" disabled={busy} onClick={()=>setDraft(undefined)}>Cancel</button><button className="primary" disabled={busy||!draft.name.trim()}>Save profile</button></div>
   </form>}
  </>}
 </section>;
}
export function ProjectProfiles({projectId, tabs, activeTab: controlledTab, onTabChange, onManageProfiles, children, after}:{projectId:string; tabs?:ProjectSettingsTab[]; activeTab?:string; onTabChange?:(tabId:string)=>void; onManageProfiles?:()=>void; children?:ReactNode; after?:ReactNode}) {
 const [data,setData]=useState<Profiles>();
 const [draft,setDraft]=useState<ReturnType<typeof emptyProfile>>();
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 const [internalTab,setInternalTab]=useState(()=>tabs?.[0]?.id??"folder");
 const activeTab=controlledTab??internalTab;
 const selectTab=(id:string)=>{setInternalTab(id);onTabChange?.(id);};
 useEffect(()=>{let current=true;setData(undefined);setDraft(undefined);setError("");void conversationProfiles(projectId).then(value=>{if(current)setData(value);}).catch(cause=>{if(current)setError(String(cause));});return()=>{current=false;};},[projectId]);
 async function apply(input:ProfileAction){setBusy(true);setError("");try{setData(await controlProfiles(projectId,input));setDraft(undefined);}catch(cause){setError(String(cause));}finally{setBusy(false);}}
 function edit(profile:ConversationProfile){setDraft({id:profile.id,name:profile.name,provider:profile.provider,model:profile.model||undefined,mode:profile.mode??"",makeDefault:profile.id===data?.defaultId});}
 const conversationsContent = <>
 <div className="project-tracker-heading">
  <h2>Conversations</h2>
  {onManageProfiles && <button type="button" className="project-tracker-manage" onClick={onManageProfiles}>Manage profiles</button>}
 </div>
 <p className="muted">Choose the profile for new prompts in this project. Open conversations keep their settings.</p>
 {error&&<p role="alert" className="error">{error}</p>}
 {!data&&!error&&<p>Loading profiles…</p>}
 {data&&<>
 <label className="profile-default">Default profile<select aria-label="Default profile" value={data.inherited?"":data.defaultId} disabled={busy} onChange={e=>void apply({action:"select",id:e.target.value})}><option value="">Inherit from workstation</option>{data.profiles.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
 {!onManageProfiles && <details className="profile-management"><summary>Manage profiles<small>Create, edit, or delete a conversation profile</small></summary><p className="muted">Profiles are shared on this workstation; each project chooses its default. Editing a profile updates future prompts in projects that use it.</p>
 <div className="profile-list">{data.profiles.map(p=><article key={p.id} className="profile-card"><div><strong>{p.name}</strong><p>{p.provider==="claude"?"Claude Code":"Codex"} · {p.model||"Engine model"} · {profileModeLabel(p.mode)}{p.id===data.defaultId&&" · Default"}</p></div><button disabled={busy} onClick={()=>edit(p)}>Edit {p.name}</button>{p.canDelete&&<button disabled={busy} onClick={()=>void apply({action:"delete",id:p.id})}>Delete {p.name}</button>}</article>)}</div>
 {!draft&&<button className="primary" disabled={busy} onClick={()=>setDraft(emptyProfile())}>Create profile</button>}
 {draft&&<form className="profile-editor" onSubmit={e=>{e.preventDefault();void apply({action:"save",...draft,model:draft.model??""});}}><h3>{draft.id?"Edit profile":"New profile"}</h3>
 <label>Profile name<input required maxLength={64} value={draft.name} onChange={e=>setDraft({...draft,name:e.target.value})}/></label>
 <label>Engine<select aria-label="Profile engine" value={draft.provider} onChange={e=>setDraft({...draft,provider:e.target.value,model:undefined})}><option value="claude">Claude Code</option><option value="codex">Codex</option></select></label>
 <TurnSettings provider={draft.provider} value={{model:draft.model,mode:draft.mode?draft.mode as ConversationMode:undefined}} onChange={options=>setDraft({...draft,model:options.model,mode:options.mode??""})} disabled={busy}/>
 <label className="profile-checkbox"><input type="checkbox" checked={draft.makeDefault} onChange={e=>setDraft({...draft,makeDefault:e.target.checked})}/> Use as default in this project</label>
 <div className="toolbar-actions"><button type="button" disabled={busy} onClick={()=>setDraft(undefined)}>Cancel</button><button className="primary" disabled={busy||!draft.name.trim()}>Save profile</button></div>
 </form>}</details>}</>}
 </>;
 if (tabs?.length) {
  return <section className="project-profiles">
   <span className="eyebrow">PROJECT CONFIGURATION</span>
   <div className="settings-subtabs" role="tablist" aria-label="Project configuration sections">
    {tabs.map(tab => (
     <button
      key={tab.id}
      id={`project-tab-${tab.id}`}
      type="button"
      role="tab"
      aria-selected={activeTab === tab.id}
      aria-controls={`project-tabpanel-${tab.id}`}
      className={`settings-subtab ${activeTab === tab.id ? "active" : ""}`}
      onClick={() => selectTab(tab.id)}
     >
      {tab.label}
     </button>
    ))}
   </div>
   {tabs.map(tab => (
    <div
     key={tab.id}
     id={`project-tabpanel-${tab.id}`}
     role="tabpanel"
     aria-labelledby={`project-tab-${tab.id}`}
     className={`settings-tabpanel ${activeTab === tab.id ? "active" : "is-hidden"}`}
    >
     {tab.id === "conversations" ? conversationsContent : tab.content}
    </div>
   ))}
  </section>;
 }
 return <section className="project-profiles"><span className="eyebrow">PROJECT CONFIGURATION</span>{children}{conversationsContent}{after}</section>;
}
