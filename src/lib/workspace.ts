import {invoke} from "@tauri-apps/api/core";
export interface BoardMapping {trackerColumns?:{name:string;statuses?:string[]}[];stageColumns?:Record<string,string[]>;trackers?:({trackerId:string}&BoardMapping)[]}
export interface ProjectContext {configured:boolean;path?:string;specPath?:string;specDefault?:string;specKind?:string;issueSpecPath?:string;issueSpecDefault?:string;issueSpecKind?:string;aiProvider?:string;aiModel?:string;skills:{id:string;command?:string}[];board?:BoardMapping;policy:{prCreationStage?:string;fullChainStopStage?:string;defaultSkillMode?:string;specFramework?:string}}
export interface Readiness {provider:string;steps:Record<string,{status:string;scope?:string;configuration?:{present:boolean;valid:boolean;needsRepair?:boolean};message?:string}>;permissions?:Record<string,unknown>;complete:boolean}
export interface ProjectRepository {identity:string;url:string;path:string;code:boolean}
export const projectContext=(projectId:string)=>invoke<ProjectContext>("project_context",{projectId});
export const projectRepositories=(projectId:string)=>invoke<ProjectRepository[]>("project_repositories",{projectId});
export const providerReadiness=(provider:string)=>invoke<Readiness>("provider_readiness",{provider});
export const installProviderSkills=(provider:string,projectId:string)=>invoke<{success:boolean;skills?:{status:string;message?:string}}>("install_provider_skills",{provider,projectId});
export const configureProviderMcp=(provider:string,replaceExisting:boolean)=>invoke("configure_provider_mcp",{provider,replaceExisting});

export const runResult=(runId:string)=>invoke<import("../domain/workflow").RunResult>("run_result",{runId});

export interface OrchestrationSettings {orchestratorEnabled:boolean;orchestratorMaxConcurrent:number;agents:{id:string;name:string;file:string}[]}
export const saveOrchestration=(projectId:string,input:OrchestrationSettings)=>invoke("save_orchestration",{projectId,input});
export const saveWorkflowPolicy=(projectId:string,input:{fullChainStopStage:string;prCreationStage:string;defaultSkillMode:string})=>invoke("save_workflow_policy",{projectId,input});

export interface RunDiff {directory:string;branch:string;isClean:boolean;complete:boolean;filesChanged:number;additions:number;deletions:number;warnings?:{code:string;message:string}[];files:{path:string;oldPath?:string;status:string;kind:string;patch:string;omittedReason?:string}[]}
export const runDiff=(runId:string,folder?:string)=>invoke<RunDiff>("run_diff",{runId,folder});
export const openRunEditor=(runId:string,folder?:string)=>invoke("open_run_editor",{runId,folder});
export const openExternalLink=(url:string)=>invoke("open_external_link",{url});
export const editorPreference=(editor?:string)=>invoke<{configured:boolean;selection:string}>("editor_preference",{editor});
export const declareReviewed=(projectId:string,taskId:string,note:string)=>invoke("declare_reviewed",{projectId,taskId,note});
export const chooseRepositoryFolder=(projectId:string,repository:string)=>invoke<string|null>("choose_repository_folder",{projectId,repository});
export const chooseSpecificationFolder=(projectId:string,kind:"macro"|"issue",usePrimary=false)=>invoke<string|null>("choose_specification_folder",{projectId,kind,usePrimary});
export type ProviderPermissions = {provider:"claude";mode:"default"|"acceptEdits"|"plan"|"auto"}|{provider:"codex";approvalsReviewer:"user"|"auto_review"};
export const saveProviderPermissions=(input:ProviderPermissions)=>invoke("save_provider_permissions",{input});
