import { useEffect, useState, type ReactNode } from "react";
import { conversationProfiles, controlProfiles, type ConversationProfile, type ProjectProfiles as Profiles, type ProfileAction } from "../lib/profiles";
import { TurnSettings } from "./TurnSettings";

const emptyProfile = () => ({id:"",name:"",provider:"claude",model:undefined as string | undefined,mode:"",makeDefault:true});
export function ProjectProfiles({projectId, children, after}:{projectId:string; children?:ReactNode;after?:ReactNode}) {
 const [data,setData]=useState<Profiles>();
 const [draft,setDraft]=useState<ReturnType<typeof emptyProfile>>();
 const [busy,setBusy]=useState(false);
 const [error,setError]=useState("");
 useEffect(()=>{let current=true;setData(undefined);setDraft(undefined);setError("");void conversationProfiles(projectId).then(value=>{if(current)setData(value);}).catch(cause=>{if(current)setError(String(cause));});return()=>{current=false;};},[projectId]);
 async function apply(input:ProfileAction){setBusy(true);setError("");try{setData(await controlProfiles(projectId,input));setDraft(undefined);}catch(cause){setError(String(cause));}finally{setBusy(false);}}
 function edit(profile:ConversationProfile){setDraft({id:profile.id,name:profile.name,provider:profile.provider,model:profile.model||undefined,mode:profile.mode??"",makeDefault:profile.id===data?.defaultId});}
 return <section className="project-profiles"><span className="eyebrow">CONFIGURATION DU PROJET</span>{children}<h2>Conversations</h2>
 <p className="muted">Choisissez le profil des nouveaux prompts. Les conversations ouvertes gardent leurs réglages.</p>
 {error&&<p role="alert" className="error">{error}</p>}
 {!data&&!error&&<p>Chargement des profils…</p>}
 {data&&<>
 <label className="profile-default">Profil par défaut<select aria-label="Profil par défaut" value={data.inherited?"":data.defaultId} disabled={busy} onChange={e=>void apply({action:"select",id:e.target.value})}><option value="">Hériter du poste</option>{data.profiles.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
 <details className="profile-management"><summary>Gérer les profils<small>Créer, modifier ou supprimer un profil de conversation</small></summary><p className="muted">Les profils sont partagés sur ce poste ; chaque projet choisit son défaut. Modifier un profil met à jour les futurs prompts des projets qui l’utilisent.</p>
 <div className="profile-list">{data.profiles.map(p=><article key={p.id} className="profile-card"><div><strong>{p.name}</strong><p>{p.provider==="claude"?"Claude Code":"Codex"} · {p.model||"Modèle du moteur"} · {p.mode==="plan"?"Plan":p.mode==="execute"?"Exécution":"Mode du poste"}{p.id===data.defaultId&&" · Par défaut"}</p></div><button disabled={busy} onClick={()=>edit(p)}>Modifier {p.name}</button>{p.canDelete&&<button disabled={busy} onClick={()=>void apply({action:"delete",id:p.id})}>Supprimer {p.name}</button>}</article>)}</div>
 {!draft&&<button className="primary" disabled={busy} onClick={()=>setDraft(emptyProfile())}>Créer un profil</button>}
 {draft&&<form className="profile-editor" onSubmit={e=>{e.preventDefault();void apply({action:"save",...draft,model:draft.model??""});}}><h3>{draft.id?"Modifier le profil":"Nouveau profil"}</h3>
 <label>Nom du profil<input required maxLength={64} value={draft.name} onChange={e=>setDraft({...draft,name:e.target.value})}/></label>
 <label>Moteur<select aria-label="Moteur du profil" value={draft.provider} onChange={e=>setDraft({...draft,provider:e.target.value,model:undefined})}><option value="claude">Claude Code</option><option value="codex">Codex</option></select></label>
 <TurnSettings provider={draft.provider} value={{model:draft.model,mode:draft.mode?draft.mode as "plan"|"execute":undefined}} onChange={options=>setDraft({...draft,model:options.model,mode:options.mode??""})} disabled={busy}/>
 <label className="profile-checkbox"><input type="checkbox" checked={draft.makeDefault} onChange={e=>setDraft({...draft,makeDefault:e.target.checked})}/> Utiliser par défaut dans ce projet</label>
 <div className="toolbar-actions"><button type="button" disabled={busy} onClick={()=>setDraft(undefined)}>Annuler</button><button className="primary" disabled={busy||!draft.name.trim()}>Enregistrer le profil</button></div>
 </form>}</details></>}
 {after}
 </section>;
}
