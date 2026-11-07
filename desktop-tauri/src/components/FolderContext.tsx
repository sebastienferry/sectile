import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { Run } from "../domain/execution";
import { attachFolder, detachFolder, projectFolders, type AttachedFolder } from "../lib/folders";
import {projectContext,projectRepositories,type ProjectContext,type ProjectRepository} from "../lib/workspace";
import { Icon } from "./Icon";

const folderName=(path:string)=>path.replace(/[\\/]+$/," ").trim().split(/[\\/]/).pop()||path;
const roleNames:Record<string,string>={primary:"Travail",changed:"Modifié",context:"Contexte",spec:"Spécifications",local:"Attaché au projet"};
function useFolders(projectId:string){
 const [folders,setFolders]=useState<AttachedFolder[]>([]);
 const [context,setContext]=useState<ProjectContext>();
 const [repositories,setRepositories]=useState<ProjectRepository[]>([]);
 const [unavailable,setUnavailable]=useState(false);
 const request=useRef(0);
 async function refresh(){const seq=++request.current;try{const items=await projectFolders(projectId);if(seq===request.current){setFolders(Array.isArray(items)?items:[]);setUnavailable(false);}
 const metadata=await Promise.allSettled([Promise.resolve().then(()=>projectContext(projectId)),Promise.resolve().then(()=>projectRepositories(projectId))]);
 if(seq===request.current){const [project,repos]=metadata;setContext(project.status==="fulfilled"?project.value:undefined);setRepositories(repos.status==="fulfilled"&&Array.isArray(repos.value)?repos.value:[]);}}catch{if(seq===request.current)setUnavailable(true);}}
 useEffect(()=>{setFolders([]);void refresh();return()=>{request.current++;};},[projectId]);
 return {folders,unavailable,refresh,context,repositories};
}
export function PromptFolders({projectId,runId,directory,folders:runFolders,readOnly=false,compact=false,onChanged,onPendingChange}:{
 projectId:string;runId?:string;directory?:string;folders?:Run["folders"];readOnly?:boolean;compact?:boolean;onChanged?:()=>void;onPendingChange?:(pending:boolean)=>void;
}){
 const {folders,unavailable,refresh,context,repositories}=useFolders(projectId);
 const [pending,setPending]=useState(false);
 const [notice,setNotice]=useState("");
 const [error,setError]=useState("");
 const [copiedPath,setCopiedPath]=useState("");
 const acting=useRef(false);
 const detailsRef=useRef<HTMLDetailsElement>(null);
 useEffect(()=>{
  function onPointerDown(event:MouseEvent){
   if(detailsRef.current?.open&&!detailsRef.current.contains(event.target as Node))detailsRef.current.removeAttribute("open");
  }
  function onKeyDown(event:KeyboardEvent){
   if(event.key==="Escape"&&detailsRef.current?.open)detailsRef.current.removeAttribute("open");
  }
  document.addEventListener("mousedown",onPointerDown);
  window.addEventListener("keydown",onKeyDown);
  return()=>{document.removeEventListener("mousedown",onPointerDown);window.removeEventListener("keydown",onKeyDown);};
 },[]);
 const configured=[...repositories.filter(r=>!r.code&&r.path).map(r=>({path:r.path,name:r.identity,role:"context",attached:false})),...([context?.specPath,context?.issueSpecPath].filter(Boolean) as string[]).map(path=>({path,name:"Spécifications",role:"spec",attached:false}))];
 const listed=[...(runFolders??[]).filter(f=>f.path!==directory),...configured,...folders.map(f=>({path:f.path,name:folderName(f.path),role:"local",attached:true}))].filter((f,index,all)=>f.path!==directory&&all.findIndex(other=>other.path===f.path)===index);
 const allEntries=[...(directory?[{path:directory,label:"Travail"}]:[]),...listed.map(f=>({path:f.path,label:f.attached?"Attaché au projet":roleNames[f.role]||f.role}))];
 async function copyFolderPath(path:string){
  try{
   await navigator.clipboard.writeText(path);
   setCopiedPath(path);
   setTimeout(()=>setCopiedPath(current=>current===path?"":current),1800);
  }catch{
   setError("Impossible de copier le chemin du dossier.");
  }
 }
 async function openInFinder(path:string){
  try{
   await invoke("open_folder_at",{path});
  }catch{
   setError("Impossible d’ouvrir le dossier dans le Finder.");
  }
 }
 async function add(){if(acting.current)return;acting.current=true;setPending(true);onPendingChange?.(true);setError("");setNotice("");try{const result=await attachFolder(projectId,runId);if(result){await refresh();onChanged?.();setNotice(result.mappedAs?`Dépôt associé au projet : ${result.mappedAs}. Disponible au prochain message.`:runId?"Dossier ajouté au projet · disponible dès le prochain message.":"Dossier ajouté au projet · disponible au lancement du prompt.");}}catch{setError("Impossible d’ajouter ce dossier. Il est peut-être déjà associé ou indisponible. Vérifiez les dossiers du projet puis réessayez.");}finally{acting.current=false;setPending(false);onPendingChange?.(false);}}
 return <section className={`prompt-folders${compact?" compact":""}`} aria-label="Contexte de travail">
 <div className="prompt-context-toolbar">{!compact&&<span className="working-directory" title={directory}><Icon name="folder"/><span>{directory||"Dossier de travail en cours de chargement…"}</span></span>}
 <details ref={detailsRef}>
  <summary title="Voir et gérer les dossiers de ce projet">
   <Icon name="folder"/> <span className={compact?"sr-only":undefined}>Dossiers · </span><span>{allEntries.length}</span>
  </summary>
  <div className="prompt-folders-popover">
   <ul>
    {allEntries.map(entry=>(
     <li key={entry.path} className="prompt-folder-row">
      <button
       type="button"
       className="prompt-folder-copy"
       title={`Cliquer pour copier : ${entry.path}`}
       aria-label={`Copier le chemin ${entry.path}`}
       onClick={()=>void copyFolderPath(entry.path)}
      >
       <strong>{entry.label}</strong>
       <code>{entry.path}</code>
       <span className="prompt-folder-copy-state" aria-hidden="true">
        {copiedPath===entry.path?<Icon name="check"/>:<Icon name="copy"/>}
       </span>
      </button>
      <button
       type="button"
       className="prompt-folder-finder"
       title="Ouvrir dans le Finder"
       aria-label={`Ouvrir ${entry.path} dans le Finder`}
       onClick={()=>void openInFinder(entry.path)}
      >
       <Icon name="external"/>
      </button>
     </li>
    ))}
   </ul>
   {!readOnly&&(
    <div className="prompt-folders-footer">
     <button
      type="button"
      className="prompt-folders-add"
      aria-label={pending?"Ajout…":"Ajouter un dossier"}
      title="Partager un dossier supplémentaire avec les conversations du projet"
      disabled={pending}
      onClick={()=>void add()}
     >
      <Icon name="plus"/>
      <span>{pending?"Ajout…":"Ajouter un dossier"}</span>
     </button>
    </div>
   )}
   <p>Les dossiers attachés sont partagés par toutes les conversations de ce projet. Leur retrait se fait dans sa configuration.</p>
   {unavailable&&<p>Dossiers du projet indisponibles. <button type="button" onClick={()=>void refresh()}>Réessayer les dossiers</button></p>}
  </div>
 </details>
 </div>
 {!compact&&listed.length>0&&<div className="prompt-context-folders" aria-label="Dossiers supplémentaires utilisés par l’agent">{listed.map(folder=><span key={folder.path} title={folder.path}><Icon name="folder"/><span>{folderName(folder.path)}</span></span>)}</div>}
 {notice&&<p role="status">{notice}</p>}{error&&<p role="alert" className="error">{error}</p>}
 </section>;
}

export function AttachedFolderSettings({projectId,onChanged}:{projectId:string;onChanged?:()=>void}){
 const {folders,unavailable,refresh}=useFolders(projectId);
 const [pending,setPending]=useState(false);
 const [error,setError]=useState("");
 const acting=useRef(false);
 async function change(action:()=>Promise<unknown>){if(acting.current)return;acting.current=true;setPending(true);setError("");try{await action();await refresh();onChanged?.();}catch{setError("Impossible de modifier les dossiers associés. Vérifiez leur disponibilité et réessayez.");}finally{acting.current=false;setPending(false);}}
 return <section className="attached-folder-settings" aria-label="Dossiers attachés au projet"><h3>Dossiers attachés</h3><p className="muted">Accessibles aux agents en complément du dossier de travail. Détacher retire l’association du projet, sans supprimer les fichiers.</p>
 {unavailable&&<p role="status">Dossiers indisponibles. <button onClick={()=>void refresh()}>Réessayer</button></p>}
 {!folders.length&&!unavailable&&<p>Aucun dossier supplémentaire.</p>}
 <ul>{folders.map(folder=><li key={folder.path}><div><strong>{folderName(folder.path)}</strong><code>{folder.path}</code><small>{folder.kind==="missing"?"Dossier introuvable":folder.kind==="git"?"Dépôt Git":"Dossier local"}{folder.duplicate&&` · Doublon de ${folder.duplicate}`}</small></div><button disabled={pending} aria-label={`Détacher ${folder.path}`} onClick={()=>void change(()=>detachFolder(projectId,folder.path))}>Détacher</button></li>)}</ul>
 <button disabled={pending} onClick={()=>void change(()=>attachFolder(projectId))}><Icon name="plus"/>Ajouter un dossier au projet</button>
 {error&&<p role="alert" className="error">{error}</p>}
 </section>;
}
