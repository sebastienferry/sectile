import {STAGES,nextTaskStep,taskStage} from './workflow.mjs'
import {expectedStage,skillResult,workflowSkill} from './skill-result.mjs'

// The engines whose task conversations end their skill through Sectile, so a
// completed skill can be told apart from a turn that merely stopped (#875).
const providers=new Set(['claude','codex','agy'])
// Handoff ends at finished, which has no next step; pickup, create_pr and the
// other skills have no stage to validate their completion against.
const offering=new Set(['clarify','specify','implement','adjust'])
const activeRun=run=>['running','queued','preparing'].includes(run?.status)
const taskKey=run=>JSON.stringify([run.projectId,run.taskId])

// Whether the selected conversation offers the next workflow step, and which:
// {skillId,label} when every condition holds, else {reason} naming the first
// that does not. The reasons are for tests and diagnostics, never shown.
//
// run: the selected run; result: its skill-result read; conversation: the last
// polled conversation state {busy,readOnly,approvals,sending}; task, project:
// the next-step read of the task; runs: the polled run list; submitting: a
// launch on the task is being submitted or waits for its console.
export function nextStepOffer({run,result,conversation,task,project,runs=[],submitting=false}={}){
 if(!run?.conversation||!run.taskId||run.kind==='console'||run.macroKey||run.skill==='discuss')return {reason:'not-a-task-conversation'}
 if(!providers.has(run.provider||'claude'))return {reason:'provider'}
 if(!conversation||conversation.id!==run.id)return {reason:'conversation-unknown'}
 if(conversation.readOnly)return {reason:'read-only'}
 if(conversation.busy||conversation.sending)return {reason:'busy'}
 if(conversation.approvals?.length)return {reason:'approval'}
 if(run.waitingSince||result?.successor?.waitingSince)return {reason:'waiting'}
 const successor=result?.activity?.id===run.id&&result.successor
 const skill=successor?workflowSkill(successor.skillId):run.skill
 if(!offering.has(skill))return {reason:'skill'}
 if(skillResult(run,result)?.kind!=='completed')return {reason:'skill-not-completed'}
 if(!task||!project)return {reason:'task-unknown'}
 // The task read for the toolbar may predate the stage move the skill result
 // already saw: the offer waits for it rather than naming a stale step.
 if(STAGES.indexOf(taskStage(task))<STAGES.indexOf(expectedStage(skill)))return {reason:'task-behind'}
 if(taskStage(task)==='finished')return {reason:'finished'}
 const step=nextTaskStep(task,project)
 if(!step.skillId)return {reason:'no-next-step'}
 if(submitting)return {reason:'submitting'}
 const key=taskKey(run)
 if(runs.some(item=>taskKey(item)===key&&blocksLaunch(item,run.id)))return {reason:'active-run'}
 return {skillId:step.skillId,label:step.label}
}

// Whether `item` counts as active on the task for a launch made from the offer:
// the idle predecessor the offer speaks for is the one run that does not.
export function blocksLaunch(item,predecessorId){
 return activeRun(item)&&item.id!==predecessorId
}
