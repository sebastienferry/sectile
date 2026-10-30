import { invoke } from "@tauri-apps/api/core";
export interface AttachedFolder {path:string;kind:string;identity?:string;remote?:string;duplicate?:string}
export interface FolderAttachment {path:string;mappedAs?:string;appliesAt:"now"|"next-turn"|"next-launch"}
export const projectFolders=(projectId:string)=>invoke<AttachedFolder[]>("project_folders",{projectId});
export const attachFolder=(projectId:string,runId?:string)=>invoke<FolderAttachment|null>("choose_attached_folder",{projectId,...(runId?{runId}:{})});
export const detachFolder=(projectId:string,path:string)=>invoke("detach_project_folder",{projectId,path});
