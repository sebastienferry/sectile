import type {Task} from "./board.ts";
import type {BoardMapping,ProjectContext} from "../lib/workspace.ts";
export const stages=["new","clarified","specified","implemented","reviewed","finished"] as const;
export type Stage=typeof stages[number];
export const stageLabels:Record<Stage,string>={new:"To clarify",clarified:"Clarified",specified:"Specified",implemented:"Implemented",reviewed:"Reviewed",finished:"Finished"};
export const skillLabels:Record<string,string>={clarify:"Clarify",specify:"Specify",implement:"Implement",adjust:"Adjust / review",handoff:"Handoff",pickup:"Run chain",discuss:"Discuss"};
export function taskStage(task:Task,board?:BoardMapping):Stage|undefined{
 if(["finished","done"].includes(task.status))return "finished";
 const labels=new Set((task.labels??[]).map(label=>label.trim().replace(/^#+/,"").toLowerCase()));
 for(const stage of [...stages].reverse()){if(labels.has(stage)||(stage==="finished"&&(labels.has("closed")||labels.has("done")))||(stage==="new"&&labels.has("untouched")))return stage;}
 const mapping=board?.trackers?.find(item=>item.trackerId===task.trackerId)??board;
 const remote=(task.trackerStatus??"").toLowerCase();
 const column=mapping?.trackerColumns?.find(col=>col.name.toLowerCase()===remote||col.statuses?.some(value=>value.toLowerCase()===remote));
 if(column){const stage=stages.find(stage=>mapping?.stageColumns?.[stage]?.includes(column.name));if(stage)return stage;}
 return ({new:"new",to_clarify:"new",backlog:"new",untouched:"new",to_specify:"clarified",clarified:"clarified",to_implement:"specified",specified:"specified",in_progress:"specified",to_test:"implemented",to_validate:"implemented",implemented:"implemented",to_close:"reviewed",reviewed:"reviewed"} as Record<string,Stage>)[task.status];
}
export function nextSkill(task:Task,project:ProjectContext):{stage?:Stage;skillId?:string;label:string;reason?:string}{
 const stage=taskStage(task,project.board);
 if(stage==="finished")return {stage,label:"Finished",reason:"This ticket is finished."};
 if(!stage)return {label:"Unknown stage",reason:"No reliable stage was provided. Refresh the ticket or explicitly select a skill."};
 let skillId=({new:"clarify",clarified:"specify",specified:"implement",implemented:"adjust",reviewed:"handoff"} as Partial<Record<Stage,string>>)[stage];
 let label=skillLabels[skillId??""]??"";
 if(stage==="implemented"&&!task.prUrl?.trim()){skillId=project.policy?.prCreationStage==="specified"?"specify":"implement";label="Create / find PR";}
 if(!project.configured)return {stage,label,reason:"Configure the project's local repository to launch a skill."};
 if(!project.skills.some(s=>s.id===skillId))return {stage,label,reason:"The expected skill is not available in this project."};
 return {stage,skillId,label};
}
export function chainProgress(task:Task,project:Pick<ProjectContext,"board"|"policy">){
 const target=project.policy?.fullChainStopStage==="reviewed"?"reviewed":"implemented";
 const stage=taskStage(task,project.board);
 return {target,reached:!!stage&&stages.indexOf(stage)>=stages.indexOf(target)} as const;
}
/** What the server recorded for a skill execution, as the local agent sends it. */
export interface SkillRecord {id:string;taskId:string;skillId:string;status:string;summary?:string;error?:string;disconnected?:boolean;waitingReason?:string;completedAt?:string}
/** The skill result of a run (`GET /desktop/run-result`): its own record, the skill its console went on to run, and the task. */
export interface RunResult {activity?:SkillRecord|null;successor?:(SkillRecord&{waitingSince?:string})|null;task?:{status:string;labels?:string[]}}

const ENGINE_MARKS: Record<string, string> = {
  claude: "Cl",
  codex: "Cx",
  agy: "Ag",
  antigravity: "Ag",
  custom: "{}",
};

export function engineMark(provider?: string): string {
  const id = String(provider || "").trim().toLowerCase();
  return ENGINE_MARKS[id] || (id ? id.slice(0, 2).replace(/^./, c => c.toUpperCase()) : "—");
}
