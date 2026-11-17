import { useEffect, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import type { Run } from "../domain/execution";
import { attachFolder, detachFolder, projectFolders, type AttachedFolder } from "../lib/folders";
import {projectContext,projectRepositories,type ProjectContext,type ProjectRepository} from "../lib/workspace";
import { Icon } from "./Icon";

const folderName=(path:string)=>path.replace(/[\\/]+$/," ").trim().split(/[\\/]/).pop()||path;
const roleNames:Record<string,string>={primary:"Working",changed:"Modified",context:"Context",spec:"Specifications",local:"Attached to project"};
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
/** A folder the agent works with, as listed in the folder popover and the composer's context tray (#914). */
export interface FolderEntry { path:string; label:string; name:string }
export function PromptFolders({projectId,runId,directory,folders:runFolders,readOnly=false,compact=false,onChanged,onPendingChange,onEntriesChange}:{
 projectId:string;runId?:string;directory?:string;folders?:Run["folders"];readOnly?:boolean;compact?:boolean;onChanged?:()=>void;onPendingChange?:(pending:boolean)=>void;
 /** Receives the listed folders whenever they change, so the composer shows them without reading them twice. */
 onEntriesChange?:(entries:FolderEntry[])=>void;
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
 const configured=[...repositories.filter(r=>!r.code&&r.path).map(r=>({path:r.path,name:r.identity,role:"context",attached:false})),...([context?.specPath,context?.issueSpecPath].filter(Boolean) as string[]).map(path=>({path,name:"Specifications",role:"spec",attached:false}))];
 const listed=[...(runFolders??[]).filter(f=>f.path!==directory),...configured,...folders.map(f=>({path:f.path,name:folderName(f.path),role:"local",attached:true}))].filter((f,index,all)=>f.path!==directory&&all.findIndex(other=>other.path===f.path)===index);
 const allEntries:FolderEntry[]=[...(directory?[{path:directory,label:"Working",name:folderName(directory)}]:[]),...listed.map(f=>({path:f.path,label:f.attached?"Attached to project":roleNames[f.role]||f.role,name:folderName(f.path)}))];
 const entriesKey=JSON.stringify(allEntries);
 const reportEntries=useRef(onEntriesChange);
 reportEntries.current=onEntriesChange;
 useEffect(()=>{reportEntries.current?.(JSON.parse(entriesKey) as FolderEntry[]);},[entriesKey]);
 async function copyFolderPath(path:string){
  try{
   await navigator.clipboard.writeText(path);
   setCopiedPath(path);
   setTimeout(()=>setCopiedPath(current=>current===path?"":current),1800);
  }catch{
   setError("Unable to copy folder path.");
  }
 }
 async function openInFinder(path:string){
  try{
   await invoke("open_folder_at",{path});
  }catch{
   setError("Unable to open folder in Finder.");
  }
 }
 async function add(){if(acting.current)return;acting.current=true;setPending(true);onPendingChange?.(true);setError("");setNotice("");try{const result=await attachFolder(projectId,runId);if(result){await refresh();onChanged?.();setNotice(result.mappedAs?`Repository mapped to project: ${result.mappedAs}. Available on next message.`:runId?"Folder added to project · available on next message.":"Folder added to project · available when prompt launches.");}}catch{setError("Unable to add this folder. It may already be mapped or unavailable. Check project folders and try again.");}finally{acting.current=false;setPending(false);onPendingChange?.(false);}}
 return <section className={`prompt-folders${compact?" compact":""}`} aria-label="Working context">
 <div className="prompt-context-toolbar">{!compact&&<span className="working-directory" title={directory}><Icon name="folder"/><span>{directory||"Loading working folder…"}</span></span>}
 <details ref={detailsRef}>
  <summary title="View and manage folders for this project">
   <Icon name="folder"/> <span className={compact?"sr-only":undefined}>Folders · </span><span>{allEntries.length}</span>
  </summary>
  <div className="prompt-folders-popover">
   <ul>
    {allEntries.map(entry=>(
     <li key={entry.path} className="prompt-folder-row">
      <button
       type="button"
       className="prompt-folder-copy"
       title={`Click to copy: ${entry.path}`}
       aria-label={`Copy path ${entry.path}`}
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
       title="Open in Finder"
       aria-label={`Open ${entry.path} in Finder`}
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
      aria-label={pending?"Adding…":"Add folder"}
      title="Share an additional folder with project conversations"
      disabled={pending}
      onClick={()=>void add()}
     >
      <Icon name="plus"/>
      <span>{pending?"Adding…":"Add folder"}</span>
     </button>
    </div>
   )}
   <p>Attached folders are shared across all conversations in this project. Remove them in project settings.</p>
   {unavailable&&<p>Project folders unavailable. <button type="button" onClick={()=>void refresh()}>Retry folders</button></p>}
   {compact&&error&&<p role="alert" className="error">{error}</p>}
  </div>
 </details>
 </div>
 {!compact&&listed.length>0&&<div className="prompt-context-folders" aria-label="Additional folders used by the agent">{listed.map(folder=><span key={folder.path} title={folder.path}><Icon name="folder"/><span>{folderName(folder.path)}</span></span>)}</div>}
 {!compact&&notice&&<p role="status">{notice}</p>}{!compact&&error&&<p role="alert" className="error">{error}</p>}
 </section>;
}

export function AttachedFolderSettings({projectId,onChanged}:{projectId:string;onChanged?:()=>void}){
 const {folders,unavailable,refresh}=useFolders(projectId);
 const [pending,setPending]=useState(false);
 const [error,setError]=useState("");
 const acting=useRef(false);
 async function change(action:()=>Promise<unknown>){if(acting.current)return;acting.current=true;setPending(true);setError("");try{await action();await refresh();onChanged?.();}catch{setError("Unable to update attached folders. Check their availability and try again.");}finally{acting.current=false;setPending(false);}}
 return <section className="attached-folder-settings" aria-label="Folders attached to project"><h3>Attached folders</h3><p className="muted">Accessible to agents alongside the working folder. Detaching removes the project association without deleting files.</p>
 {unavailable&&<p role="status">Folders unavailable. <button onClick={()=>void refresh()}>Retry</button></p>}
 {!folders.length&&!unavailable&&<p>No additional folders.</p>}
 <ul>{folders.map(folder=><li key={folder.path}><div><strong>{folderName(folder.path)}</strong><code>{folder.path}</code><small>{folder.kind==="missing"?"Folder not found":folder.kind==="git"?"Git repository":"Local folder"}{folder.duplicate&&` · Duplicate of ${folder.duplicate}`}</small></div><button disabled={pending} aria-label={`Detach ${folder.path}`} onClick={()=>void change(()=>detachFolder(projectId,folder.path))}>Detach</button></li>)}</ul>
 <button disabled={pending} onClick={()=>void change(()=>attachFolder(projectId))}><Icon name="plus"/>Add folder to project</button>
 {error&&<p role="alert" className="error">{error}</p>}
 </section>;
}
