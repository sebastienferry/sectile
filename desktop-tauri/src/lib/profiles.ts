import { invoke } from "@tauri-apps/api/core";
export interface ConversationProfile { id: string; name: string; provider: string; model?: string; mode?: "plan" | "execute"; canDelete: boolean }
export interface ProjectProfiles { profiles: ConversationProfile[]; defaultId: string; inherited: boolean }
export type ProfileAction = {action:"select"|"delete";id:string} | {action:"save";id:string;name:string;provider:string;model:string;mode:string;makeDefault:boolean};
export const conversationProfiles = (projectId:string) => invoke<ProjectProfiles>("conversation_profiles",{projectId});
export const controlProfiles = (projectId:string,input:ProfileAction) => invoke<ProjectProfiles>("control_conversation_profiles",{projectId,input});
